'use strict'

const crypto = require('crypto')
const fs = require('fs')
const path = require('path')
const { execFileSync } = require('child_process')
const { canonical } = require('../fair-launch/signed-authority.js')

const fail = (message) => { throw new Error(message) }
const exactKeys = (value, keys, label) => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail(`${label} must be an object`)
  const actual = Object.keys(value).sort()
  const expected = [...keys].sort()
  if (JSON.stringify(actual) !== JSON.stringify(expected)) fail(`${label} keys are not exact`)
}
const address = (value, label) => {
  if (!/^0x[0-9a-fA-F]{40}$/.test(String(value || ''))) fail(`${label} is not an address`)
  return String(value).toLowerCase()
}
const hash = (value, label, bytes = 32) => {
  if (!new RegExp(`^0x[0-9a-fA-F]{${bytes * 2}}$`).test(String(value || ''))) fail(`${label} is not a ${bytes}-byte hash`)
  return String(value).toLowerCase()
}
const sha = (value, label) => {
  if (!/^[0-9a-f]{64}$/.test(String(value || ''))) fail(`${label} is not lowercase SHA-256`)
  return String(value)
}
const commit = (value, label) => {
  if (!/^[0-9a-f]{40}$/.test(String(value || ''))) fail(`${label} is not a commit`)
  return String(value)
}
const bool = (value, label) => {
  if (typeof value !== 'boolean') fail(`${label} is not boolean`)
  return value
}
const integer = (value, label, min = 0) => {
  if (!Number.isSafeInteger(value) || value < min) fail(`${label} is not a safe integer`)
  return value
}
const fileSha = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const strictJsonParse = (raw, label) => {
  const program = 'import json,sys\n' +
    'def pairs(items):\n d={}\n for k,v in items:\n  if k in d: raise ValueError("duplicate JSON key: "+k)\n  d[k]=v\n return d\n' +
    'try:\n v=json.loads(sys.stdin.read(),object_pairs_hook=pairs)\n print(json.dumps(v,separators=(",",":"),ensure_ascii=False))\n' +
    'except Exception as e:\n print(str(e),file=sys.stderr)\n sys.exit(1)\n'
  let last
  for (const binary of ['python3', 'python']) {
    try {
      const normalized = execFileSync(binary, ['-c', program], { input: raw, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
      return JSON.parse(normalized)
    } catch (error) {
      last = error
      const detail = String(error.stderr || error.message).trim()
      if (error.code === 'ENOENT' || /Python was not found/.test(detail)) continue
      fail(`${label} is not strict JSON: ${detail}`)
    }
  }
  fail(`${label} strict JSON parser unavailable: ${last ? last.message : 'python unavailable'}`)
}
const envExact = (name, expected, normalize = String) => {
  if (normalize(process.env[name] || '') !== normalize(expected)) fail(`${name} does not match admission`)
}

function validate(admission, file, root, mode, trustedRepo) {
  exactKeys(admission, ['schema', 'admissionId', 'revision', 'status', 'environment', 'profile', 'issuedAt', 'permissions', 'identities', 'chain', 'bundle', 'components', 'authorities', 'paymentTokens', 'migrations', 'releaseScope'], 'admission')
  if (admission.schema !== 'joincare.fair-launch.test-server-release-admission.v1') fail('admission schema mismatch')
  if (admission.status !== 'ACTIVE_CANDIDATE_BUILD_ONLY') fail('admission status mismatch')
  if (admission.environment !== 'test-server' || admission.profile !== 'test-server-fair-launch-bsc-mainnet') fail('admission environment/profile mismatch')
  integer(admission.revision, 'revision', 1)
  if (!Number.isFinite(Date.parse(admission.issuedAt))) fail('issuedAt invalid')

  exactKeys(admission.permissions, ['packageBuildPermitted', 'operationalApplyPermitted', 'artifactUploadPermitted', 'databaseMigrationPermitted', 'serviceMutationPermitted', 'scannerStartPermitted', 'materializerStartPermitted', 'walletSubmissionPermitted', 'adminDispatchPermitted'], 'permissions')
  if (!bool(admission.permissions.packageBuildPermitted, 'packageBuildPermitted')) fail('package build is not admitted')
  for (const key of Object.keys(admission.permissions).filter((key) => key !== 'packageBuildPermitted')) {
    if (bool(admission.permissions[key], key)) fail(`${key} must remain false in candidate admission`)
  }

  exactKeys(admission.identities, ['deployBaseline', 'contract', 'backend', 'frontend', 'admin'], 'identities')
  for (const key of Object.keys(admission.identities)) {
    exactKeys(admission.identities[key], ['commit'], `identities.${key}`)
    commit(admission.identities[key].commit, `identities.${key}.commit`)
  }

  exactKeys(admission.chain, ['chainId', 'sourceRevision', 'proxyAddress', 'implementationAddress', 'startBlock', 'deploymentTransactionHash', 'deploymentBlockHash', 'confirmationPolicy', 'requiredSuccessorBlocks', 'proxyRuntimeCodeHash', 'implementationRuntimeCodeHash', 'contractVersionHash'], 'chain')
  if (integer(admission.chain.chainId, 'chainId', 1) !== 56) fail('chainId must be 56')
  integer(admission.chain.startBlock, 'startBlock', 1)
  if (commit(admission.chain.sourceRevision, 'sourceRevision') !== admission.identities.contract.commit) fail('sourceRevision must equal contract commit')
  if (admission.chain.confirmationPolicy !== '20_blocks' || integer(admission.chain.requiredSuccessorBlocks, 'requiredSuccessorBlocks', 1) !== 20) fail('confirmation policy mismatch')
  address(admission.chain.proxyAddress, 'proxyAddress')
  address(admission.chain.implementationAddress, 'implementationAddress')
  hash(admission.chain.deploymentTransactionHash, 'deploymentTransactionHash')
  hash(admission.chain.deploymentBlockHash, 'deploymentBlockHash')
  hash(admission.chain.proxyRuntimeCodeHash, 'proxyRuntimeCodeHash')
  hash(admission.chain.implementationRuntimeCodeHash, 'implementationRuntimeCodeHash')
  hash(admission.chain.contractVersionHash, 'contractVersionHash')

  exactKeys(admission.bundle, ['sha256', 'bundleDigest', 'fullSetDigest', 'initializerHash', 'aggregateAbiDigest', 'selectorMapDigest', 'storageLayoutDigest'], 'bundle')
  sha(admission.bundle.sha256, 'bundle.sha256')
  for (const key of Object.keys(admission.bundle).filter((key) => key !== 'sha256')) hash(admission.bundle[key], `bundle.${key}`)

  const componentKeys = ['logic', 'runtime', 'core', 'release', 'funds', 'admin', 'views', 'dispatcher', 'proxy']
  if (!Array.isArray(admission.components) || admission.components.length !== componentKeys.length) fail('components length mismatch')
  const seenComponents = new Set()
  for (const item of admission.components) {
    exactKeys(item, ['key', 'address', 'runtimeCodeHash'], 'component')
    if (!componentKeys.includes(item.key) || seenComponents.has(item.key)) fail('component key invalid or duplicated')
    seenComponents.add(item.key)
    address(item.address, `component ${item.key} address`)
    hash(item.runtimeCodeHash, `component ${item.key} runtimeCodeHash`)
  }
  const component = Object.fromEntries(admission.components.map((item) => [item.key, item]))
  if (address(component.proxy.address, 'proxy component') !== address(admission.chain.proxyAddress, 'proxy chain') || hash(component.proxy.runtimeCodeHash, 'proxy component hash') !== hash(admission.chain.proxyRuntimeCodeHash, 'proxy chain hash')) fail('proxy component cross-binding mismatch')
  if (address(component.dispatcher.address, 'dispatcher component') !== address(admission.chain.implementationAddress, 'implementation chain') || hash(component.dispatcher.runtimeCodeHash, 'dispatcher component hash') !== hash(admission.chain.implementationRuntimeCodeHash, 'implementation chain hash')) fail('implementation component cross-binding mismatch')

  exactKeys(admission.authorities, ['governanceSafe', 'governanceTimelock', 'emergencySafe', 'unpauseTimelock', 'treasurySafe', 'platformFeeSafe', 'projectProceedsSafe', 'operator', 'platformFeeBps'], 'authorities')
  for (const key of Object.keys(admission.authorities).filter((key) => key !== 'platformFeeBps')) address(admission.authorities[key], `authorities.${key}`)
  if (integer(admission.authorities.platformFeeBps, 'platformFeeBps') !== 250) fail('platformFeeBps must be 250')

  if (!Array.isArray(admission.paymentTokens) || admission.paymentTokens.length !== 2) fail('paymentTokens must contain USDT and USDC')
  const tokenSymbols = new Set()
  for (const token of admission.paymentTokens) {
    exactKeys(token, ['symbol', 'address', 'decimals', 'policyVersion', 'enabledForNewProjects'], 'payment token')
    if (!['USDT', 'USDC'].includes(token.symbol) || tokenSymbols.has(token.symbol)) fail('payment token symbol invalid or duplicated')
    tokenSymbols.add(token.symbol)
    address(token.address, `${token.symbol} address`)
    if (integer(token.decimals, `${token.symbol} decimals`) !== 18 || integer(token.policyVersion, `${token.symbol} policyVersion`) !== 0 || bool(token.enabledForNewProjects, `${token.symbol} enabledForNewProjects`)) fail(`${token.symbol} must remain unadmitted at package admission`)
  }

  if (!Array.isArray(admission.migrations) || admission.migrations.length !== 6) fail('migrations must contain 0913-0918')
  const migrationFiles = new Set()
  for (const migration of admission.migrations) {
    exactKeys(migration, ['file', 'sha256'], 'migration')
    if (!/^migrations\/2026_09_1[3-8]_[a-z0-9_]+\.sql$/.test(migration.file) || migrationFiles.has(migration.file)) fail('migration path invalid or duplicated')
    migrationFiles.add(migration.file)
    sha(migration.sha256, `${migration.file} sha256`)
    if (mode === 'source') {
      const migrationPath = path.join(root, migration.file)
      if (!fs.statSync(migrationPath).isFile() || fileSha(migrationPath) !== migration.sha256) fail(`migration checksum mismatch: ${migration.file}`)
    }
  }

  exactKeys(admission.releaseScope, ['components', 'excluded'], 'releaseScope')
  if (JSON.stringify(admission.releaseScope.components) !== JSON.stringify(['backend', 'job', 'notice', 'frontend', 'admin'])) fail('release components mismatch')
  if (JSON.stringify(admission.releaseScope.excluded) !== JSON.stringify(['contracts', 'deployment-scripts', 'database-dumps', 'secrets'])) fail('release exclusions mismatch')

  if (mode === 'source') {
    envExact('RELEASE_PROFILE', admission.profile)
    envExact('RELEASE_ENVIRONMENT', admission.environment)
    envExact('VITE_FAIR_LAUNCH_CHAIN_ID', admission.chain.chainId, Number)
    envExact('VITE_FAIR_LAUNCH_PROXY_ADDRESS', admission.chain.proxyAddress, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS', admission.chain.implementationAddress, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_IMPLEMENTATION_CODE_HASH', admission.chain.implementationRuntimeCodeHash, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_PROXY_CODE_HASH', admission.chain.proxyRuntimeCodeHash, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_START_BLOCK', admission.chain.startBlock, Number)
    envExact('VITE_FAIR_LAUNCH_DEPLOYMENT_BLOCK_HASH', admission.chain.deploymentBlockHash, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_CONFIRMATION_POLICY', admission.chain.confirmationPolicy)
    envExact('VITE_FAIR_LAUNCH_CONTRACT_VERSION_HASH', admission.chain.contractVersionHash, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_FULL_SET_DIGEST', admission.bundle.fullSetDigest, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_AGGREGATE_ABI_DIGEST', admission.bundle.aggregateAbiDigest, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_SELECTOR_MAP_DIGEST', admission.bundle.selectorMapDigest, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST', admission.bundle.storageLayoutDigest, (v) => String(v).toLowerCase())
    const admittedToken = Object.fromEntries(admission.paymentTokens.map((token) => [token.symbol, token]))
    envExact('VITE_FAIR_LAUNCH_USDT_ADDRESS', admittedToken.USDT.address, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_USDC_ADDRESS', admittedToken.USDC.address, (v) => String(v).toLowerCase())
    envExact('VITE_FAIR_LAUNCH_STABLECOIN_DECIMALS', 18, Number)
    envExact('FAIR_LAUNCH_DEPLOY_BASELINE_COMMIT', admission.identities.deployBaseline.commit)
    envExact('FAIR_LAUNCH_CONTRACT_COMMIT', admission.identities.contract.commit)
    envExact('FAIR_LAUNCH_BACKEND_COMMIT', admission.identities.backend.commit)
    envExact('FAIR_LAUNCH_FRONTEND_COMMIT', admission.identities.frontend.commit)
    envExact('FAIR_LAUNCH_ADMIN_COMMIT', admission.identities.admin.commit)
    for (const item of admission.components) {
      const prefix = `FAIR_LAUNCH_COMPONENT_${item.key.toUpperCase()}`
      envExact(`${prefix}_ADDRESS`, item.address, (v) => String(v).toLowerCase())
      envExact(`${prefix}_RUNTIME_CODE_HASH`, item.runtimeCodeHash, (v) => String(v).toLowerCase())
    }
    const authorityEnv = {
      FAIR_LAUNCH_GOVERNANCE_SAFE: 'governanceSafe', FAIR_LAUNCH_GOVERNANCE_TIMELOCK: 'governanceTimelock',
      FAIR_LAUNCH_EMERGENCY_SAFE: 'emergencySafe', FAIR_LAUNCH_UNPAUSE_TIMELOCK: 'unpauseTimelock',
      FAIR_LAUNCH_TREASURY_SAFE: 'treasurySafe', FAIR_LAUNCH_PLATFORM_FEE_SAFE: 'platformFeeSafe',
      FAIR_LAUNCH_PROJECT_PROCEEDS_SAFE: 'projectProceedsSafe', FAIR_LAUNCH_OPERATOR: 'operator',
    }
    for (const [envName, key] of Object.entries(authorityEnv)) envExact(envName, admission.authorities[key], (v) => String(v).toLowerCase())
    envExact('FAIR_LAUNCH_PLATFORM_FEE_BPS', admission.authorities.platformFeeBps, Number)
    admission.migrations.forEach((migration, index) => envExact(`FAIR_LAUNCH_MIGRATION_091${index + 3}_SHA256`, migration.sha256))
    for (const key of ['VITE_FAIR_LAUNCH_WALLET_SUBMISSION', 'VITE_FAIR_LAUNCH_ADMIN_DISPATCH', 'VITE_FAIR_LAUNCH_MATERIALIZATION']) envExact(key, 'false')

    const expectedCommits = JSON.parse(process.env.FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS || '{}')
    exactKeys(expectedCommits, ['contract', 'backend', 'frontend', 'admin'], 'FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS')
    for (const key of Object.keys(expectedCommits)) {
      if (commit(expectedCommits[key], `actual ${key} commit`) !== admission.identities[key].commit) fail(`${key} commit is not admitted`)
    }
    execFileSync('git', ['-C', root, 'merge-base', '--is-ancestor', admission.identities.deployBaseline.commit, 'HEAD'], { stdio: 'ignore' })
  }

  if (mode === 'package') {
    const manifestFile = path.join(root, 'release-manifest.json')
    if (!fs.statSync(manifestFile).isFile()) fail('packaged release manifest missing')
    const manifest = strictJsonParse(fs.readFileSync(manifestFile, 'utf8'), 'release manifest')
    const expectedIdentities = { contract: admission.identities.contract, backend: admission.identities.backend, frontend: admission.identities.frontend, admin: admission.identities.admin }
    if (manifest.profile !== admission.profile || manifest.environment !== admission.environment) fail('manifest profile/environment not bound to admission')
    if (!manifest.admission || manifest.admission.id !== admission.admissionId || manifest.admission.revision !== admission.revision || manifest.admission.sha256 !== fileSha(file) || manifest.admission.package_build_permitted !== true || manifest.admission.operational_apply_permitted !== false) fail('manifest admission binding mismatch')
    if (!manifest.identities || !/^[0-9a-f]{40}$/.test(manifest.identities.deploy?.commit || '')) fail('release-tooling deploy commit invalid')
    if (!trustedRepo) fail('trusted release-tooling repository required for package verification')
    execFileSync('git', ['-C', trustedRepo, 'cat-file', '-e', `${manifest.identities.deploy.commit}^{commit}`], { stdio: 'ignore' })
    execFileSync('git', ['-C', trustedRepo, 'merge-base', '--is-ancestor', admission.identities.deployBaseline.commit, manifest.identities.deploy.commit], { stdio: 'ignore' })
    for (const [key, value] of Object.entries(expectedIdentities)) if (canonical(manifest.identities[key]) !== canonical(value)) fail(`manifest ${key} identity not bound to admission`)
    const expectedAuthority = {
      chain: admission.chain,
      bundle: admission.bundle,
      components: admission.components,
      authorities: admission.authorities,
      paymentTokens: admission.paymentTokens,
      migrations: admission.migrations,
      capabilities: {
        walletSubmissionPermitted: false,
        adminDispatchPermitted: false,
        materializationPermitted: false,
      },
    }
    if (!manifest.fair_launch || canonical(manifest.fair_launch) !== canonical(expectedAuthority)) fail('manifest Fair Launch authority not bound to admission')
  }

  return {
    admissionId: admission.admissionId,
    revision: admission.revision,
    admissionSha256: fileSha(file),
    status: admission.status,
    packageBuildPermitted: admission.permissions.packageBuildPermitted,
    operationalApplyPermitted: admission.permissions.operationalApplyPermitted,
  }
}

try {
  const [mode, fileArg, rootArg = process.cwd(), trustedRepoArg = ''] = process.argv.slice(2)
  if (!['source', 'package'].includes(mode) || !fileArg) fail('usage: validate-fair-launch-release-admission.js source|package FILE [ROOT]')
  const file = path.resolve(fileArg)
  const root = path.resolve(rootArg)
  const raw = fs.readFileSync(file, 'utf8')
  const admission = strictJsonParse(raw, 'release admission')
  process.stdout.write(`${JSON.stringify(validate(admission, file, root, mode, trustedRepoArg ? path.resolve(trustedRepoArg) : ''))}\n`)
} catch (error) {
  process.stderr.write(`fair_launch_release_admission_invalid: ${error.message}\n`)
  process.exit(1)
}

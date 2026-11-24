'use strict';

const childProcess = require('node:child_process');
const { ethers, network } = require('hardhat');
const local = require('./fairLaunchPersistentLocalE2E');
const { aggregateInterface, deployFacetSet, manifest: fullSetManifest, currentSourceIdentity } = require('../fairLaunchArchitecture');

const PROJECT_IDS = Object.freeze({ successful: '102', failed: '101', cancelled: '103' });

async function readSigningKey() {
  const chunks = [];
  for await (const chunk of process.stdin) chunks.push(chunk);
  const input = local.parseJsonNoDuplicateKeys(Buffer.concat(chunks).toString('utf8'), 'signing pipe JSON');
  if (!input || Object.keys(input).length !== 1 || !input.privateKeyPkcs8) throw new Error('signing pipe input is invalid');
  const bytes = Buffer.from(input.privateKeyPkcs8, 'base64');
  const key = require('node:crypto').createPrivateKey({ key: bytes, format: 'der', type: 'pkcs8' });
  bytes.fill(0); input.privateKeyPkcs8 = '';
  return key;
}

async function codeIdentity(contract, extra = {}) {
  const address = (await contract.getAddress()).toLowerCase();
  const code = await ethers.provider.getCode(address);
  if (code === '0x') throw new Error(`empty runtime code at ${address}`);
  return { ...extra, address, runtimeCodeHash: ethers.keccak256(code).toLowerCase() };
}
async function receiptFacts(tx) {
  const receipt = await tx.wait();
  const block = await ethers.provider.getBlock(receipt.blockNumber);
  if (!block || receipt.status !== 1) throw new Error('deployment transaction failed');
  return { txHash: tx.hash.toLowerCase(), blockNumber: String(receipt.blockNumber), blockHash: receipt.blockHash.toLowerCase(), transactionIndex: receipt.index, blockTimestamp: String(block.timestamp), from: tx.from.toLowerCase(), to: tx.to ? tx.to.toLowerCase() : null, contractAddress: receipt.contractAddress ? receipt.contractAddress.toLowerCase() : null, valueRaw: tx.value.toString(), inputHash: ethers.keccak256(tx.data).toLowerCase() };
}
async function deployed(factory, ...args) {
  const contract = await factory.deploy(...args);
  await contract.waitForDeployment();
  return contract;
}

async function main() {
  local.rejectForbiddenEnvironment();
  const runId = local.exactRunId(process.env.JC_FAIR_LAUNCH_LOCAL_E2E_RUN_ID);
  const rpc = local.exactRpcUrl(process.env.JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL);
  const paths = local.runPaths(runId, process.env.JC_FAIR_LAUNCH_LOCAL_E2E_STATE_ROOT);
  local.verifyProcessAuthority(paths, runId, rpc.url);
  const networkIdentity = await local.verifyRpc(rpc.url);
  const trust = local.verifyPublicTrustAuthority(paths, runId, rpc.url, networkIdentity.genesisHash);
  const signingKey = await readSigningKey();
  if (network.name !== 'localE2E') throw new Error('deployment requires localE2E network');
  const chain = await ethers.provider.getNetwork();
  if (chain.chainId !== 31337n) throw new Error('deployment chain must be 31337');
  const signers = await ethers.getSigners();
  if (signers.length < 14) throw new Error('persistent local node lacks deterministic test signers');
  const [governance, operator, guardian, unpauseTimelock, treasury, successfulOwner, failedOwner, cancelledOwner, browserUser, safeOwnerA, safeOwnerB, safeOwnerC] = signers;
  const iface = await aggregateInterface();
  const facetSet = await deployFacetSet();
  const Proxy = await ethers.getContractFactory('FairLaunchERC1967Proxy');
  const Token = await ethers.getContractFactory('FairLaunchTokenMock');
  const Safe = await ethers.getContractFactory('FairLaunchSafeExecutor');
  const safeOwners = [safeOwnerA.address, safeOwnerB.address, safeOwnerC.address];
  const proceedsSafe = await deployed(Safe, safeOwners, 2);
  const platformFeeSafe = await deployed(Safe, safeOwners, 2);
  const paymentToken = await deployed(Token, 'Local Browser USDT', 'tUSDT', 6);
  const successfulToken = await deployed(Token, 'Browser Success Token', 'BSPASS', 18);
  const failedToken = await deployed(Token, 'Browser Failed Token', 'BSFAIL', 18);
  const cancelledToken = await deployed(Token, 'Browser Cancelled Token', 'BSCAN', 18);
  const proxy = await Proxy.deploy(await facetSet.dispatcher.getAddress(), iface.encodeFunctionData('initialize', [
    governance.address, operator.address, guardian.address, unpauseTimelock.address, treasury.address,
    await platformFeeSafe.getAddress(), 250,
  ]));
  await proxy.waitForDeployment();
  const proxyFacts = await receiptFacts(proxy.deploymentTransaction());
  const fairLaunch = new ethers.Contract(await proxy.getAddress(), iface, governance);
  const projectSafeDigest = ethers.id(`local-browser-${runId}-project-safe`);
  const feeSafeDigest = ethers.id(`local-browser-${runId}-fee-safe`);
  const paymentPolicyDigest = ethers.id(`local-browser-${runId}-payment-policy`);
  const releasePolicyDigest = ethers.id(`local-browser-${runId}-release-policy`);
  const paymentIdentity = await codeIdentity(paymentToken, { symbol: 'tUSDT', decimals: 6 });
  const tokenRows = [];
  for (const [key, token] of [['successful', successfulToken], ['failed', failedToken], ['cancelled', cancelledToken]]) {
    tokenRows.push(await codeIdentity(token, { key, symbol: key === 'successful' ? 'BSPASS' : key === 'failed' ? 'BSFAIL' : 'BSCAN', decimals: 18 }));
  }
  await (await fairLaunch.configureBeneficiarySafe(await proceedsSafe.getAddress(), true, projectSafeDigest, 1)).wait();
  await (await fairLaunch.configureBeneficiarySafe(await platformFeeSafe.getAddress(), true, feeSafeDigest, 1)).wait();
  await (await fairLaunch.configureReleasePolicy(releasePolicyDigest, 1, true)).wait();
  await (await fairLaunch.registerPaymentTokenPolicyRevision(await paymentToken.getAddress(), [true, 6, 1, paymentIdentity.runtimeCodeHash, paymentPolicyDigest])).wait();
  await (await fairLaunch.setPaymentTokenAdmission(await paymentToken.getAddress(), 1, true)).wait();
  const tokens = { successful: successfulToken, failed: failedToken, cancelled: cancelledToken };
  const tokenIdentity = Object.fromEntries(tokenRows.map((row) => [row.key, row]));
  const policyDigest = {};
  for (const key of Object.keys(tokens)) {
    policyDigest[key] = ethers.id(`local-browser-${runId}-${key}-token-policy`);
    await (await fairLaunch.configureProjectToken(await tokens[key].getAddress(), [true, false, 18, 1, tokenIdentity[key].runtimeCodeHash, policyDigest[key]])).wait();
  }
  const latest = await ethers.provider.getBlock('latest');
  // Persistent Hardhat advances timestamps while the three projects are created,
  // funded and activated. Keep a bounded setup margin, then move exactly to it.
  const startTime = latest.timestamp + 10 * 60;
  const endTime = startTime + 12 * 60 * 60;
  const saleAmount = ethers.parseUnits('1000000', 18);
  const projectRows = [];
  async function createProject(key, owner, softCap) {
    const token = tokens[key], tokenAddress = await token.getAddress();
    const params = {
      projectId: PROJECT_IDS[key], projectToken: tokenAddress, paymentToken: await paymentToken.getAddress(), projectOwner: owner.address,
      projectProceedsReceiver: await proceedsSafe.getAddress(), projectProceedsReceiverAttestationDigest: projectSafeDigest, projectProceedsReceiverAttestationVersion: 1,
      expectedPlatformFeeReceiver: await platformFeeSafe.getAddress(), expectedPlatformFeeReceiverAttestationDigest: feeSafeDigest, expectedPlatformFeeReceiverAttestationVersion: 1,
      expectedPlatformFeeBps: 250, saleTokenAmount: saleAmount, softCap, minContribution: ethers.parseUnits('1', 6), maxContribution: 0,
      startTime, endTime, expectedProjectTokenPolicyDigest: policyDigest[key], expectedProjectTokenPolicyVersion: 1,
      expectedPaymentTokenPolicyDigest: paymentPolicyDigest, expectedPaymentTokenPolicyVersion: 1,
      expectedReleasePolicyDigest: releasePolicyDigest, expectedReleasePolicyVersion: 1,
    };
    const createTx = await fairLaunch.connect(operator).createProject(params, [endTime], [10000]);
    const creation = await receiptFacts(createTx);
    const creationReceipt = await ethers.provider.getTransactionReceipt(createTx.hash);
    const creationLogs = creationReceipt.logs.map((log) => { try { return { parsed: iface.parseLog(log), log }; } catch { return null; } }).filter((item) => item?.parsed?.name === 'FairLaunchProjectCreated');
    if (creationLogs.length !== 1 || creationLogs[0].log.address.toLowerCase() !== (await fairLaunch.getAddress()).toLowerCase()) throw new Error('project creation event identity is ambiguous');
    await (await token.mint(owner.address, saleAmount)).wait();
    await (await token.connect(owner).approve(await fairLaunch.getAddress(), saleAmount)).wait();
    await (await fairLaunch.connect(owner).depositProjectTokens(PROJECT_IDS[key], saleAmount)).wait();
    await (await fairLaunch.connect(operator).activateProject(PROJECT_IDS[key])).wait();
    projectRows.push({ key, projectId: PROJECT_IDS[key], projectToken: tokenAddress.toLowerCase(), paymentToken: (await paymentToken.getAddress()).toLowerCase(), projectOwner: owner.address.toLowerCase(), projectProceedsReceiver: (await proceedsSafe.getAddress()).toLowerCase(), platformFeeReceiver: (await platformFeeSafe.getAddress()).toLowerCase(), platformFeeBps: 250, startTime: String(startTime), endTime: String(endTime), saleTokenAmountRaw: saleAmount.toString(), softCapRaw: softCap.toString(), minContributionRaw: ethers.parseUnits('1',6).toString(), maxContributionRaw: '0', totalTokensDepositedRaw: saleAmount.toString(), creation, creationEventLogIndex: creationLogs[0].log.index });
  }
  await createProject('successful', successfulOwner, ethers.parseUnits('100', 6));
  await createProject('failed', failedOwner, ethers.parseUnits('100000', 6));
  await createProject('cancelled', cancelledOwner, ethers.parseUnits('100', 6));
  // Keep the cancellation scenario pristine and Active. Cancellation, refund,
  // and terminal token withdrawal must happen after runtime admission so the
  // Backend can materialize the real lifecycle under one opaque capability.
  for (const row of projectRows) { row.initialStatus = 'active'; row.statusCode = 1; row.terminalReasonCode = 0; }
  await network.provider.send('evm_setNextBlockTimestamp', [startTime]);
  const observationTx = await browserUser.sendTransaction({ to: browserUser.address, value: 0 });
  const observation = await receiptFacts(observationTx);
  const successful = await fairLaunch.getProject(PROJECT_IDS.successful);
  const failed = await fairLaunch.getProject(PROJECT_IDS.failed);
  const cancelled = await fairLaunch.getProject(PROJECT_IDS.cancelled);
  if (Number(successful.status) !== 1 || successful.totalTokensDeposited !== saleAmount) throw new Error('successful browser project is not active and fully deposited');
  if (Number(failed.status) !== 1 || failed.totalTokensDeposited !== saleAmount) throw new Error('failed-scenario browser project is not active and fully deposited');
  if (Number(cancelled.status) !== 1 || cancelled.totalTokensDeposited !== saleAmount) throw new Error('cancellation-scenario browser project is not active and fully deposited');
  for (const [key, live] of [['successful',successful],['failed',failed],['cancelled',cancelled]]) projectRows.find((row) => row.key === key).pristineStorage = local.normalizedProjectStorage(live);
  const fullSet = await fullSetManifest(facetSet);
  const source = currentSourceIdentity();
  const contracts = {
    proxy: await codeIdentity(proxy), dispatcher: await codeIdentity(facetSet.dispatcher), coreFacet: await codeIdentity(facetSet.core),
    releaseFacet: await codeIdentity(facetSet.release), fundsFacet: await codeIdentity(facetSet.funds), adminFacet: await codeIdentity(facetSet.admin),
    viewsFacet: await codeIdentity(facetSet.views), logicLibrary: await codeIdentity(facetSet.logic), runtimeLibrary: await codeIdentity(facetSet.runtime),
  };
  const createdAt = new Date();
  const expiresAt = new Date(Math.min(createdAt.getTime() + 20 * 60 * 60 * 1000, endTime * 1000));
  const payload = {
    schemaVersion: local.MANIFEST_SCHEMA, authority: local.AUTHORITY, runId, createdAt: createdAt.toISOString(), expiresAt: expiresAt.toISOString(),
    network: { name: 'hardhat-local-persistent', chainId: 31337, rpcUrl: rpc.url, genesisHash: networkIdentity.genesisHash },
    contracts,
    assets: { paymentToken: paymentIdentity, projectTokens: tokenRows, faucet: { strategy: 'unrestricted-token-mint', nativeGasStrategy: 'frontend-local-hardhat_setBalance', storesPrivateKeys: false } },
    safes: { proceeds: await codeIdentity(proceedsSafe, { threshold: 2 }), platformFee: await codeIdentity(platformFeeSafe, { threshold: 2 }), owners: safeOwners.map((address) => address.toLowerCase()) },
    projects: projectRows,
    roles: { governance: governance.address.toLowerCase(), operator: operator.address.toLowerCase(), pauseGuardian: guardian.address.toLowerCase(), unpauseTimelock: unpauseTimelock.address.toLowerCase(), treasury: treasury.address.toLowerCase(), browserSeedAccount: browserUser.address.toLowerCase() },
    deployment: { proxy: proxyFacts, observation },
    source: { commit: source.sourceCommit, sourceTreeDigest: source.sourceTreeDigest, aggregateAbiDigest: fullSet.aggregateAbiDigest, selectorMapDigest: fullSet.selectorMapDigest, storageLayoutDigest: fullSet.storageLayoutDigest, fullSetDigest: fullSet.fullSetDigest, dirty: childProcess.execFileSync('git', ['status','--porcelain'], { cwd: local.REPOSITORY_ROOT, encoding: 'utf8' }).trim().length > 0 },
  };
  const envelope = local.signManifestPayload(payload, signingKey, trust);
  local.createCanonicalJsonOnce(paths.manifest, envelope);
  local.noPrivateMaterial(paths.manifest);
  local.verifyManifestSignatureIntegrity(local.readStrictJson(paths.manifest), trust.publicKeyJwk, trust.keyId);
  for (const [key, identity] of Object.entries(contracts)) {
    const observed = ethers.keccak256(await ethers.provider.getCode(identity.address)).toLowerCase();
    if (observed !== identity.runtimeCodeHash) throw new Error(`post-publication ${key} code identity mismatch`);
  }
  process.stdout.write(`${JSON.stringify({ status: 'deployed', runId, rpcUrl: rpc.url, manifest: paths.manifest, payloadSha256: envelope.signature.payloadSha256, projectIds: PROJECT_IDS })}\n`);
}

main().catch((error) => { process.stderr.write(`${error.stack || error.message || error}\n`); process.exitCode = 1; });

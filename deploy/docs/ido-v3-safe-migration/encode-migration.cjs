// Offline only: no provider, private key, wallet, RPC or broadcast capability.
const fs = require('node:fs')
const path = require('node:path')
const { Interface, getAddress, ZeroAddress } = require('ethers')
const argv = process.argv.slice(2)
try {
  const decode = argv[0] === 'decode'
  if (decode) argv.shift()
  const kind = argv.shift()
  if (!['v3', 'safe'].includes(kind)) throw Error('Use v3|safe, or decode v3|safe <hex>')
  const abi = JSON.parse(fs.readFileSync(path.join(__dirname,
    kind === 'v3' ? 'v3-governance-migration.abi.json' : 'safe-owner-management.abi.json')))
  const iface = new Interface(abi)
  if (decode) {
    if (argv.length !== 1) throw Error('One calldata hex required')
    const tx = iface.parseTransaction({ data: argv[0] })
    if (!tx) throw Error('Unknown selector')
    console.log(JSON.stringify({ signature: tx.signature, arguments: Array.from(tx.args) }, (_,v) => typeof v === 'bigint' ? v.toString() : v, 2))
  } else {
    const name = argv.shift()
    const allowed = kind === 'v3' ? ['transferOwnership', 'acceptOwnership', 'setOperator'] :
      ['swapOwner', 'addOwnerWithThreshold', 'removeOwner', 'changeThreshold']
    if (!allowed.includes(name)) throw Error('Method not in migration allowlist')
    const fragment = iface.getFunction(name)
    if (argv.length !== fragment.inputs.length) throw Error('Wrong number of arguments')
    const values = argv.map((value, i) => {
      if (fragment.inputs[i].type === 'address') {
        const address = getAddress(value)
        if (address === ZeroAddress) throw Error('Zero address rejected by migration tool')
        return address
      }
      if (!/^[1-9][0-9]*$/.test(value)) throw Error('Positive integer required')
      return BigInt(value)
    })
    console.log(JSON.stringify({ offline: true, method: fragment.format('sighash'),
      target: kind === 'v3' ? 'V3 Proxy 0x0D4040875742975269c55E16C8884d3Ee9510B8E' : 'The SAME Safe whose owners are being changed (self-call)',
      operation: 'CALL (0)', value: '0', data: iface.encodeFunctionData(fragment, values),
      warning: 'No authority/nonce/simulation is granted. Recheck current owners, linked-list predecessor and target before execution.' }, null, 2))
  }
} catch (e) { console.error(e.message); process.exitCode = 1 }

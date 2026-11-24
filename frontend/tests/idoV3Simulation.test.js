import test from 'node:test'
import assert from 'node:assert/strict'
import { Interface } from 'ethers'
import { simulateV3Transaction } from '../src/services/idoV3Simulation.js'

const transaction = { from: '0x'+'1'.repeat(40), to: '0x'+'2'.repeat(40), data: '0x12345678', value: '0x0' }
test('one exact eth_estimateGas simulation, no call/code/storage/balance requests or fallback', async () => {
  const requests = []
  const gas = await simulateV3Transaction('https://rpc.invalid', transaction, async (url, options) => {
    requests.push(JSON.parse(options.body))
    assert.equal(url, 'https://rpc.invalid')
    return { ok: true, json: async () => ({ id: 1, result: '0x186a0' }) }
  })
  assert.equal(gas, 125000n)
  assert.deepEqual(requests, [{ jsonrpc: '2.0', id: 1, method: 'eth_estimateGas', params: [transaction] }])
})

test('simulation reverts preserve safe approval/cap/proof errors without additional queries', async () => {
  for (const [signature, args, code] of [
    ['ERC20InsufficientAllowance(address,uint256,uint256)', [transaction.from,0,1], 'IDO_V3_ALLOWANCE_INSUFFICIENT'],
    ['ERC20InsufficientBalance(address,uint256,uint256)', [transaction.from,0,1], 'IDO_V3_BALANCE_INSUFFICIENT'],
    ['ExceedsHardCap()', [], 'IDO_V3_HARD_CAP_EXCEEDED'],
    ['ExceedsMaxPerUser()', [], 'IDO_V3_USER_LIMIT_EXCEEDED'],
    ['WhitelistVersionChanged(uint64,uint64)', [1,2], 'IDO_V3_POLICY_CHANGED'],
    ['NoTokensToClaim()', [], 'IDO_V3_ACTION_UNAVAILABLE'],
  ]) {
    const abi = new Interface(['error '+signature])
    let requests = 0
    await assert.rejects(simulateV3Transaction('https://rpc.invalid', transaction, async () => {
      requests++
      return { ok: true, json: async () => ({ id: 1, error: { data: abi.encodeErrorResult(signature.split('(')[0], args) } }) }
    }), { code })
    assert.equal(requests, 1)
  }
})

test('invalid/unavailable/timed-out simulation never returns fallback gas', async () => {
  for (const patch of [{ value: '0x1' }, { to: 'bad' }, { from: '' }, { data: '0x' }]) {
    await assert.rejects(simulateV3Transaction('https://rpc.invalid', { ...transaction, ...patch },
      async () => { throw Error('must not request malformed transactions') }), { code: 'IDO_V3_IDENTITY_UNAVAILABLE' })
  }
  for (const result of ['0x0', 'garbage', '0x10000000']) {
    await assert.rejects(simulateV3Transaction('https://rpc.invalid', transaction,
      async () => ({ ok: true, json: async () => ({ id: 1, result }) })), { code: 'IDO_V3_STATE_UNAVAILABLE' })
  }
  let signal
  await assert.rejects(simulateV3Transaction('https://rpc.invalid', transaction, async (_, options) => {
    signal = options.signal
    return new Promise(() => {})
  }, 5), { code: 'IDO_V3_STATE_UNAVAILABLE' })
  assert.equal(signal.aborted, true)
})

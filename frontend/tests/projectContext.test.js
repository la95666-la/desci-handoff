import assert from 'node:assert/strict'
import test from 'node:test'

import { normalizeProjectContext } from '../src/services/projectContext.js'

const legacy = {
  chainId: 97,
  contractAddress: '0x1111111111111111111111111111111111111111',
  paymentTokenAddress: '0x2222222222222222222222222222222222222222',
}

test('normalizes a complete V2 identity without symbol-based fallback', () => {
  const context = normalizeProjectContext({
    id: 7,
    contractVersion: 'v2_stablecoin',
    chainId: 97,
    contractAddress: '0x3333333333333333333333333333333333333333',
    paymentTokenAddress: '0x4444444444444444444444444444444444444444',
    paymentTokenDecimals: 6,
    tokenAddress: '0x5555555555555555555555555555555555555555',
    tokenDecimals: 18,
    paymentToken: 'USDT',
  }, legacy)
  assert.equal(context.paymentTokenDecimals, 6)
  assert.equal(context.tokenDecimals, 18)
  assert.equal(context.contractAddress, '0x3333333333333333333333333333333333333333')
})

test('exact V3 context is writable only through its dedicated identity and proof path', () => {
  const context = normalizeProjectContext({
    id: '42950728196620289', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
    chainId: 56, contractAddress: '0x0d4040875742975269c55e16c8884d3ee9510b8e',
    paymentTokenAddress: '0x55d398326f99059ff775485246999027b3197955',
    tokenAddress: '0xf77c5b5faabb9945ac8701ad08ef09e12dfc768a',
    paymentTokenDecimals: 18, tokenDecimals: 18, onchainLocked: true,
  }, legacy)
  assert.equal(context.id, '42950728196620289')
  assert.equal(context.readOnlyFunding, false)
  assert.notEqual(context.contractAddress, legacy.contractAddress)
  assert.throws(() => normalizeProjectContext({ ...context, contractVersion: 'future_v3' }, legacy), /Unsupported project funding identity/)
})

test('rejects incomplete or invalid V2 identities', () => {
  const base = { id: 1, contractVersion: 'v2_stablecoin', chainId: 97, paymentTokenDecimals: 6, tokenDecimals: 18 }
  assert.throws(() => normalizeProjectContext(base, legacy), /contractAddress/)
  assert.throws(() => normalizeProjectContext({ ...base, contractAddress: legacy.contractAddress, paymentTokenAddress: legacy.paymentTokenAddress, tokenAddress: legacy.contractAddress, chainId: 0 }, legacy), /chainId/)
  assert.throws(() => normalizeProjectContext({ ...base, contractAddress: legacy.contractAddress, paymentTokenAddress: legacy.paymentTokenAddress, tokenAddress: legacy.contractAddress, paymentTokenDecimals: 19 }, legacy), /payment token decimals/)
})

test('keeps legacy defaults isolated from V2', () => {
  const context = normalizeProjectContext({ id: 3, contractVersion: 'v1_legacy' }, legacy)
  assert.equal(context.contractAddress, legacy.contractAddress)
  assert.equal(context.paymentTokenAddress, legacy.paymentTokenAddress)
  assert.equal(context.paymentTokenDecimals, 18)
})

test('keeps untrusted Fair Launch read-only and never injects capped-presale addresses', () => {
  const context = normalizeProjectContext({
    id: 9,
    fundingMode: 'fair_launch',
    contractVersion: 'fair_launch_v1',
    paymentTokenAddress: '0x1111111111111111111111111111111111111111',
    tokenAddress: '0x2222222222222222222222222222222222222222',
    paymentTokenDecimals: 18,
    tokenDecimals: 18,
  }, legacy)
  assert.equal(context.readOnlyFunding, true)
  assert.equal(context.contractAddress, '')
  assert.notEqual(context.paymentTokenAddress, legacy.paymentTokenAddress)
})

test('rejects unknown or mismatched identities instead of falling back to legacy', () => {
  assert.throws(
    () => normalizeProjectContext({ id: 10, fundingMode: 'fair_launch', contractVersion: 'v2_stablecoin' }, legacy),
    /Unsupported project funding identity/,
  )
  assert.throws(
    () => normalizeProjectContext({ id: 11, contractVersion: 'future_v2' }, legacy),
    /Unsupported project funding identity/,
  )
})

test('V3 is exactly bound and never inherits V2 or legacy addresses', () => {
  const project = {
    id: '41376991395446785', fundingMode: 'capped_presale', contractVersion: 'ido_v3_stablecoin',
    chainId: 97, contractAddress: '0x3333333333333333333333333333333333333333',
    paymentTokenAddress: '0x4444444444444444444444444444444444444444',
    tokenAddress: '0x5555555555555555555555555555555555555555',
    paymentTokenDecimals: 6, tokenDecimals: 18,
  }
  const context = normalizeProjectContext(project, legacy)
  assert.equal(context.id, project.id)
  assert.equal(context.contractAddress, project.contractAddress)
  assert.equal(context.readOnlyFunding, false)
  assert.notEqual(context.contractAddress, legacy.contractAddress)
  assert.throws(() => normalizeProjectContext({ ...project, contractAddress: '' }, legacy), /V3 project contractAddress/)
  assert.throws(() => normalizeProjectContext({ ...project, chainId: 0 }, legacy), /V3 project chainId/)
  assert.throws(() => normalizeProjectContext({ ...project, id: Number(project.id) }, legacy), /V3 project ID/)
  assert.throws(() => normalizeProjectContext({ ...project, fundingMode: 'fair_launch' }, legacy), /Unsupported project funding identity/)
})

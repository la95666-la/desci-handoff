import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  addGasSafetyBuffer,
  canSafelyFallbackGasEstimate,
  canSafelyFallbackTrustedGasRpc,
  canSafelyFallbackRawSend,
  estimateGasViaTrustedRpc,
  isSuccessfulReceipt,
  isValidTransactionHash,
  trustedGasRpcUrl,
} from '../src/services/contract.js'
import { ContractService, contractService } from '../src/services/contract.js'
import { WALLET_ERROR_CODES } from '../src/services/errors.js'
import { web3Service } from '../src/services/web3.js'

const contractSource = readFileSync(new URL('../src/services/contract.js', import.meta.url), 'utf8')

test('accepts only canonical 32-byte EVM transaction hashes', () => {
  assert.equal(
    isValidTransactionHash(
      '0x733a96b18b944741cb6c11e90aa105b01254fcae8378c46447a3e12fd0283b46',
    ),
    true,
  )
  assert.equal(
    isValidTransactionHash('0x7a022eca510aab629e5a23cc4d279d257dbc89d2f9'),
    false,
  )
  assert.equal(isValidTransactionHash(''), false)
  assert.equal(isValidTransactionHash(null), false)
})

test('raw-send fallback is allowed only when the wallet explicitly does not support the method', () => {
  assert.equal(canSafelyFallbackRawSend({ code: -32601, message: 'Method not found' }), true)
  assert.equal(canSafelyFallbackRawSend(new Error('unsupported method: eth_sendTransaction')), true)
  assert.equal(canSafelyFallbackRawSend({ code: 4001, message: 'User rejected' }), false)
  assert.equal(canSafelyFallbackRawSend(new Error('request timed out')), false)
  assert.equal(canSafelyFallbackRawSend(new Error('network error')), false)
})

test('gas estimates receive a rounded-up 25 percent safety buffer', () => {
  assert.equal(addGasSafetyBuffer(211015n), 263769n)
  assert.equal(addGasSafetyBuffer(1n), 2n)
})

test('gas fallback is limited to unavailable estimation and never hides contract reverts', () => {
  assert.equal(canSafelyFallbackGasEstimate({ code: -32601, message: 'Method not found' }), true)
  assert.equal(canSafelyFallbackGasEstimate({ code: 'GAS_ESTIMATE_TIMEOUT' }), false)
  assert.equal(canSafelyFallbackGasEstimate(new Error('unsupported method: eth_estimateGas')), true)
  assert.equal(canSafelyFallbackGasEstimate({ code: 'CALL_EXCEPTION', message: 'execution reverted' }), false)
  assert.equal(canSafelyFallbackGasEstimate(new Error('insufficient allowance')), false)
})

test('trusted RPC fallback to the wallet is limited to infrastructure failures', () => {
  assert.equal(canSafelyFallbackTrustedGasRpc({ code: 'TRUSTED_RPC_TIMEOUT' }), true)
  assert.equal(canSafelyFallbackTrustedGasRpc({ code: 'TRUSTED_RPC_UNAVAILABLE' }), true)
  assert.equal(canSafelyFallbackTrustedGasRpc({ code: 'TRUSTED_RPC_METHOD_UNSUPPORTED' }), true)
  assert.equal(canSafelyFallbackTrustedGasRpc({ code: 'TRUSTED_RPC_CHAIN_MISMATCH' }), false)
  assert.equal(canSafelyFallbackTrustedGasRpc({ code: 'TRUSTED_RPC_SIMULATION_FAILED' }), false)
})

test('trusted gas RPCs are fixed by chain and never taken from the wallet provider', () => {
  assert.equal(trustedGasRpcUrl(56), 'https://bsc-dataseed.bnbchain.org')
  assert.equal(trustedGasRpcUrl(97), 'https://data-seed-prebsc-1-s1.bnbchain.org:8545')
  assert.equal(trustedGasRpcUrl(31337), '')
  assert.equal(trustedGasRpcUrl('not-a-chain'), '')
})

test('supported BSC contract reads use the fixed official RPC instead of the wallet runtime route', () => {
  const originalProvider = web3Service.provider
  const walletProvider = { wallet: true }
  web3Service.provider = walletProvider
  try {
    const service = new ContractService()
    const mainnet = service._readRunner(56)
    const testnet = service._readRunner(97)
    assert.notEqual(mainnet, walletProvider)
    assert.notEqual(testnet, walletProvider)
    assert.equal(mainnet._getConnection().url, 'https://bsc-dataseed.bnbchain.org')
    assert.equal(testnet._getConnection().url, 'https://data-seed-prebsc-1-s1.bnbchain.org:8545')
    assert.equal(service._readRunner(56), mainnet)
    assert.equal(service._readRunner(31337), walletProvider)
  } finally {
    web3Service.provider = originalProvider
  }
})

test('payment allowance, project totals and wallet position reads are bound to project-chain read runners', () => {
  assert.match(contractSource, /_getProjectContract\(project, writable = false\)[\s\S]*?this\._readRunner\(context\.chainId\)/)
  assert.match(contractSource, /getPaymentTokenAllowance\(project, ownerAddress\)[\s\S]*?getTokenAllowanceOnChain\([\s\S]*?context\.chainId/)
  assert.match(contractSource, /getPaymentTokenBalance\(project, ownerAddress\)[\s\S]*?getTokenBalanceOnChain\([\s\S]*?context\.chainId/)
})

test('trusted RPC fallback performs identical read-only call and estimate before returning buffered gas', async () => {
  const tx = {
    from: '0x2222222222222222222222222222222222222222',
    to: '0x1111111111111111111111111111111111111111',
    data: '0x1234',
    value: 0n,
  }
  const requests = []
  const gasLimit = await estimateGasViaTrustedRpc(tx, 56, 100, async (url, options) => {
    assert.equal(url, 'https://bsc-dataseed.bnbchain.org')
    assert.equal(options.method, 'POST')
    const request = JSON.parse(options.body)
    requests.push(request)
    const result = {
      eth_chainId: '0x38',
      eth_call: '0x',
      eth_estimateGas: '0x33847', // 211015
    }[request.method]
    return { ok: true, json: async () => ({ jsonrpc: '2.0', id: request.id, result }) }
  })
  assert.equal(gasLimit, 263769n)
  assert.deepEqual(requests.map((request) => request.method), ['eth_chainId', 'eth_call', 'eth_estimateGas'])
  const rpcTx = { from: tx.from, to: tx.to, data: tx.data, value: '0x0' }
  assert.deepEqual(requests[1].params, [rpcTx, 'latest'])
  assert.deepEqual(requests[2].params, [rpcTx])
})

test('trusted read-only RPC success supplies gas without calling the wallet estimator', async () => {
  const originalProvider = web3Service.provider
  let walletEstimateCalls = 0
  const methods = []
  web3Service.provider = {
    estimateGas: async () => { walletEstimateCalls += 1; return 999999n },
  }
  try {
    const gasLimit = await contractService._estimateGasLimit(
      '0x1111111111111111111111111111111111111111',
      '0x1234',
      '0x2222222222222222222222222222222222222222',
      300000n,
      {
        chainId: 56,
        timeoutMs: 5,
        trustedRpcTimeoutMs: 100,
        fetcher: async (_, options) => {
          const method = JSON.parse(options.body).method
          methods.push(method)
          return { ok: true, json: async () => ({ result: ({ eth_chainId: '0x38', eth_call: '0x', eth_estimateGas: '0x33847' })[method] }) }
        },
      },
    )
    assert.equal(gasLimit, 263769n)
    assert.deepEqual(methods, ['eth_chainId', 'eth_call', 'eth_estimateGas'])
    assert.equal(walletEstimateCalls, 0)
  } finally {
    web3Service.provider = originalProvider
  }
})

test('trusted RPC revert or chain mismatch blocks before wallet fallback', async () => {
  const originalProvider = web3Service.provider
  let walletEstimateCalls = 0
  web3Service.provider = { estimateGas: async () => { walletEstimateCalls += 1; return 211015n } }
  try {
    for (const responseFor of [
      (method) => method === 'eth_chainId' ? { result: '0x61' } : { result: '0x' },
      (method) => method === 'eth_chainId' ? { result: '0x38' } : method === 'eth_call' ? { error: { code: 3, message: 'private revert text' } } : { result: '0x33847' },
    ]) {
      await assert.rejects(
        () => contractService._estimateGasLimit(
          '0x1111111111111111111111111111111111111111',
          '0x1234',
          '0x2222222222222222222222222222222222222222',
          300000n,
          {
            chainId: 56,
            timeoutMs: 5,
            trustedRpcTimeoutMs: 100,
            fetcher: async (_, options) => {
              const request = JSON.parse(options.body)
              return { ok: true, json: async () => ({ jsonrpc: '2.0', id: request.id, ...responseFor(request.method) }) }
            },
          },
        ),
        (error) => ['TRUSTED_RPC_CHAIN_MISMATCH', 'TRUSTED_RPC_SIMULATION_FAILED'].includes(error?.code) && !/private revert text/i.test(error.message),
      )
    }
    assert.equal(walletEstimateCalls, 0)
  } finally {
    web3Service.provider = originalProvider
  }
})

test('trusted RPC timeout falls back once to the bounded wallet estimator', async () => {
  const originalProvider = web3Service.provider
  let walletEstimateCalls = 0
  web3Service.provider = {
    estimateGas: async (transaction) => {
      walletEstimateCalls += 1
      assert.equal(transaction.data, '0x1234')
      return 211015n
    },
  }
  try {
    const gasLimit = await contractService._estimateGasLimit(
      '0x1111111111111111111111111111111111111111',
      '0x1234',
      '0x2222222222222222222222222222222222222222',
      300000n,
      {
        chainId: 56,
        timeoutMs: 100,
        trustedRpcTimeoutMs: 5,
        fetcher: () => new Promise(() => {}),
      },
    )
    assert.equal(gasLimit, 263769n)
    assert.equal(walletEstimateCalls, 1)
  } finally {
    web3Service.provider = originalProvider
  }
})

test('trusted RPC timeout plus wallet timeout fails closed and never invents gas', async () => {
  const originalProvider = web3Service.provider
  web3Service.provider = { estimateGas: () => new Promise(() => {}) }
  try {
    await assert.rejects(
      () => contractService._estimateGasLimit(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        300000n,
        {
          chainId: 56,
          timeoutMs: 5,
          trustedRpcTimeoutMs: 5,
          fetcher: () => new Promise(() => {}),
        },
      ),
      (error) => error?.code === 'GAS_ESTIMATE_TIMEOUT',
    )
  } finally {
    web3Service.provider = originalProvider
  }
})

test('Fair Launch estimate timeout uses the same trusted read-only RPC fallback', async () => {
  const originalProvider = web3Service.provider
  web3Service.provider = { estimateGas: () => new Promise(() => {}) }
  try {
    const gasLimit = await contractService._estimateFairLaunchGasLimit(
      '0x1111111111111111111111111111111111111111',
      '0x1234',
      '0x2222222222222222222222222222222222222222',
      300000n,
      {
        chainId: 56,
        timeoutMs: 5,
        trustedRpcTimeoutMs: 100,
        fetcher: async (_, options) => {
          const request = JSON.parse(options.body)
          return { ok: true, json: async () => ({ result: ({ eth_chainId: '0x38', eth_call: '0x', eth_estimateGas: '0x33847' })[request.method] }) }
        },
      },
    )
    assert.equal(gasLimit, 263769n)
  } finally {
    web3Service.provider = originalProvider
  }
})

test('a stalled wallet gas estimate times out and a late result cannot become a gas limit', async () => {
  const originalProvider = web3Service.provider
  let finishEstimate
  web3Service.provider = {
    estimateGas: () => new Promise((resolve) => { finishEstimate = resolve }),
  }
  try {
    await assert.rejects(
      () => contractService._estimateGasLimit(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        300000n,
        5,
      ),
      (error) => error?.code === 'GAS_ESTIMATE_TIMEOUT',
    )
    finishEstimate(211015n)
    await new Promise((resolve) => setTimeout(resolve, 10))
  } finally {
    web3Service.provider = originalProvider
  }
})

test('purchase never sends after an estimate timeout, including after the estimate resolves late', async () => {
  const originalProvider = web3Service.provider
  const originalEnsure = contractService._ensureProjectSigner
  const originalEstimate = contractService._estimateGasLimit
  const originalSend = contractService._dualChannelSend
  let finishEstimate
  let sendCount = 0
  web3Service.provider = {
    estimateGas: () => new Promise((resolve) => { finishEstimate = resolve }),
  }
  contractService._ensureProjectSigner = async () => ({
    id: '41357045785427969',
    chainId: 97,
    contractAddress: '0x1111111111111111111111111111111111111111',
    paymentTokenDecimals: 18,
  })
  contractService._estimateGasLimit = function (to, data, from, fallback) {
    return originalEstimate.call(this, to, data, from, fallback, 5)
  }
  contractService._dualChannelSend = async () => { sendCount += 1; return '0x' + '1'.repeat(64) }
  const originalSigner = web3Service.signer
  const originalSignerAccount = web3Service._signerAccount
  web3Service.signer = { address: '0x2222222222222222222222222222222222222222' }
  web3Service._signerAccount = '0x2222222222222222222222222222222222222222'
  try {
    await assert.rejects(
      () => contractService.purchase({ saleMode: 'capped_presale' }, '1'),
      (error) => error?.code === 'GAS_ESTIMATE_TIMEOUT' && error?.purchasePhase === 'estimate',
    )
    assert.equal(sendCount, 0)
    finishEstimate(211015n)
    await new Promise((resolve) => setTimeout(resolve, 10))
    assert.equal(sendCount, 0)
  } finally {
    web3Service.provider = originalProvider
    web3Service.signer = originalSigner
    web3Service._signerAccount = originalSignerAccount
    contractService._ensureProjectSigner = originalEnsure
    contractService._estimateGasLimit = originalEstimate
    contractService._dualChannelSend = originalSend
  }
})

test('dynamic gas limit uses the live estimate plus buffer', async () => {
  const originalProvider = web3Service.provider
  let request
  web3Service.provider = {
    estimateGas: async (tx) => {
      request = tx
      return 211015n
    },
  }
  try {
    const gasLimit = await contractService._estimateGasLimit(
      '0x1111111111111111111111111111111111111111',
      '0x1234',
      '0x2222222222222222222222222222222222222222',
      300000n,
    )
    assert.equal(gasLimit, 263769n)
    assert.deepEqual(request, {
      to: '0x1111111111111111111111111111111111111111',
      data: '0x1234',
      from: '0x2222222222222222222222222222222222222222',
      value: 0n,
    })
  } finally {
    web3Service.provider = originalProvider
  }
})

test('unsupported gas estimation uses the conservative fallback', async () => {
  const originalProvider = web3Service.provider
  web3Service.provider = {
    estimateGas: async () => {
      const error = new Error('Method not found')
      error.code = -32601
      throw error
    },
  }
  try {
    assert.equal(
      await contractService._estimateGasLimit(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        300000n,
      ),
      300000n,
    )
  } finally {
    web3Service.provider = originalProvider
  }
})

test('business revert during gas estimation blocks the transaction instead of using fallback', async () => {
  const originalProvider = web3Service.provider
  const revert = new Error('execution reverted: ExceedsMaxPerUser')
  revert.code = 'CALL_EXCEPTION'
  web3Service.provider = { estimateGas: async () => { throw revert } }
  try {
    await assert.rejects(
      () => contractService._estimateGasLimit(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        300000n,
      ),
      (error) => error === revert,
    )
  } finally {
    web3Service.provider = originalProvider
  }
})

test('purchase, claim and refund use the project-chain trusted estimator', () => {
  assert.equal(
    contractSource.match(/_estimateGasLimit\(context\.contractAddress, data, from, 300000n, \{ chainId: context\.chainId \}\)/g)?.length,
    5,
  )
  assert.doesNotMatch(contractSource, /_dualChannelSend\(context\.contractAddress, data, from, (?:150000|200000)\)/)
})

test('purchase, claim and refund pass the project chain to estimation before one wallet send', async () => {
  for (const [method, args] of [['purchase', ['1']], ['claim', []], ['refund', []]]) {
    const service = new ContractService()
    const observed = []
    service._ensureProjectSigner = async () => ({
      id: '41357045785427969',
      chainId: 56,
      contractAddress: '0x1111111111111111111111111111111111111111',
      paymentTokenDecimals: 18,
    })
    service._estimateGasLimit = async (to, data, from, fallback, options) => {
      observed.push({ stage: 'estimate', to, data, from, fallback, options })
      return 263769n
    }
    service._dualChannelSend = async (to, data, from, gasLimit) => {
      observed.push({ stage: 'send', to, data, from, gasLimit })
      return '0x' + 'a'.repeat(64)
    }
    service._waitForReceipt = async () => ({ status: 1 })
    const originalSigner = web3Service.signer
    const originalSignerAccount = web3Service._signerAccount
    web3Service.signer = { address: '0x2222222222222222222222222222222222222222' }
    web3Service._signerAccount = '0x2222222222222222222222222222222222222222'
    try {
      const result = await service[method]({ fundingMode: 'capped_presale', contractVersion: 'v2_stablecoin' }, ...args)
      assert.equal(result.status, 'success', method)
      assert.equal(observed.length, 2, method)
      assert.equal(observed[0].stage, 'estimate', method)
      assert.deepEqual(observed[0].options, { chainId: 56 }, method)
      assert.equal(observed[1].stage, 'send', method)
      assert.equal(observed[1].gasLimit, 263769n, method)
      assert.equal(observed[1].data, observed[0].data, method)
    } finally {
      web3Service.signer = originalSigner
      web3Service._signerAccount = originalSignerAccount
    }
  }
})

test('other project writes avoid ethers implicit wallet estimation', () => {
  for (const method of ['approveProjectToken', 'depositProjectTokens', 'withdrawRaisedFunds', 'withdrawProjectTokens']) {
    const start = contractSource.indexOf(`async ${method}(`)
    const end = contractSource.indexOf('\n    }', start)
    const source = contractSource.slice(start, end)
    assert.match(source, /_estimateGasLimit\(/, method)
    assert.match(source, /_dualChannelSend\(/, method)
    assert.doesNotMatch(source, /\.wait\(|\.approve\(|\.depositProjectTokens\(|\.withdrawRaisedFunds\(|\.withdrawProjectTokens\(/, method)
  }
})

test('legacy JCC approval also avoids ethers implicit wallet estimation', () => {
  const start = contractSource.indexOf('async approveJCC(')
  const end = contractSource.indexOf('\n    }', start)
  const source = contractSource.slice(start, end)
  assert.match(source, /_estimateGasLimit\(ADDRESSES\.JCC/)
  assert.match(source, /_dualChannelSend\(ADDRESSES\.JCC/)
  assert.doesNotMatch(source, /_getJCCSigner\(\)\.approve|\.wait\(/)
})

test('receipt success accepts ethers and JSON-RPC status formats without treating revert as success', () => {
  assert.equal(isSuccessfulReceipt({ status: 1 }), true)
  assert.equal(isSuccessfulReceipt({ status: 1n }), true)
  assert.equal(isSuccessfulReceipt({ status: '0x1' }), true)
  assert.equal(isSuccessfulReceipt({ status: 0 }), false)
  assert.equal(isSuccessfulReceipt({ status: '0x0' }), false)
  assert.equal(isSuccessfulReceipt(null), false)
})

test('receipt polling uses the fixed trusted RPC first and skips runtime and wallet fallbacks on success', async () => {
  const originalWindow = globalThis.window
  const originalProvider = web3Service.provider
  const expectedHash = '0x' + 'd'.repeat(64)
  const expectedReceipt = { status: '0x1', transactionHash: expectedHash }
  const urls = []
  let walletProviderCalls = 0
  globalThis.window = { __JC_ACTIVE_RPC_URL: 'https://runtime-rpc.example' }
  web3Service.provider = {
    getTransactionReceipt: async () => { walletProviderCalls += 1; return expectedReceipt },
  }
  try {
    const result = await contractService._waitForReceipt(expectedHash, {
      chainId: 56,
      timeoutMs: 100,
      queryTimeoutMs: 20,
      fetcher: async (url, options) => {
        urls.push(url)
        const request = JSON.parse(options.body)
        assert.equal(request.method, 'eth_getTransactionReceipt')
        assert.deepEqual(request.params, [expectedHash])
        return { ok: true, json: async () => ({ jsonrpc: '2.0', result: expectedReceipt }) }
      },
    })
    assert.equal(result, expectedReceipt)
    assert.deepEqual(urls, ['https://bsc-dataseed.bnbchain.org'])
    assert.equal(walletProviderCalls, 0)
  } finally {
    if (originalWindow === undefined) delete globalThis.window
    else globalThis.window = originalWindow
    web3Service.provider = originalProvider
  }
})

test('receipt polling falls back from trusted RPC to runtime RPC, then stops before the wallet', async () => {
  const originalWindow = globalThis.window
  const originalProvider = web3Service.provider
  const expectedHash = '0x' + 'e'.repeat(64)
  const expectedReceipt = { status: '0x1', transactionHash: expectedHash }
  const urls = []
  let walletProviderCalls = 0
  globalThis.window = { __JC_ACTIVE_RPC_URL: 'https://runtime-rpc.example' }
  web3Service.provider = { getTransactionReceipt: async () => { walletProviderCalls += 1; return expectedReceipt } }
  try {
    const result = await contractService._waitForReceipt(expectedHash, {
      chainId: 56,
      timeoutMs: 100,
      queryTimeoutMs: 20,
      fetcher: async (url) => {
        urls.push(url)
        return { ok: true, json: async () => ({ result: url.includes('bnbchain.org') ? null : expectedReceipt }) }
      },
    })
    assert.equal(result, expectedReceipt)
    assert.deepEqual(urls, ['https://bsc-dataseed.bnbchain.org', 'https://runtime-rpc.example'])
    assert.equal(walletProviderCalls, 0)
  } finally {
    if (originalWindow === undefined) delete globalThis.window
    else globalThis.window = originalWindow
    web3Service.provider = originalProvider
  }
})

test('a pending trusted receipt request is bounded before runtime and wallet fallback', async () => {
  const originalWindow = globalThis.window
  const originalProvider = web3Service.provider
  const expectedHash = '0x' + 'f'.repeat(64)
  const expectedReceipt = { status: '0x1', transactionHash: expectedHash }
  const urls = []
  let walletProviderCalls = 0
  globalThis.window = { __JC_ACTIVE_RPC_URL: 'https://runtime-rpc.example' }
  web3Service.provider = { getTransactionReceipt: async () => { walletProviderCalls += 1; return expectedReceipt } }
  try {
    const result = await contractService._waitForReceipt(expectedHash, {
      chainId: 56,
      timeoutMs: 100,
      queryTimeoutMs: 5,
      fetcher: (url) => {
        urls.push(url)
        if (url.includes('bnbchain.org')) return new Promise(() => {})
        return Promise.resolve({ ok: true, json: async () => ({ result: null }) })
      },
    })
    assert.equal(result, expectedReceipt)
    assert.deepEqual(urls, ['https://bsc-dataseed.bnbchain.org', 'https://runtime-rpc.example'])
    assert.equal(walletProviderCalls, 1)
  } finally {
    if (originalWindow === undefined) delete globalThis.window
    else globalThis.window = originalWindow
    web3Service.provider = originalProvider
  }
})

test('receipt polling fails closed after every read-only path remains unavailable', async () => {
  const originalWindow = globalThis.window
  const originalProvider = web3Service.provider
  const expectedHash = '0x' + '9'.repeat(64)
  globalThis.window = { __JC_ACTIVE_RPC_URL: 'https://runtime-rpc.example' }
  web3Service.provider = { getTransactionReceipt: () => new Promise(() => {}) }
  try {
    await assert.rejects(
      () => contractService._waitForReceipt(expectedHash, {
        chainId: 56,
        timeoutMs: 15,
        queryTimeoutMs: 3,
        pollIntervalMs: 1,
        fetcher: () => new Promise(() => {}),
      }),
      (error) => error?.code === WALLET_ERROR_CODES.TRANSACTION_RECEIPT_TIMEOUT && error?.transactionHash === expectedHash,
    )
  } finally {
    if (originalWindow === undefined) delete globalThis.window
    else globalThis.window = originalWindow
    web3Service.provider = originalProvider
  }
})

test('an invalid raw-send response never triggers a second transaction submission', async () => {
  const originalRawSend = web3Service.sendRawTransaction
  const originalProvider = web3Service.provider
  const originalSigner = web3Service.signer
  let signerSendCount = 0
  web3Service.sendRawTransaction = async () => '0x7a022eca510aab629e5a23cc4d279d257dbc89d2f9'
  web3Service.provider = {}
  web3Service.signer = {
    sendTransaction: async () => {
      signerSendCount += 1
      return { hash: '0x' + '1'.repeat(64) }
    },
  }
  try {
    await assert.rejects(
      () => contractService._dualChannelSend(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        200000,
      ),
      (error) => error.code === WALLET_ERROR_CODES.TRANSACTION_BROADCAST_UNCERTAIN,
    )
    assert.equal(signerSendCount, 0)
  } finally {
    web3Service.sendRawTransaction = originalRawSend
    web3Service.provider = originalProvider
    web3Service.signer = originalSigner
  }
})

test('explicit unsupported raw-send method safely falls back to the signer once', async () => {
  const originalRawSend = web3Service.sendRawTransaction
  const originalProvider = web3Service.provider
  const originalSigner = web3Service.signer
  const expectedHash = '0x' + 'a'.repeat(64)
  let signerSendCount = 0
  web3Service.sendRawTransaction = async () => {
    const error = new Error('Method not found')
    error.code = -32601
    throw error
  }
  web3Service.provider = {}
  web3Service.signer = {
    sendTransaction: async () => {
      signerSendCount += 1
      return { hash: expectedHash }
    },
  }
  try {
    assert.equal(
      await contractService._dualChannelSend(
        '0x1111111111111111111111111111111111111111',
        '0x1234',
        '0x2222222222222222222222222222222222222222',
        200000,
      ),
      expectedHash,
    )
    assert.equal(signerSendCount, 1)
  } finally {
    web3Service.sendRawTransaction = originalRawSend
    web3Service.provider = originalProvider
    web3Service.signer = originalSigner
  }
})

test('wallet confirmation may remain pending without a client timeout or duplicate fallback send', async () => {
  const originalRawSend = web3Service.sendRawTransaction
  const originalProvider = web3Service.provider
  const originalSigner = web3Service.signer
  const expectedHash = '0x' + 'b'.repeat(64)
  let resolveWallet
  let rawSendCount = 0
  let signerSendCount = 0
  web3Service.sendRawTransaction = () => {
    rawSendCount += 1
    return new Promise((resolve) => { resolveWallet = resolve })
  }
  web3Service.provider = {}
  web3Service.signer = {
    sendTransaction: async () => {
      signerSendCount += 1
      return { hash: '0x' + 'c'.repeat(64) }
    },
  }
  try {
    const submission = contractService._dualChannelSend(
      '0x1111111111111111111111111111111111111111',
      '0x1234',
      '0x2222222222222222222222222222222222222222',
      200000,
    )
    await new Promise((resolve) => setTimeout(resolve, 25))
    assert.equal(rawSendCount, 1)
    assert.equal(signerSendCount, 0)
    resolveWallet(expectedHash)
    assert.equal(await submission, expectedHash)
    assert.equal(rawSendCount, 1)
    assert.equal(signerSendCount, 0)
  } finally {
    web3Service.sendRawTransaction = originalRawSend
    web3Service.provider = originalProvider
    web3Service.signer = originalSigner
  }
})

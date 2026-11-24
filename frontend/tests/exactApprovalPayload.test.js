import test from 'node:test'
import assert from 'node:assert/strict'
import { Interface } from 'ethers'
import { ContractService } from '../src/services/contract.js'
import { web3Service, Web3Service } from '../src/services/web3.js'

const spender = '0x1111111111111111111111111111111111111111'
const wallet = '0x2222222222222222222222222222222222222222'
const paymentToken = '0x3333333333333333333333333333333333333333'
const projectToken = '0x4444444444444444444444444444444444444444'
const abi = new Interface(['function approve(address spender,uint256 amount)'])
const hash = '0x' + 'a'.repeat(64)

test('V3 approval methods deliver exact nonzero token quantities through the real wallet RPC seam', async () => {
  const original = web3Service.sendRawTransaction
  const walletService = new Web3Service()
  let request
  walletService._getEIP1193Provider = () => ({ request: async (args) => { request = args; return hash } })
  web3Service.sendRawTransaction = walletService.sendRawTransaction.bind(walletService)
  try {
    for (const [method, amount, decimals, expectedToken, expectedRaw] of [
      ['approveV3PaymentToken', '1', 18, paymentToken, 1000000000000000000n],
      ['approveV3PaymentToken', '1', 6, paymentToken, 1000000n],
      ['approveV3PaymentToken', '10', 18, paymentToken, 10000000000000000000n],
      ['approveV3PaymentToken', '10', 6, paymentToken, 10000000n],
      ['approveV3PaymentToken', '0.01', 18, paymentToken, 10000000000000000n],
      ['approveV3ProjectToken', '800000', 18, projectToken, 800000000000000000000000n],
    ]) {
      const service = new ContractService()
      const context = { id: '43359438123302913', chainId: 56, contractAddress: spender, paymentTokenAddress: paymentToken,
        tokenAddress: projectToken, paymentTokenDecimals: decimals, tokenDecimals: decimals }
      service._v3Signer = async () => ({ context, from: wallet })
      service.readV3PurchaseAuthority = async () => ({ state: {}, proof: [] })
      // Submission policy is tested separately. Capture its immutable request
      // at the real sender without any network or human signature.
      service._v3Submit = async (c, to, data, from, action) => {
        assert.equal(to, expectedToken)
        assert.equal(from, wallet)
        assert.equal(action, method === 'approveV3PaymentToken' ? 'approvePaymentToken' : 'approveProjectToken')
        return service._v3WalletSend(to, data, from, 120000n, c.chainId)
      }
      assert.equal(await service[method](context, amount, wallet), hash)
      assert.equal(request.method, 'eth_sendTransaction')
      assert.equal(request.params.length, 1)
      const tx = request.params[0]
      assert.equal(tx.to, expectedToken)
      assert.equal(tx.from, wallet)
      assert.equal(tx.value, '0x0') // Correct: approve never transfers BNB.
      assert.equal(tx.chainId, '0x38')
      assert.equal(tx.data.slice(0, 10), '0x095ea7b3')
      const decoded = abi.decodeFunctionData('approve', tx.data)
      assert.equal(decoded.spender.toLowerCase(), spender)
      assert.equal(decoded.amount, expectedRaw) // Never zero or hidden MAX.
      assert.equal(tx.data.length, 138) // Standard selector + two exact ABI words.
    }
  } finally { web3Service.sendRawTransaction = original }
})

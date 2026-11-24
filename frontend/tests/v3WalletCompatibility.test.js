import test from 'node:test'
import assert from 'node:assert/strict'
import { Interface, parseUnits } from 'ethers'
import { ContractService } from '../src/services/contract.js'
import { web3Service } from '../src/services/web3.js'
import { v3WalletPayload, v3WalletErrorKind } from '../src/services/v3WalletCompatibility.js'
import { recordV3Pending, releaseV3Unbroadcast, listV3Pending, v3PendingMetadata, v3SubmissionIdentity } from '../src/services/idoV3Pending.js'
import { watchWalletResume } from '../src/utils/walletResume.js'

const owner = '0x2222222222222222222222222222222222222222'
const token = '0x3333333333333333333333333333333333333333'
const proxy = '0x1111111111111111111111111111111111111111'
const hash = '0x'+'a'.repeat(64)
const abi = new Interface(['function approve(address spender,uint256 value)'])
const data = abi.encodeFunctionData('approve', [proxy, parseUnits('10',18)])
const project = { id:'43359438123302913', chainId:56, contractAddress:proxy, paymentTokenAddress:token }

test('mobile approval payload is canonical exact 10 USDT, explicit chain, not native transfer amount', () => {
  const payload = v3WalletPayload(token, data, owner, 125000n, 56)
  assert.equal(payload.chainId,'0x38'); assert.equal(payload.value,'0x0')
  assert.equal(abi.decodeFunctionData('approve',payload.data).value,10000000000000000000n)
  for(const wrong of [data+'00',abi.encodeFunctionData('approve',[proxy,0n])]) assert.throws(()=>v3WalletPayload(token,wrong,owner,125000n,56))
})

test('wallet error classification trusts structured codes, never timeout/message or a known broadcast hash', () => {
  assert.equal(v3WalletErrorKind({code:4001}),'rejected')
  assert.equal(v3WalletErrorKind({code:'UNKNOWN_ERROR',info:{error:{code:4001}}}),'rejected')
  assert.equal(v3WalletErrorKind({code:-32601}),'unsupported')
  assert.equal(v3WalletErrorKind({code:4200}),'unsupported')
  for(const e of [{message:'user rejected'}, {message:'method not found'}, {code:-32603},
    {code:4001,transactionHash:hash}, {code:-32601,error:{hash}}]) assert.equal(v3WalletErrorKind(e),'unknown')
})

test('explicit cancel releases only this unbroadcast placeholder; unsupported safely falls back once; unknown stays fenced', async () => {
  const oldWindow=globalThis.window, oldNav=Object.getOwnPropertyDescriptor(globalThis,'navigator')
  const saved={raw:web3Service.sendRawTransaction,signer:web3Service.signer,provider:web3Service.provider,
    get:web3Service._getEIP1193Provider,chain:web3Service._signerChainId}
  const values=new Map(), connector={request:()=>{throw Error('Unexpected request')}}
  globalThis.window={localStorage:{getItem:k=>values.get(k)||null,setItem:(k,v)=>values.set(k,v)}}
  Object.defineProperty(globalThis,'navigator',{configurable:true,value:{locks:{request:async(_,__,fn)=>fn({})}}})
  let rawCalls=0,fallbackCalls=0,rawError,fallbackError
  web3Service.provider={};web3Service._signerChainId=56;web3Service._getEIP1193Provider=()=>connector
  web3Service.signer={getAddress:async()=>owner,sendTransaction:async tx=>{
    fallbackCalls++;assert.equal(tx.data,data);assert.equal(tx.value,0n);assert.equal(tx.chainId,56n)
    assert.equal(tx.gasLimit,125000n);assert.equal(tx.nonce,undefined)
    if(fallbackError)throw fallbackError
    return {hash}
  }}
  web3Service.sendRawTransaction=async()=>{rawCalls++;if(rawError)throw rawError;return hash}
  const s=new ContractService();s._v3InvestorPreflight=async()=>125000n
  s._v3WaitSubmission=async()=>({hash,status:'success'})
  const run=()=>s._v3Submit(project,token,data,owner,'approvePaymentToken')
  try {
    rawError={code:4001};await assert.rejects(run(),{code:'IDO_V3_WALLET_REJECTED'});assert.equal(listV3Pending(project,owner).length,0)
    values.clear();rawError={code:-32601};const r=await run();assert.equal(r.status,'submitted');assert.equal(fallbackCalls,1)
    values.clear();fallbackError={code:4001};await assert.rejects(run(),{code:'IDO_V3_WALLET_REJECTED'});assert.equal(listV3Pending(project,owner).length,0)
    values.clear();rawError={code:-32603};const before=fallbackCalls
    await assert.rejects(run(),{code:'IDO_V3_SUBMISSION_UNRESOLVED'});assert.equal(fallbackCalls,before)
    assert.equal(listV3Pending(project,owner).length,1)
    const sends=rawCalls;await assert.rejects(run(),{code:'IDO_V3_SUBMISSION_UNRESOLVED'});assert.equal(rawCalls,sends)
    const identity=v3SubmissionIdentity(project,owner,'approvePaymentToken')
    assert.equal(releaseV3Unbroadcast(identity,'another-request'),false)
    values.clear();recordV3Pending(identity,'',{...v3PendingMetadata(token,owner,data),requestId:'old'})
    recordV3Pending(identity,hash);assert.equal(releaseV3Unbroadcast(identity,'old'),false)
  } finally {
    web3Service.sendRawTransaction=saved.raw;web3Service.signer=saved.signer;web3Service.provider=saved.provider
    web3Service._getEIP1193Provider=saved.get;web3Service._signerChainId=saved.chain
    globalThis.window=oldWindow;if(oldNav)Object.defineProperty(globalThis,'navigator',oldNav);else delete globalThis.navigator
  }
})

test('slow wallet prompt has no timer or resend; callback later returns original hash', async () => {
  const saved=web3Service.sendRawTransaction
  let resolve,count=0
  web3Service.sendRawTransaction=()=>{count++;return new Promise(r=>resolve=r)}
  try {
    const s=new ContractService();const result=s._v3WalletSend(token,data,owner,125000n,56)
    await new Promise(r=>setTimeout(r,25));assert.equal(count,1)
    resolve(hash);assert.equal(await result,hash);assert.equal(count,1)
  } finally {web3Service.sendRawTransaction=saved}
})

test('changing connector during explicit unsupported response never uses a different signer', async () => {
  const old={raw:web3Service.sendRawTransaction,signer:web3Service.signer,provider:web3Service.provider,get:web3Service._getEIP1193Provider,chain:web3Service._signerChainId}
  let current={},fallback=0
  web3Service._getEIP1193Provider=()=>current;web3Service.provider={};web3Service._signerChainId=56
  web3Service.signer={getAddress:async()=>owner,sendTransaction:async()=>{fallback++;return {hash}}}
  web3Service.sendRawTransaction=async()=>{current={};throw {code:-32601}}
  try {await assert.rejects(new ContractService()._v3WalletSend(token,data,owner,125000n,56),{code:'IDO_V3_WALLET_UNSUPPORTED'});assert.equal(fallback,0)}
  finally {web3Service.sendRawTransaction=old.raw;web3Service.signer=old.signer;web3Service.provider=old.provider;web3Service._getEIP1193Provider=old.get;web3Service._signerChainId=old.chain}
})

test('mobile resume coalesces events and disposes; hidden pages never refresh or send', async () => {
  const doc=new EventTarget(),win=new EventTarget();doc.visibilityState='hidden'
  win.setTimeout=setTimeout;win.clearTimeout=clearTimeout
  let reads=0;const stop=watchWalletResume(()=>{reads++},doc,win)
  win.dispatchEvent(new Event('focus'));await new Promise(r=>setTimeout(r,120));assert.equal(reads,0)
  doc.visibilityState='visible';doc.dispatchEvent(new Event('visibilitychange'));win.dispatchEvent(new Event('pageshow'));win.dispatchEvent(new Event('focus'))
  await new Promise(r=>setTimeout(r,130));assert.equal(reads,1)
  stop();win.dispatchEvent(new Event('focus'));await new Promise(r=>setTimeout(r,120));assert.equal(reads,1)
})

test('purchase can follow canonical mined approval without deleting its finality fence; other/unknown records block', async () => {
  const old=globalThis.window,values=new Map()
  globalThis.window={localStorage:{getItem:k=>values.get(k)||null,setItem:(k,v)=>values.set(k,v)}}
  const s=new ContractService(),blockHash='0x'+'b'.repeat(64)
  let canonical=blockHash,status=1
  const entry=v3PendingMetadata(token,owner,data),identity=v3SubmissionIdentity(project,owner,'approvePaymentToken')
  s._v3ReadRunner=()=>({getNetwork:async()=>({chainId:56n}),getTransactionReceipt:async()=>({hash,status,blockNumber:10,blockHash,to:token,from:owner}),
    getTransaction:async()=>({hash,chainId:56n,value:0n,data,blockHash,to:token,from:owner}),getBlock:async()=>({hash:canonical})})
  try {
    recordV3Pending(identity,'',entry)
    assert.equal(await s._v3PendingAllowsPurchase(project,owner,'purchase(uint256,uint256)'),false)
    recordV3Pending(identity,hash)
    assert.equal(await s._v3PendingAllowsPurchase(project,owner,'purchase(uint256,uint256)'),true)
    assert.equal(listV3Pending(project,owner).length,1)
    assert.equal(await s._v3PendingAllowsPurchase(project,owner,'approvePaymentToken'),false)
    canonical='0x'+'c'.repeat(64)
    assert.equal(await s._v3PendingAllowsPurchase(project,owner,'purchase(uint256,uint256)'),false)
    canonical=blockHash;status=0
    assert.equal(await s._v3PendingAllowsPurchase(project,owner,'purchase(uint256,uint256)'),false)
  }finally{globalThis.window=old}
})

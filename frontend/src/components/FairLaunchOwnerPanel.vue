<script setup>
import { computed, ref } from 'vue'
import { contractService } from '../services/contract.js'
import { isUnresolvedWalletTransactionError } from '../services/fairLaunchWallet.js'
import { fairLaunchActionAvailable, formatDecimalString, formatRawUnits, trustedFairLaunchRuntime } from '../utils/fairLaunch.js'

const props = defineProps({ project:{type:Object,required:true}, language:{type:String,default:'en'} })
const emit = defineEmits(['refresh'])
const busy=ref(''), message=ref(''), txHash=ref('')
const zh=computed(()=>props.language!=='en')
const t=computed(()=>zh.value?{
  title:'Fair Launch 項目操作',sale:'精確銷售量',deposited:'已充值',remaining:'尚需充值',raised:'已募集',
  pending:'已結算，等待 Release；提款仍關閉',approve:'精確授權剩餘項目 Token',deposit:'充值剩餘項目 Token',
  failed:'取回失敗項目 Token',busy:'處理中',off:'受信執行身份未就緒',safe:'項目收益與平台費只能由各自固定 Safe 發起並收回。項目方 EOA 不提供提款按鈕。',
  submitted:'交易已提交；等待 canonical event、ledger 與 reconciliation，不在本頁提前記作財務完成。',unresolved:'交易狀態未決，禁止重送；請使用交易雜湊或錢包記錄查證。'
}:{
  title:'Fair Launch project actions',sale:'Exact sale amount',deposited:'Deposited',remaining:'Remaining deposit',raised:'Raised',
  pending:'Settled, awaiting Release; payouts remain closed',approve:'Approve exact remaining project tokens',deposit:'Deposit remaining project tokens',
  failed:'Withdraw failed-project tokens',busy:'Pending',off:'Trusted runtime identity unavailable',safe:'Project proceeds and platform fees can only be initiated and received by their fixed Safes. No misleading project-owner EOA withdrawal button is provided.',
  submitted:'Transaction submitted. Financial completion waits for the canonical event, ledger application, and reconciliation.',unresolved:'Transaction status is unresolved. Do not resend; verify the transaction hash or wallet history.'
})
const runtime=computed(()=>trustedFairLaunchRuntime(props.project))
const gate=(a)=>fairLaunchActionAvailable(props.project,a).enabled
const fenced=(action,signature)=>contractService.hasFairLaunchUnresolved(props.project,action,signature,action==='depositProjectTokens'?[BigInt(props.project.id??props.project.projectId),BigInt(remainingRaw.value||0)]:[BigInt(props.project.id??props.project.projectId)])
const saleRaw=computed(()=>String(props.project.saleTokenAmountRaw??''))
const depositedRaw=computed(()=>String(props.project.totalTokensDepositedRaw??props.project.depositedRaw??'0'))
const remainingRaw=computed(()=>{try{const n=BigInt(saleRaw.value)-BigInt(depositedRaw.value);return n>0n?n.toString():'0'}catch{return ''}})
const tokenDecimals=computed(()=>Number(props.project.tokenDecimals??0))

async function run(name,fn){
  busy.value=name;message.value='';txHash.value=''
  try{
    const r=await fn()
    txHash.value=r?.hash||r?.transactions?.at(-1)?.hash||''
    message.value=t.value.submitted
    emit('refresh')
  }catch(e){
    if(isUnresolvedWalletTransactionError(e)){
      txHash.value=e?.transactionHash||''
      message.value=t.value.unresolved
      return
    }
    message.value=e?.message||'Failed'
  }finally{busy.value=''}
}
</script>
<template>
  <section class="owner-fair" aria-labelledby="owner-fair-title">
    <h4 id="owner-fair-title">{{t.title}}</h4>
    <p v-if="!runtime.enabled" class="warn">{{t.off}}</p>
    <p v-if="project.status==='SuccessfulPendingRelease'" class="warn">{{t.pending}}</p>
    <p class="safe-guidance">{{t.safe}}</p>
    <dl>
      <div><dt>{{t.sale}}</dt><dd>{{formatDecimalString(project.saleTokenAmount)}} {{project.tokenSymbol}}</dd></div>
      <div><dt>{{t.deposited}}</dt><dd>{{formatRawUnits(depositedRaw,tokenDecimals)}} {{project.tokenSymbol}}</dd></div>
      <div><dt>{{t.remaining}}</dt><dd>{{formatRawUnits(remainingRaw,tokenDecimals)}} {{project.tokenSymbol}}</dd></div>
      <div><dt>{{t.raised}}</dt><dd>{{formatDecimalString(project.totalRaised)}} {{project.paymentToken}}</dd></div>
    </dl>
    <div class="actions">
      <button v-if="gate('depositProjectTokens')&&remainingRaw!=='0'" :disabled="busy || fenced('depositProjectTokens','depositProjectTokens(uint256,uint256)')" @click="run('approve',()=>contractService.approveFairLaunchProjectToken(project))">{{busy==='approve'?t.busy:t.approve}}</button>
      <button v-if="gate('depositProjectTokens')&&remainingRaw!=='0'" :disabled="busy || fenced('depositProjectTokens','depositProjectTokens(uint256,uint256)')" class="primary" @click="run('deposit',()=>contractService.depositFairLaunchProjectTokens(project))">{{busy==='deposit'?t.busy:t.deposit}}</button>
      <button v-if="gate('withdrawFailedProjectTokens')" :disabled="busy || fenced('withdrawFailedProjectTokens','withdrawFailedProjectTokens(uint256)')" class="danger" @click="run('failed',()=>contractService.withdrawFairLaunchFailedProjectTokens(project))">{{busy==='failed'?t.busy:t.failed}}</button>
    </div>
    <p v-if="message" role="status" aria-live="polite" class="status">{{message}} <a v-if="txHash" :href="contractService.getTxUrl(txHash)" target="_blank" rel="noopener">{{txHash.slice(0,12)}}…</a></p>
  </section>
</template>
<style scoped>.owner-fair{border:1px solid #dfe5d7;border-radius:10px;padding:14px;background:rgba(142,240,0,.04)}h4{font-weight:800}.warn,.status,.safe-guidance{margin-top:8px;border-radius:7px;background:#fff7e6;padding:8px;color:#754500;font-size:11px}.safe-guidance{background:#eef6ff;color:#174a73}dl{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin:10px 0}dl div{border:1px solid #dfe5d7;border-radius:7px;padding:8px}dt{font-size:10px;color:#697064}dd{margin:3px 0 0;font-weight:750}.actions{display:flex;flex-wrap:wrap;gap:8px}.actions button{min-height:44px;border:1px solid #b9c2b2;border-radius:8px;padding:8px 12px;font-weight:750}.actions .primary{background:#8ef000;color:#172000}.actions .danger{background:#fff1f0;color:#a8071a}.actions button:disabled{opacity:.45}.actions button:focus-visible{outline:3px solid #1677ff;outline-offset:2px}.status a{color:inherit;text-decoration:underline}@media(max-width:600px){dl{grid-template-columns:1fr}}</style>

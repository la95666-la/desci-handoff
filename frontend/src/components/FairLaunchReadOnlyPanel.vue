<script setup>
import { computed, ref, watch } from 'vue'
import { useWalletStore } from '../stores/wallet'
import { contractService } from '../services/contract.js'
import { getFairLaunchPosition, hasFairLaunchLocalApiAuthority } from '../services/api.js'
import { canBootstrapFairLaunchPosition, mergeFairLaunchPositionIdentity } from '../services/fairLaunchApiContract.js'
import { isUnresolvedWalletTransactionError } from '../services/fairLaunchWallet.js'
import {
  clampPercentForBar,
  decimalRatioPercent,
  decimalToScaledInteger,
  fairLaunchActionAvailable,
  fairLaunchSettlementState,
  formatCanonicalSaleTokenAmount,
  formatDecimalString,
  formatRawUnits,
  normalizeFinancialState,
  trustedFairLaunchRuntime,
} from '../utils/fairLaunch.js'

const props = defineProps({ project: { type: Object, required: true }, language: { type: String, default: 'en' }, isDark: { type: Boolean, default: false } })
const wallet = useWalletStore()
const amount = ref('')
const position = ref(null)
const allowanceRaw = ref(0n)
const balanceRaw = ref(0n)
const busy = ref('')
const notice = ref('')
const lastHash = ref('')

const isZh = computed(() => props.language && props.language !== 'en')
const text = computed(() => isZh.value ? {
  title:'FAIR LAUNCH', raised:'已募集', softCap:'軟頂', hardCap:'硬頂', noHardCap:'無硬頂', saleAmount:'固定銷售代幣總量', progress:'募集進度', contribution:'我的貢獻', estimate:'非約束預估分配', finalAllocation:'最終分配', claimed:'已領取', claimable:'可領取', balance:'可用餘額', yourPosition:'我的認購', participate:'參與本輪募集', enter:'輸入貢獻金額', approve:'授權支付代幣', contribute:'立即參與', claim:'領取代幣', refund:'申請退款', connect:'連接錢包', pending:'交易處理中', unknown:'資料不可用', pendingRelease:'本輪已結算，正在等待 Release；領取及項目提款尚未開放。', noPromise:'預估分配按目前募集比例計算，最終分配以鏈上結算結果為準。', zeroFirst:'如已有不同授權額，錢包會先提示歸零，再提示設定本次精確額度。', broadcast:'廣播狀態不確定，禁止重送；請依交易雜湊或錢包記錄查證。', live:'進行中', upcoming:'即將開始', successful:'募集成功', failed:'募集失敗', canceled:'項目已取消', ended:'已結束', insufficient:'可用餘額不足', submitted:'交易已提交，正在等待鏈上確認與資料同步。'
} : {
  title:'FAIR LAUNCH', raised:'Raised', softCap:'Soft cap', hardCap:'Hard cap', noHardCap:'No hard cap', saleAmount:'Fixed token allocation', progress:'Funding progress', contribution:'My contribution', estimate:'Non-binding estimated allocation', finalAllocation:'Final allocation', claimed:'Claimed', claimable:'Claimable', balance:'Available balance', yourPosition:'Your position', participate:'Participate in this round', enter:'Enter contribution amount', approve:'Approve payment token', contribute:'Participate now', claim:'Claim tokens', refund:'Request refund', connect:'Connect wallet', pending:'Transaction pending', unknown:'Unavailable', pendingRelease:'This round is settled, awaiting Release; claim and payouts remain closed.', noPromise:'Estimated allocation follows the current funding ratio. Final allocation is determined by the on-chain settlement.', zeroFirst:'If a different allowance exists, your wallet will first ask to reset it to zero and then set this exact amount.', broadcast:'Broadcast status is uncertain. Do not resend; verify the hash or wallet history.', live:'Live', upcoming:'Coming soon', successful:'Successful', failed:'Failed', canceled:'Canceled', ended:'Ended', insufficient:'Insufficient balance', submitted:'Transaction submitted. Waiting for on-chain confirmation and data synchronization.'
})
const bootstrapEligible = computed(() => canBootstrapFairLaunchPosition(props.project, wallet, {
  localAuthorityRead: hasFairLaunchLocalApiAuthority(),
}))
// Ordinary-detail totals are display-only and never feed a transaction amount.
const raised = computed(() => String(props.project.totalRaised ?? props.project.total_raised ?? '0'))
const softCap = computed(() => String(props.project.softCap ?? props.project.soft_cap ?? '0'))
const percent = computed(() => { try { return decimalRatioPercent(raised.value, softCap.value, 1) } catch { return '0.0' } })
const barPercent = computed(() => clampPercentForBar(percent.value))
const saleAmount = computed(() => formatCanonicalSaleTokenAmount(props.project))
const paymentDecimals = computed(() => effectiveProject.value ? Number(effectiveProject.value.paymentTokenDecimals) : null)
const tokenDecimals = computed(() => effectiveProject.value ? Number(effectiveProject.value.tokenDecimals) : null)
const amountRaw = computed(() => {
  if (paymentDecimals.value === null) return 0n
  try { return decimalToScaledInteger(amount.value, paymentDecimals.value) } catch { return 0n }
})
const contributionState = computed(() => normalizeFinancialState(position.value?.projection?.contribution))
const claimedState = computed(() => normalizeFinancialState(position.value?.projection?.claimed))
const claimableState = computed(() => normalizeFinancialState(position.value?.projection?.claimable))
const estimatedState = computed(() => normalizeFinancialState(position.value?.projection?.estimatedAllocation))
const estimatedRaw = computed(() => BigInt(estimatedState.value.raw ?? 0))
const finalAllocationState = computed(() => normalizeFinancialState(position.value?.projection?.finalAllocation))
const state = computed(() => fairLaunchSettlementState(props.project))
const statusInfo = computed(() => {
  if (state.value === 'active' || state.value === 'soft_cap_reached_live') return { label: text.value.live, tone: 'live' }
  if (state.value === 'upcoming') return { label: text.value.upcoming, tone: 'upcoming' }
  if (state.value === 'released_success' || state.value === 'pending_release' || state.value === 'ended_success_pending_settlement') return { label: text.value.successful, tone: 'successful' }
  if (state.value === 'failed' || state.value === 'ended_failed_pending_settlement' || state.value === 'refund') return { label: text.value.failed, tone: 'failed' }
  if (state.value === 'canceled') return { label: text.value.canceled, tone: 'canceled' }
  return { label: text.value.ended, tone: 'ended' }
})
const effectiveProject = computed(() => {
  if (!wallet.address || !position.value) return null
  try { return mergeFairLaunchPositionIdentity(props.project, position.value, wallet.address) }
  catch { return null }
})
const runtime = computed(() => trustedFairLaunchRuntime(effectiveProject.value))
const actionGate = (name) => effectiveProject.value
  ? fairLaunchActionAvailable(effectiveProject.value, name)
  : { enabled: false, reason: 'trusted_position_not_loaded' }
const isFenced = (action) => {
  const project = effectiveProject.value
  if (!project) return false
  const calls = {
    contribute: ['contribute(uint256,uint256)', [BigInt(project.id ?? project.projectId), amountRaw.value]],
    claim: ['claim(uint256)', [BigInt(project.id ?? project.projectId)]],
    refund: ['refund(uint256)', [BigInt(project.id ?? project.projectId)]],
  }
  const call = calls[action]
  return call ? contractService.hasFairLaunchUnresolved(project, action, call[0], call[1]) : false
}
const canContribute = computed(() => effectiveProject.value && paymentDecimals.value !== null && runtime.value.enabled && actionGate('contribute').enabled && !isFenced('contribute') && amountRaw.value > 0n && amountRaw.value <= balanceRaw.value)
const canApprove = computed(() => effectiveProject.value && paymentDecimals.value !== null && runtime.value.enabled && actionGate('contribute').enabled && !isFenced('contribute') && amountRaw.value > 0n)
const needsApproval = computed(() => amountRaw.value > 0n && allowanceRaw.value < amountRaw.value)
const balanceDisplay = computed(() => paymentDecimals.value === null ? text.value.unknown : formatRawUnits(balanceRaw.value.toString(), paymentDecimals.value))
const canClaim = computed(() => runtime.value.enabled && actionGate('claim').enabled && !isFenced('claim'))
const canRefund = computed(() => runtime.value.enabled && actionGate('refund').enabled && !isFenced('refund'))

async function refresh() {
  position.value = null
  allowanceRaw.value = 0n
  balanceRaw.value = 0n
  if (!bootstrapEligible.value) return
  try {
    // The ordinary project DTO is used only to choose this authenticated read.
    // No token read or transaction is attempted until the strict position DTO
    // has supplied and validated the complete trusted runtime/action identity.
    const p = await getFairLaunchPosition(props.project.id ?? props.project.projectId ?? props.project.projectID, wallet.address)
    const trustedProject = mergeFairLaunchPositionIdentity(props.project, p, wallet.address)
    position.value = p
    if (fairLaunchActionAvailable(trustedProject, 'contribute').enabled) {
      const [allowance, balance] = await Promise.all([
        contractService.getPaymentTokenAllowance(trustedProject, wallet.address),
        contractService.getPaymentTokenBalance(trustedProject, wallet.address),
      ])
      allowanceRaw.value = BigInt(allowance.raw)
      balanceRaw.value = BigInt(balance.raw)
    }
    notice.value = ''
  } catch (error) {
    position.value = null
    notice.value = error?.message || text.value.unknown
  }
}
async function run(name, fn) {
  busy.value = name; notice.value = text.value.pending; lastHash.value = ''
  try { const result = await fn(); lastHash.value = result?.hash || result?.transactions?.at(-1)?.hash || ''; notice.value = `✓ ${text.value.submitted}`; await refresh() }
  catch (error) {
    if (isUnresolvedWalletTransactionError(error)) {
      lastHash.value = error?.transactionHash || ''
      notice.value = text.value.broadcast
      return
    }
    notice.value = error?.message || text.value.unknown
  }
  finally { busy.value = '' }
}
async function connect() { await wallet.connectWallet(); await refresh() }
const walletProject = () => {
  if (!effectiveProject.value || !runtime.value.enabled) throw new Error('Trusted Fair Launch position is unavailable')
  return effectiveProject.value
}
const approve = () => run('approve', () => contractService.approveFairLaunchPaymentToken(walletProject(), amountRaw.value.toString()))
const contribute = () => run('contribute', () => contractService.contributeFairLaunch(walletProject(), amountRaw.value.toString()))
const claim = () => run('claim', () => contractService.claimFairLaunch(walletProject()))
const refund = () => run('refund', () => contractService.refundFairLaunch(walletProject()))
watch(() => [wallet.address, wallet.isConnected, wallet.isLoggedIn, wallet.sessionRevision, props.project], refresh, { immediate: true, deep: false })
</script>

<template>
  <section id="active-funding-panel" class="fair-panel scroll-mt-[110px]" :class="{ 'is-dark': isDark }" aria-labelledby="fair-title">
    <header class="section-heading">
      <span class="section-mark" aria-hidden="true"></span>
      <h2 id="fair-title">{{ text.title }}</h2>
      <span class="status-badge" :class="`status-${statusInfo.tone}`">
        <span v-if="statusInfo.tone === 'live'" class="status-dot"></span>{{ statusInfo.label }}
      </span>
    </header>

    <article class="funding-card">
      <p v-if="state === 'pending_release'" class="release" role="status">{{ text.pendingRelease }}</p>

      <div class="raised-row">
        <div>
          <span class="eyebrow">{{ text.raised }}</span>
          <strong>{{ formatDecimalString(raised) }} <small>{{ project.paymentToken }}</small></strong>
        </div>
        <b>{{ percent }}%</b>
      </div>
      <div class="progress-track" role="progressbar" :aria-label="text.progress" :aria-valuenow="barPercent" aria-valuemin="0" aria-valuemax="100">
        <i :style="{ width: `${barPercent}%` }"></i>
      </div>
      <p class="progress-caption">{{ text.progress }} · {{ formatDecimalString(raised) }} / {{ formatDecimalString(softCap) }} {{ project.paymentToken }}</p>

      <div class="terms-list" aria-label="Funding terms">
        <div class="term-row"><span>{{ text.softCap }}</span><strong>{{ formatDecimalString(softCap) }} {{ project.paymentToken }}</strong></div>
        <div class="term-row"><span>{{ text.hardCap }}</span><strong>{{ text.noHardCap }}</strong></div>
        <div class="term-row"><span>{{ text.saleAmount }}</span><strong>{{ saleAmount }} {{ project.tokenSymbol }}</strong></div>
      </div>

      <section v-if="wallet.address" class="position-section" aria-labelledby="fair-position-title">
        <h3 id="fair-position-title">{{ text.yourPosition }}</h3>
        <div class="position-list">
          <div class="term-row"><span>{{ text.balance }}</span><strong>{{ balanceDisplay }} {{ project.paymentToken }}</strong></div>
          <div class="term-row"><span>{{ text.contribution }}</span><strong>{{ contributionState.available ? formatRawUnits(contributionState.raw, paymentDecimals) : text.unknown }} {{ project.paymentToken }}</strong></div>
          <div class="term-row"><span>{{ text.estimate }}</span><strong>{{ formatRawUnits(estimatedRaw.toString(), tokenDecimals) }} {{ project.tokenSymbol }}</strong></div>
          <div class="term-row"><span>{{ text.finalAllocation }}</span><strong>{{ finalAllocationState.available ? formatRawUnits(finalAllocationState.raw, tokenDecimals) : text.unknown }} {{ project.tokenSymbol }}</strong></div>
          <div class="term-row"><span>{{ text.claimed }}</span><strong>{{ claimedState.available ? formatRawUnits(claimedState.raw, tokenDecimals) : text.unknown }} {{ project.tokenSymbol }}</strong></div>
          <div class="term-row"><span>{{ text.claimable }}</span><strong>{{ claimableState.available ? formatRawUnits(claimableState.raw, tokenDecimals) : text.unknown }} {{ project.tokenSymbol }}</strong></div>
        </div>
      </section>

      <section v-if="effectiveProject && actionGate('contribute').enabled" class="participation-section" aria-labelledby="fair-participation-title">
        <h3 id="fair-participation-title">{{ text.participate }}</h3>
        <div class="amount-field">
          <input id="fair-contribution" v-model="amount" inputmode="decimal" autocomplete="off" :placeholder="text.enter" :disabled="!runtime.enabled || !!busy" />
          <span>{{ project.paymentToken }}</span>
        </div>
        <p class="helper-text">{{ text.noPromise }}</p>
        <p v-if="needsApproval" class="helper-text">{{ text.zeroFirst }}</p>
        <button v-if="amountRaw > balanceRaw" class="action-button" disabled>{{ text.insufficient }}</button>
        <button v-else-if="needsApproval" class="action-button primary" :disabled="!canApprove || !!busy" @click="approve">{{ busy === 'approve' ? text.pending : text.approve }}</button>
        <button v-else class="action-button primary" :disabled="!canContribute || !!busy" @click="contribute">{{ busy === 'contribute' ? text.pending : text.contribute }}</button>
      </section>

      <div class="terminal-actions">
        <button v-if="canClaim" class="action-button primary" :disabled="!!busy" @click="claim">{{ busy === 'claim' ? text.pending : text.claim }}</button>
        <button v-if="canRefund" class="action-button danger" :disabled="!!busy" @click="refund">{{ busy === 'refund' ? text.pending : text.refund }}</button>
        <button v-if="!wallet.address" class="action-button primary" @click="connect">{{ text.connect }}</button>
      </div>

      <p v-if="notice" class="notice" role="status" aria-live="polite">{{ notice }} <a v-if="lastHash" :href="contractService.getTxUrl(lastHash)" target="_blank" rel="noopener">{{ lastHash.slice(0, 12) }}…</a></p>
    </article>
  </section>
</template>

<style scoped>
.fair-panel {
  --panel-bg: #fdfdfd;
  --subtle-bg: #f6f6f6;
  --highlight-bg: #fafff5;
  --border: #e5e7eb;
  --text: #212121;
  --muted: #808080;
  color: var(--text);
}
.section-heading { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.section-mark { width: 18px; height: 18px; flex: 0 0 auto; transform: rotate(15deg); border-radius: 3px 12px 3px 3px; background: #9bff00; }
.section-heading h2 { font-size: 16px; font-weight: 700; }
.status-badge { display: inline-flex; min-height: 24px; align-items: center; gap: 6px; margin-left: auto; border-radius: 999px; padding: 4px 10px; font-size: 10px; font-weight: 700; letter-spacing: 1px; text-transform: uppercase; }
.status-live { background: #9bff00; color: #212121; }
.status-upcoming { background: #fef3c7; color: #92400e; }
.status-successful { background: #dcfce7; color: #166534; }
.status-failed { background: #fee2e2; color: #991b1b; }
.status-canceled { background: #ffedd5; color: #9a3412; }
.status-ended { background: #e5e7eb; color: #4b5563; }
.status-dot { width: 6px; height: 6px; border-radius: 999px; background: currentColor; animation: fair-status-pulse 2s ease-out infinite; }
.funding-card { border: 1px solid var(--border); border-radius: 10px; background: var(--panel-bg); padding: 24px; }
.release, .notice { margin-bottom: 20px; border: 1px solid #ffd591; border-radius: 10px; background: #fff7e6; padding: 12px 14px; color: #754500; font-size: 12px; line-height: 1.55; }
.raised-row { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; }
.eyebrow { display: block; margin-bottom: 5px; color: var(--muted); font-size: 11px; font-weight: 500; }
.raised-row strong { display: block; font-size: 24px; font-weight: 600; line-height: 1; }
.raised-row strong small { color: var(--muted); font-size: 12px; font-weight: 500; }
.raised-row b { color: #3d5a00; font-size: 13px; font-weight: 600; }
.progress-track { height: 6px; margin-top: 14px; overflow: hidden; border-radius: 999px; background: rgba(5, 11, 0, 0.15); }
.progress-track i { display: block; height: 100%; border-radius: inherit; background: #9bff00; transition: width 0.5s ease; }
.progress-caption { margin-top: 8px; color: var(--muted); font-size: 11px; text-align: right; }
.terms-list { margin-top: 24px; }
.term-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 9px 0; font-size: 13px; }
.term-row + .term-row { border-top: 1px solid var(--border); }
.term-row span { color: var(--muted); }
.term-row strong { max-width: 62%; text-align: right; font-weight: 500; overflow-wrap: anywhere; }
.position-section, .participation-section { margin-top: 22px; border-top: 1px solid var(--border); padding-top: 20px; }
.position-section h3, .participation-section h3 { margin-bottom: 10px; font-size: 13px; font-weight: 600; }
.position-list { border-radius: 10px; background: var(--subtle-bg); padding: 5px 12px; }
.amount-field { position: relative; }
.amount-field input { box-sizing: border-box; width: 100%; height: 48px; border: 1px solid var(--border); border-radius: 10px; background: var(--panel-bg); padding: 0 76px 0 14px; color: var(--text); font-size: 14px; outline: none; transition: border-color 0.2s ease; }
.amount-field input::placeholder { color: var(--muted); }
.amount-field input:focus { border-color: #050b00; }
.amount-field input:disabled { cursor: not-allowed; opacity: 0.5; }
.amount-field span { position: absolute; top: 50%; right: 14px; transform: translateY(-50%); color: var(--muted); font-size: 12px; font-weight: 600; }
.helper-text { margin-top: 10px; color: var(--muted); font-size: 10px; line-height: 1.55; }
.action-button { width: 100%; min-height: 48px; margin-top: 14px; border-radius: 999px; padding: 10px 20px; font-size: 14px; font-weight: 600; transition: background 0.2s ease, border-color 0.2s ease; }
.action-button.primary { background: #9bff00; color: #050b00; }
.action-button.primary:hover:not(:disabled) { background: #8bed00; }
.action-button.danger { border: 1px solid #fecaca; background: #fff; color: #b91c1c; }
.action-button.danger:hover:not(:disabled) { border-color: #ef4444; }
.action-button:disabled { cursor: not-allowed; background: var(--subtle-bg); color: var(--muted); opacity: 1; }
.terminal-actions:empty { display: none; }
.notice { margin-top: 16px; margin-bottom: 0; overflow-wrap: anywhere; }
.notice a { color: inherit; text-decoration: underline; }
.action-button:focus-visible, .amount-field input:focus-visible { outline: 3px solid #1677ff; outline-offset: 2px; }
.fair-panel.is-dark {
  --panel-bg: #1f1f1f;
  --subtle-bg: #262626;
  --highlight-bg: rgba(155, 255, 0, 0.1);
  --border: rgba(255, 255, 255, 0.09);
  --text: #fafaf7;
  --muted: #a5a79f;
}
.fair-panel.is-dark .funding-card {
  background:
    radial-gradient(135% 95% at 100% 0%, rgba(155, 255, 0, 0.11) 0%, transparent 50%),
    var(--panel-bg) !important;
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.28);
}
.fair-panel.is-dark .position-list { background: var(--subtle-bg) !important; }
.fair-panel.is-dark .amount-field input { background: var(--subtle-bg) !important; }
.fair-panel.is-dark .progress-track { background: rgba(255, 255, 255, 0.15); }
.fair-panel.is-dark .raised-row b { color: #bef264; }
.fair-panel.is-dark .amount-field input:focus { border-color: #9ca3af; }
.fair-panel.is-dark .action-button.danger { background: transparent; }
@keyframes fair-status-pulse { 0% { box-shadow: 0 0 0 0 rgba(33, 33, 33, 0.35); } 70%, 100% { box-shadow: 0 0 0 5px rgba(33, 33, 33, 0); } }
@media (prefers-reduced-motion: reduce) { .status-dot { animation: none; } }
@media (max-width: 600px) { .funding-card { padding: 20px; } .raised-row strong { font-size: 21px; } .term-row { font-size: 12px; } }
</style>

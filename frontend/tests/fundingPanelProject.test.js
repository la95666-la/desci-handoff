import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { toFundingPanelProject } from '../src/utils/fundingPanelProject.js'

const projectID = '41065961574367233'

test('public detail supplies the capped presale panel with a precise ID and raised amount', () => {
  const detail = { totalRaised: '1000', hardCap: '1000', status: 'Successful' }
  const panel = toFundingPanelProject(detail, projectID)
  assert.equal(panel.id, projectID)
  assert.equal(panel.raised, '1000')
  assert.equal(Number(panel.raised) / Number(panel.hardCap) * 100, 100)
  assert.equal(panel.status, 'Successful')
  assert.equal(detail.id, undefined)
})

test('initial mapped data and an explicitly reported zero remain valid', () => {
  assert.equal(toFundingPanelProject({ raised: '250' }, projectID).raised, '250')
  assert.equal(toFundingPanelProject({ totalRaised: '0', raised: '250' }, projectID).raised, '0')
  assert.equal(toFundingPanelProject(null, projectID).raised, '0')
})

test('V3 shared panel retains backend Unix-second sale boundaries and string ID', () => {
  const detail = { contractVersion: 'ido_v3_stablecoin', fundingMode: 'capped_presale', startTime: 1_800_000_001, endTime: 1_800_086_401, totalRaised: '0' }
  const panel = toFundingPanelProject(detail, projectID)
  assert.equal(panel.id, projectID)
  assert.equal(panel.startTime, 1_800_000_001)
  assert.equal(panel.endTime, 1_800_086_401)
  assert.equal(panel.raised, '0')
  assert.equal(toFundingPanelProject({ start_time: 10, end_time: 20 }, projectID).endTime, 20)
})

test('unified capped-presale detail passes normalized data to panel', () => {
  const source = readFileSync(new URL('../src/views/LaunchpadDetailPreLaunch.vue', import.meta.url), 'utf8')
  assert.match(source, /toFundingPanelProject\(detail\.value \|\| props\.project, projectID\.value\)/)
  assert.match(source, /<LaunchpadActiveFundingPanel[\s\S]*?:project="fundingPanelProject"/)
})

test('receipt success starts independent chain reads before success modal, without waiting for backend indexing', () => {
  const source = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const purchase = source.slice(source.indexOf('async function handlePurchase()'), source.indexOf('async function handleRefund()'))
  assert.match(purchase, /const result = isV3\.value[\s\S]*?purchaseV3\([\s\S]*?: await contractService\.purchase\(/)
  assert.match(purchase, /const projectId = String\(props\.project\.id\)/)
  assert.match(purchase, /readConfirmedChain\(\)[\s\S]*?type: 'success'/)
  assert.match(purchase, /contractService\.getProject\(isV3\.value \? v3Context\.value : project\)/)
  assert.match(purchase, /contractService\.getUserPurchase\(isV3\.value \? v3Context\.value : project, address\)/)
  assert.match(purchase, /position\.paymentAmountRaw < expectedUserRaw/)
  assert.match(purchase, /raised\.value = nextRaisedFloor/)
  assert.match(purchase, /paymentAmountRaw: expectedUserRaw/)
  assert.match(purchase, /confirmedUserFloor = expectedUserRaw/)
  assert.match(purchase, /setTimeout\(\(\) => \{ if \(stillCurrent\(\)\)/)
  assert.doesNotMatch(purchase, /await (?:readConfirmedChain|loadOnChainData|refreshRaised)/)
  assert.doesNotMatch(purchase, /wallet\.refreshBalances/)
})

test('confirmed approval immediately opens purchase without waiting for a wallet-provider allowance read', () => {
  const source = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  const approval = source.slice(source.indexOf('async function handleApprove()'), source.indexOf('async function handlePurchase()'))
  assert.match(approval, /const approvedRaw = contractService\.parsePaymentAmount/)
  assert.match(approval, /paymentAllowance\.value = approvedRaw/)
  assert.match(approval, /confirmedAllowanceFloor = approvedRaw/)
  assert.match(approval, /getPaymentTokenAllowance\(isV3\.value \? v3Context\.value : props\.project, wallet\.address\)/)
  assert.doesNotMatch(approval, /await contractService\.getPaymentTokenAllowance/)
})

test('panel guards confirmed chain values from stale backend data and discards late responses for other projects or wallets', () => {
  const source = readFileSync(new URL('../src/components/LaunchpadActiveFundingPanel.vue', import.meta.url), 'utf8')
  assert.match(source, /nextPurchase\.paymentAmountRaw < confirmedUserFloor/)
  assert.match(source, /nextRaised < confirmedRaisedFloor/)
  assert.match(source, /nextAllowance < confirmedAllowanceFloor/)
  assert.match(source, /sequence !== fundingReadSequence/)
  assert.match(source, /sequence !== raisedReadSequence/)
  assert.match(source, /projectId !== String\(props\.project\?\.id \?\? ''\)/)
  assert.match(source, /address\?\.toLowerCase\(\) !== wallet\.address\?\.toLowerCase\(\)/)
})

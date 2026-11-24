/**
 * AppKit 可選網絡列表（錢包彈窗裡能切到的鏈）。
 *
 * 純函數，node --test 直接可測。規則與 networkPolicy.createLoginNetworkPolicy 的
 * isMainnetBuild 判定保持同一套顯式受信配置：
 *   - 主網構建：默認只暴露 56；雙鏈發布形態（顯式聲明了 trustedProjectChains 含 97）
 *     額外暴露 97（Launchpad 寫交易按需切換），默認網絡仍是 56（列表首位）
 *   - 本地 31337 構建：31337 排首位，絕不把默認網絡設成主網
 *   - 測試構建：97 排首位
 */
export function orderAppKitNetworks({
  isMainnetBuild,
  checkInChainId,
  bsc,
  bscTestnet,
  hardhat,
  trustedProjectChains,
}) {
  if (isMainnetBuild) {
    const extra = [...new Set(
      (Array.isArray(trustedProjectChains) ? trustedProjectChains : [])
        .map(Number)
        .filter(Number.isFinite)
    )]
    // 目前唯一支持的項目鏈是 BSC testnet(97)；主網(56)已在列表中
    if (extra.includes(97)) return [bsc, bscTestnet]
    return [bsc]
  }
  if (Number(checkInChainId) === 31337 && hardhat) {
    return [hardhat, bscTestnet, bsc]
  }
  return hardhat ? [bscTestnet, bsc, hardhat] : [bscTestnet, bsc]
}

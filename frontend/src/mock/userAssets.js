// Mock user assets
// TODO: 後端 API 接入後替換為真實數據
export const mockUserAssets = {
  claimable: [
    {
      id: 1011,
      projectId: 1011,
      projectName: 'P2X1 Monoclonal Antibody',
      projectNameCn: 'P2X1單克隆抗體',
      tokenSymbol: 'P2X1',
      totalTokens: 1000,       // 用戶購買的總代幣 (200 JCC / 0.2 = 1000 P2X1)
      totalClaimed: 0,
      claimableAmount: 500,    // 當前可領取 (TGE 50% = 500)
      nextUnlockAmount: 300,   // 下次解鎖數量
      nextUnlockTime: 1772013129000, // 17:52:09
      logo: '/brand/joincare-avatar-green-black.png'
    }
  ],
  investments: [],
  transactions: [],
  purchases: [],
  claims: [],
  refunds: [],
  refundable: [],

  // ── 項目方數據（B端） ──
  // status 字段將由前端從鏈上查詢覆蓋
  myProjects: [
    {
      projectId: 1008,
      projectName: 'P2X1 Monoclonal Antibody',
      projectNameCn: 'P2X1單克隆抗體',
      tokenSymbol: 'P2X1',
      projectToken: '0xbCA8400D0e76E417746A0fd0bBf6cdedfcD42153',
      status: 'loading',
      totalRaised: '0',
      hardCap: '1000',
      softCap: '500',
      tokensSold: '0',
      deposited: '0',
      fundsWithdrawn: false,
      tokensWithdrawn: false,
      logo: '/brand/joincare-avatar-green-black.png'
    },
    {
      projectId: 1011,
      projectName: 'P2X1 Monoclonal Antibody',
      projectNameCn: 'P2X1單克隆抗體',
      tokenSymbol: 'P2X1',
      projectToken: '0xbCA8400D0e76E417746A0fd0bBf6cdedfcD42153',
      status: 'loading',
      totalRaised: '0',
      hardCap: '1000',
      softCap: '500',
      tokensSold: '0',
      deposited: '0',
      fundsWithdrawn: false,
      tokensWithdrawn: false,
      logo: '/brand/joincare-avatar-green-black.png'
    }
  ]
}

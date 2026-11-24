// TODO: 後端 API 接入後此文件將廢棄，所有項目數據由後端 API 提供
// 接口預留: GET /api/projects → 項目列表
//          GET /api/projects/{projectId} → 項目詳情
export const mockProjects = [
  {
    id: 'liang-patent-commercialization',
    name: 'Academician Liang Jing Team Patent Commercialization Project',
    nameCn: '梁京院士團隊專利成果轉化項目',
    logo: '/projects/liang-patent-logo.png',
    category: 'Patent Commercialization',
    categoryCn: '專利成果轉化',
    shortDescription: 'A seed-stage translational project built around patented biomedical research achievements from Academician Liang Jing’s team.',
    shortDescriptionCn: '圍繞梁京院士團隊生物醫藥專利成果的種子期轉化項目，聚焦專利成果孵化與產業合作。',
    overview: '<p>This candidate project is positioned around the commercialization of patented scientific achievements from Academician Liang Jing’s team. The project focuses on turning laboratory intellectual property into a translational biomedical program that can support follow-on validation, industrial collaboration, and future incubation.</p><p>The current mock profile is prepared for early-stage project presentation. Release schedule, target raise, token terms, and funding timeline are intentionally left pending until the candidate project materials are finalized.</p>',
    overviewCn: '<p>該候選項目圍繞梁京院士團隊的專利科研成果轉化展開，重點推動實驗室知識產權進入生物醫藥轉化場景，支持後續驗證、產業合作與項目孵化。</p><p>目前資料作為早期候選項目展示使用，釋放時間、募集資金、代幣條款與融資時間表暫時保留為待公布狀態，待項目材料完善後再補充。</p>',
    tags: ['Patent Transfer', 'Translational Medicine', 'Seed Stage'],
    tagsCn: ['專利轉化', '轉化醫學', '種子期'],
    tokenSymbol: '',
    tokenPrice: null,
    hardCap: null,
    softCap: null,
    raised: null,
    maxPerUser: null,
    startTime: null,
    endTime: null,
    vestingTimes: [],
    vestingPercents: [],
    status: 'candidate',
    disableDetail: false,
    logoBg: '#FFFFFF',
    socialLinks: {
      website: '',
      twitter: '',
      telegram: '',
      discord: ''
    }
  },
  {
    id: 'p2x1-functional-antibody',
    name: 'Functional P2X1 Monoclonal Antibody Program',
    nameCn: '新型功能性 P2X1 單抗藥物研發',
    logo: '/projects/p2x1-logo.png',
    category: 'Biologics',
    categoryCn: '生物藥',
    shortDescription: 'A candidate antibody-drug program targeting P2X1 ion-channel biology, using native-structure P2X1 nanodisc antigens to develop functional antibodies including P007/hP007A.',
    shortDescriptionCn: '面向 P2X1 離子通道靶點的新型功能性單抗候選項目，基於天然構象 P2X1 Nanodisc 抗原開發 P007/hP007A 等功能性抗體。',
    overview: '<p>The program focuses on P2X1, an ATP-gated calcium ion channel and a difficult membrane-protein target. Source materials describe preparation of native-structure hP2X1 nanodisc antigens, functional antibody screening, P007 humanization into hP007A, and validation across neutrophil chemotaxis, sepsis, thrombosis, oncology, and anti-aging research directions.</p><p>The disclosed patent portfolio includes functional P2X1 monoclonal antibody preparation and application, anti-platelet aggregation applications, PCT filings, and P2X1 flow-cytometry antibody development. Funding amount, release schedule, and token terms remain pending for the mock stage.</p>',
    overviewCn: '<p>該項目聚焦 ATP 門控鈣離子通道 P2X1，屬於研發難度較高的膜蛋白靶點。資料顯示，團隊已完成天然構象 hP2X1 Nanodisc 抗原制備、功能性抗體篩選、P007 人源化形成 hP007A，並在中性粒細胞趨化、膿毒症、血栓、腫瘤及抗衰老等方向進行功能驗證。</p><p>公開專利組合涵蓋功能性 P2X1 單抗制備及應用、抗血小板凝集應用、PCT 申請及 P2X1 流式檢測抗體研發。現階段作為 mock 展示，募集資金、釋放計劃與代幣條款暫時保留為待公布。</p>',
    tags: ['P2X1', 'Monoclonal Antibody', 'Ion Channel', 'Oncology'],
    tagsCn: ['P2X1', '單克隆抗體', '離子通道', '腫瘤'],
    tokenSymbol: '',
    tokenPrice: null,
    hardCap: null,
    softCap: null,
    raised: null,
    maxPerUser: null,
    startTime: null,
    endTime: null,
    vestingTimes: [],
    vestingPercents: [],
    status: 'candidate',
    disableDetail: false,
    logoBg: '#FFFFFF',
    socialLinks: {
      website: '',
      twitter: '',
      telegram: '',
      discord: ''
    }
  }
]

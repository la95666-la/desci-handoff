// 靜態新聞數據（前端固定內容，無需後端 API）
export const mockNews = [
  {
    id: 1,
    category: 'Press Release',
    categoryCn: '新聞發布',
    title: 'JoinCare Foundation Launches with $4M Seed Funding',
    titleCn: 'JoinCare基金會啟動，獲得400萬美元種子輪融資',
    excerpt: 'Revolutionary DeSci platform secures initial funding to democratize biomedical research and accelerate drug discovery through blockchain technology.',
    excerptCn: '革命性的DeSci平臺獲得初始資金，通過區塊鏈技術民主化生物醫學研究并加速藥物發現。',
    content: `
      <p>San Francisco, CA - January 15, 2025 - JoinCare Foundation, a pioneering decentralized science (DeSci) platform, today announced the successful closure of its $4 million seed funding round. The investment will accelerate the development of blockchain-based infrastructure for biomedical research funding and intellectual property management.</p>
      
      <h3>Democratizing Medical Research</h3>
      <p>JoinCare is building a revolutionary platform that enables global communities to directly fund and own shares in breakthrough medical research. By leveraging blockchain technology, the platform creates transparent, efficient pathways for research funding while giving contributors real ownership through tokenized IP-NFTs.</p>
      
      <h3>Key Investors</h3>
      <p>The seed round was led by prominent Web3 and biotech-focused venture capital firms, including several strategic partners from the pharmaceutical industry who recognize the transformative potential of decentralized science.</p>
      
      <h3>Platform Features</h3>
      <ul>
        <li>Transparent milestone-based funding distribution</li>
        <li>IP-NFT tokenization of research outcomes</li>
        <li>Community governance for project selection</li>
        <li>Real-time progress tracking on-chain</li>
      </ul>
      
      <h3>Looking Ahead</h3>
      <p>The funding will be used to expand the engineering team, onboard initial research projects, and build partnerships with academic institutions and pharmaceutical companies. JoinCare aims to launch its mainnet platform in Q2 2025.</p>
    `,
    contentCn: `
      <p>舊金山，加州 - 2025年1月15日 - JoinCare基金會，一個開創性的去中心化科學（DeSci）平臺，今天宣布成功完成400萬美元種子輪融資。這筆投資將加速基於區塊鏈的生物醫學研究資金和知識產權管理基礎設施的開發。</p>
      
      <h3>醫學研究民主化</h3>
      <p>JoinCare正在構建一個革命性的平臺，使全球社區能夠直接資助突破性醫學研究并擁有其股份。通過利用區塊鏈技術，該平臺為研究資金創建了透明、高效的途徑，同時通過代幣化的IP-NFT為貢獻者提供真正的所有權。</p>
      
      <h3>主要投資者</h3>
      <p>種子輪由知名的Web3和生物技術風險投資公司領投，包括來自制藥行業的幾個戰略合作伙伴，他們認識到去中心化科學的變革潛力。</p>
      
      <h3>平臺特性</h3>
      <ul>
        <li>透明的基於里程碑的資金分配</li>
        <li>研究成果的IP-NFT代幣化</li>
        <li>項目選擇的社區治理</li>
        <li>鏈上實時進度跟蹤</li>
      </ul>
      
      <h3>展望未來</h3>
      <p>這筆資金將用於擴大工程團隊、引入初始研究項目，以及與學術機構和制藥公司建立合作伙伴關系。JoinCare計劃在2025年第二季度推出其主網平臺。</p>
    `,
    date: '2025-01-15',
    author: 'JoinCare Team',
    authorCn: 'JoinCare團隊',
    source: 'TechCrunch',
    image: 'https://images.unsplash.com/photo-1559757148-5c350d0d3c56?w=1200&h=800&fit=crop',
    tags: ['Funding', 'DeSci', 'Blockchain'],
    tagsCn: ['融資', '去中心化科學', '區塊鏈']
  },
  {
    id: 2,
    category: 'Research',
    categoryCn: '研究進展',
    title: 'P2X1 Antibody Project Reaches Milestone NFT #002',
    titleCn: 'P2X1抗體項目達成里程碑NFT #002',
    excerpt: 'Community-funded oncology research completes CMC report, demonstrating the power of decentralized science funding models.',
    excerptCn: '社區資助的腫瘤學研究完成CMC報告，展示去中心化科學資助模式的力量。',
    content: `
      <p>The P2X1 Monoclonal Antibody project, one of JoinCare's flagship research initiatives, has successfully completed its second major milestone: the Chemistry, Manufacturing, and Controls (CMC) report.</p>
      
      <h3>Milestone Achievement</h3>
      <p>This milestone represents a critical step in the drug development process, documenting the manufacturing process, quality control measures, and stability data for the P2X1 antibody candidate. The complete report has been permanently stored on Arweave and verified on-chain through Milestone NFT #002.</p>
      
      <h3>Community Impact</h3>
      <p>Over 500 community members contributed to funding this milestone, collectively owning a share of the intellectual property through IP-NFTs. This demonstrates the viability of decentralized funding for early-stage biomedical research.</p>
      
      <h3>Next Steps</h3>
      <p>The project is now moving toward IND-enabling studies, with Milestone NFT #003 targeting the completion of preclinical safety studies. Community members can track progress in real-time through the JoinCare platform.</p>
      
      <h3>Scientific Significance</h3>
      <p>P2X1 is a challenging ATP-gated calcium ion channel target with potential applications in both oncology and anti-aging therapeutics. The successful progression of this project validates the DeSci model for funding high-risk, high-reward research.</p>
    `,
    contentCn: `
      <p>P2X1單克隆抗體項目是JoinCare的旗艦研究計劃之一，已成功完成其第二個重要里程碑：化學、制造和控制（CMC）報告。</p>
      
      <h3>里程碑成就</h3>
      <p>這一里程碑代表了藥物開發過程中的關鍵步驟，記錄了P2X1抗體候選藥物的制造過程、質量控制措施和穩定性數據。完整報告已永久存儲在Arweave上，并通過里程碑NFT #002在鏈上驗證。</p>
      
      <h3>社區影響</h3>
      <p>超過500名社區成員為這一里程碑提供了資金，通過IP-NFT共同擁有知識產權的份額。這證明了去中心化資助早期生物醫學研究的可行性。</p>
      
      <h3>下一步</h3>
      <p>該項目現在正朝著IND啟用研究邁進，里程碑NFT #003的目標是完成臨床前安全性研究。社區成員可以通過JoinCare平臺實時跟蹤進度。</p>
      
      <h3>科學意義</h3>
      <p>P2X1是一個具有挑戰性的ATP門控鈣離子通道靶點，在腫瘤學和抗衰老治療方面都有潛在應用。該項目的成功進展驗證了DeSci模式用於資助高風險、高回報研究的有效性。</p>
    `,
    date: '2025-01-10',
    author: 'Dr. Sarah Chen',
    authorCn: '陳博士',
    source: 'Nature Biotechnology',
    image: 'https://images.unsplash.com/photo-1532187863486-abf9dbad1b69?w=1200&h=800&fit=crop',
    tags: ['P2X1', 'Oncology', 'Milestone'],
    tagsCn: ['P2X1', '腫瘤學', '里程碑']
  },
  {
    id: 3,
    category: 'Partnership',
    categoryCn: '合作伙伴',
    title: 'JoinCare Partners with Leading Academic Institutions',
    titleCn: 'JoinCare與頂尖學術機構建立合作',
    excerpt: 'Strategic collaborations with top universities to bridge traditional research and Web3 innovation in life sciences.',
    excerptCn: '與頂級大學的戰略合作，連接傳統研究與生命科學領域的Web3創新。',
    content: `
      <p>JoinCare Foundation announced today strategic partnerships with three leading academic institutions to accelerate the adoption of decentralized science (DeSci) principles in biomedical research.</p>
      
      <h3>Partner Institutions</h3>
      <p>The partnerships include collaborations with research centers at Stanford University, MIT, and the University of Cambridge. These institutions will work with JoinCare to pilot new models of research funding and IP management.</p>
      
      <h3>Collaboration Framework</h3>
      <ul>
        <li>Joint research projects funded through the JoinCare platform</li>
        <li>Educational programs on blockchain and DeSci for researchers</li>
        <li>Development of best practices for IP-NFT tokenization</li>
        <li>Shared governance models for community-funded research</li>
      </ul>
      
      <h3>Academic Perspective</h3>
      <p>"Traditional research funding models are slow and often favor incremental over breakthrough science," said Prof. James Wilson from Stanford. "JoinCare's platform offers an exciting alternative that could democratize access to funding for high-risk, high-reward projects."</p>
      
      <h3>Impact on Research</h3>
      <p>These partnerships are expected to bring several early-stage research projects onto the JoinCare platform in 2025, spanning areas from regenerative medicine to novel drug discovery.</p>
    `,
    contentCn: `
      <p>JoinCare基金會今天宣布與三所領先的學術機構建立戰略合作伙伴關系，以加速去中心化科學（DeSci）原則在生物醫學研究中的應用。</p>
      
      <h3>合作機構</h3>
      <p>合作伙伴包括斯坦福大學、麻省理工學院和劍橋大學的研究中心。這些機構將與JoinCare合作，試點研究資金和知識產權管理的新模式。</p>
      
      <h3>合作框架</h3>
      <ul>
        <li>通過JoinCare平臺資助的聯合研究項目</li>
        <li>為研究人員提供區塊鏈和DeSci教育項目</li>
        <li>開發IP-NFT代幣化的最佳實踐</li>
        <li>社區資助研究的共享治理模式</li>
      </ul>
      
      <h3>學術觀點</h3>
      <p>"傳統的研究資助模式緩慢，往往偏向漸進式而非突破性科學，"斯坦福大學的詹姆斯·威爾遜教授說。"JoinCare的平臺提供了一個令人興奮的替代方案，可以使高風險、高回報項目的資金獲取民主化。"</p>
      
      <h3>對研究的影響</h3>
      <p>預計這些合作伙伴關系將在2025年將幾個早期研究項目引入JoinCare平臺，涵蓋從再生醫學到新藥發現的各個領域。</p>
    `,
    date: '2025-01-05',
    author: 'Partnership Team',
    authorCn: '合作團隊',
    source: 'Science Daily',
    image: 'https://images.unsplash.com/photo-1523240795612-9a054b0db644?w=1200&h=800&fit=crop',
    tags: ['Partnership', 'Academia', 'Collaboration'],
    tagsCn: ['合作', '學術界', '協作']
  },
  {
    id: 4,
    category: 'Breakthrough',
    categoryCn: '研究突破',
    title: 'Jiuge Biotech Advances Membrane Protein Production with P2X1 Antibody P007',
    titleCn: '玖格生物在P2X1抗體P007膜蛋白制備技術領域取得突破',
    excerpt: 'Jiuge Biotech has achieved a significant breakthrough in membrane protein production. Its P2X1 monoclonal antibody P007 shows remarkable clinical potential in anti-aging research and treatment of pancreatic cancer and sepsis.',
    excerptCn: '玖格生物在膜蛋白制備領域取得重大突破，其P2X1單克隆抗體P007在抗衰老研究、胰腺癌及膿毒癥治療中展現出卓越的臨床潛力。',
    content: `
      <p>Amid rising R&D costs and increasing innovation challenges in the global pharmaceutical industry, Jiuge Biotech has achieved a significant breakthrough in membrane protein production, opening new possibilities for international pharmaceutical collaborations.</p>

      <h3>Cracking the 'Sandcastle' Conundrum</h3>
      <p>G protein-coupled receptors (GPCRs) and ion channels account for half of all FDA-approved drug targets. Yet these transmembrane proteins have long been dubbed 'sandcastles' in antibody drug development — their intricate structure causes their native conformation to collapse once removed from the cell membrane. Jiuge Biotech's novel membrane protein production technology addresses this global bottleneck through unique process design and innovative purification strategy. The company's recombinant ion channel proteins retain their native conformation and exhibit biological activity matching that of endogenous proteins.</p>

      <h3>Breakthrough in Three Therapeutic Areas</h3>
      <p>Preclinical data for the P007 mAb have shown promising results:</p>
      <ul>
        <li><strong>Oncology:</strong> Targets pancreatic cancer by remodeling the tumor immune microenvironment and inhibiting tumor cell proliferation and invasion.</li>
        <li><strong>Anti-aging:</strong> Preclinical studies show the mAb can extend the average lifespan of experimental mice by approximately 34%, with over 70% of treated mice achieving significant benefits.</li>
        <li><strong>Critical Care:</strong> Effectively restores normal neutrophil chemotaxis in sepsis patients, correcting systemic inflammatory imbalances.</li>
      </ul>

      <h3>Deep Technical Expertise</h3>
      <p>The core R&D team has accumulated over a decade of expertise in the antibody field. Founders Fan Chunlei and Cheng Yinkai established Jucheng Biotechnology in 2012, with groundbreaking results published in top journals like Science and Cell. Their COVID-19 antigen detection kits ranked third in accuracy in authoritative inspections by Germany's BfArM.</p>

      <h3>Global Development Blueprint</h3>
      <p>Jiuge Biotech plans to accelerate clinical data collection and registration in both China and the United States, while expanding its pipeline for additional "undruggable" targets in neurological diseases, autoimmune disorders, and other therapeutic areas.</p>
    `,
    contentCn: `
      <p>面對全球制藥行業研發成本上升、創新挑戰加劇的背景，中國生物技術公司杭州玖格生物在膜蛋白制備領域取得重大突破，為國際制藥合作開辟新可能。</p>

      <h3>破解行業"沙堡"難題</h3>
      <p>G蛋白偶聯受體（GPCR）和離子通道占FDA批準藥物靶點的半數，卻因其復雜跨膜結構一直被稱為抗體藥物開發中的"沙堡"。玖格生物通過獨特的工藝設計和創新純化策略，攻克了這一全球性技術瓶頸。其重組離子通道蛋白可保留天然構象和生物活性，為功能性單抗藥物開發建立了完整技術框架。</p>

      <h3>三大治療領域突破性前景</h3>
      <p>P007單抗的臨床前數據已展現出令人振奮的結果：</p>
      <ul>
        <li><strong>腫瘤學：</strong>靶向"癌中之王"胰腺癌，通過重塑腫瘤免疫微環境抑制腫瘤細胞增殖與侵襲。</li>
        <li><strong>抗衰老：</strong>臨床前動物研究顯示，該單抗可將實驗小鼠平均壽命延長約34%，超過70%的受治小鼠獲得顯著獲益。</li>
        <li><strong>危重癥醫學：</strong>有效恢復膿毒癥患者的中性粒細胞趨化功能，糾正全身炎癥反應失衡。</li>
      </ul>

      <h3>深厚技術積累與國際合作潛力</h3>
      <p>核心研發團隊在抗體領域積累了逾十年深厚專業知識。創始人樊春磊和程銀鍇於2012年創立聚成生物，研究成果發表於Science、Cell等頂級期刊。其新冠抗原檢測試劑盒在德國聯邦藥品與醫療器械研究所（BfArM）的權威隨機檢查中精度排名第三。</p>

      <h3>全球發展藍圖</h3>
      <p>杭州玖格生物計劃加速P007單抗在中美兩國的臨床數據收集和註冊流程，同時依托成熟的膜蛋白制備平臺，加快拓展神經系統疾病、自身免疫性疾病等更多"不可成藥"靶點的藥物管線。</p>
    `,
    date: '2026-01-28',
    author: 'Guest Contributor',
    authorCn: '特約撰稿人',
    source: 'EU Reporter',
    image: '/news-assets/1.jpg',
    tags: ['P2X1', 'Membrane Proteins', 'Oncology', 'Anti-aging'],
    tagsCn: ['P2X1', '膜蛋白', '腫瘤學', '抗衰老'],
    externalUrl: 'https://www.eureporter.co/health/2026/01/28/hangzhou-jiuge-biotech-advances-membrane-protein-production-with-p2x1-antibody-p007/'
  },
  {
    id: 5,
    category: 'Research',
    categoryCn: '研究進展',
    title: 'Jiuge Biotech Achieves Milestone in Membrane Protein Production Technology',
    titleCn: '玖格生物在膜蛋白制備技術領域取得階段性進展',
    excerpt: 'Jiuge Biotech has made phased progress in membrane protein production, with the P007 monoclonal antibody advancing through its technical pipeline from target protein preparation to antibody discovery.',
    excerptCn: '玖格生物在膜蛋白制備技術領域取得階段性進展，已初步建立從靶點蛋白制備到抗體發現的技術流程，P007單抗研究持續推進。',
    content: `
      <p>Jiuge Biotech has recently made phased progress in the field of membrane protein production technology. This technology primarily addresses structural stability challenges of recombinant membrane proteins, and has preliminarily established a technical pipeline from target protein preparation to antibody discovery.</p>

      <h3>Ion Channel Research Context</h3>
      <p>Ion channels, as functional components on biological membranes, are one of the important pathways for cellular signal transduction. In biopharmaceutical R&D, the in vitro preparation of membrane proteins often faces difficulties such as structural instability. Jiuge Biotech's technology platform attempts to provide solutions to these challenges through process design and optimization of purification strategies.</p>

      <h3>Experimental Validation</h3>
      <p>Through internal experimental verification, some recombinant membrane proteins prepared using this technology can maintain the target structure under specific conditions. Based on this platform, research has been applied to relevant targets and the technology continues to be optimized.</p>

      <h3>Future Plans</h3>
      <p>A company spokesperson stated that Jiuge Biotech will continue to focus on biologics R&D for ion channel targets, strictly advance the P007 monoclonal antibody research in accordance with regulatory requirements, and expand biologic R&D work for more potential targets to empower the biopharmaceutical industry through technological innovation.</p>
    `,
    contentCn: `
      <p>近日，杭州玖格生物在膜蛋白制備技術領域取得階段性進展。該技術主要針對重組膜蛋白結構穩定性方面的技術挑戰進行探索，目前已初步建立從靶點蛋白制備到抗體發現的技術流程。</p>

      <h3>離子通道研究背景</h3>
      <p>離子通道作為生物膜上的功能組件，是細胞信號傳導的重要通路之一。在生物醫藥研發中，膜蛋白的體外制備常面臨結構不穩定等技術難點。玖格生物此次推出的技術平臺，通過工藝設計與純化策略的優化，嘗試為上述技術難點提供可能的解決思路。</p>

      <h3>實驗驗證結果</h3>
      <p>經內部實驗驗證，該技術制備的部分重組膜蛋白在特定條件下可維持目標結構。基於該技術平臺，此項研究已在相關靶點開展應用探索，目前該技術正在持續優化中。</p>

      <h3>未來規劃</h3>
      <p>杭州玖格生物相關負責人表示，未來公司將持續深耕離子通道靶點生物制劑研發領域，嚴格按照相關法規要求推進P007單抗的研究，同時拓展更多潛在靶點的生物制劑研發工作，以科技創新賦能生物醫藥產業發展。</p>
    `,
    date: '2026-02-10',
    author: 'ScienceNet Editorial',
    authorCn: 'ScienceNet編輯部',
    source: 'ScienceNet',
    image: '/news-assets/2.png',
    tags: ['Membrane Proteins', 'P007', 'Ion Channel', 'Biopharmaceutical'],
    tagsCn: ['膜蛋白', 'P007單抗', '離子通道', '生物制藥'],
    externalUrl: 'https://news.sciencenet.cn/htmlnews/2026/2/560085.shtm'
  }
]


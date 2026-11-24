-- 补齐 project 2 (P2X1) + 3 (Smarto Neuro) 的封面 + 繁体中文内容
-- 前端 pickField/pick 会根据 language 自动 pick _zh 字段，空则 fallback 到 english
SET NAMES utf8mb4;

-- Project 2 · 繁体中文内容 + cover
UPDATE projects SET
  cover_url        = '/figma/v2/project-card-image-3.png',
  tagline_zh       = '首個功能性 P2X1 離子通道單株抗體 — 一個平台，三大適應症（膿毒症、腫瘤、抗衰老）。',
  story_html_zh    = '<h2>背景 — 為何離子通道單抗是最難的一類生物藥</h2>
<p>G 蛋白偶聯受體 (GPCR) 與離子通道合計約占 FDA 批准藥物靶點的一半 (分別 34% 與 13%)，但至今只有兩款 GPCR 單株抗體 (<em>erenumab</em>、<em>mogamulizumab</em>) 進入臨床。抗體在選擇性、生體可用率、半衰期與效應功能上均優於小分子；但保持天然構象的抗原製備長期是產業瓶頸。</p>
<p>離子通道又比 GPCR 更難。GPCR 是七次跨膜的單體膜蛋白；離子通道則是<strong>多亞基、多次跨膜的功能複合體，且在開放與關閉之間切換不同構象</strong>。輝瑞生物藥部門公開研發 GPCR / 離子通道抗體藥，但進展緩慢。</p>

<h2>我們的方法 — Nanodisc 保留天然構象抗原</h2>
<p>P2X1 屬於 ATP 門控 (P2X) 家族：由三個同源亞基構成，每亞基兩次跨膜。范春雷教授團隊擴增 hP2X1-mFc.copGFP/293 細胞，製備<strong>保有天然四級結構的 hP2X1 Nanodisc</strong>，並經 Protein A/G 純化。這種天然構象抗原是整個平台的關鍵突破。</p>

<h2>從免疫到人源化 hP007A</h2>
<ul>
  <li>以 hP2X1 Nanodisc 免疫 7 隻 Balb/c 小鼠；其中 1 隻血清效價達 1:204,800（OD ≥ 1）。</li>
  <li>融合細胞 + ELISA 篩選獲得 16 個陽性克隆；前 8 個（P001–P008）進行後續驗證。</li>
  <li><strong>P007 與 P008</strong> 能將 LPS 抑制的中性粒細胞趨化能力從 CFS = 6 恢復至 CFS ≈ 16（健康對照水平），p &lt; 0.0001。</li>
  <li>19 種 CDR 移植至人源 IgG1 組合，篩得親和力最佳的 <strong>hP007A</strong>：Kd 從 0.389 μM（小鼠）降至 <strong>3.97 nM</strong>（人源化）— 提升 100 倍。</li>
</ul>

<h2>三大適應症 — 同一平台</h2>
<p>P2X1 在血小板、中性粒細胞、巨噬細胞及多種腫瘤細胞上表現。作為 ATP 門控鈣離子通道，它參與 LPS 誘導的全身炎症、細胞凋亡、突觸傳遞與血小板活化 — 一款抗體可撬動三個獨立適應症：</p>
<ol>
  <li><strong>膿毒症 / 嚴重內毒素血症</strong> — hP007A 提升 LPS 誘導膿毒症模型小鼠的生存率。</li>
  <li><strong>實體腫瘤</strong> — 0.92 mg/kg P007 單藥於 NOD-SCID + PBMC 的 H460（人肺癌）模型中實現<em>完全</em>抑制腫瘤生長，療效超越 Durvalumab（臨床 PD-L1 抗體），且瘤組織內部呈現空腔。</li>
  <li><strong>抗衰老 / 長壽（XD001）</strong> — 配套研發項目，影片記錄小鼠表型延壽至相當於人類約 132 歲。</li>
</ol>',
  research_html_zh = '<h2>關鍵臨床前數據</h2>
<h3>親和力（SPR，hP2X1 Nanodisc 芯片）</h3>
<table><thead><tr><th>抗體</th><th>Kd</th><th>備註</th></tr></thead>
<tbody>
<tr><td>P007（小鼠）</td><td>0.389 μM</td><td>原始克隆</td></tr>
<tr><td><strong>hP007A</strong>（人源化）</td><td><strong>3.97 nM</strong></td><td>CDR 移植至人源 IgG1</td></tr>
</tbody></table>

<h3>中性粒細胞趨化能力恢復</h3>
<p>LPS 抑制後的中性粒細胞（CFS = 6，****p &lt; 0.0001 vs 正常對照）。P007 與 P008 將 CFS 恢復至 16 與 15.8，與健康對照組無顯著差異（p &gt; 0.05）。</p>

<h3>患者體內 P2X1 表現</h3>
<ul>
  <li>膿毒症 / 感染患者（n = 53）：中性粒細胞表面 P2X1 顯著高於健康對照組；趨化評分與 P2X1 水平呈負相關。</li>
  <li>腫瘤患者：中性粒細胞 P2X1 表現上升；r = −0.83，****p &lt; 0.0001 與趨化能力呈負相關。</li>
</ul>

<h3>腫瘤抑制（體內，NOD-SCID + 人 PBMC）</h3>
<ul>
  <li>H460 肺癌模型：0.92 mg/kg P007 → <strong>完全</strong>抑制生長；優於 Durvalumab 陽性對照。</li>
  <li>A375 人黑色素瘤模型：3.68 mg/kg P007 劑量依賴性體積縮小，組織學觀察到空腔形成。</li>
</ul>

<h3>膿毒症生存率提升</h3>
<p>LPS 誘導致死性內毒素血症小鼠：hP007A（1 號）相對於 4 個 CDR2 定點突變克隆（2–5 號）呈現顯著的生存曲線分離，驗證功能表位。</p>

<h3>配套流式檢測抗體 FCM001</h3>
<p>P2X1 流式檢測抗體（專利 CN118652341A）在特異性與靈敏度均優於市售試劑。已完成技術轉讓：與<strong>中翰盛泰生物</strong>簽訂 200 萬元人民幣轉讓合約。</p>',
  risks_html_zh    = '<h3>研發階段風險</h3>
<ul>
  <li><strong>CMC 放大。</strong> hP007A 抗原製備需 Nanodisc 配方 — CDMO 資質認證處於關鍵路徑。時程：1.5–2 年，預算約 1,000 萬元人民幣。</li>
  <li><strong>雙軌監管申報。</strong> 項目將同時申報 NMPA 與 FDA IND；抗血栓、抗腫瘤、抗衰老可能拆分為獨立 IND。</li>
  <li><strong>首類離子通道單抗。</strong> 監管方對此類藥物的先例有限，預期需比常規 GPCR 抗體提供額外的安全性研究。</li>
  <li><strong>對融資進度依賴。</strong> 目前實驗室階段 5 年花費 1,000 萬元人民幣；CMC + 臨床前 + I 期臨床再需 ~2,100 萬元人民幣，受融資節點制約。延遲會層層影響。</li>
  <li><strong>Token / Phase 2 揭露。</strong> 本次 Phase 1 僅提供訂閱與內容展示；不涉及鏈上發售、購買合約與釋放時程。Phase 2（合約 + 交易）將另行公告，並附獨立的風險揭露。</li>
</ul>'
WHERE project_id = 2;

-- Project 2 · JSON 内 _zh 字段
UPDATE projects SET team_json = JSON_SET(team_json, '$[0].name_zh', "范春雷 教授", '$[0].role_zh', "項目負責人", '$[0].bio_zh', "浙江中醫藥大學生命科學學院。hP2X1 Nanodisc 抗原平台與 8 個 P2X1 功能性單株抗體克隆的主要發明人。三項已公開專利發明人（CN118638232A、CN118717967A、CN118652341A）與一項 PCT（國際檢索報告正面）。") WHERE project_id = 2;
UPDATE projects SET team_json = JSON_SET(team_json, '$[1].name_zh', "孫炳偉 教授", '$[1].role_zh', "臨床負責人", '$[1].bio_zh', "蘇州市立醫院。發現內毒素抑制中性粒細胞趨化功能與中性粒細胞膜上 P2X1 過表達相關 — 這是 P2X1 抗體項目的原始生物學洞見。") WHERE project_id = 2;
UPDATE projects SET team_json = JSON_SET(team_json, '$[2].name_zh', "JoinCare 基金會", '$[2].role_zh', "孵化夥伴", '$[2].bio_zh', "主導孵化，構建 CMC-CDMO 合作與雙軌（NMPA / FDA）IND 申報。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[0].quarter_zh', "2020–2025", '$[0].title_zh', "抗原、單抗、人源化階段完成", '$[0].desc_zh', "hP2X1 Nanodisc 抗原；8 個小鼠克隆（P001–P008）；P007 人源化為 hP007A（Kd 3.97 nM）。5 年預算 1,000 萬元人民幣。專利 CN118638232A、CN118717967A、CN118652341A + PCT。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[1].quarter_zh', "2026 Q3–Q4", '$[1].title_zh', "上線前訂閱與內容展示", '$[1].desc_zh', "此頁面。Notify-me + 收藏 + 官方 Updates。無鏈上發售。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[2].quarter_zh', "2026 Q4 – 2028 Q2", '$[2].title_zh', "CMC（委託 CDMO）", '$[2].desc_zh', "細胞株構建 → 規模化生產 → 穩定性研究。預算約 1,000 萬元人民幣。受種子輪關賬制約。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[3].quarter_zh', "2027 Q2 – Q4", '$[3].title_zh', "抗血栓適應症臨床前藥效 + 安全性", '$[3].desc_zh', "與 CMC 中期並行。預算約 600 萬元人民幣，委託 CDMO。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[4].quarter_zh', "2028 Q3", '$[4].title_zh', "IND 申報（NMPA + FDA）— 抗血栓", '$[4].desc_zh', "進入 I 期臨床。License-out 預期回收資金。") WHERE project_id = 2;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[5].quarter_zh', "2027 – 2028", '$[5].title_zh', "抗腫瘤與抗衰老適應症線", '$[5].desc_zh', "獨立的臨床前 + IND 申報。抗衰老 XD001 為配套項目。") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[0].title_zh', "專利 · 功能性 P2X1 膜蛋白單抗製備與應用", '$[0].kind_zh', "專利 · CN118638232A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[1].title_zh', "專利 · 抗血小板凝集功能性 P2X1 單抗應用", '$[1].kind_zh', "專利 · CN118717967A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[2].title_zh', "專利 · P2X1 流式檢測單株抗體", '$[2].kind_zh', "專利 · CN118652341A") WHERE project_id = 2;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[3].title_zh', "PCT 國際檢索報告 — 新穎性、創造性、產業實用性均確認", '$[3].kind_zh', "PCT 報告") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[0].q_zh', "為什麼 P2X1 抗體比 GPCR 抗體更難?", '$[0].a_zh', "GPCR 是七次跨膜單體膜蛋白。P2X1 是同源三聚體，每亞基兩次跨膜，且開放與關閉呈不同構象。以天然四級結構製備抗原是產業瓶頸 — 而 hP2X1 Nanodisc 平台正是解決此問題。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[1].q_zh', "Phase 1 只是訂閱頁，我實際加入的是什麼?", '$[1].a_zh', "Phase 1 = Notify-me 訂閱 + 官方 Updates + 收藏 + （Warmup 階段）白名單登記。無鏈上發售。Phase 2（合約、購買、釋放時程）將另行公告，並附獨立的風險揭露。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[2].q_zh', "如何從一款抗體衍生出三個適應症?", '$[2].a_zh', "P2X1 在血小板（血栓）、中性粒細胞（膿毒症、炎症）與多種腫瘤細胞（腫瘤）上表現。同一克隆 — 不同的作用機制。每個適應症獨立進行臨床前與 IND 申報。") WHERE project_id = 2;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[3].q_zh', "預期融資時程?", '$[3].a_zh', "目前種子資本覆蓋 CMC（1,000 萬元人民幣，約 2 年）與主要適應症臨床前（600 萬元人民幣，約 6 個月）。NMPA + FDA IND 申報目標為 2028 Q3。後續抗腫瘤 / 抗衰老 IND 靠 license-out 回收循環資金。") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[0].label_zh', "P2X1 代幣（TGE 後）") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[1].label_zh', "釋放合約") WHERE project_id = 2;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[2].label_zh', "公開發售合約") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[0].label_zh', "公開發售（Phase 2）") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[1].label_zh', "研發 + CMC 儲備") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[2].label_zh', "團隊 + 顧問") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[3].label_zh', "生態 / 社群") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[4].label_zh', "基金會金庫") WHERE project_id = 2;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[5].label_zh', "流動性") WHERE project_id = 2;


-- Project 3 · 繁体中文内容 + cover
UPDATE projects SET
  cover_url        = '/figma/v2/project-card-image-4.png',
  tagline_zh       = '60 年的 GABA 抑制神經受體 / gephyrin 理論體系，落地為三條神經健康產品線 — FDA 保健品、40Hz 二類醫療器械、D-P 新藥管線。',
  story_html_zh    = '<h2>理論 — 兩代 GABA 抑制神經受體科學家</h2>
<p><strong>1969 · 理查・奧爾森教授（UCLA）</strong> — 被譽為「GABA 受體之父」。發現大腦由 GABA 抑制與興奮兩個系統平衡運作；健康即平衡。抑制系統失效時，興奮系統釋放神經毒素，導致神經元受損並中斷傳導。</p>
<p><strong>2004 · 梁京教授</strong> — 提出導向 / 支撐蛋白（gephyrin）假說。當大腦 gephyrin 降至基線 50% 以下，GABA-A 受體功能崩潰。此假說目前被視為阿茲海默症的<em>第四大病理假說</em>。</p>
<p><strong>2020 · 線粒體層面延伸</strong> — 腦細胞線粒體受損 → ATP 下降 → gephyrin 下降 → GABA-A 信號流失 → 神經退化。</p>
<p><strong>2021 · 神經免疫缺損假說</strong> — 過度活化的小膠質細胞（壓力、社交隔離、睡眠不足所致）引發神經炎症。</p>

<h2>化合物 — 從 6,000 種植物黃酮到 D-P</h2>
<p>自 2003 年，梁京實驗室篩選了 6,000 多種天然植物黃酮。<strong>D-黃酮組合</strong>（天然小分子黃酮混合）作為 GABA-A 正向調節劑脫穎而出：可恢復 gephyrin、提升線粒體 ATP、快速穿越血腦屏障、無依賴性。2018 年以 FDA 註冊的 Smarto-One 保健品問世。</p>
<p>2020 年：對天然骨架進行藥物化學優化，得到<strong>單體化合物 D-P</strong> — 穩定性與功效提升 50 倍，並實現<strong>15 分鐘內快速穿越血腦屏障</strong>。D-P 為新藥管線的基底化合物（D-P1 針對阿茲海默症中早期 + BPSD 症狀；D-P2 針對抑鬱 / 情感障礙）。</p>

<h2>三條產品線，同一套理論</h2>
<ol>
  <li><strong>保健品（FDA 註冊）— Smarto-One / Carefreeing™ 系列。</strong>已於美中兩地跨境電商上線。288 人志願者研究顯示能改善睡眠 / 壓力 / 情緒。目標客群為 25–40 歲都市女性；三年營收計劃 1.2 億 / 3.5 億 / 6 億人民幣（2026–2028）。</li>
  <li><strong>二類醫療器械 — Smarto Light™ 40Hz。</strong>非侵入式伽瑪波多感官（聲 + 光 + 振動）刺激。基於 MIT（蔡立慧院士）40Hz 伽瑪研究。二類醫療器械註冊申請中。與北京和輝光電（ISO 9001，申請 ISO 13485）合作生產。</li>
  <li><strong>新藥 — D-P1 / D-P2。</strong>D-P1：阿茲海默症中早期治療 + BPSD 症狀改善。D-P2：情感障礙。兩者實驗室階段資料均已完成，等待 I 期臨床。配套 IVD：4 個檢測試劑盒（AD7C-NTP、Aβ1-42、P-Tau-181、Aβ42）。</li>
</ol>',
  research_html_zh = '<h2>證據基礎</h2>

<h3>D-黃酮 — 人體與動物實驗證據</h3>
<ul>
  <li>增強 GABA-A 受體張力電流與相位電流（Shen 等，<em>J. Neuroscience</em> 2012）。</li>
  <li>降低轉基因 AD 小鼠 Aβ 斑塊負荷（Liang 等，<em>J. Neurochem</em> 2014）。</li>
  <li>改善 AD 模型囓齒動物的焦慮、記憶、運動功能。透過小膠質細胞正常化逆轉壓力誘導的神經炎症（Liang 等，<em>J. Neuroinflammation</em> 2022）。</li>
  <li>288 人人體觀察研究 — 減少睡眠障礙、壓力與情緒問題（PLOS ONE 2022，Smarto-One）。</li>
  <li>USC IRB 核准的人體試驗；2018 年至今累計 20,000+ 真實消費者研究。</li>
</ul>

<h3>D-P — 第二代優化單體</h3>
<ul>
  <li>相較 D-黃酮天然骨架，穩定性與功效提升 50 倍。</li>
  <li>15 分鐘內穿越血腦屏障（解決小分子生體可用率瓶頸）。</li>
  <li>8 小時全身清除，無殘留、無依賴、無戒斷風險。</li>
  <li>實驗室階段數據 + 可重現性 + 穩定製備方法均已完成。等待 I 期臨床。</li>
</ul>

<h3>40Hz 伽瑪醫療器械 — MIT 臨床證據</h3>
<ul>
  <li>Aβ40 清除率提升 28%（Nature — 類淋巴機制）。</li>
  <li>Tau 磷酸化降低 37%（Tau P301S 模型）。</li>
  <li>MMSE 認知衰退速度減緩 83%（n = 76 雙盲）。</li>
  <li>ADCS-ADL 評分下降速率減緩 84%（p &lt; 0.001）。</li>
  <li>默認模式網路連接度提升 41%；6 週後突觸蛋白標記物提升 68%。</li>
  <li>相較口服多奈哌齊（6 個月）：MMSE 改善 +3.2 vs +1.5；副作用發生率 &lt; 1% vs 28%；日均成本 ¥80 vs ¥220；依從性 98% vs 63%。</li>
</ul>

<h3>市場 — 保健品管線</h3>
<ul>
  <li>中國神經健康保健品市場：2025 年預估 1,800 億元人民幣，年複合增長率 15%+（Frost &amp; Sullivan）。</li>
  <li>中國情感障礙：1.32 億患者，人均年支出 1.7 萬元人民幣，市場空間 324.7 億元人民幣。</li>
  <li>中國睡眠障礙：3 億患者，人均年支出 932 元人民幣，市場空間 4,000 億元人民幣。</li>
</ul>',
  risks_html_zh    = '<h3>三條監管路徑 — 三種風險特徵</h3>
<ul>
  <li><strong>保健品（Smarto-One）</strong> — 已獲美國 FDA 膳食補充劑註冊 + 中國食品質量檢驗認證。監管風險最低；產品已產生現金流。風險在於競爭（中國市場多數同類產品缺乏臨床驗證 — 市場教育成本）。</li>
  <li><strong>二類醫療器械（Smarto Light™ 40Hz）</strong> — 兩份二類申請進行中。標準醫療器械時程（18–24 個月）；生產夥伴（北京和輝光電）正申請 ISO 13485。IP：MIT 40Hz 方法（授權）+ 專有三合一（聲 / 光 / 振動）方案 + 原創 40Hz 音樂版權。</li>
  <li><strong>新藥（D-P1 / D-P2）</strong> — 完整的 FDA / NMPA 新藥路徑。IND 前研究 3–6 個月；前置研究 6 個月；臨床前 18 個月；IND 審查 3–6 個月；分期臨床 1 年；NDA 6–12 個月。並非全新化合物（天然 D-黃酮母骨架已具備消費級安全紀錄），但整體時程仍為 4–5 年。</li>
</ul>

<h3>融資架構</h3>
<ul>
  <li>種子輪：目標人民幣 4,500 萬元，釋出 20–25% 股權。關賬期限公告為 2026 年 1 月 30 日。</li>
  <li>資金用途：藥品研發 + 產品研發 + 跨境電商品牌 + 線上市場投流 + 渠道建設 + 生產。</li>
  <li>完整財務模型與其他戰略文件需在簽署 NDA 後提供。</li>
  <li><strong>本次 Phase 1 僅提供訂閱與內容展示</strong> — 無鏈上發售、無購買合約、無釋放時程上線。Phase 2（合約 + 交易）將另行公告，並附獨立的風險揭露。</li>
</ul>'
WHERE project_id = 3;

-- Project 3 · JSON 内 _zh 字段
UPDATE projects SET team_json = JSON_SET(team_json, '$[0].name_zh', "梁京 教授", '$[0].role_zh', "科學負責人", '$[0].bio_zh', "美國南加州大學藥學院教授。英國皇家醫學院院士（2019 迄今）。倫敦「新科學家」評選（2012）。提出「導向 / 支撐蛋白 — gephyrin」假說（2004），後被公認為阿茲海默症第四大病理假說。發表 82 篇同行評議論文；2012 年 J. Neuroscience 論文入選 NIH「影響世界的 1000 篇」。2008 年 NIH 大腦挑戰基金首席科學家。《中國藥理學》期刊主編。多項技術發明專利。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[1].name_zh', "理查・奧爾森 教授（追念）", '$[1].role_zh', "奠基理論", '$[1].bio_zh', "UCLA 醫學院終身傑出教授。被譽為「GABA 受體之父」。獲諾貝爾委員會頒發神經科學終身成就獎。NIH 職業發展大獎；Javits 神經科學研究獎；美國 HHS 心血管成就獎。發表 266 篇 SCI 論文；擔任《藥物學》《神經科學》《癲癇病》《酒精研究》編委。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[2].name_zh', "王蓉 教授", '$[2].role_zh', "中國實驗室負責人", '$[2].bio_zh', "首都醫科大學宣武醫院中心實驗室主任。國家老年疾病臨床醫學研究中心 AD 外周標誌物檢測與早期干預聯盟主任委員。北京神經內科學會基礎與轉化醫學專業委員會主任委員。神經絲蛋白 AD7c-NTP 尿液檢測發明人。發表 32 篇第一作者、92 篇通訊作者論文，其中 SCI 37 篇。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[3].name_zh', "于順 教授", '$[3].role_zh', "中國實驗室 · 帕金森病", '$[3].bio_zh', "首都醫科大學宣武醫院國家老年疾病臨床研究中心。北京市老年病醫療研究中心分子診斷實驗室主任。首都醫科大學帕金森病研究所副所長。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[4].name_zh', "劉魚 教授", '$[4].role_zh', "中國實驗室 · 輸血醫學", '$[4].bio_zh', "北京協和醫學院教授。約翰霍普金斯大學醫學院訪問學者。中國醫學科學院輸血研究所輸血醫學實驗中心主任。") WHERE project_id = 3;
UPDATE projects SET team_json = JSON_SET(team_json, '$[5].name_zh', "趙凡", '$[5].role_zh', "行銷長 · 商務負責人", '$[5].bio_zh', "前抖音市場部 CMO / 服飾快消行業總經理。操盤業務規模 5,000 億元人民幣，年預算 70 億元人民幣，管理團隊 250 人。負責 B 端生態、C 端品牌建設與跨境電商渠道。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[0].quarter_zh', "2003–2018", '$[0].title_zh', "D-黃酮發現 → Smarto-One 上市", '$[0].desc_zh', "篩選 6,000+ 種黃酮 → D-黃酮組合 → FDA 保健品註冊 → 中美渠道 + 20,000 人真實研究。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[1].quarter_zh', "2020", '$[1].title_zh', "D-P 單體化合物完成", '$[1].desc_zh', "結構優化單體，穩定性與功效提升 50 倍，實現 15 分鐘穿越血腦屏障。實驗室階段 + 可重現性 + 製備方法均已完成。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[2].quarter_zh', "2026 Q3–Q4", '$[2].title_zh', "上線前訂閱與內容展示", '$[2].desc_zh', "此頁面。Notify-me + 收藏 + 官方 Updates。無鏈上發售。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[3].quarter_zh', "2026 Q4", '$[3].title_zh', "種子輪關賬（人民幣 4,500 萬元，釋出 20–25% 股權）", '$[3].desc_zh', "期限 2026-01-30。資金用途對應藥品研發 + 醫療器械建設 + 渠道拓展。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[4].quarter_zh', "2027", '$[4].title_zh', "D-P1 IND（阿茲海默症中早期 + BPSD）", '$[4].desc_zh', "18 個月臨床前 → IND 申報。並行：跨境電商 3.5 億元人民幣營收計劃（渠道矩陣 + 明星直播帶貨）。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[5].quarter_zh', "2027", '$[5].title_zh', "Smarto Light™ 40Hz — 二類醫療器械上市", '$[5].desc_zh', "兩份二類申請預期核准。居家眼罩 + 醫院睡眠艙 SKU。ISO 13485 定案。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[6].quarter_zh', "2028", '$[6].title_zh', "D-P2 IND（情感障礙 / 抑鬱）", '$[6].desc_zh', "CMC 中期後 6 個月臨床前。IVD 4 檢測面板配套上市，用於 AD 早期篩查。") WHERE project_id = 3;
UPDATE projects SET roadmap_json = JSON_SET(roadmap_json, '$[7].quarter_zh', "2028", '$[7].title_zh', "商務第三年 · 6 億元人民幣營收計劃", '$[7].desc_zh', "抖音店播矩陣 + 天貓 / 京東 + 山姆 + 100 家線下零售。毛利率目標 82%。投資者退出：IPO / 併購（對標湯臣倍健收購 Life-Space PS 5–8 倍）。") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[0].title_zh', "美國 FDA · 膳食補充劑註冊（Carefreeing™ / Smarto-One）", '$[0].kind_zh', "監管") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[1].title_zh', "中國食品質量檢驗認證", '$[1].kind_zh', "監管") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[2].title_zh', "普尼測試 · 急性經口毒性認證", '$[2].kind_zh', "安全") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[3].title_zh', "D-黃酮 — 18 篇代表性同行評議論文", '$[3].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[4].title_zh', "MIT 40Hz 伽瑪 — Nature 類淋巴機制 + NCT-03556280 II 期參考", '$[4].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[5].title_zh', "Smarto-One PLOS ONE 2022 — 288 人壓力 / 焦慮 / 睡眠研究", '$[5].kind_zh', "論文") WHERE project_id = 3;
UPDATE projects SET documents_json = JSON_SET(documents_json, '$[6].title_zh', "湯臣倍健收購 Life-Space（PS 5–8 倍退出參考）", '$[6].kind_zh', "退出對標") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[0].q_zh', "D-黃酮（Smarto-One）與 D-P 新藥有什麼區別？", '$[0].a_zh', "D-黃酮是天然植物黃酮混合物，自 2018 年起以 FDA 註冊的膳食補充劑形式銷售。D-P 是同一骨架經藥物化學優化的單體化合物 — 穩定性與功效提升 50 倍，15 分鐘穿越血腦屏障，現進入完整的新藥路徑（阿茲海默症 D-P1、抑鬱 D-P2）。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[1].q_zh', "為何同時發展三條產品線？", '$[1].a_zh', "同一套理論（GABA-A / gephyrin 修復），三條監管路徑。保健品已產生現金流，資助研發。二類醫療器械 18–24 個月回本。新藥為長線高價值資產。跨產品線交叉補貼。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[2].q_zh', "Phase 1 只是訂閱頁，我實際加入的是什麼？", '$[2].a_zh', "Phase 1 = Notify-me 訂閱 + 官方 Updates + 收藏 + （Warmup 階段）白名單登記。無鏈上發售。Phase 2（合約 + 購買 + 釋放時程）將另行公告，並附獨立的風險揭露。") WHERE project_id = 3;
UPDATE projects SET faq_json = JSON_SET(faq_json, '$[3].q_zh', "種子輪何時關賬？", '$[3].a_zh', "種子輪目標人民幣 4,500 萬元，釋出 20–25% 股權。公告期限：2026-01-30。詳細財務模型與其他戰略文件需在簽署 NDA 後提供。") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[0].label_zh', "SMRT 代幣（TGE 後）") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[1].label_zh', "釋放合約") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[2].label_zh', "公開發售合約") WHERE project_id = 3;
UPDATE projects SET onchain_json = JSON_SET(onchain_json, '$[3].label_zh', "基金會金庫多簽") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[0].label_zh', "公開發售（Phase 2）") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[1].label_zh', "研發 — D-P 新藥") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[2].label_zh', "醫療器械 + IVD 建設") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[3].label_zh', "團隊 + 顧問") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[4].label_zh', "跨境電商") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[5].label_zh', "基金會金庫") WHERE project_id = 3;
UPDATE projects SET tokenomics_json = JSON_SET(tokenomics_json, '$.allocations[6].label_zh', "流動性") WHERE project_id = 3;


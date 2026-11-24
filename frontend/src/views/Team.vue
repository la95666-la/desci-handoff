<script setup>
import { computed, inject } from 'vue'
import JcFloatingNavbar from '../components/JcFloatingNavbar.vue'
import HomeFooter from '../components/HomeFooter.vue'

const props = defineProps({
  language: {
    type: String,
    default: 'en',
  },
})

const isHomeDark = inject('isHomeDark', false)

const COPY = {
  en: {
    kicker: 'JoinCare Team',
    titleMain: 'A global research network for',
    titleAccent: 'biomedical translation.',
    lead: 'JoinCare is built by contributors across biomedical innovation, AI, Web3 infrastructure, and community operations, with advisors who help connect rigorous science to real-world deployment.',
    linkedin: 'LinkedIn',
    members: [
      {
        position: 'CEO',
        name: 'Prof. Peter Sachsenmeier',
        role: 'Chief Executive Officer',
        image: '/team-assets/peter-sachsenmeier.jpg',
        alt: 'Prof. Peter Sachsenmeier speaking at an event',
        linkedin: 'https://cn.linkedin.com/in/psachsenmeier',
        bio: [
          'Prof. Peter Sachsenmeier has served as a Fellow of the German National Academy of Science and Engineering since 2006 and has held academic roles associated with the University of Oxford and Brandenburg University of Technology. His work spans digital transformation, industrial innovation, Industry 4.0, smart cities, and intelligent manufacturing.',
          'He has also contributed to projects involving the United Nations Environment Programme, the World Health Organization, the World Bank, and UNESCO. As CEO of JoinCare, he guides international strategy, public-health engagement, academic collaboration, and long-term ecosystem development.',
        ],
      },
      {
        position: 'CTO',
        name: 'Natasha Sachsenmeier',
        role: 'Chief Technology Officer',
        image: '/uploads/2026-09/b60a289e18443cbc4b88f31743b77dac.png',
        alt: 'Natasha Sachsenmeier, Chief Technology Officer of JoinCare',
        linkedin: 'https://www.linkedin.com/in/natasha-sachsenmeier/',
        bio: [
          'Natasha Sachsenmeier is a machine learning engineer, entrepreneur and interdisciplinary thinker with degrees in mathematics, philosophy and music. An alumna of Trinity College, Cambridge and the Royal Academy of Music, she pursued an international career as a violinist before founding an award-winning online concert platform. She subsequently retrained in artificial intelligence, completing an AI and ML programme through Imperial College Business School with the highest marks in her cohort and winning the capstone Bayesian optimisation competition.',
          'Her experience spans the AI pipeline, from data acquisition and analysis to bespoke model-building and the post-training and evaluation of larger systems. She regularly publishes online articles on topics such as alignment and cognition. At JoinCare, she brings cross-disciplinary vision and technical rigour to the development and deployment of the platform’s CareAgents.',
        ],
      },
      {
        position: 'COO',
        name: 'Quentin Liu',
        role: 'Chief Operating Officer',
        image: '/team-assets/quentin-liu.jpg',
        monogram: 'QL',
        visualLabel: 'Operations · Investment · Technology',
        alt: 'Quentin Liu, Chief Operating Officer of JoinCare',
        linkedin: 'https://www.linkedin.com/in/quentin-liu-53a121336/',
        bio: [
          'Quentin Liu is a cross-border technology investor and operator with an MSc in Computer Science and an MBA from London Business School. He previously worked at The Coca-Cola Company and SAP, contributing to global supply chain systems, enterprise digital transformation, and cross-border business operations.',
          'His experience spans healthcare, new energy, and advanced manufacturing, with expertise in technical due diligence, project evaluation, cross-border resource integration, and post-investment value creation. At JoinCare, he brings a practical, cross-disciplinary approach to operational execution, global partnerships, and advancing early-stage biomedical innovation.',
        ],
      },
    ],
    principlesKicker: 'How we work',
    principles: [
      ['Scientific diligence', 'We prioritize verifiable research, transparent review, and measurable translational value.'],
      ['Global collaboration', 'We connect laboratories, founders, clinicians, and community contributors across regions.'],
      ['Patient-centered translation', 'Every initiative is evaluated by its potential to move biomedical ideas closer to people who need them.'],
    ],
  },
  tw: {
    kicker: 'JoinCare 團隊',
    titleMain: '面向生醫轉譯的',
    titleAccent: '全球研究網絡。',
    lead: 'JoinCare 由生醫創新、AI、Web3 基礎設施與社群營運等領域的貢獻者共同推進，並由顧問團協助把嚴謹科研連接到真實世界的落地場景。',
    linkedin: 'LinkedIn',
    members: [
      {
        position: 'CEO',
        name: 'Prof. Peter Sachsenmeier',
        role: '執行長',
        image: '/team-assets/peter-sachsenmeier.jpg',
        alt: 'Peter Sachsenmeier 教授在活動中演講',
        linkedin: 'https://cn.linkedin.com/in/psachsenmeier',
        bio: [
          'Peter Sachsenmeier 教授自 2006 年起擔任德國國家科學與工程院院士，並曾任與牛津大學、布蘭登堡工業大學相關的學術職務。他的工作橫跨數位轉型、產業創新、工業 4.0、智慧城市與智慧製造。',
          '他亦參與過聯合國環境規劃署、世界衛生組織、世界銀行與 UNESCO 相關項目。作為 JoinCare 執行長，他領導國際策略、公共衛生協作、學術合作與長期生態發展。',
        ],
      },
      {
        position: 'CTO',
        name: 'Natasha Sachsenmeier',
        role: '技術長',
        image: '/uploads/2026-09/b60a289e18443cbc4b88f31743b77dac.png',
        alt: 'JoinCare 技術長 Natasha Sachsenmeier',
        linkedin: 'https://www.linkedin.com/in/natasha-sachsenmeier/',
        bio: [
          'Natasha Sachsenmeier 是一位機器學習工程師、創業家與跨領域思考者，擁有數學、哲學與音樂領域的學位。她曾就讀於劍橋大學三一學院及英國皇家音樂學院，並以小提琴家的身分展開國際演奏生涯，之後創立了一個屢獲殊榮的線上音樂會平台。此後，她轉向人工智慧領域，於帝國理工商學院完成人工智慧與機器學習課程，不僅取得該屆最高成績，也在期末專題的貝葉斯最佳化競賽中獲得冠軍。',
          '她的經驗涵蓋完整的 AI 開發流程，從資料蒐集與分析、客製化模型建構，到大型系統的後訓練與評估。她亦定期發表探討 AI 對齊與認知等主題的文章。在 JoinCare，Natasha 以跨領域視野與嚴謹的技術能力，推動平台 CareAgents 的開發與部署。',
        ],
      },
      {
        position: 'COO',
        name: 'Quentin Liu（劉君庭）',
        role: '營運長',
        image: '/team-assets/quentin-liu.jpg',
        monogram: 'QL',
        visualLabel: '營運 · 投資 · 科技',
        alt: 'JoinCare 營運長 Quentin Liu（劉君庭）',
        linkedin: 'https://www.linkedin.com/in/quentin-liu-53a121336/',
        bio: [
          'Quentin Liu（劉君庭）是跨境科技投資人與營運實務專家，擁有電腦科學碩士學位及倫敦商學院 MBA。他曾任職於可口可樂公司與 SAP，參與全球供應鏈系統、企業數位轉型及跨境業務營運。',
          '他的經驗涵蓋醫療健康、新能源與先進製造，專長於技術盡職調查、項目評估、跨境資源整合與投後價值創造。在 JoinCare，他以務實、跨領域的方式推動營運執行、全球合作夥伴關係，以及早期生醫創新的落地。',
        ],
      },
    ],
    principlesKicker: '我們的工作方式',
    principles: [
      ['科學嚴謹性', '重視可驗證研究、透明評審，以及明確可衡量的轉譯價值。'],
      ['全球協作', '連接不同地區的實驗室、創辦者、臨床專家與社群貢獻者。'],
      ['以患者為中心的轉譯', '每個項目都會以其是否能讓生醫想法更接近真實需求來評估。'],
    ],
  },
}

const t = computed(() => COPY[props.language === 'en' ? 'en' : 'tw'])
</script>

<template>
  <div class="team-page" :class="{ 'home-dark': isHomeDark }">
    <JcFloatingNavbar active="" />

    <main class="team-main">
      <section class="team-hero">
        <div class="team-hero-copy">
          <p class="team-kicker">{{ t.kicker }}</p>
          <h1 class="team-hero-title">
            <span class="team-title-main">{{ t.titleMain }}</span>
            <span class="team-title-accent font-playfair italic font-normal">{{ t.titleAccent }}</span>
          </h1>
          <p>{{ t.lead }}</p>
        </div>
        <div class="team-hero-mark" aria-hidden="true">
          <span />
          <span />
          <span />
          <span />
        </div>
      </section>

      <section
        v-for="(member, index) in t.members"
        :key="member.position"
        class="profile-section"
        :class="{ 'profile-section--reverse': index % 2 === 1 }"
      >
        <div class="profile-photo-wrap" :class="{ 'profile-photo-wrap--fallback': !member.image }">
          <img
            v-if="member.image"
            :src="member.image"
            :alt="member.alt"
            class="profile-photo"
          >
          <div v-else class="profile-monogram-panel" role="img" :aria-label="member.alt">
            <span class="profile-orbit profile-orbit--one" aria-hidden="true" />
            <span class="profile-orbit profile-orbit--two" aria-hidden="true" />
            <span class="profile-monogram">{{ member.monogram }}</span>
            <span class="profile-visual-label">{{ member.visualLabel }}</span>
          </div>
        </div>

        <article class="profile-copy" :aria-labelledby="`team-member-${index}`">
          <p class="team-kicker">{{ member.position }}</p>
          <div class="profile-heading">
            <h2 :id="`team-member-${index}`">{{ member.name }}</h2>
            <a
              v-if="member.linkedin"
              :href="member.linkedin"
              target="_blank"
              rel="noopener noreferrer"
              class="linkedin-link"
              :aria-label="`${member.name} ${t.linkedin}`"
            >
              {{ t.linkedin }}
            </a>
          </div>
          <p class="profile-role">{{ member.role }}</p>
          <div class="profile-body">
            <p v-for="paragraph in member.bio" :key="paragraph">{{ paragraph }}</p>
          </div>
        </article>
      </section>

      <section class="principles-section">
        <p class="team-kicker">{{ t.principlesKicker }}</p>
        <div class="principles-grid">
          <article v-for="item in t.principles" :key="item[0]" class="principle-card">
            <span class="principle-dot" />
            <h3>{{ item[0] }}</h3>
            <p>{{ item[1] }}</p>
          </article>
        </div>
      </section>
    </main>

    <HomeFooter :language="props.language" />
  </div>
</template>

<style scoped>
.team-page {
  min-height: 100vh;
  color: #181818;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(246, 246, 241, 0.95)),
    url('/figma/v2/noise-bg.png');
  background-size: auto, 500px 500px;
}

.team-page.home-dark {
  color: #f5f5ef;
  background:
    linear-gradient(180deg, rgba(35, 35, 34, 0.96), rgba(26, 26, 25, 0.98)),
    url('/figma/v2/noise-bg.png');
  background-size: auto, 500px 500px;
}

.team-main {
  width: min(1180px, calc(100vw - 48px));
  margin: 0 auto;
  padding: 150px 0 96px;
}

.team-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  gap: 56px;
  align-items: end;
  padding-bottom: 72px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.home-dark .team-hero {
  border-bottom-color: rgba(255, 255, 255, 0.1);
}

.team-kicker {
  margin: 0;
  color: #86e800;
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 0.18em;
  line-height: 1.2;
  text-transform: uppercase;
}

.team-hero-title {
  max-width: 900px;
  margin: 20px 0 0;
  font-size: clamp(48px, 7vw, 92px);
  letter-spacing: -0.045em;
  line-height: 0.98;
}

.team-title-main,
.team-title-accent {
  display: block;
}

.team-title-main {
  font-weight: 600;
}

.team-title-accent {
  margin-top: 0.04em;
  font-weight: 400;
  letter-spacing: -0.035em;
}

.team-hero-copy > p:last-child {
  max-width: 720px;
  margin: 28px 0 0;
  color: rgba(24, 24, 24, 0.68);
  font-size: 20px;
  line-height: 1.65;
}

.home-dark .team-hero-copy > p:last-child {
  color: rgba(245, 245, 239, 0.72);
}

.team-hero-mark {
  position: relative;
  width: 220px;
  height: 220px;
}

.team-hero-mark span {
  position: absolute;
  width: 52px;
  height: 52px;
  border-radius: 8px;
  background: #86e800;
}

.team-hero-mark span:nth-child(1) { right: 30px; top: 20px; }
.team-hero-mark span:nth-child(2) { left: 26px; top: 76px; }
.team-hero-mark span:nth-child(3) { right: 82px; bottom: 28px; }
.team-hero-mark span:nth-child(4) {
  left: 72px;
  bottom: 78px;
  background: transparent;
  border: 1px dashed rgba(134, 232, 0, 0.55);
}

.profile-section {
  display: grid;
  grid-template-columns: minmax(280px, 380px) minmax(0, 1fr);
  gap: 56px;
  align-items: stretch;
  padding: 72px 0;
}

.profile-section + .profile-section {
  border-top: 1px solid rgba(0, 0, 0, 0.1);
}

.home-dark .profile-section + .profile-section {
  border-top-color: rgba(255, 255, 255, 0.1);
}

.profile-section--reverse {
  grid-template-columns: minmax(0, 1fr) minmax(280px, 380px);
}

.profile-section--reverse .profile-photo-wrap {
  order: 2;
}

.profile-section--reverse .profile-copy {
  order: 1;
}

.profile-photo-wrap,
.profile-copy,
.principle-card {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.08);
}

.home-dark .profile-photo-wrap,
.home-dark .profile-copy,
.home-dark .principle-card {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(18, 18, 18, 0.72);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.3);
}

.profile-photo-wrap {
  overflow: hidden;
  min-height: 520px;
}

.profile-photo {
  width: 100%;
  height: 100%;
  min-height: 520px;
  display: block;
  object-fit: cover;
  object-position: center top;
}

.profile-photo-wrap--fallback {
  position: relative;
  display: grid;
  overflow: hidden;
  isolation: isolate;
  background:
    radial-gradient(circle at 78% 18%, rgba(134, 232, 0, 0.34), transparent 34%),
    linear-gradient(145deg, #f4f7ed 0%, #e7ecdf 100%);
}

.home-dark .profile-photo-wrap--fallback {
  background:
    radial-gradient(circle at 78% 18%, rgba(134, 232, 0, 0.24), transparent 34%),
    linear-gradient(145deg, #1b1d18 0%, #10110f 100%);
}

.profile-monogram-panel {
  position: relative;
  display: flex;
  min-height: 520px;
  flex-direction: column;
  align-items: flex-start;
  justify-content: flex-end;
  padding: 42px;
}

.profile-monogram-panel::before {
  position: absolute;
  z-index: -1;
  top: 42px;
  right: 42px;
  width: 22px;
  height: 22px;
  border-radius: 7px;
  background: #86e800;
  box-shadow:
    -44px 54px 0 rgba(134, 232, 0, 0.7),
    -8px 104px 0 rgba(134, 232, 0, 0.34);
  content: '';
}

.profile-orbit {
  position: absolute;
  z-index: -1;
  border: 1px solid rgba(24, 24, 24, 0.12);
  border-radius: 50%;
}

.home-dark .profile-orbit {
  border-color: rgba(255, 255, 255, 0.12);
}

.profile-orbit--one {
  top: 88px;
  left: 52px;
  width: 230px;
  height: 230px;
}

.profile-orbit--two {
  top: 136px;
  left: 100px;
  width: 134px;
  height: 134px;
  border-style: dashed;
  border-color: rgba(134, 232, 0, 0.72);
}

.profile-monogram {
  font-size: clamp(92px, 10vw, 148px);
  font-weight: 600;
  letter-spacing: -0.09em;
  line-height: 0.78;
}

.profile-visual-label {
  max-width: 240px;
  margin-top: 28px;
  color: rgba(24, 24, 24, 0.62);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.14em;
  line-height: 1.5;
  text-transform: uppercase;
}

.home-dark .profile-visual-label {
  color: rgba(245, 245, 239, 0.62);
}

.profile-copy {
  padding: 44px;
}

.profile-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  margin-top: 18px;
}

.profile-heading h2 {
  margin: 0;
  font-size: clamp(32px, 4vw, 58px);
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1.04;
}

.linkedin-link {
  display: inline-flex;
  align-items: center;
  min-height: 42px;
  padding: 0 18px;
  border-radius: 8px;
  color: #111;
  background: #86e800;
  font-size: 14px;
  font-weight: 800;
  text-decoration: none;
}

.profile-role {
  margin: 18px 0 0;
  color: rgba(24, 24, 24, 0.68);
  font-size: 18px;
  font-weight: 700;
}

.home-dark .profile-role {
  color: rgba(245, 245, 239, 0.68);
}

.profile-body {
  display: grid;
  gap: 20px;
  margin-top: 34px;
  color: rgba(24, 24, 24, 0.72);
  font-size: 17px;
  line-height: 1.75;
}

.profile-body p {
  margin: 0;
}

.home-dark .profile-body {
  color: rgba(245, 245, 239, 0.76);
}

.principles-section {
  padding-top: 8px;
}

.principles-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
  margin-top: 24px;
}

.principle-card {
  padding: 28px;
}

.principle-dot {
  display: block;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  background: #86e800;
}

.principle-card h3 {
  margin: 22px 0 0;
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1.18;
}

.principle-card p {
  margin: 14px 0 0;
  color: rgba(24, 24, 24, 0.66);
  font-size: 15px;
  line-height: 1.65;
}

.home-dark .principle-card p {
  color: rgba(245, 245, 239, 0.68);
}

@media (max-width: 900px) {
  .team-main {
    width: min(100vw - 28px, 680px);
    padding: 118px 0 64px;
  }

  .team-hero {
    grid-template-columns: 1fr;
    gap: 28px;
    padding-bottom: 48px;
  }

  .team-hero-title {
    font-size: clamp(40px, 12vw, 64px);
    letter-spacing: -0.04em;
  }

  .team-hero-copy > p:last-child {
    font-size: 17px;
  }

  .team-hero-mark {
    width: 170px;
    height: 72px;
  }

  .team-hero-mark span {
    width: 32px;
    height: 32px;
  }

  .team-hero-mark span:nth-child(1) { left: 0; top: 10px; right: auto; }
  .team-hero-mark span:nth-child(2) { left: 46px; top: 30px; }
  .team-hero-mark span:nth-child(3) { left: 92px; top: 14px; right: auto; bottom: auto; }
  .team-hero-mark span:nth-child(4) { left: 138px; top: 28px; bottom: auto; }

  .profile-section {
    grid-template-columns: 1fr;
    gap: 20px;
    padding: 48px 0;
  }

  .profile-section--reverse {
    grid-template-columns: 1fr;
  }

  .profile-section--reverse .profile-photo-wrap,
  .profile-section--reverse .profile-copy {
    order: initial;
  }

  .profile-photo-wrap,
  .profile-photo {
    min-height: 420px;
  }

  .profile-monogram-panel {
    min-height: 420px;
    padding: 30px;
  }

  .profile-orbit--one {
    top: 62px;
    left: 34px;
    width: 190px;
    height: 190px;
  }

  .profile-orbit--two {
    top: 102px;
    left: 74px;
    width: 112px;
    height: 112px;
  }

  .profile-copy {
    padding: 28px;
  }

  .profile-heading h2 {
    font-size: clamp(30px, 9vw, 44px);
    overflow-wrap: anywhere;
  }

  .principles-grid {
    grid-template-columns: 1fr;
  }
}
</style>

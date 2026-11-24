import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { WagmiPlugin } from '@wagmi/vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import router from './router'
import './style.css'
import App from './App.vue'
import { bootstrapCopywriting } from './services/copywriting'
// 初始化 Reown AppKit + Wagmi（副作用：注册 <w3m-* /> Web Components）
import { wagmiConfig } from './services/reownAppKit'
import { printBanner } from './utils/banner'

printBanner()

// 关闭浏览器原生滚动恢复：刷新时不再跳回上次位置
if ('scrollRestoration' in window.history) {
  window.history.scrollRestoration = 'manual'
}
window.scrollTo(0, 0)

const app = createApp(App)
const pinia = createPinia()
const queryClient = new QueryClient()

app.use(pinia)
app.use(router)
app.use(WagmiPlugin, { config: wagmiConfig })
app.use(VueQueryPlugin, { queryClient })

// 后端 copywriting_config 覆盖静态 TRANSLATIONS（mount 前同步等，确保首屏拿到最新文案）；
// 失败静默，回退到 i18n/*.js 内置文案。
bootstrapCopywriting().finally(() => {
  app.mount('#app')
})

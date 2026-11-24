import { onBeforeUnmount, onMounted, ref } from 'vue'

// 移动端断点：<= 768px 判定为 mobile
// 单例 ref，跨组件共享
const MOBILE_BREAKPOINT = 768
const isMobile = ref(false)

let mediaQuery = null
let listenerRegistered = false

function updateIsMobile() {
  if (typeof window === 'undefined') return
  isMobile.value = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`).matches
}

function registerListener() {
  if (listenerRegistered || typeof window === 'undefined') return
  mediaQuery = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`)
  isMobile.value = mediaQuery.matches
  // 兼容旧 Safari
  const handler = () => (isMobile.value = mediaQuery.matches)
  if (mediaQuery.addEventListener) mediaQuery.addEventListener('change', handler)
  else mediaQuery.addListener(handler)
  listenerRegistered = true
}

/**
 * 使用示例:
 *   const { isMobile } = useIsMobile()
 *   <div v-if="isMobile">Mobile Layout</div>
 *   <div v-else>Desktop Layout</div>
 */
export function useIsMobile() {
  onMounted(() => {
    registerListener()
    updateIsMobile()
  })
  onBeforeUnmount(() => {
    // 保持全局 ref，不 unregister listener（多组件共享）
  })
  return { isMobile }
}

// SSR / 立即取值场景
export function getIsMobile() {
  if (typeof window === 'undefined') return false
  return window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT}px)`).matches
}

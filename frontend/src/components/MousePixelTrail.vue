<script setup>
/**
 * <MousePixelTrail /> — 鼠标移动时拖一条由小方格组成的轨迹
 *  · 移动 → 每隔 N 像素生成一个小方块
 *  · 停止 → 整条尾迹快速淡出
 *  · 全局固定层（fixed inset:0），pointer-events:none，不影响交互
 */
import { ref, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  color:        { type: String, default: '#9BFF00' },
  size:         { type: Number, default: 36 },         // 单格 px（与首页 PixelGrid 统一）
  /** 尾迹最大长度（同时存在的方块数） */
  maxLength:    { type: Number, default: 5 },
  /** 鼠标移动多少像素才掉一个新方块 */
  spawnEvery:   { type: Number, default: 55 },
  /** 单个方块寿命 ms（结束直接 snap 消失） */
  lifeMs:       { type: Number, default: 320 },
  /** 鼠标静止多少 ms 后整体瞬间清零 */
  idleFadeMs:   { type: Number, default: 90 },
  /** 每个方块的随机偏移 px */
  jitter:       { type: Number, default: 24 },
})

const dots = ref([])
let nextId = 0
let lastX = -9999, lastY = -9999
let lastMoveTime = 0
let rafId = null

const rand = (max) => (Math.random() * 2 - 1) * max

const onMove = (e) => {
  const now = performance.now()
  const dx = e.clientX - lastX
  const dy = e.clientY - lastY
  const dist = Math.hypot(dx, dy)
  lastMoveTime = now
  if (dist < props.spawnEvery) return
  lastX = e.clientX
  lastY = e.clientY
  dots.value.push({
    id: nextId++,
    x: e.clientX + rand(props.jitter),
    y: e.clientY + rand(props.jitter),
    born: now,
    baseAlpha: 0.6 + Math.random() * 0.4, // 60%~100% 基础透明度
  })
  // Cap length: drop oldest
  if (dots.value.length > props.maxLength) {
    dots.value.splice(0, dots.value.length - props.maxLength)
  }
}

const tick = (now) => {
  const timeSinceMove = now - lastMoveTime
  // 鼠标停止 idleFadeMs 后：所有方块立即清零（"粗暴"）
  if (timeSinceMove > props.idleFadeMs) {
    if (dots.value.length) dots.value = []
    rafId = requestAnimationFrame(tick)
    return
  }
  // 否则每个方块保持 baseAlpha 直到寿命结束，瞬间消失
  // 仅在生命最后 25% 才掉一点 alpha，做最低限度过渡
  const next = []
  for (const d of dots.value) {
    const age = now - d.born
    if (age >= props.lifeMs) continue
    const lifeT = age / props.lifeMs
    const tail = lifeT > 0.75 ? (1 - (lifeT - 0.75) / 0.25) : 1
    d.opacity = d.baseAlpha * tail
    next.push(d)
  }
  dots.value = next
  rafId = requestAnimationFrame(tick)
}

onMounted(() => {
  window.addEventListener('mousemove', onMove, { passive: true })
  rafId = requestAnimationFrame(tick)
})
onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onMove)
  if (rafId) cancelAnimationFrame(rafId)
})
</script>

<template>
  <div class="mp-trail-root" aria-hidden="true">
    <span
      v-for="d in dots" :key="d.id"
      class="mp-trail-pixel"
      :style="{
        background: color,
        width: size + 'px',
        height: size + 'px',
        left: (d.x - size / 2) + 'px',
        top: (d.y - size / 2) + 'px',
        opacity: d.opacity ?? 1,
      }"
    ></span>
  </div>
</template>

<style scoped>
.mp-trail-root {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 9998;        /* 低于 splash(1000) 和 modal，但高于普通内容 */
}
.mp-trail-pixel {
  position: fixed;
  border-radius: 2px;
  pointer-events: none;
  will-change: opacity, transform;
}
</style>

<script setup>
/**
 * <PixelGrid /> — Conway's Game of Life · ASCII 字符矩阵
 *  · 每个格子是 monospace 字符（默认 ■）
 *  · 每隔 stepMs 应用康威生命游戏规则（环面拓扑、四角连边）
 *  · 模式陷入静态/振荡若干步后自动注入新种子，保持生命循环
 *  · IntersectionObserver 视口外暂停
 */
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'

const props = defineProps({
  cols:            { type: Number, default: 12 },
  rows:            { type: Number, default: 12 },
  color:           { type: String, default: '#9BFF00' },
  cellSize:        { type: String, default: '14px' },
  gap:             { type: String, default: '0' },
  /** 活细胞的字符 — 默认 █ 满块字符（紧贴无缝） */
  glyph:           { type: String, default: '█' },
  /** 鼠标干扰半径（以 cellSize 为单位），0 = 关闭 */
  mouseDisturbRadius: { type: Number, default: 2.5 },
  /** 鼠标干扰强度（0~1，越大覆盖力越强） */
  mouseDisturbStrength: { type: Number, default: 0.45 },
  /** 初始随机活细胞比例 0~1（从稀疏起步，由生命游戏自然生长） */
  initialDensity:  { type: Number, default: 0.08 },
  /** 每一代的间隔（ms） */
  stepMs:          { type: Number, default: 380 },
  /** 多少代无变化就重新注入新种子 */
  staleSteps:      { type: Number, default: 5 },
  /** 永远不参与（占位掩码） */
  maskIndexes:     { type: Array, default: () => [] },
  /** 全死后是否自动重新洒种子 */
  autoReseed:      { type: Boolean, default: true },

  // === 兼容旧版 props（已被生命游戏取代，保留 no-op 不破坏旧模板） ===
  borderRadius:    { type: String, default: '0' },
  glowRadius:      { type: Number, default: 0 },
  scaleMin:        { type: Number, default: 0 },
  flashDuration:   { type: Number, default: 0 },
  idleMin:         { type: Number, default: 0 },
  idleMax:         { type: Number, default: 0 },
  startStaggerMax: { type: Number, default: 0 },
  jitter:          { type: Number, default: 0 },
  jitterRotate:    { type: Number, default: 0 },
  jitterScale:     { type: Number, default: 0 },
  growMs:          { type: Number, default: 0 },
  aliveMinMs:      { type: Number, default: 0 },
  aliveMaxMs:      { type: Number, default: 0 },
  dieMs:           { type: Number, default: 0 },
  propagateProb:   { type: Number, default: 0 },
  spreadDelayMs:   { type: Number, default: 0 },
  seedCount:       { type: Number, default: 0 },
})

const root = ref(null)
const grid = ref([])   // boolean[]，对应每个 cell 当前是否活
let stepTimer = null
let observer = null
let running = false
let prevHash = ''
let staleCount = 0

const total = () => props.cols * props.rows
const inMask = (i) => props.maskIndexes.includes(i)

// 每个 cell 一个独立 alpha，随机分配；细胞重生时重新随机
const randAlpha = () => 0.45 + Math.random() * 0.55

const seed = () => {
  const n = total()
  grid.value = Array.from({ length: n }, (_, i) => ({
    alive: !inMask(i) && Math.random() < props.initialDensity,
    alpha: randAlpha(),
  }))
  prevHash = ''
  staleCount = 0
}

// 环面（toroidal）邻居计数 — 边缘会绕回另一侧，小网格也能产生 glider 等漂移图案
const countLive = (i) => {
  const c = i % props.cols
  const r = (i / props.cols) | 0
  let cnt = 0
  for (let dr = -1; dr <= 1; dr++) {
    for (let dc = -1; dc <= 1; dc++) {
      if (dr === 0 && dc === 0) continue
      const nr = (r + dr + props.rows) % props.rows
      const nc = (c + dc + props.cols) % props.cols
      if (grid.value[nr * props.cols + nc]?.alive) cnt++
    }
  }
  return cnt
}

const step = () => {
  const n = total()
  const next = new Array(n)
  let liveCount = 0
  for (let i = 0; i < n; i++) {
    if (inMask(i)) { next[i] = { alive: false, alpha: 1 }; continue }
    const cell = grid.value[i]
    // 正在被鼠标"消灭"中的格子 — 冻结状态，等定时器把它清零，
    // 不参与本次康威规则，以免被复活或提前消失
    if (cell.dying) { next[i] = cell; liveCount++; continue }
    const k = countLive(i)
    const survives = cell.alive ? (k === 2 || k === 3) : (k === 3)
    const alpha = !cell.alive && survives ? randAlpha() : cell.alpha
    next[i] = { alive: survives, alpha }
    if (survives) liveCount++
  }
  const hash = next.map(c => (c.alive ? 1 : 0)).join('')
  if (hash === prevHash || liveCount === 0) {
    staleCount++
    if (props.autoReseed && staleCount >= props.staleSteps) {
      const sprinkle = Math.max(3, Math.floor(n * 0.08))
      for (let s = 0; s < sprinkle; s++) {
        const k = (Math.random() * n) | 0
        if (!inMask(k)) next[k] = { alive: true, alpha: randAlpha() }
      }
      staleCount = 0
    }
  } else {
    staleCount = 0
  }
  prevHash = hash
  grid.value = next
}

const start = () => {
  if (running) return
  running = true
  if (grid.value.length === 0) seed()
  if (stepTimer) clearInterval(stepTimer)
  stepTimer = setInterval(step, props.stepMs)
}
const stop = () => {
  running = false
  if (stepTimer) { clearInterval(stepTimer); stepTimer = null }
}

seed()  // setup 期间就生成初始网格，避免 SSR / 首帧空白

// === 鼠标"消灭"效果：鼠标划过的活细胞 → 变灰停留 → 缩小消失 ===
// 与 CSS @keyframes pixel-death 总时长保持一致（1.1s）
const KILL_DURATION_MS = 1100
const killTimers = new Map() // i -> timeoutId

const scheduleDeath = (idx) => {
  if (killTimers.has(idx)) return
  const tid = setTimeout(() => {
    killTimers.delete(idx)
    const cur = grid.value[idx]
    if (cur?.dying) {
      const next = grid.value.slice()
      next[idx] = { alive: false, alpha: cur.alpha, dying: false }
      grid.value = next
    }
  }, KILL_DURATION_MS)
  killTimers.set(idx, tid)
}

let mouseRafScheduled = false
let lastMouseX = 0, lastMouseY = 0
const onMouseMove = (e) => {
  if (!props.mouseDisturbRadius || props.mouseDisturbRadius <= 0) return
  lastMouseX = e.clientX
  lastMouseY = e.clientY
  if (mouseRafScheduled) return
  mouseRafScheduled = true
  requestAnimationFrame(applyMouseDisturb)
}
const applyMouseDisturb = () => {
  mouseRafScheduled = false
  if (!root.value) return
  const rect = root.value.getBoundingClientRect()
  const mx = lastMouseX - rect.left
  const my = lastMouseY - rect.top
  const cellPx = parseFloat(props.cellSize) || 14
  const radius = cellPx * props.mouseDisturbRadius
  if (mx < -radius || my < -radius || mx > rect.width + radius || my > rect.height + radius) return
  const next = grid.value.slice()
  let changed = false
  for (let i = 0; i < next.length; i++) {
    if (inMask(i)) continue
    const cell = next[i]
    if (!cell.alive || cell.dying) continue
    const c = i % props.cols
    const r = (i / props.cols) | 0
    const cx = (c + 0.5) * cellPx
    const cy = (r + 0.5) * cellPx
    const dist = Math.hypot(mx - cx, my - cy)
    if (dist >= radius) continue
    const intensity = 1 - dist / radius
    if (Math.random() < intensity * props.mouseDisturbStrength) {
      next[i] = { alive: true, alpha: cell.alpha, dying: true }
      scheduleDeath(i)
      changed = true
    }
  }
  if (changed) grid.value = next
}

onMounted(() => {
  observer = new IntersectionObserver((es) => {
    for (const e of es) e.isIntersecting ? start() : stop()
  }, { threshold: 0 })
  if (root.value) observer.observe(root.value)
  window.addEventListener('mousemove', onMouseMove, { passive: true })
})

onBeforeUnmount(() => {
  stop()
  observer?.disconnect()
  window.removeEventListener('mousemove', onMouseMove)
  for (const tid of killTimers.values()) clearTimeout(tid)
  killTimers.clear()
})

watch(() => [props.cols, props.rows, JSON.stringify(props.maskIndexes)], () => {
  seed()
  if (running) { stop(); start() }
})
</script>

<template>
  <div
    ref="root"
    class="life-grid"
    :style="{
      gridTemplateColumns: `repeat(${cols}, ${cellSize})`,
      gridAutoRows: cellSize,
      gap,
      '--life-color': color,
    }"
  >
    <span
      v-for="(cell, i) in grid" :key="i"
      class="life-cell"
      :class="{ 'is-alive': cell.alive && !cell.dying, 'is-dying': cell.dying }"
      :style="{ '--alpha': cell.alpha }"
    ></span>
  </div>
</template>

<style scoped>
.life-grid {
  display: grid;
  width: fit-content;
}
.life-cell {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--life-color);
  opacity: 0;
  pointer-events: none;
  user-select: none;
  transition: opacity 0.14s linear;
  will-change: opacity, transform, background-color;
}
.life-cell.is-alive {
  opacity: var(--alpha, 1);
}
/* 杀死：先变灰停留 ~800ms，再缩小淡出。1.1s 与 KILL_DURATION_MS 同步 */
.life-cell.is-dying {
  animation: pixel-death 1.1s ease-in forwards;
}
@keyframes pixel-death {
  0%   { background: var(--life-color); transform: scale(1);   opacity: var(--alpha, 1); }
  9%   { background: #9CA3AF;           transform: scale(1);   opacity: var(--alpha, 1); } /* ~100ms 变灰 */
  82%  { background: #9CA3AF;           transform: scale(1);   opacity: var(--alpha, 1); } /* 9%→82% ≈ 800ms 停留 */
  100% { background: #9CA3AF;           transform: scale(0.18); opacity: 0; }              /* ~200ms 缩小淡出 */
}
</style>

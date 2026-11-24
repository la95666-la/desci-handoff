<script setup>
/**
 * <DnaModel /> — 纯 Three.js 程序化双螺旋
 *  · 两条 sin/cos 错相 π 的参数曲线 → TubeGeometry 作为外侧骨架
 *  · 36 对 InstancedMesh 圆柱作为碱基对
 *  · 240 个 InstancedMesh 小球散布在 helix 周围作为粒子
 *  · 同款玻璃材质 + 鼠标 / 滚动 / idle 自转交互
 *  · 不再依赖 GLB 文件
 */
import { ref, onMounted, onBeforeUnmount, inject, watch } from 'vue'
import * as THREE from 'three'
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js'

// 注入页面主题，决定 DNA 材质用浅 / 深两套灰
const isDark = inject('isHomeDark', ref(false))
// 暗主题：骨架颜色压暗一档；亮主题：当前的中灰
const palette = (dark) => ({
  backboneA: dark ? 0x2d3239 : 0x596068,
  backboneB: dark ? 0x2d362f : 0x59635a,
  attenuationA: dark ? 0x1a1e23 : 0x3d434c,
  attenuationB: dark ? 0x1a221c : 0x3d4740,
  basePair:   dark ? 0x121820 : 0x1a2028,
  bpAttenuation: dark ? 0x05080c : 0x0b1018,
})

const props = defineProps({
  modelScale: { type: Number, default: 0.6 },
  // ===== 双螺旋几何参数 =====
  helixLength:    { type: Number, default: 20 },    // 沿 Y 轴总高（两端可超出可见区）
  helixRadius:    { type: Number, default: 0.94 },  // 两条骨架到中心的半径
  tubeRadius:     { type: Number, default: 0.13 },  // 骨架管粗
  turns:          { type: Number, default: 5 },     // 总缠绕圈数（按长度等比加）
  basePairs:      { type: Number, default: 50 },    // 碱基对数（按长度等比加）
  basePairRadius: { type: Number, default: 0.05 },  // 碱基对圆柱半径
  basePairGap:    { type: Number, default: 0.18 },  // 中间断口宽度（世界单位）
  particleCount:  { type: Number, default: 140 },   // 周围粒子数（少而散）
  // ===== 交互参数 =====
  maxRotX:       { type: Number, default: 0.25 },
  maxRotY:       { type: Number, default: 0.4 },
  idleAfter:     { type: Number, default: 1500 },
  idleSpinSpeed: { type: Number, default: 0.002 },
  lerpFactor:    { type: Number, default: 0.06 },
})

const containerRef = ref(null)
let renderer, scene, camera, model
let animationFrame
let resizeObserver
let mouseMoveHandler
let lastMouseMoveTime = 0

const target  = { x: 0, y: 0 }
const current = { x: 0, y: 0 }
const baseRotation = { x: 0, y: 0, z: 0 }
let scrollTarget = 0
let scrollCurrent = 0
let baseCameraZ = 0
let scrollHandler

// === 程序生成双螺旋 group ===
function buildDnaGroup() {
  const group = new THREE.Group()
  const {
    helixLength: L, helixRadius: R, tubeRadius: T,
    turns, basePairs, basePairRadius, basePairGap, particleCount,
  } = props

  // 参数曲线：t∈[0,1] → 3D 螺旋点
  // 长轴沿 Y（垂直）— 与 GLB Y-up 导出一致，idle 自转才能绕螺旋中心轴转
  class HelixCurve extends THREE.Curve {
    constructor(phase) { super(); this.phase = phase }
    getPoint(t, dst = new THREE.Vector3()) {
      const a = t * Math.PI * 2 * turns + this.phase
      return dst.set(Math.cos(a) * R, (t - 0.5) * L, Math.sin(a) * R)
    }
  }

  // 玻璃材质：MeshPhysicalMaterial.transmission 提供折射"通透感"——
  // 不再依赖 alpha 透明 + depthWrite:false（会导致旋转时 InstancedMesh 内部背面盖前面 = 穿模）。
  // 结构件（骨架 + 碱基对）改为 transparent:false / depthWrite:true，靠 transmission 出玻璃感，深度排序正确
  const p = palette(isDark.value)
  const structuralParams = {
    color: p.basePair,
    transparent: false,
    transmission: 0.65,
    thickness: 1.2,
    roughness: 0.06,
    metalness: 0,
    ior: 1.5,
    clearcoat: 0.9,
    clearcoatRoughness: 0.04,
    attenuationColor: p.bpAttenuation,
    attenuationDistance: 2.2,
    envMapIntensity: 1.6,
    side: THREE.FrontSide,
    depthWrite: true,
  }
  const backboneMatA = new THREE.MeshPhysicalMaterial({
    ...structuralParams,
    color: p.backboneA,
    attenuationColor: p.attenuationA,
  })
  const backboneMatB = new THREE.MeshPhysicalMaterial({
    ...structuralParams,
    color: p.backboneB,
    attenuationColor: p.attenuationB,
  })
  // 碱基对：稍微透明（opacity 0.72），但仍 depthWrite=true，旋转不穿模
  const bpMat = new THREE.MeshPhysicalMaterial({
    ...structuralParams,
    transparent: true,
    opacity: 0.72,
    transmission: 0.75,
  })
  // 粒子：保留 alpha 透明，但也开 depthWrite=true，避免一团粒子之间互盖
  const particleMat = new THREE.MeshPhysicalMaterial({
    ...structuralParams,
    transparent: true,
    opacity: 0.55,
    transmission: 0.85,
    attenuationDistance: 4,
    depthWrite: true,
  })

  // 1) 两条骨架：TubeGeometry 沿 HelixCurve(0) 与 HelixCurve(π)，各自带不同偏色
  const curveA = new HelixCurve(0)
  const curveB = new HelixCurve(Math.PI)
  group.add(new THREE.Mesh(new THREE.TubeGeometry(curveA, 260, T, 12, false), backboneMatA))
  group.add(new THREE.Mesh(new THREE.TubeGeometry(curveB, 260, T, 12, false), backboneMatB))

  // 2) 碱基对：每对用两段 CapsuleGeometry —— 从各自骨架出发，中间留断口、端口呈圆形
  //    A、B 两点距离恒等于 2R（直径），所以两段 capsule 可共用同一个固定几何
  const fullDist = 2 * R
  const capsTotalLen = Math.max(basePairRadius * 2 + 0.001, (fullDist - basePairGap) / 2)
  const capsCylLen = Math.max(0.001, capsTotalLen - 2 * basePairRadius) // CapsuleGeometry.length 是圆柱段部分（不含两端球冠）
  const bpGeo = new THREE.CapsuleGeometry(basePairRadius, capsCylLen, 4, 8)
  const bpInst = new THREE.InstancedMesh(bpGeo, bpMat, basePairs * 2)
  const dummy = new THREE.Object3D()
  const pa = new THREE.Vector3(), pb = new THREE.Vector3()
  const upY = new THREE.Vector3(0, 1, 0)
  let bpIdx = 0
  for (let i = 0; i < basePairs; i++) {
    const t = i / (basePairs - 1)
    curveA.getPoint(t, pa)
    curveB.getPoint(t, pb)
    const dir = new THREE.Vector3().subVectors(pb, pa).normalize()
    // A 段中心：从 pa 沿 dir 向内 capsTotalLen/2
    const centerA = pa.clone().add(dir.clone().multiplyScalar(capsTotalLen / 2))
    // B 段中心：从 pb 沿 -dir 向内 capsTotalLen/2
    const centerB = pb.clone().add(dir.clone().multiplyScalar(-capsTotalLen / 2))
    for (const c of [centerA, centerB]) {
      dummy.position.copy(c)
      dummy.quaternion.setFromUnitVectors(upY, dir)
      dummy.scale.set(1, 1, 1)
      dummy.updateMatrix()
      bpInst.setMatrixAt(bpIdx++, dummy.matrix)
    }
  }
  bpInst.instanceMatrix.needsUpdate = true
  group.add(bpInst)

  // 3) 周围粒子：InstancedMesh 小球，随机散布在 helix 外圈圆柱壳
  const sGeo = new THREE.SphereGeometry(1, 8, 6)
  const sInst = new THREE.InstancedMesh(sGeo, particleMat, particleCount)
  for (let i = 0; i < particleCount; i++) {
    const t = Math.random()
    const ang = Math.random() * Math.PI * 2
    // 半径范围 0.5R ~ 3.0R（之前 0.4~1.8）→ 更散
    const r = R * (0.5 + Math.random() ** 0.7 * 2.5)
    const sz = 0.025 + Math.random() ** 2 * 0.07
    // Y 高度从 1.15L 拉到 1.45L（顶/底各超出 22.5%）
    dummy.position.set(Math.cos(ang) * r, (t - 0.5) * L * 1.45, Math.sin(ang) * r)
    dummy.quaternion.identity()
    dummy.scale.setScalar(sz)
    dummy.updateMatrix()
    sInst.setMatrixAt(i, dummy.matrix)
  }
  sInst.instanceMatrix.needsUpdate = true
  group.add(sInst)

  return group
}

onMounted(() => {
  const container = containerRef.value
  if (!container) return

  const getSize = () => ({
    w: container.clientWidth || 600,
    h: container.clientHeight || 600,
  })
  const { w, h } = getSize()

  scene = new THREE.Scene()

  camera = new THREE.PerspectiveCamera(45, w / h, 0.1, 100)
  camera.position.set(0, 0, 12)
  camera.lookAt(0, 0, 0)

  renderer = new THREE.WebGLRenderer({
    alpha: true,
    antialias: true,
    powerPreference: 'high-performance',
  })
  renderer.setSize(w, h)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.setClearColor(0x000000, 0)
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.05
  container.appendChild(renderer.domElement)

  // 玻璃折射 / 反射需要环境贴图
  const pmrem = new THREE.PMREMGenerator(renderer)
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture

  scene.add(new THREE.AmbientLight(0x222a33, 0.25))
  const key = new THREE.DirectionalLight(0xffffff, 0.8)
  key.position.set(4, 6, 5); scene.add(key)
  const rim = new THREE.DirectionalLight(0xc8d8f0, 0.9)
  rim.position.set(-3, 2, -6); scene.add(rim)

  // === 程序生成 DNA === (替代 GLB load)
  model = buildDnaGroup()

  // 居中 + 整体缩放
  model.scale.setScalar(props.modelScale)
  const box = new THREE.Box3().setFromObject(model)
  const center = box.getCenter(new THREE.Vector3())
  const size = box.getSize(new THREE.Vector3())
  model.position.sub(center)
  model.position.x += size.x * 0.35     // 向右偏

  // 与原 GLB 一致的初始斜放
  model.rotation.z = -0.38
  model.rotation.y =  0.25
  model.rotation.x =  0.08

  scene.add(model)

  // 相机距离 —— Y 维用固定值（6.8 世界单位）
  // 加长 helix 时画面不再拉远，多出来的螺旋会自然超出可见区两端
  const targetDim = Math.max(size.x * 1.2, 6.8)
  const fov = camera.fov * Math.PI / 180
  const dist = (targetDim / 2) / Math.tan(fov / 2) * 0.95    // 系数越小越近
  camera.position.set(0, 0, dist)
  camera.lookAt(0, 0, 0)

  baseRotation.x = model.rotation.x
  baseRotation.y = model.rotation.y
  baseRotation.z = model.rotation.z
  baseCameraZ = camera.position.z

  // === 主题切换：实时更新 DNA 材质颜色（避免重建几何） ===
  // model.children 顺序：[0] backboneA tube, [1] backboneB tube,
  // [2] base pairs InstancedMesh, [3] particles InstancedMesh
  watch(isDark, (v) => {
    if (!model) return
    const np = palette(v)
    const mats = model.children.map(c => c.material).filter(Boolean)
    const update = (m, color, attn) => {
      m.color.setHex(color)
      if (m.attenuationColor) m.attenuationColor.setHex(attn)
      m.needsUpdate = true
    }
    if (mats[0]) update(mats[0], np.backboneA, np.attenuationA)
    if (mats[1]) update(mats[1], np.backboneB, np.attenuationB)
    if (mats[2]) update(mats[2], np.basePair,  np.bpAttenuation)
    if (mats[3]) update(mats[3], np.basePair,  np.bpAttenuation)
  })

  // === Mouse move tracking ===
  mouseMoveHandler = (e) => {
    const rect = container.getBoundingClientRect()
    const cx = rect.left + rect.width / 2
    const cy = rect.top + rect.height / 2
    const nx = (e.clientX - cx) / (window.innerWidth / 2)
    const ny = (e.clientY - cy) / (window.innerHeight / 2)
    target.y = Math.max(-1, Math.min(1, nx)) * props.maxRotY
    target.x = Math.max(-1, Math.min(1, ny)) * props.maxRotX
    lastMouseMoveTime = performance.now()
  }
  window.addEventListener('mousemove', mouseMoveHandler, { passive: true })

  // === Scroll dolly ===
  scrollHandler = () => {
    const sy = window.scrollY || window.pageYOffset || 0
    scrollTarget = Math.min(1, Math.max(0, sy / 800))
  }
  window.addEventListener('scroll', scrollHandler, { passive: true })
  scrollHandler()

  // === Resize ===
  const onResize = () => {
    if (!container) return
    const { w, h } = getSize()
    if (w === 0 || h === 0) return
    camera.aspect = w / h
    camera.updateProjectionMatrix()
    renderer.setSize(w, h)
  }
  resizeObserver = new ResizeObserver(onResize)
  resizeObserver.observe(container)

  const animate = () => {
    animationFrame = requestAnimationFrame(animate)
    if (baseCameraZ > 0) {
      scrollCurrent += (scrollTarget - scrollCurrent) * 0.08
      camera.position.z = baseCameraZ * (1 - scrollCurrent * 0.55)
    }
    if (model) {
      const idle = performance.now() - lastMouseMoveTime > props.idleAfter
      if (idle) target.y += props.idleSpinSpeed
      current.x += (target.x - current.x) * props.lerpFactor
      current.y += (target.y - current.y) * props.lerpFactor
      const scrollTilt = scrollCurrent * 1.05
      model.rotation.x = baseRotation.x + current.x + scrollTilt
      model.rotation.y = baseRotation.y + current.y
      model.rotation.z = baseRotation.z
    }
    renderer.render(scene, camera)
  }
  animate()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animationFrame)
  window.removeEventListener('mousemove', mouseMoveHandler)
  if (scrollHandler) window.removeEventListener('scroll', scrollHandler)
  if (resizeObserver) resizeObserver.disconnect()
  if (renderer) {
    renderer.dispose()
    if (renderer.domElement.parentNode) {
      renderer.domElement.parentNode.removeChild(renderer.domElement)
    }
  }
  if (scene) {
    scene.traverse((obj) => {
      if (obj.geometry) obj.geometry.dispose()
      if (obj.material) {
        const mats = Array.isArray(obj.material) ? obj.material : [obj.material]
        mats.forEach((m) => m.dispose?.())
      }
    })
  }
})
</script>

<template>
  <div ref="containerRef" class="dna-model-host"></div>
</template>

<style scoped>
.dna-model-host {
  position: relative;
  width: 100%;
  height: 100%;
  pointer-events: none;
  overflow: hidden;
}
.dna-model-host :deep(canvas) {
  display: block;
  width: 100% !important;
  height: 100% !important;
}
</style>

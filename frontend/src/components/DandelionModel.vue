<script setup>
/**
 * <DandelionModel /> — 程序化蒲公英（puff ball 阶段）
 *  · 两株组合：一株「高 + 大」 + 一株「矮 + 小」
 *  · 每株：弯曲柄（CatmullRomCurve3 + TubeGeometry）+ 中心花托 + 辐射种子 + 顶端绒毛伞
 *  · 主题双色（浅 / 深）+ 鼠标 / 滚动 / idle 自转交互
 */
import { ref, onMounted, onBeforeUnmount, inject, watch } from 'vue'
import * as THREE from 'three'
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js'

const isDark = inject('isHomeDark', ref(false))
const palette = (dark) => ({
  // 暗背景：高亮白 ↔ 亮背景：中性灰（保留"白绒毛"质感，仅压暗使可见）
  hair:       dark ? 0xd6e0ec : 0x808080, // 暗：冷白；亮：偏深中性灰
  receptacle: dark ? 0x4a3a1f : 0x6a6055, // 暗：深棕；亮：低饱和暖暗灰（融入整体灰）
  seedStem:   dark ? 0x9aa3ad : 0x707070, // 种子柄；亮模式深中性灰
  groundStem: dark ? 0x4a6b3a : 0x6b8a4a, // 茎绿（原值）
})

const props = defineProps({
  modelScale:    { type: Number, default: 0.42 },  // 缩小 — 之前 0.55 太大
  // 交互
  maxRotX:       { type: Number, default: 0.18 },
  maxRotY:       { type: Number, default: 0.28 },
  idleAfter:     { type: Number, default: 1500 },
  idleSpinSpeed: { type: Number, default: 0.002 },
  lerpFactor:    { type: Number, default: 0.06 },
  // 只渲染右侧一株大蒲公英（移动端空间紧张时用）
  singleStalk:   { type: Boolean, default: false },
})

const containerRef = ref(null)
let renderer, scene, camera, model
let animationFrame
let resizeObserver
let mouseMoveHandler, scrollHandler, mouseLeaveHandler
let lastMouseMoveTime = 0

const target = { x: 0, y: 0 }
const current = { x: 0, y: 0 }
// 鼠标是否在 hero canvas 内（飞絮聚拢 / 散开开关）
const mouseInHero = { value: false }
// 鼠标在世界坐标的位置（z=0 平面投影），用于飞絮聚拢
const mouseWorld = { x: 0, y: 0 }
const baseRotation = { x: 0, y: 0, z: 0 }
let baseCameraZ = 0
let scrollTarget = 0, scrollCurrent = 0

// 收集所有材质供主题切换批量更新
const themedMaterials = { hair: [], receptacle: [], seedStem: [], groundStem: [] }

function fibonacciDir(i, total, out) {
  const phi = Math.acos(1 - 2 * (i + 0.5) / total)
  const theta = Math.PI * (1 + Math.sqrt(5)) * (i + 0.5)
  out.set(
    Math.sin(phi) * Math.cos(theta),
    Math.sin(phi) * Math.sin(theta),
    Math.cos(phi),
  )
  return out
}

/**
 * 单株蒲公英：弯柄 + puff（中心花托 + 辐射种子 + 顶端绒毛伞）
 * @param {object} cfg
 *   stemHeight   茎从地面到 puff 中心的高度
 *   bendDir      +1 / -1 弯曲朝向（x 正 / 负）
 *   bendAmount   弯曲幅度（世界单位）
 *   stemRadius   茎粗
 *   puffRadius   puff 半径（种子柄长度）
 *   seedCount    种子数
 *   hairsPerSeed 每颗顶端绒毛数
 *   hairLength   单根绒毛长度
 *   coneAngle    伞冠张开半角（弧度）
 *   receptacleR  花托球半径
 */
function buildSingleDandelion(cfg) {
  const {
    stemHeight, bendDir, bendAmount, stemRadius,
    puffRadius, seedCount, hairsPerSeed, hairLength, coneAngle, receptacleR,
    baseOffsetX = 0,  // 茎根相对 puff 的 X 偏移（正值 = 根向 +X，茎向 -X 倾）
  } = cfg
  const p = palette(isDark.value)
  const group = new THREE.Group()

  // ===== 茎（geometry + vertex shader 动态弯曲）=====
  // 静态：从 (baseOffsetX, -stemHeight, 0) 走到 (0, 0, 0) — puff 在原位，根可侧偏
  // 运行时：shader 按 progress² × uWind 把上段往 X 推（progress=0 在基部不动）
  const stemCurve = new THREE.CatmullRomCurve3([
    new THREE.Vector3(baseOffsetX, -stemHeight, 0),
    new THREE.Vector3(baseOffsetX * 0.55 + bendDir * bendAmount * 0.10, -stemHeight * 0.5, 0),
    new THREE.Vector3(baseOffsetX * 0.15, -stemHeight * 0.15, 0),
    new THREE.Vector3(0, 0, 0),
  ])
  const stemGeo = new THREE.TubeGeometry(stemCurve, 48, stemRadius, 8, false)
  const stemMat = new THREE.MeshStandardMaterial({
    color: p.groundStem, roughness: 0.88, metalness: 0,
  })
  // 每株一份 uniforms，存在 group.userData 供 animate 推数据
  const windUniforms = {
    uWind:       { value: 0 },
    uStemHeight: { value: stemHeight },
  }
  stemMat.onBeforeCompile = (shader) => {
    shader.uniforms.uWind = windUniforms.uWind
    shader.uniforms.uStemHeight = windUniforms.uStemHeight
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', `
        #include <common>
        uniform float uWind;
        uniform float uStemHeight;
      `)
      .replace('#include <begin_vertex>', `
        // progress = 0 at base (y=-stemHeight), 1 at top (y=0)
        float jcP = clamp((position.y + uStemHeight) / uStemHeight, 0.0, 1.0);
        vec3 transformed = vec3(position.x + jcP * jcP * uWind, position.y, position.z);
      `)
  }
  group.add(new THREE.Mesh(stemGeo, stemMat))
  themedMaterials.groundStem.push(stemMat)
  // 给外部 animate 用：每帧设 uWind + 同步 puffGroup.position.x = uWind
  group.userData.windUniforms = windUniforms

  // ===== puff 子 group：receptacle + seedInst + hairInst 整体随 stem tip 平移 =====
  const puffGroup = new THREE.Group()
  group.add(puffGroup)
  group.userData.puffGroup = puffGroup

  // 花托
  const recMat = new THREE.MeshPhysicalMaterial({
    color: p.receptacle, roughness: 0.65, metalness: 0.08,
  })
  puffGroup.add(new THREE.Mesh(
    new THREE.SphereGeometry(receptacleR, 24, 16),
    recMat,
  ))
  themedMaterials.receptacle.push(recMat)

  // ===== 种子柄（puff 内辐射，连接花托与绒毛伞）=====
  // 跟绒毛融合：半透 + 沿长度方向轻微 fade（基部稍实，末端略虚）
  const seedStemMat = new THREE.MeshStandardMaterial({
    color: p.seedStem, roughness: 0.92, metalness: 0,
    transparent: true, opacity: 0.55,
    depthWrite: false,
  })
  seedStemMat.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace('#include <uv_vertex>', '#include <uv_vertex>\nvJcUv = uv;')
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace(
        'vec4 diffuseColor = vec4( diffuse, opacity );',
        // 末端（uv.y=1）alpha 衰减到 0.35（不全透，否则跟伞冠基部脱节）
        'float jcStemFade = 1.0 - smoothstep(0.3, 1.0, vJcUv.y) * 0.65;\nvec4 diffuseColor = vec4( diffuse, opacity * jcStemFade );'
      )
  }
  const seedStemH = puffRadius - receptacleR
  const seedStemR = stemRadius * 0.45
  const seedStemGeo = new THREE.CylinderGeometry(seedStemR, seedStemR * 0.8, seedStemH, 4, 1, false)
  const seedInst = new THREE.InstancedMesh(seedStemGeo, seedStemMat, seedCount)

  const dummy = new THREE.Object3D()
  const dir = new THREE.Vector3()
  const upY = new THREE.Vector3(0, 1, 0)
  for (let i = 0; i < seedCount; i++) {
    fibonacciDir(i, seedCount, dir)
    const mid = dir.clone().multiplyScalar((receptacleR + puffRadius) / 2)
    dummy.position.copy(mid)
    dummy.quaternion.setFromUnitVectors(upY, dir)
    dummy.scale.set(1, 1, 1)
    dummy.updateMatrix()
    seedInst.setMatrixAt(i, dummy.matrix)
  }
  seedInst.instanceMatrix.needsUpdate = true
  puffGroup.add(seedInst)
  themedMaterials.seedStem.push(seedStemMat)

  // ===== 绒毛伞 =====
  //  · 整体透明度压低（不再抢戏）
  //  · 沿绒毛长度方向（CylinderGeometry 的 uv.y：0 = 基部、1 = 末端）做渐变透明
  //    末端 alpha 接近 0 → 视觉上自然消散
  const hairMat = new THREE.MeshStandardMaterial({
    color: p.hair, roughness: 0.96, metalness: 0,
    transparent: true, opacity: 0.45,    // baseline 整体压低，基部不再"太白"
    depthWrite: false,
  })
  hairMat.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace('#include <uv_vertex>', '#include <uv_vertex>\nvJcUv = uv;')
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace(
        'vec4 diffuseColor = vec4( diffuse, opacity );',
        // 仅末端 fade：从 uv.y=0.35 开始衰减，到 1.0 完全透明
        // 整体 opacity 已经低到 0.45，所以基部本身就半透
        'float jcFadeTip = 1.0 - smoothstep(0.35, 1.0, vJcUv.y);\nvec4 diffuseColor = vec4( diffuse, opacity * jcFadeTip );'
      )
  }
  // segments=8 for smoother UV interpolation along length
  const hairR = stemRadius * 0.28
  const hairGeo = new THREE.CylinderGeometry(hairR, hairR * 0.15, hairLength, 4, 8, false)
  const hairInst = new THREE.InstancedMesh(hairGeo, hairMat, seedCount * hairsPerSeed)

  const tip = new THREE.Vector3()
  const sub = new THREE.Vector3()
  const t1 = new THREE.Vector3()
  const t2 = new THREE.Vector3()
  let hi = 0
  for (let i = 0; i < seedCount; i++) {
    fibonacciDir(i, seedCount, dir)
    tip.copy(dir).multiplyScalar(puffRadius)
    if (Math.abs(dir.y) < 0.99) {
      t1.set(0, 1, 0).cross(dir).normalize()
    } else {
      t1.set(1, 0, 0).cross(dir).normalize()
    }
    t2.crossVectors(dir, t1).normalize()
    for (let j = 0; j < hairsPerSeed; j++) {
      const ang = (j / hairsPerSeed) * Math.PI * 2
      sub.copy(dir).multiplyScalar(Math.cos(coneAngle))
        .addScaledVector(t1, Math.sin(coneAngle) * Math.cos(ang))
        .addScaledVector(t2, Math.sin(coneAngle) * Math.sin(ang))
        .normalize()
      const mid = tip.clone().add(sub.clone().multiplyScalar(hairLength / 2))
      dummy.position.copy(mid)
      dummy.quaternion.setFromUnitVectors(upY, sub)
      dummy.scale.set(1, 1, 1)
      dummy.updateMatrix()
      hairInst.setMatrixAt(hi++, dummy.matrix)
    }
  }
  hairInst.instanceMatrix.needsUpdate = true
  puffGroup.add(hairInst)
  themedMaterials.hair.push(hairMat)

  return group
}

function buildScene() {
  const root = new THREE.Group()
  // 收集 wrap 给 animate 用：每株围绕 wrap 的 z 轴旋转 = 围绕茎根弯倒
  const leanWraps = []

  // base-pivot wrap：wrap 原点 = 茎根；dandelion 内部 +Y stemHeight → puff 回到 worldY
  // windParams：每株独立的风感（coef 响应系数 / phase 相位 / f1,a1,f2,a2 双频微震参数）
  function placeWithBasePivot(cfg, worldX, worldY, baseTiltZ = 0, windParams = null) {
    const d = buildSingleDandelion(cfg)
    d.position.y = cfg.stemHeight
    const wrap = new THREE.Group()
    wrap.position.set(worldX, worldY - cfg.stemHeight, 0)
    wrap.rotation.z = baseTiltZ
    wrap.userData.baseTiltZ = baseTiltZ
    wrap.userData.windParams = windParams || {
      coef: 1.0, phase: 0,
      f1: 1.05, a1: 0.07,
      f2: 2.4,  a2: 0.035,
    }
    wrap.add(d)
    root.add(wrap)
    leanWraps.push(wrap)
    return wrap
  }

  // 大株：右侧；puff_y 1.45 → 1.85（再上移 0.40）
  //  基部不动 → stemHeight = 7.05 + 0.40 = 7.45
  // singleStalk 时把大株 worldX 归零，让整体居中显示
  const bigStalkX = props.singleStalk ? 0 : 0.80
  placeWithBasePivot({
    stemHeight:   7.45,
    bendDir:     -1,
    bendAmount:   0.55,
    stemRadius:   0.020,
    puffRadius:   0.60,
    seedCount:    130,
    hairsPerSeed: 6,
    hairLength:   0.20,
    coneAngle:    0.36,
    receptacleR:  0.11,
  }, bigStalkX, 1.85, 0, {
    // 大株：重一些，响应慢，慢频微摇
    coef: 0.95, phase: 0,
    f1: 0.95, a1: 0.07,
    f2: 2.20, a2: 0.030,
  })

  // 小株：左侧；singleStalk 时跳过（移动端只保留大株）
  if (!props.singleStalk) {
    placeWithBasePivot({
      stemHeight:   5.95,
      bendDir:      1,
      bendAmount:   0.46,
      stemRadius:   0.015,
      puffRadius:   0.34,
      seedCount:    90,
      hairsPerSeed: 5,
      hairLength:   0.13,
      coneAngle:    0.34,
      receptacleR:  0.07,
      baseOffsetX:  0.85,
    }, -0.55, 1.05, -0.05, {
      // 小株：轻 + 茎细，响应快 1.25×，相位错开，高频颤
      coef: 1.25, phase: 2.4,
      f1: 1.42, a1: 0.06,
      f2: 3.05, a2: 0.045,
    })
  }

  root.userData.leanWraps = leanWraps

  // 飘散粒子：单独 group，运行时独立旋转（不受 wind 影响）
  const driftGroup = new THREE.Group()
  const driftCount = 10  // 减少：28 → 10
  const p = palette(isDark.value)
  const driftMat = new THREE.MeshStandardMaterial({
    color: p.hair, roughness: 0.95, transparent: true, opacity: 0.45,
  })
  const driftGeo = new THREE.SphereGeometry(0.007, 6, 5)  // 缩小：0.013 → 0.007
  const driftInst = new THREE.InstancedMesh(driftGeo, driftMat, driftCount)
  const dummy = new THREE.Object3D()
  for (let i = 0; i < driftCount; i++) {
    dummy.position.set(
      (Math.random() - 0.5) * 6,
      0.5 + Math.random() * 2.4,
      (Math.random() - 0.5) * 1.8,
    )
    dummy.quaternion.identity()
    dummy.scale.setScalar(0.5 + Math.random() * 1.6)
    dummy.updateMatrix()
    driftInst.setMatrixAt(i, dummy.matrix)
  }
  driftInst.instanceMatrix.needsUpdate = true
  driftGroup.add(driftInst)
  root.add(driftGroup)
  themedMaterials.hair.push(driftMat)
  root.userData.driftGroup = driftGroup

  // ===== 前景：3 颗飘飞的蒲公英绒毛（每颗 = 小种子 + 短茎 + 张开的伞 pappus）=====
  const fgGroup = new THREE.Group()

  // 共享材质 — 跟主 puff 同色系，但伞 hair 单独建一个带 fade shader 的
  const fgSeedBodyMat = new THREE.MeshStandardMaterial({
    color: 0x8a7350, roughness: 0.7,
  })
  const fgStemMat = new THREE.MeshStandardMaterial({
    color: p.seedStem, roughness: 0.9,
    transparent: true, opacity: 0.6, depthWrite: false,
  })
  const fgHairMat = new THREE.MeshStandardMaterial({
    color: p.hair, roughness: 0.96,
    transparent: true, opacity: 0.55, depthWrite: false,
  })
  fgHairMat.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace('#include <uv_vertex>', '#include <uv_vertex>\nvJcUv = uv;')
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vJcUv;')
      .replace(
        'vec4 diffuseColor = vec4( diffuse, opacity );',
        'float jcF = 1.0 - smoothstep(0.35, 1.0, vJcUv.y);\nvec4 diffuseColor = vec4( diffuse, opacity * jcF );'
      )
  }
  themedMaterials.hair.push(fgHairMat)
  themedMaterials.seedStem.push(fgStemMat)

  // 单颗飘飞绒毛工厂
  function buildFlyingSeed(scale, tiltZ = 0) {
    const g = new THREE.Group()
    const N_HAIR = 8
    const HAIR_LEN = 0.18
    const STEM_LEN = 0.22
    const SEED_LEN = 0.05

    // 茎（朝 -Y）
    const stem = new THREE.Mesh(
      new THREE.CylinderGeometry(0.003, 0.003, STEM_LEN, 4, 1),
      fgStemMat,
    )
    stem.position.y = -STEM_LEN / 2
    g.add(stem)

    // 种子（小椭球）
    const body = new THREE.Mesh(
      new THREE.CylinderGeometry(0.006, 0.002, SEED_LEN, 6, 1),
      fgSeedBodyMat,
    )
    body.position.y = -STEM_LEN - SEED_LEN / 2
    g.add(body)

    // 伞 pappus：8 根绒毛，cone 较张开
    const hairsInst = new THREE.InstancedMesh(
      new THREE.CylinderGeometry(0.003, 0.001, HAIR_LEN, 4, 6),
      fgHairMat, N_HAIR,
    )
    const _upY = new THREE.Vector3(0, 1, 0)
    const _dummy = new THREE.Object3D()
    const cone = 0.55 // 张得更开（飞行时阻力让伞撑大）
    for (let j = 0; j < N_HAIR; j++) {
      const ang = (j / N_HAIR) * Math.PI * 2
      const d = new THREE.Vector3(
        Math.sin(cone) * Math.cos(ang),
        Math.cos(cone),
        Math.sin(cone) * Math.sin(ang),
      ).normalize()
      const mid = d.clone().multiplyScalar(HAIR_LEN / 2)
      _dummy.position.copy(mid)
      _dummy.quaternion.setFromUnitVectors(_upY, d)
      _dummy.scale.set(1, 1, 1)
      _dummy.updateMatrix()
      hairsInst.setMatrixAt(j, _dummy.matrix)
    }
    hairsInst.instanceMatrix.needsUpdate = true
    g.add(hairsInst)

    g.scale.setScalar(scale)
    g.rotation.z = tiltZ
    return g
  }

  // 3 颗散布在左下前景（z > 0 = 靠近相机）
  // 位置考虑 model.scale=0.42 + model.position.y=-0.6 → 这些是本地坐标
  // 世界 y = local_y × 0.42 - 0.6（要在可见区 ±0.66 内）
  // scale 也乘 0.42，需放大到 1.8-2.6 才不至于看不见
  const fgSeeds = [
    { pos: [-1.6, 0.6, 2.0],   scale: 0.73, rotZ:  0.45 },  // 1/3：2.2 → 0.73
    { pos: [-1.0, 1.4, 1.4],   scale: 0.53, rotZ: -0.30 },  // 1/3：1.6 → 0.53
    { pos: [-2.0, 2.0, 1.0],   scale: 0.63, rotZ:  0.90 },  // 1/3：1.9 → 0.63
  ]
  for (const s of fgSeeds) {
    const seed = buildFlyingSeed(s.scale, s.rotZ)
    seed.position.set(...s.pos)
    fgGroup.add(seed)
  }
  root.add(fgGroup)
  root.userData.fgGroup = fgGroup

  return root
}

function updateMaterialsForTheme(dark) {
  const np = palette(dark)
  themedMaterials.hair.forEach((m)       => { m.color.setHex(np.hair);       m.needsUpdate = true })
  themedMaterials.receptacle.forEach((m) => { m.color.setHex(np.receptacle); m.needsUpdate = true })
  themedMaterials.seedStem.forEach((m)   => { m.color.setHex(np.seedStem);   m.needsUpdate = true })
  themedMaterials.groundStem.forEach((m) => { m.color.setHex(np.groundStem); m.needsUpdate = true })
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

  camera = new THREE.PerspectiveCamera(38, w / h, 0.1, 100)
  camera.position.set(0, 0, 10)
  camera.lookAt(0, 0, 0)

  renderer = new THREE.WebGLRenderer({
    alpha: true, antialias: true, powerPreference: 'high-performance',
  })
  renderer.setSize(w, h)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.setClearColor(0x000000, 0)
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.05
  container.appendChild(renderer.domElement)

  const pmrem = new THREE.PMREMGenerator(renderer)
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture

  scene.add(new THREE.AmbientLight(0xe8eef5, 0.4))
  const key = new THREE.DirectionalLight(0xffffff, 1.0)
  key.position.set(4, 5, 4); scene.add(key)
  const rim = new THREE.DirectionalLight(0xc8d4ee, 0.55)
  rim.position.set(-3, -1, -5); scene.add(rim)

  model = buildScene()
  model.scale.setScalar(props.modelScale)

  // ⚠ Framing 只算 camera 距离决定尺寸；NOT auto-recenter
  // 此前每次改 puff_y 都被这里自动反向补偿掉了 → 直接固定 anchor
  // model.position 固定（你想整体平移就改这两个数；想动单株就改 placeWithBasePivot 的 worldX/Y）
  // 容器扩到全 hero 宽后，puff 中心从 hero 70% → 50%，需向右补偿约 0.5 世界单位
  // 保持 puff 在 hero 视觉同样位置（右侧 70% 区）
  // singleStalk 时略微右移让蒲公英偏向 hero 右侧（避开左侧文字/CTA）
  model.position.x = props.singleStalk ? 0.4 : 0.5
  model.position.y = -0.6   // 整体下移
  scene.add(model)

  // ⚠ camera 距离用"设计常量"，跟 puff 实际位置解耦
  // → 改 puff 位置时镜头不会一起跟着远近，保持稳定取景
  // 想整体放大缩小：改 framingDim（大→远→小）或 distMultiplier
  const framingDim     = 3.0     // 世界单位下"想框住的区域宽度"
  const distMultiplier = 1.05    // 镜头微调（>1 远；<1 近）
  const fov = camera.fov * Math.PI / 180
  const dist = (framingDim * props.modelScale / 2) / Math.tan(fov / 2) * distMultiplier
  camera.position.set(0, 0, dist)
  camera.lookAt(0, 0, 0)

  baseRotation.x = model.rotation.x
  baseRotation.y = model.rotation.y
  baseRotation.z = model.rotation.z
  baseCameraZ = camera.position.z

  watch(isDark, (v) => updateMaterialsForTheme(v))

  mouseMoveHandler = (e) => {
    const rect = container.getBoundingClientRect()
    const cx = rect.left + rect.width / 2
    const cy = rect.top + rect.height / 2
    const nx = (e.clientX - cx) / Math.max(rect.width / 2, 1)
    const ny = (e.clientY - cy) / Math.max(rect.height / 2, 1)
    // 鼠标 X → wind 驱动茎弯；XY 共同 → 前景飞絮跟随
    target.x = Math.max(-1, Math.min(1, nx)) * props.maxRotY
    target.y = Math.max(-1, Math.min(1, ny)) * props.maxRotY
    lastMouseMoveTime = performance.now()
    // 判断鼠标是否在 hero canvas 矩形内
    mouseInHero.value = (
      e.clientX >= rect.left && e.clientX <= rect.right &&
      e.clientY >= rect.top  && e.clientY <= rect.bottom
    )
    if (mouseInHero.value) {
      // 把鼠标投到 z=0 世界平面：相机 FOV 决定可见区高，aspect 决定宽
      const visH = 2 * Math.tan((camera.fov * Math.PI / 180) / 2) * camera.position.z
      const visW = visH * camera.aspect
      mouseWorld.x =  nx * visW / 2
      mouseWorld.y = -ny * visH / 2   // clientY 向下 → world y 向上
    }
  }
  window.addEventListener('mousemove', mouseMoveHandler, { passive: true })

  mouseLeaveHandler = () => { mouseInHero.value = false }
  container.addEventListener('mouseleave', mouseLeaveHandler, { passive: true })

  scrollHandler = () => {
    const sy = window.scrollY || window.pageYOffset || 0
    scrollTarget = Math.min(1, Math.max(0, sy / 800))
  }
  window.addEventListener('scroll', scrollHandler, { passive: true })
  scrollHandler()

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
      camera.position.z = baseCameraZ * (1 - scrollCurrent * 0.45)
    }
    if (model) {
      const t = performance.now() / 1000

      // ===== Wind: 鼠标 X 主驱 + 每株独立 idleSway（去同步）=====
      current.x += (target.x - current.x) * props.lerpFactor
      current.y += (target.y - current.y) * props.lerpFactor
      const mouseWind = current.x * 0.85   // 同一阵风，两株共享驱动

      const leanWraps = model.userData.leanWraps || []
      for (const wrap of leanWraps) {
        const d = wrap.children[0]
        const uni = d?.userData?.windUniforms
        const puff = d?.userData?.puffGroup
        const wp = wrap.userData.windParams
        if (!uni || !puff || !wp) continue
        // 每株自己的双频微震：不同 freq/phase → 即使没鼠标也错位摇
        const personalSway = Math.sin(t * wp.f1 + wp.phase) * wp.a1
                           + Math.sin(t * wp.f2 + wp.phase * 1.7) * wp.a2
        // coef：响应系数（小株柔，coef>1 弯得更多）
        const wind = mouseWind * wp.coef + personalSway
        uni.uWind.value = wind
        puff.position.x = wind
      }

      // ===== 背景 drift 粒子：缓慢旋转 + 上下浮动 =====
      const drift = model.userData.driftGroup
      if (drift) {
        drift.rotation.y += 0.0025
        drift.position.y = Math.sin(t * 0.6) * 0.04
      }
      // ===== 前景飘飞绒毛 =====
      //  鼠标在 hero 内 → 缓慢聚拢到鼠标周围（各带 cluster 偏移，不重叠）
      //  鼠标离开 hero → 缓慢散回各自基础位置
      const fg = model.userData.fgGroup
      if (fg) {
        const baseXArr = [-1.6, -1.0, -2.0]
        const baseYArr = [ 0.6,  1.4,  2.0]
        // 每颗在鼠标周围的小偏移（cluster 形态，避免叠成一团）
        const clusterDx = [ 0.05, -0.08,  0.10]
        const clusterDy = [-0.04,  0.07, -0.05]
        // 鼠标在世界系；转换到 model 内部本地坐标（除掉 model.scale + offset）
        const ms = props.modelScale
        const localMouseX = (mouseWorld.x - model.position.x) / ms
        const localMouseY = (mouseWorld.y - model.position.y) / ms

        // 每颗各自的 lerp 速率，避免三颗整齐划一
        // 越小越慢；当前最快 0.0035 ≈ 1s 追 19%，最慢 0.0018 ≈ 1s 追 10%
        const lerpRates = [0.0035, 0.0022, 0.0028]
        fg.children.forEach((seed, i) => {
          // 自转：每颗各自频率
          seed.rotation.y += 0.004 + i * 0.0012
          seed.rotation.x = Math.sin(t * 0.4 + i * 1.7) * 0.18
          // 目标位置：聚拢 or 散开
          let tx, ty
          if (mouseInHero.value) {
            tx = localMouseX + clusterDx[i]
            ty = localMouseY + clusterDy[i]
          } else {
            tx = baseXArr[i] + Math.sin(t * 0.3 + i * 0.9) * 0.08
            ty = baseYArr[i] + Math.cos(t * 0.25 + i * 1.4) * 0.06
          }
          const r = lerpRates[i]
          seed.position.x += (tx - seed.position.x) * r
          seed.position.y += (ty - seed.position.y) * r
        })
      }

      // model 自身：仅承接 scroll 微俯仰
      model.rotation.x = baseRotation.x + scrollCurrent * 0.3
      model.rotation.y = baseRotation.y
      model.rotation.z = baseRotation.z
    }
    renderer.render(scene, camera)
  }
  animate()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animationFrame)
  window.removeEventListener('mousemove', mouseMoveHandler)
  if (mouseLeaveHandler && containerRef.value) {
    containerRef.value.removeEventListener('mouseleave', mouseLeaveHandler)
  }
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
  <div ref="containerRef" class="dandelion-host"></div>
</template>

<style scoped>
.dandelion-host {
  position: relative;
  width: 100%;
  height: 100%;
  pointer-events: none;
  overflow: hidden;
}
.dandelion-host :deep(canvas) {
  display: block;
  width: 100% !important;
  height: 100% !important;
}
</style>

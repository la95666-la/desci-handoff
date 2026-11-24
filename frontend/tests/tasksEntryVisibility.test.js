import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const read = (path) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const root = fileURLToPath(new URL('../', import.meta.url))

const sourceFiles = (directory) => fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
  const target = path.join(directory, entry.name)
  if (entry.isDirectory()) return sourceFiles(target)
  return /\.(?:vue|js|ts|jsx|tsx)$/.test(entry.name) ? [target] : []
})

const relative = (file) => path.relative(root, file).replaceAll('\\', '/')

test('public Task Hall entries are release-enabled while the route remains centrally gated', () => {
  assert.match(read('src/config/featureVisibility.js'), /SHOW_TASKS_ENTRY\s*=\s*true/)

  const router = read('src/router/index.js')
  assert.match(router, /path:\s*['"]\/tasks['"]/)
  assert.match(router, /name:\s*['"]TaskHall['"]/)

  const guardedEntries = [
    'src/components/JcFloatingNavbar.vue',
    'src/components/Navbar.vue',
    'src/components/HomeFooter.vue',
    'src/views/Home.vue',
    'src/views/Ecosystem.vue',
    'src/views/Dashboard.vue',
  ]

  for (const path of guardedEntries) {
    assert.match(read(path), /SHOW_TASKS_ENTRY/, `${path} must use the release visibility switch`)
  }

  assert.match(
    read('src/components/HomeFooter.vue'),
    /<router-link\s+v-if="SHOW_TASKS_ENTRY"\s+to="\/tasks"/,
    'shared desktop/mobile footer Task Hall link must be guarded',
  )
  assert.match(
    read('src/views/Home.vue'),
    /<router-link\s+v-if="SHOW_TASKS_ENTRY"\s+to="\/tasks"/,
    'desktop home footer Task Hall link must be guarded',
  )
  assert.doesNotMatch(
    read('src/views/Home.vue'),
    /goDocs\s*=\s*\(\)\s*=>\s*router\.push\(['"]\/tasks['"]\)/,
    'Docs must never be an indirect Task Hall entry',
  )

  const floatingNavbar = read('src/components/JcFloatingNavbar.vue')
  assert.match(
    floatingNavbar,
    /<a\s+v-if="SHOW_TASKS_ENTRY"[^>]*class="\['drawer-link'[^>]*@click="goQuestCenter\(\); closeDrawer\(\)"/,
    'mobile navigation drawer Task Hall entry must be guarded',
  )
  assert.match(
    floatingNavbar,
    /<a\s+v-if="SHOW_TASKS_ENTRY"[^>]*:class="cls\('quest'\)"[^>]*@click="goQuestCenter"/,
    'desktop floating navigation Task Hall entry must be guarded',
  )

  assert.match(
    read('src/components/Navbar.vue'),
    /\.filter\(\(item\)\s*=>\s*SHOW_TASKS_ENTRY\s*\|\|\s*item\.key\s*!==\s*['"]quest['"]\)/,
    'global desktop/mobile navigation must filter the Task Hall item',
  )

  const home = read('src/views/Home.vue')
  const mobileFooterStart = home.indexOf('<footer class="m-footer">')
  const desktopStart = home.indexOf('DESKTOP HOME')
  assert.ok(mobileFooterStart >= 0 && desktopStart > mobileFooterStart, 'home mobile footer boundary must remain auditable')
  const mobileFooter = home.slice(mobileFooterStart, desktopStart)
  assert.doesNotMatch(
    mobileFooter,
    /(?:\/tasks|footerLinkQuest|Quest Center|任務中心|任务中心)/i,
    'home mobile footer must not expose a Task Hall label or route',
  )

  for (const file of ['src/views/Ecosystem.vue', 'src/views/Dashboard.vue']) {
    const source = read(file)
    const taskControls = [...source.matchAll(/<button\s+v-if="SHOW_TASKS_ENTRY"[^>]*?(?:handleJoin|router\.push\('\/tasks'\))[^>]*>/g)]
    assert.ok(taskControls.length >= 2, `${file} desktop/mobile Task Hall controls must each be guarded`)
  }
})

test('every public source reference to /tasks is explicitly guarded or is an allowed route/callback', () => {
  const allowedUnguarded = new Set([
    // These files classify /tasks for shell styling or route-scoped notices;
    // they do not render a public navigation entry.
    'src/App.vue',
    'src/components/ImportantNewsSpotlight.vue',
    'src/router/index.js',
    'src/views/TaskHall.vue',
  ])

  const references = sourceFiles(path.join(root, 'src'))
    .map((file) => ({ file: relative(file), source: fs.readFileSync(file, 'utf8') }))
    .filter(({ source }) => /(['"])\/tasks\1/.test(source))

  assert.ok(references.length > 0, 'expected preserved Task Hall route references')

  for (const { file, source } of references) {
    if (allowedUnguarded.has(file)) continue
    assert.match(source, /SHOW_TASKS_ENTRY/, `${file} references /tasks without the release visibility switch`)
  }
})

test('all footer Task Hall labels and links are guarded across desktop and mobile sources', () => {
  const footerSources = sourceFiles(path.join(root, 'src'))
    .map((file) => ({ file: relative(file), source: fs.readFileSync(file, 'utf8') }))
    .filter(({ file, source }) => /footer/i.test(file) || /<footer\b/i.test(source))

  for (const { file, source } of footerSources) {
    if (!/(?:footerLinkQuest|to=["']\/tasks["']|Quest Center|任務中心|任务中心)/i.test(source)) continue
    assert.match(source, /SHOW_TASKS_ENTRY/, `${file} exposes an unguarded Task Hall footer entry`)
  }
})

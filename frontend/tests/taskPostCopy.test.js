import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const taskHall = fs.readFileSync(new URL('../src/views/TaskHall.vue', import.meta.url), 'utf8')

test('every remote task uses its Admin-configured copy before the shared task-key default', () => {
  assert.match(taskHall, /function resolveTaskCopy\(remoteValue, localizedValue\)/)
  assert.match(taskHall, /return remoteValue \|\| localizedValue/)
  assert.match(taskHall, /title:\s+resolveTaskCopy\(i\.title, t\.value\[`\$\{base\}_title`\]\)/)
  assert.match(taskHall, /desc:\s+resolveTaskCopy\(i\.description, t\.value\[`\$\{base\}_desc`\]\)/)
  assert.match(taskHall, /cta:\s+resolveTaskCopy\(i\.cta_label, t\.value\[`\$\{base\}_cta`\]\)/)
})

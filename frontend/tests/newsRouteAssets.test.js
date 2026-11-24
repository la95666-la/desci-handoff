import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const publicDir = path.join(root, 'public')
const router = fs.readFileSync(path.join(root, 'src/router/index.js'), 'utf8')
const mockNews = fs.readFileSync(path.join(root, 'src/mock/news.js'), 'utf8')

test('news SPA routes do not collide with a public/news directory', () => {
  assert.match(router, /path:\s*['"]\/news['"]/)
  assert.match(router, /path:\s*['"]\/news\/:id['"]/)
  assert.equal(fs.existsSync(path.join(publicDir, 'news')), false)
  assert.equal(fs.existsSync(path.join(publicDir, 'news-assets', '1.jpg')), true)
  assert.equal(fs.existsSync(path.join(publicDir, 'news-assets', '2.png')), true)
})

test('local news images use the non-routing asset directory', () => {
  assert.match(mockNews, /image:\s*['"]\/news-assets\/1\.jpg['"]/)
  assert.match(mockNews, /image:\s*['"]\/news-assets\/2\.png['"]/)
  assert.doesNotMatch(mockNews, /image:\s*['"]\/news\//)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { pickHeroFeaturedNews } from '../src/utils/heroFeaturedNews.js'

test('Hero hides when no valid featured article is returned', () => {
  for (const items of [null, undefined, [], {}, [{ title: 'Not featured', cover_url: '/cover.jpg', is_featured: 0 }],
    [{ title: 'No cover', is_featured: 1 }], [{ cover_url: '/cover.jpg', is_featured: 1 }]]) {
    assert.equal(pickHeroFeaturedNews(items), null)
  }
})

test('Hero selects only an explicitly featured, displayable article', () => {
  assert.deepEqual(pickHeroFeaturedNews([
    { id: 1, title: 'Disabled', cover_url: '/old.jpg', is_featured: 0 },
    { id: 2, title: 'Enabled', cover_url: '/new.jpg', is_featured: 1 }
  ]), { title: 'Enabled', cover: '/new.jpg', target: '/news/2' })
  assert.deepEqual(pickHeroFeaturedNews([{ title: 'External', cover_url: '/cover.jpg', source_url: 'https://example.org', is_featured: '1' }]),
    { title: 'External', cover: '/cover.jpg', target: 'https://example.org' })
  assert.deepEqual(pickHeroFeaturedNews([
    { title: 'No cover', is_featured: 1 }, { title: ' ', cover_url: '/blank.jpg', is_featured: 1 },
    { title: ' ', cover_url: '/blank2.jpg', is_featured: 1 },
    { id: 4, title: 'Fourth', cover_url: '/fourth.jpg', is_featured: 1 }
  ]), { title: 'Fourth', cover: '/fourth.jpg', target: '/news/4' })
})

test('Home renders Hero news only when a featured card exists, without a static fallback', () => {
  const home = readFileSync(new URL('../src/views/Home.vue', import.meta.url), 'utf8')
  const translations = readFileSync(new URL('../src/i18n/home.js', import.meta.url), 'utf8')
  assert.match(home, /const heroNewsCard = computed\(\(\) => pickHeroFeaturedNews\(remoteMediaCards\.value\)\)/)
  assert.match(home, /<article\s+v-if="heroNewsCard"\s+class="case-study-card/)
  assert.doesNotMatch(home, /caseStudyCopy|\/jp2026\.png/)
  assert.doesNotMatch(translations, /caseStudyCopy|JoinCare backs Japan/)
})

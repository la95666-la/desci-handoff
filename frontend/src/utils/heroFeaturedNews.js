// Only an explicitly featured, displayable news item can occupy the Hero.
// Empty, failed, or malformed list responses never authorize a static card.
export function pickHeroFeaturedNews(items) {
  if (!Array.isArray(items)) return null
  const item = items.find(n =>
    (n?.is_featured === 1 || n?.is_featured === '1') &&
    typeof n.title === 'string' && n.title.trim() &&
    typeof n.cover_url === 'string' && n.cover_url.trim()
  )
  if (!item) return null
  return {
    title: item.title,
    cover: item.cover_url,
    target: item.source_url || (item.id ? `/news/${item.id}` : '')
  }
}

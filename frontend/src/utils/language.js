export function normalizeUiLanguage(value) {
  if (value == null) return 'en'
  const normalized = String(value).trim().toLowerCase()
  return normalized === '' || normalized === 'en' || normalized === 'en-us' ? 'en' : 'tw'
}

export function isTraditionalUiLanguage(value) {
  return normalizeUiLanguage(value) === 'tw'
}

export function backendLanguage(value) {
  return isTraditionalUiLanguage(value) ? 'zh-tw' : 'en-us'
}

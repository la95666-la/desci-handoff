// Mobile wallets background the browser. Coalesce pageshow/focus/visibility
// into one read-only refresh; never invoke a signing method on resume.
export function watchWalletResume(refresh, doc = document, win = window) {
  let pending = null, disposed = false, timer = null
  const resume = () => {
    if (disposed || doc.visibilityState === 'hidden' || timer || pending) return
    timer = win.setTimeout(() => {
      timer = null
      if (disposed || doc.visibilityState === 'hidden') return
      pending = Promise.resolve().then(refresh).catch(() => {}).finally(() => { pending = null })
    }, 100)
  }
  doc.addEventListener('visibilitychange', resume)
  win.addEventListener('pageshow', resume)
  win.addEventListener('focus', resume)
  return () => {
    disposed = true
    if (timer) win.clearTimeout(timer)
    doc.removeEventListener('visibilitychange', resume)
    win.removeEventListener('pageshow', resume)
    win.removeEventListener('focus', resume)
  }
}

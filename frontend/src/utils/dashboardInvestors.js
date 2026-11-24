// Shared V2/V3 list state: a failed request is not an empty investment history.
// Per-request ownership also prevents an old account/page response overwriting
// a newer request after refresh or wallet switching.
export async function loadDashboardInvestors(states, projectId, page, read, isCurrent = () => true) {
  const id = String(projectId)
  const previous = states[id]
  states[id] = { items: previous?.items || [], total: previous?.total || 0,
    page: previous?.page || 1, pageSize: previous?.pageSize || 10, loading: true, loaded: false, error: false }
  const state = states[id]
  try {
    const data = await read(id, page, state.pageSize)
    if (!isCurrent() || states[id] !== state) return
    if (!Array.isArray(data?.items) || !Number.isSafeInteger(data.total) || data.total < 0) throw Error('Invalid investor response')
    Object.assign(state, { items: data.items, total: data.total, page: data.page || page,
      pageSize: data.pageSize || state.pageSize, loaded: true })
  } catch {
    if (isCurrent() && states[id] === state) state.error = true
  } finally {
    if (isCurrent() && states[id] === state) state.loading = false
  }
}

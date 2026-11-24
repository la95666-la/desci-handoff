// Release-time visibility switch for the public Task Hall entry points.
// Keep every public entry bound to this single switch so future release gates
// can hide or restore the section without changing the /tasks implementation.
export const SHOW_TASKS_ENTRY = true

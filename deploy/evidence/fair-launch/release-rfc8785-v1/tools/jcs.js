'use strict';

function validateString(value) {
  for (let i = 0; i < value.length; i += 1) {
    const current = value.charCodeAt(i);
    if (current >= 0xd800 && current <= 0xdbff) {
      const next = value.charCodeAt(i + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) throw new Error('lone high surrogate');
      i += 1;
    } else if (current >= 0xdc00 && current <= 0xdfff) {
      throw new Error('lone low surrogate');
    }
  }
}
function rejectInvalid(value) {
  if (typeof value === 'number' && !Number.isFinite(value)) throw new Error('non-finite JSON number');
  if (typeof value === 'string') validateString(value);
  if (Array.isArray(value)) value.forEach(rejectInvalid);
  else if (value && typeof value === 'object') {
    for (const [key, child] of Object.entries(value)) { validateString(key); rejectInvalid(child); }
  }
}
function canonicalize(value) {
  rejectInvalid(value);
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalize).join(',')}]`;
  return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonicalize(value[key])}`).join(',')}}`;
}
module.exports = { canonicalize, rejectInvalid, validateString };

import DOMPurify from 'dompurify'

/**
 * 富文本 XSS 消毒。所有后端来的 HTML 在 v-html 输出前必须过这里
 * （即使后端 sanitize.RichHTML 已清洗，前端再做一层防御）。
 */
export function sanitize(html) {
  return html ? DOMPurify.sanitize(html) : ''
}

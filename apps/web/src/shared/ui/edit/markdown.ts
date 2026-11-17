import DOMPurify, { type Config } from 'dompurify'
import { Marked } from 'marked'

// GFM covers tables, task lists and strikethrough; `breaks` keeps single
// newlines visible so plain-text descriptions pasted from IM stay readable.
const markdownParser = new Marked({ gfm: true, breaks: true })

const SANITIZE_CONFIG: Config = {
  USE_PROFILES: { html: true },
  FORBID_ATTR: ['style'],
  ADD_ATTR: ['target'],
}

let hooksInstalled = false

function ensureHooks() {
  if (hooksInstalled) return
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node.nodeType === 1 && node.tagName === 'A') {
      node.setAttribute('target', '_blank')
      node.setAttribute('rel', 'noopener noreferrer nofollow')
    }
  })
  hooksInstalled = true
}

/**
 * Render untrusted markdown source to sanitized HTML. The output only ever
 * contains the HTML profile (no SVG/MathML), no inline styles, and outbound
 * links are hardened with `target="_blank"` + `rel="noopener noreferrer"`.
 */
export function renderMarkdown(source: string): string {
  ensureHooks()
  const html = markdownParser.parse(source, { async: false })
  return DOMPurify.sanitize(html, SANITIZE_CONFIG)
}

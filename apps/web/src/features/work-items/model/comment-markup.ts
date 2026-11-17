/** Safe comment markup shared by native modules and the legacy Vue seam. */
const AT_BODY_RE = /@([\p{L}\p{N}_.-]+)/gu

export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function commentBodyHtml(
  body: string,
  mentionLabels?: Map<string, string>,
): string {
  const raw = String(body || '')
  if (!raw) return ''
  let out = ''
  let last = 0
  AT_BODY_RE.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = AT_BODY_RE.exec(raw)) !== null) {
    const username = match[1]
    out += escapeHtml(raw.slice(last, match.index))
    const label = mentionLabels?.get(username)
      || mentionLabels?.get(username.toLowerCase())
      || username
    out += `<span class="wt-cmt-at" data-user-username="${escapeHtml(username)}" tabindex="0">@${escapeHtml(label)}</span>`
    last = match.index + match[0].length
  }
  out += escapeHtml(raw.slice(last))
  return out.replace(/\n/g, '<br>')
}

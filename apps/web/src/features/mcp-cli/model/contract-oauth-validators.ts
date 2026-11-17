function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function safeLoopbackRedirect(value: unknown): string {
  const candidate = text(value)
  try {
    const url = new URL(candidate)
    const host = url.hostname.toLowerCase()
    if (
      url.protocol !== 'http:'
      || !['localhost', '127.0.0.1', '[::1]'].includes(host)
      || url.username
      || url.password
      || url.hash
    ) return ''
    return candidate
  } catch {
    return ''
  }
}

export function safeOAuthResource(value: unknown): string {
  const candidate = text(value)
  try {
    const url = new URL(candidate)
    const local = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname.toLowerCase())
    if (
      (url.protocol !== 'https:' && !(url.protocol === 'http:' && local))
      || url.username
      || url.password
      || url.search
      || url.hash
    ) return ''
    return candidate
  } catch {
    return ''
  }
}

export function safeOAuthState(value: unknown): string {
  const candidate = typeof value === 'string' ? value : ''
  return candidate.length >= 1 && candidate.length <= 1024 && !/[\s\u0000-\u001f\u007f]/.test(candidate)
    ? candidate
    : ''
}

export function safeOAuthScope(value: unknown): string {
  const scopes = [...new Set(text(value).split(/\s+/).filter(Boolean))]
  if (!scopes.includes('pm:tools') || scopes.some((scope) => !['pm:tools', 'offline_access'].includes(scope))) {
    return ''
  }
  return scopes.includes('offline_access') ? 'pm:tools offline_access' : 'pm:tools'
}
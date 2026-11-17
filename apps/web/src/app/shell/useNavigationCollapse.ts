import { useState } from 'react'

const STORAGE_KEY = 'pm.navigation.collapsed'

export function useNavigationCollapse() {
  const [collapsed, setCollapsed] = useState(() => {
    try { return localStorage.getItem(STORAGE_KEY) === 'true' } catch { return false }
  })
  const toggle = () => {
    const next = !collapsed
    setCollapsed(next)
    try { localStorage.setItem(STORAGE_KEY, String(next)) } catch { /* Session-only when storage is unavailable. */ }
  }
  return { collapsed, toggle }
}

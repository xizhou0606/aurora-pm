import { useEffect, useState } from 'react'
import type { JoinTarget } from '../api/project-requests'
export function useJoinRequestOpen(target: JoinTarget) {
 const [open, setOpen] = useState(false)
 useEffect(() => {
  const sync = () => { if (window.location.hash === (target.kind === 'workspace' ? `#workspace-requests-${target.id}` : '#access-requests') || (target.kind === 'plan_unit' && window.location.hash === `#project-requests-${target.id}`)) setOpen(true) }
  sync(); window.addEventListener('hashchange', sync)
  return () => window.removeEventListener('hashchange', sync)
 }, [target.kind, target.id])
 return { open, onOpenChange: setOpen }
}

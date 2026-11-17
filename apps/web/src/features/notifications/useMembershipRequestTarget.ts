import { useEffect, useState } from 'react'
function readTarget() {
 const match = window.location.hash.match(/^#(workspace|project)-requests-([1-9]\d*)$/)
 const id = Number(match?.[2])
 if (!match || !Number.isSafeInteger(id)) return undefined
 return { kind: match[1] === 'workspace' ? 'workspace' as const : 'plan_unit' as const, id }
}
export function useMembershipRequestTarget() {
 const [target, setTarget] = useState(readTarget)
 useEffect(() => { const sync = () => setTarget(readTarget()); window.addEventListener('hashchange', sync); return () => window.removeEventListener('hashchange', sync) }, [])
 return target
}

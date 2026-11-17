// Shared semantic tokens for document cursors and table participants. Blue is
// reserved for this viewer; resolve hash collisions within the current roster.
const remoteColors = [
  { ring: 'ring-emerald-500', caret: 'border-emerald-500', label: 'bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100', selection: 'bg-emerald-500/15' },
  { ring: 'ring-violet-500', caret: 'border-violet-500', label: 'bg-violet-100 text-violet-900 dark:bg-violet-950 dark:text-violet-100', selection: 'bg-violet-500/15' },
  { ring: 'ring-amber-500', caret: 'border-amber-500', label: 'bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-100', selection: 'bg-amber-500/15' },
  { ring: 'ring-rose-500', caret: 'border-rose-500', label: 'bg-rose-100 text-rose-900 dark:bg-rose-950 dark:text-rose-100', selection: 'bg-rose-500/15' },
  { ring: 'ring-cyan-500', caret: 'border-cyan-500', label: 'bg-cyan-100 text-cyan-900 dark:bg-cyan-950 dark:text-cyan-100', selection: 'bg-cyan-500/15' },
  { ring: 'ring-orange-500', caret: 'border-orange-500', label: 'bg-orange-100 text-orange-900 dark:bg-orange-950 dark:text-orange-100', selection: 'bg-orange-500/15' },
  { ring: 'ring-fuchsia-500', caret: 'border-fuchsia-500', label: 'bg-fuchsia-100 text-fuchsia-900 dark:bg-fuchsia-950 dark:text-fuchsia-100', selection: 'bg-fuchsia-500/15' },
  { ring: 'ring-lime-500', caret: 'border-lime-500', label: 'bg-lime-100 text-lime-900 dark:bg-lime-950 dark:text-lime-100', selection: 'bg-lime-500/15' },
  { ring: 'ring-teal-500', caret: 'border-teal-500', label: 'bg-teal-100 text-teal-900 dark:bg-teal-950 dark:text-teal-100', selection: 'bg-teal-500/15' },
  { ring: 'ring-pink-500', caret: 'border-pink-500', label: 'bg-pink-100 text-pink-900 dark:bg-pink-950 dark:text-pink-100', selection: 'bg-pink-500/15' },
] as const
export type CollaborationColor = { ring: string; caret: string; label: string; selection: string }
export const localCollaborationColor: CollaborationColor = { ring: 'ring-primary', caret: 'border-primary', label: 'bg-primary text-primary-foreground', selection: 'bg-primary/10' }

export function collaborationColorsFor(ids: readonly string[], selfId?: string): Map<string, CollaborationColor> {
  const assigned = new Map<string, CollaborationColor>()
  const used = new Set<number>()
  for (const id of [...new Set(ids)].sort()) {
    if (id === selfId) { assigned.set(id, localCollaborationColor); continue }
    let hash = 0
    for (const char of id) hash = (hash * 31 + char.charCodeAt(0)) >>> 0
    let index = hash % remoteColors.length
    for (let offset = 0; offset < remoteColors.length && used.has(index); offset++) index = (index + 1) % remoteColors.length
    used.add(index)
    assigned.set(id, remoteColors[index])
  }
  return assigned
}

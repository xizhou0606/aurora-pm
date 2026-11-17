import { useRef, useState } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button } from '@/shared/ui'
import { PhPlus } from '@/shared/ui/icons'
import type { GridCommands } from '../../model/grid-port'
import type { Row } from '../../model/types'

export function GridCreateRow({ commands, canCreate, editing, onCreated, compact = false }: {
  commands: GridCommands; canCreate: boolean; editing: boolean; onCreated: (row: Row) => void; compact?: boolean
}) {
  const creating = useRef(false)
  const [phase, setPhase] = useState<'idle' | 'pending'>('idle')
  const create = async () => {
    if (creating.current || commands.pending || !canCreate || editing) return
    creating.current = true
    setPhase('pending')
    try { const row = await commands.onCreateEmpty(); if (row) onCreated(row) }
    catch { /* The command owner retains failure/unknown-result feedback. */ }
    finally { creating.current = false; setPhase('idle') }
  }
  return <Button type="button" size={compact ? 'icon-xs' : 'sm'} variant="ghost" className={`transition-none active:not-aria-[haspopup]:translate-y-0 active:not-aria-[haspopup]:scale-100 ${compact ? 'shrink-0' : 'h-9 w-full justify-start gap-3 rounded-none px-4 text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground'}`} aria-label={text.addRow} title={text.addRow} disabled={!canCreate || commands.pending || editing || phase === 'pending'} onClick={() => void create()}><PhPlus className="size-4" aria-hidden="true" />{!compact ? text.addRow : null}</Button>
}

export function GridFooter({ total, commands, editing, canCreate, onCreated }: { total: number; commands: GridCommands; editing: boolean; canCreate?: boolean; onCreated?: (row: Row) => void }) {
  return <footer className="flex h-9 shrink-0 items-center gap-1 border-t border-border/70 bg-muted/30 px-3">
    {onCreated ? <GridCreateRow compact commands={commands} canCreate={canCreate === true} editing={editing} onCreated={onCreated} /> : null}
    <span className="flex-1 text-xs tabular-nums text-muted-foreground">{text.rowCount(total)}</span>
  </footer>
}

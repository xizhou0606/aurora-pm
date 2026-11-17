import { copy } from '@/shared/copy'

export function UserMention({ id, name, username }: { id?: number; name: string; username?: string }) {
  return <span tabIndex={0} data-comment-mention="" data-user-id={id} data-user-username={username} className="inline-block max-w-full cursor-default rounded bg-primary/10 px-1 py-0.5 text-sm font-medium text-primary outline-none focus-visible:ring-2 focus-visible:ring-ring">{copy.ui.mention.label(name)}</span>
}

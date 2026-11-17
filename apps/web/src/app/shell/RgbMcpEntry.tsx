import { copy } from '@/shared/copy'

export function RgbMcpEntry({
  href,
  onActivate,
}: {
  href?: string
  onActivate?: () => void
}) {
  const className = 'pm-rgb-mcp-entry h-7 shrink-0 rounded-md px-1.5 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card'
  const label = <span className="pm-rgb-mcp-entry__label">{copy.app.shell.mcpCliEntry}</span>

  if (href) {
    return (
      <a
        href={href}
        data-slot="rgb-mcp-entry"
        aria-label={copy.app.shell.rgbMcpEntryLabel}
        className={className}
      >
        {label}
      </a>
    )
  }
  if (!onActivate) return null
  return (
    <button
      type="button"
      data-slot="rgb-mcp-entry"
      aria-label={copy.app.shell.rgbMcpEntryLabel}
      onClick={onActivate}
      className={className}
    >
      {label}
    </button>
  )
}

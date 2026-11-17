const COVER_THEMES = [
  { sky: '#E4ECFA', mist: '#F3F6FC', paper: '#FFFFFF', rule: '#D5E0F2', ink: '#3A4F73', accent: '#3D6BDB', chip: '#EEF3FF' },
  { sky: '#E5F3EE', mist: '#F3FAF7', paper: '#FFFFFF', rule: '#D2E8DE', ink: '#1F5A4A', accent: '#2A9B7A', chip: '#E8F6F0' },
  { sky: '#EEE7F8', mist: '#F7F3FB', paper: '#FFFFFF', rule: '#E0D5EF', ink: '#5A3A7A', accent: '#7C5CBF', chip: '#F3ECFA' },
  { sky: '#E7F1F6', mist: '#F4F8FB', paper: '#FFFFFF', rule: '#D3E3EC', ink: '#2A5A6A', accent: '#3A8FA8', chip: '#EAF4F8' },
  { sky: '#F3EAE3', mist: '#FAF5F1', paper: '#FFFFFF', rule: '#E8D8CC', ink: '#6A4630', accent: '#C57A45', chip: '#F7EEE6' },
] as const

export function RequirementCoverFallback({ seed }: { seed: number }) {
  const theme = COVER_THEMES[Math.abs(seed) % COVER_THEMES.length]
  const id = `requirement-cover-${seed}`
  return (
    <svg viewBox="0 0 320 160" className="absolute inset-0 size-full" aria-hidden="true">
      <defs>
        <linearGradient id={`${id}-sky`} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor={theme.sky} />
          <stop offset="1" stopColor={theme.mist} />
        </linearGradient>
        <filter id={`${id}-paper`} x="-12%" y="-12%" width="124%" height="124%">
          <feDropShadow dx="0" dy="6" stdDeviation="8" floodColor={theme.ink} floodOpacity="0.12" />
        </filter>
      </defs>
      <rect width="320" height="160" fill={`url(#${id}-sky)`} />
      {Array.from({ length: 40 }, (_, index) => {
        const x = 12 + (index % 10) * 32
        const y = 10 + Math.floor(index / 10) * 38
        return <circle key={index} cx={x} cy={y} r="1.15" fill={theme.ink} opacity="0.08" />
      })}
      <g transform="translate(78 26) rotate(-7 82 54)">
        <rect width="128" height="108" rx="10" fill={theme.chip} />
      </g>
      <g filter={`url(#${id}-paper)`} transform="translate(96 22) rotate(5 74 58)">
        <rect width="148" height="116" rx="12" fill={theme.paper} />
        <rect x="14" y="16" width="52" height="8" rx="4" fill={theme.accent} />
        <rect x="14" y="36" width="120" height="6" rx="3" fill={theme.rule} />
        <rect x="14" y="50" width="104" height="6" rx="3" fill={theme.rule} />
        <rect x="14" y="64" width="88" height="6" rx="3" fill={theme.rule} />
        <rect x="14" y="86" width="36" height="14" rx="7" fill={theme.chip} />
        <rect x="56" y="86" width="44" height="14" rx="7" fill={theme.chip} />
        <circle cx="128" cy="93" r="11" fill={theme.accent} />
        <path d="M123.5 93.2 126.6 96.4 133 89.6" fill="none" stroke={theme.paper} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      </g>
    </svg>
  )
}

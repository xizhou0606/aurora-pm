import { copy } from '@/shared/copy'

export function LoginAside() {
  return (
    <aside data-slot="login-aside">
      <div data-slot="login-stage">
        <span data-orb="one" />
        <span data-orb="two" />
        <WorkbenchScene />
      </div>
      <p data-slot="login-aside-title">
        <span>{copy.auth.heroTitle}</span>
        {copy.auth.heroAccent}
      </p>
      <p data-slot="login-aside-lede">{copy.auth.heroLede}</p>
    </aside>
  )
}

function WorkbenchScene() {
  return (
    <svg viewBox="0 0 240 260" aria-hidden="true" className="h-[min(42vh,260px)] w-full">
      <g data-lane="cloud-back">
        <ellipse cx="68" cy="216" rx="48" ry="9" fill="var(--login-aside-cloud)" />
        <ellipse cx="176" cy="222" rx="52" ry="10" fill="var(--login-aside-cloud)" />
      </g>

      <g data-lane="orbit-a" transform="rotate(-24 120 118)">
        <ellipse
          cx="120"
          cy="118"
          rx="88"
          ry="34"
          fill="none"
          stroke="var(--login-aside-frame)"
          strokeWidth="0.85"
        />
        <circle r="2.3" fill="var(--login-aside-spark)">
          <animateMotion
            dur="6.4s"
            repeatCount="indefinite"
            path="M208 118 A88 34 0 1 1 32 118 A88 34 0 1 1 208 118"
          />
        </circle>
      </g>

      <g data-lane="orbit-b" transform="rotate(28 120 118)">
        <ellipse
          cx="120"
          cy="118"
          rx="36"
          ry="76"
          fill="none"
          stroke="var(--login-aside-path)"
          strokeWidth="0.8"
          strokeDasharray="2.2 5.5"
        />
        <circle r="2.05" fill="var(--login-aside-ring)">
          <animateMotion
            dur="9.2s"
            repeatCount="indefinite"
            path="M156 118 A36 76 0 1 0 84 118 A36 76 0 1 0 156 118"
          />
        </circle>
      </g>

      <path
        data-lane="flow"
        d="M58 104 H182 A18 18 0 0 1 200 122 V148 A18 18 0 0 1 182 166 H58 A18 18 0 0 1 40 148 V122 A18 18 0 0 1 58 104"
        fill="none"
        stroke="var(--login-aside-path)"
        strokeWidth="0.75"
        strokeLinecap="round"
        strokeDasharray="5 9"
      />

      <g data-lane="board">
        <rect
          x="91"
          y="90"
          width="58"
          height="56"
          rx="15"
          fill="var(--login-aside-panel)"
          stroke="var(--login-aside-frame)"
          strokeWidth="0.9"
        />
        <rect data-bar="1" x="102" y="104" width="28" height="2.6" rx="1.3" fill="var(--login-aside-bar-a)" />
        <rect data-bar="2" x="102" y="113" width="36" height="2.6" rx="1.3" fill="var(--login-aside-bar-b)" />
        <rect data-bar="3" x="102" y="122" width="20" height="2.6" rx="1.3" fill="var(--login-aside-bar-c)" />
      </g>

      <g data-lane="spark">
        <circle cx="188" cy="56" r="1.25" fill="var(--login-aside-spark)" />
        <circle cx="52" cy="68" r="1.05" fill="var(--login-aside-orb)" />
      </g>

      <g data-lane="cloud-front">
        <ellipse cx="88" cy="234" rx="26" ry="6.5" fill="var(--login-aside-cloud-strong)" />
        <ellipse cx="154" cy="238" rx="32" ry="7.5" fill="var(--login-aside-cloud-strong)" />
      </g>
    </svg>
  )
}

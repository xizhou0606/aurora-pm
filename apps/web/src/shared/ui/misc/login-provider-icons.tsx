type LoginProviderIconProps = {
  className?: string
}

export function FeishuLoginIcon({ className }: LoginProviderIconProps) {
  return (
    <svg
      aria-hidden="true"
      className={className}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path fill="#3370ff" d="M11.7 2.16a1.14 1.14 0 0 1 1.61 0l3.03 3.03-4.64 4.64-3.83-3.82 3.83-3.85Z" />
      <path fill="#00d6b9" d="M5.19 7.66a1.14 1.14 0 0 1 0-1.61l.66-.66 5.85 5.85-3.82 3.83-2.69-2.69a3.34 3.34 0 0 1 0-4.72Z" />
      <path fill="#7b67ee" d="m12.51 10.43 4.64-4.64 1.66 1.66a3.34 3.34 0 0 1 0 4.72l-2.69 2.69-3.61-4.43Z" />
      <path fill="#14c0ff" d="m8.69 15.88 3.82-3.83 3.82 3.83-3.02 3.03a1.14 1.14 0 0 1-1.61 0l-3.01-3.03Z" />
      <path fill="#2b5bdb" d="m12.51 10.43 3.61 4.43-2.81 2.81a1.14 1.14 0 0 1-1.61 0l-2.81-2.81 3.62-4.43Z" opacity=".96" />
    </svg>
  )
}

export function GoogleLoginIcon({ className }: LoginProviderIconProps) {
  return (
    <svg
      aria-hidden="true"
      className={className}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path fill="#4285f4" d="M21.6 12.23c0-.71-.06-1.4-.18-2.06H12v3.9h5.38a4.6 4.6 0 0 1-2 3.02v2.54h3.24c1.9-1.75 2.98-4.33 2.98-7.4Z" />
      <path fill="#34a853" d="M12 22c2.7 0 4.98-.9 6.63-2.42l-3.24-2.53c-.9.6-2.05.96-3.39.96-2.61 0-4.82-1.76-5.61-4.13H3.05v2.6A10 10 0 0 0 12 22Z" />
      <path fill="#fbbc05" d="M6.39 13.88A6 6 0 0 1 6.07 12c0-.65.11-1.29.32-1.88v-2.6H3.05A10 10 0 0 0 2 12c0 1.61.38 3.13 1.05 4.48l3.34-2.6Z" />
      <path fill="#ea4335" d="M12 5.99c1.47 0 2.79.5 3.82 1.5l2.88-2.87A9.65 9.65 0 0 0 12 2a10 10 0 0 0-8.95 5.52l3.34 2.6C7.18 7.75 9.39 5.99 12 5.99Z" />
    </svg>
  )
}

export function TelegramLoginIcon({ className }: LoginProviderIconProps) {
  return (
    <svg
      aria-hidden="true"
      className={className}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        fill="#229ed9"
        d="M21.56 3.11a1.26 1.26 0 0 0-1.29-.2L2.96 9.58c-1.18.46-1.16 1.12-.21 1.41l4.44 1.39 1.72 5.34c.21.58.11.81.72.81.47 0 .68-.22.94-.47l2.13-2.07 4.43 3.27c.82.45 1.4.22 1.6-.76l2.91-13.72c.29-1.2-.46-1.74-1.21-1.4L7.57 11.26c-.88.36-.87.87-.16 1.09l3.3 1.03 7.66-4.83c.36-.22.69-.1.42.14l-6.2 5.59-.24 3.44c.35 0 .5-.16.69-.35l1.66-1.61 3.45 2.55 2.4-11.32-9.08 8.23-3.69-1.15-2.57-.8 15.22-5.86c.67-.24 1.25.15 1.13.7Z"
      />
    </svg>
  )
}

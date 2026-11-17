import { copy } from '@/shared/copy'
import { BrandLockup } from '@/shared/ui'

export function LoginBrand() {
  return (
    <BrandLockup
      name={copy.app.shortName}
      kicker={copy.auth.heroKicker}
      className="text-(--login-chrome)"
      kickerClassName="text-(--login-chrome-muted)"
    />
  )
}

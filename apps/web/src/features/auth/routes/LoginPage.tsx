import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'

import { safePMRedirect } from '@/shared/config'
import { copy } from '@/shared/copy'
import type { ExternalLoginProvider, LinkableLoginProvider } from '@/shared/model'
import {
  ThemeAppearanceControl,
  useAppearance,
} from '@/shared/ui'
import { LoginAside } from '../components/LoginAside'
import { LoginBrand } from '../components/LoginBrand'
import { LoginFog } from '../components/LoginFog'
import { LoginForm } from '../components/LoginForm'
import { ExternalRegistrationForm } from '../components/ExternalRegistrationForm'
import {
  feishuLoginStartPath,
  googleLoginStartPath,
  telegramLoginStartPath,
} from '../api/api'
import { useLoginWorkflow } from '../utils/workflows'
import {
  resolveLoginDestination,
  type LoginDestination,
} from '../utils/loginDestination'

export function LoginPage() {
  const navigate = useNavigate()
  const search = useSearch({ from: '/login' })
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const { resolved: fogTheme } = useAppearance()
  const processedLoginCode = useRef('')
  const [registration] = useState(() => {
    const provider = search.login_provider
    const registrationCode = search.external_registration_code
    return registrationCode && (provider === 'google' || provider === 'telegram')
      ? { provider: provider as LinkableLoginProvider, registrationCode }
      : null
  })
  const { errorMessage, exchangeLoginCode, login, pending, registerExternal } =
    useLoginWorkflow<LoginDestination>({
      resolveDestination: resolveLoginDestination,
    })

  const completeLogin = useCallback(async (resolution: LoginDestination) => {
    const redirect = safePMRedirect(search.redirect)
    if (resolution.status === 'identity_status') {
      await navigate({ to: '/identity-status', replace: true } as never)
      return
    }
    if (resolution.status === 'password_reset_required') {
      await navigate({ to: '/reset-password', replace: true } as never)
      return
    }
    if (resolution.status === 'ok') {
      await navigate({ to: redirect, replace: true } as never)
      return
    }
    await navigate({
      to: '/select-workspace',
      search: {
        redirect,
        empty: resolution.status === 'empty' ? '1' : undefined,
        error: undefined,
      },
      replace: true,
    })
  }, [navigate, search.redirect])

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const resolution = await login(username, password)
    if (resolution) await completeLogin(resolution)
  }

  const submitRegistration = useCallback(async (
    input: Parameters<typeof registerExternal>[0],
  ) => {
    const resolution = await registerExternal(input)
    if (resolution) await completeLogin(resolution)
  }, [completeLogin, registerExternal])

  const externalReturnTo = useCallback((provider: ExternalLoginProvider) => {
    const returnTo = new URL('/login', window.location.origin)
    if (search.redirect) returnTo.searchParams.set('redirect', search.redirect)
    returnTo.searchParams.set('login_provider', provider)
    return `${returnTo.pathname}${returnTo.search}`
  }, [search.redirect])

  const handleExternalLogin = useCallback((provider: ExternalLoginProvider) => {
    const returnTo = externalReturnTo(provider)
    const paths: Record<ExternalLoginProvider, (returnTo: string) => string> = {
      feishu: feishuLoginStartPath,
      google: googleLoginStartPath,
      telegram: telegramLoginStartPath,
    }
    window.location.assign(paths[provider](returnTo))
  }, [externalReturnTo])

  useEffect(() => {
    if (!search.external_registration_code) return
    void navigate({
      to: '/login',
      search: {
        redirect: search.redirect,
        login_code: undefined,
        external_registration_code: undefined,
        login_provider: undefined,
      },
      replace: true,
    } as never)
  }, [navigate, search.external_registration_code, search.redirect])

  useEffect(() => {
    if (registration) return
    const loginCode = search.login_code
    if (!loginCode || processedLoginCode.current === loginCode) return
    processedLoginCode.current = loginCode

    void (async () => {
      await navigate({
        to: '/login',
        search: {
          redirect: search.redirect,
          login_code: undefined,
          external_registration_code: undefined,
          login_provider: undefined,
        },
        replace: true,
      } as never)
      const resolution = await exchangeLoginCode(
        loginCode,
        search.login_provider || 'feishu',
      )
      if (resolution) await completeLogin(resolution)
    })().catch(() => undefined)
  }, [
    completeLogin,
    exchangeLoginCode,
    navigate,
    registration,
    search.external_registration_code,
    search.login_code,
    search.login_provider,
    search.redirect,
  ])

  return (
    <main
      data-slot="login-page"
      data-fog-theme={fogTheme}
      className="relative isolate z-0 min-h-dvh overflow-hidden bg-(--login-fallback) text-(--login-text)"
    >
      <section data-slot="login-main">
        <LoginFog theme={fogTheme} />
        <div data-slot="login-iris" aria-hidden="true" />
        <div data-slot="login-grain" aria-hidden="true" />
        <div data-slot="login-veil" aria-hidden="true" />

        <header data-slot="login-chrome">
          <LoginBrand />
        </header>

        <div data-slot="login-form-wrap">
          {registration ? (
            <ExternalRegistrationForm
              errorMessage={errorMessage}
              pending={pending}
              provider={registration.provider}
              registrationCode={registration.registrationCode}
              onSubmit={submitRegistration}
            />
          ) : (
            <LoginForm
              errorMessage={errorMessage}
              pending={pending}
              username={username}
              password={password}
              showPassword={showPassword}
              onUsernameChange={setUsername}
              onPasswordChange={setPassword}
              onToggleShowPassword={() => setShowPassword((v) => !v)}
              onSubmit={submit}
              onExternalLogin={handleExternalLogin}
            />
          )}
        </div>
      </section>

      <LoginAside />

      <div data-slot="login-theme">
        <ThemeAppearanceControl
          labels={{
            menu: copy.auth.themeMenu,
            light: copy.auth.themeLight,
            dark: copy.auth.themeDark,
            system: copy.auth.themeSystem,
          }}
          triggerClassName="text-(--login-theme-fg) hover:bg-(--login-theme-hover) focus-visible:ring-2 focus-visible:ring-(--login-theme-ring)"
        />
      </div>
    </main>
  )
}

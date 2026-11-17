export const externalLoginProviders = ['feishu', 'google', 'telegram'] as const

export type ExternalLoginProvider = (typeof externalLoginProviders)[number]

export const linkableLoginProviders = ['google', 'telegram'] as const

export type LinkableLoginProvider = (typeof linkableLoginProviders)[number]

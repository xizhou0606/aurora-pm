/// <reference types="@rsbuild/core/types" />

interface ImportMetaEnv {
  readonly VITE_PUBLIC_URL?: string
  readonly VITE_DEV_PROXY_TARGET?: string
  readonly VITE_PM_API_BASE_URL?: string
  readonly VITE_PM_HOST_API_BASE_URL?: string
  readonly VITE_PM_PRODUCT_NAME?: string
  readonly VITE_PM_APP_VERSION?: string
  readonly VITE_PM_VERSION_SUMMARY?: string
  readonly VITE_PM_RELEASE_DATE?: string
  readonly VITE_PM_RELEASE_NOTES?: string
  readonly VITE_PM_PRIMARY_COLOR?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app/App'
import { startAppRuntime } from '@/app/bootstrap'
import '@/shared/styles.css'
import { applyPMBrand } from '@/shared/config'
import { isPMMicroApp } from '@/shared/runtime'
import { hydrateAppearance } from '@/shared/ui/theme/appearance'

applyPMBrand()
hydrateAppearance()
document.documentElement.dataset.pmEmbedded = String(isPMMicroApp())
startAppRuntime()

const rootElement = document.getElementById('react-root')

if (!rootElement) {
  throw new Error('React root element was not found')
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

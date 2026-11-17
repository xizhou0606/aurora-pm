import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'

import { appearanceStore, PM_UI_MODE_DARK } from './appearance'
import { ThemeAppearanceControl } from './theme-appearance-control'

describe('ThemeAppearanceControl', () => {
  it('writes the shared dark UI mode onto the document', async () => {
    render(<ThemeAppearanceControl />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.appearance.menu }))
    fireEvent.click(await screen.findByRole('menuitemradio', { name: copy.ui.appearance.dark }))

    expect(appearanceStore.getState().appearance).toBe('dark')
    expect(document.documentElement.dataset.uiMode).toBe(PM_UI_MODE_DARK)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })
})

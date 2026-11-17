import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { ThemeAppearanceMenu } from './theme-appearance-menu'

const labels = {
  menu: '外观',
  light: '浅色',
  dark: '深色',
  system: '跟随系统',
}

describe('ThemeAppearanceMenu', () => {
  it('exposes an appearance trigger for the current resolved theme', () => {
    render(
      <ThemeAppearanceMenu
        appearance="light"
        resolved="light"
        labels={labels}
        onChange={() => undefined}
      />,
    )
    expect(screen.getByRole('button', { name: '外观' })).toBeTruthy()
  })
})

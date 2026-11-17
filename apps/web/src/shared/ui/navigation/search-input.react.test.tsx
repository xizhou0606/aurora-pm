import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { SearchInput } from './search-input'

describe('SearchInput', () => {
  it('provides one shared icon-bearing search control', () => {
    const { container } = render(<SearchInput aria-label="搜索任务" placeholder="输入关键词" />)

    const input = screen.getByRole('searchbox', { name: '搜索任务' })
    expect(input.getAttribute('placeholder')).toBe('输入关键词')
    expect(input.classList.contains('pl-8')).toBe(true)
    expect(container.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('suppresses the browser-native cancel affordance when the shared clear button is present', () => {
    render(
      <SearchInput
        aria-label="搜索任务"
        value="keyword"
        clearable
        onChange={() => undefined}
        onClear={() => undefined}
      />,
    )

    expect(screen.getByRole('searchbox', { name: '搜索任务' }).classList.contains('pm-search-input')).toBe(true)
    expect(screen.getAllByRole('button')).toHaveLength(1)
  })
})

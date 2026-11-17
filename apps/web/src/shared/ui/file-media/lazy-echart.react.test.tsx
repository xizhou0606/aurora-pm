import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { LazyEChart } from './lazy-echart'

vi.mock('./echart-runtime', () => ({
  EChartRuntime: ({ className }: { className?: string }) => (
    <div data-testid="echart-runtime" className={className} />
  ),
}))

describe('LazyEChart', () => {
  it('keeps an expanded summary in normal flow and scrolls its rows inside a fluid chart', async () => {
    const { container } = render(
      <LazyEChart
        option={{ series: [] }}
        ariaLabel="负责人需求数 Top 10"
        emptyTitle="暂无数据"
        loadingLabel="正在加载"
        loadErrorLabel="加载失败"
        retryLabel="重试"
        fluid
        summary={{
          caption: '负责人需求数 Top 10 数据摘要',
          columns: ['成员', '负责需求数'],
          rows: Array.from({ length: 10 }, (_, index) => [`成员 ${index + 1}`, index + 1]),
        }}
      />,
    )

    expect((await screen.findByTestId('echart-runtime')).className).toContain('h-full')
    expect(container.querySelector('[data-slot="lazy-echart"]')?.className).toContain('overflow-hidden')
    expect(container.querySelector('[data-slot="lazy-echart-stage"]')?.className).toContain('min-h-0')

    const summary = container.querySelector('summary') as HTMLElement
    expect(summary.textContent).toBe('负责人需求数 Top 10 数据摘要')
    fireEvent.click(summary)
    const scroller = summary.parentElement?.querySelector('div')
    expect(summary.parentElement?.hasAttribute('open')).toBe(true)
    expect(scroller?.className).toContain('max-h-36')
    expect(scroller?.className).toContain('overflow-auto')
  })
})

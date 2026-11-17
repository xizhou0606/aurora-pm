import { render } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'
import { Icon } from './icon'
import { ICON_DATA } from './icons.data'
import { HugeTask01, PhListChecks, PhListChecksFill } from './icons.generated'

describe('Hugeicons shared renderer', () => {
  it('renders every registered glyph without opaque root fill or remote assets', () => {
    const names = Object.keys(ICON_DATA) as Array<keyof typeof ICON_DATA>
    const { container } = render(<>{names.map(name => <Icon key={name} name={name} size={16} />)}</>)
    const icons = container.querySelectorAll('svg')
    expect(icons.length).toBe(names.length)
    for (const icon of icons) {
      expect(icon.getAttribute('viewBox')).toBe('0 0 24 24')
      expect(icon.getAttribute('fill')).toBe('none')
      expect(icon.getAttribute('aria-hidden')).toBe('true')
      expect(icon.children.length).toBeGreaterThan(0)
      expect(icon.querySelector('script, image, use, foreignObject')).toBeNull()
    }
  })
  it('retains the old component APIs without loading a second icon family', () => {
    expect(PhListChecks).toBe(HugeTask01)
    expect(PhListChecksFill).toBe(HugeTask01)
  })
  it('preserves explicit accessible labels and sizing', () => {
    const { getByRole } = render(<HugeTask01 aria-label="Task" size={18} />)
    const icon = getByRole('img', { name: 'Task' })
    expect(icon.hasAttribute('aria-hidden')).toBe(false)
    expect(icon.getAttribute('width')).toBe('18')
  })
})

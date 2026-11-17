import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useRef, useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { ResponsivePanel } from './responsive-panel'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
function mockViewport(isDesktop: boolean) {
  ;(globalThis as any).matchMedia = vi.fn((query: string) => ({
      matches: query === '(min-width: 768px)' ? isDesktop : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
}

function controlledViewport(initialDesktop: boolean) {
  let isDesktop = initialDesktop
  const listeners = new Set<(event: MediaQueryListEvent) => void>()
  const mediaQuery = {
    get matches() {
      return isDesktop
    },
    media: '(min-width: 768px)',
    onchange: null,
    addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener)
    },
    removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.delete(listener)
    },
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }
  ;(globalThis as any).matchMedia = vi.fn(() => mediaQuery)

  return {
    setDesktop(nextDesktop: boolean) {
      isDesktop = nextDesktop
      const event = { matches: isDesktop, media: mediaQuery.media } as MediaQueryListEvent
      listeners.forEach((listener) => listener(event))
    },
  }
}

function ControlledPanel() {
  const [open, setOpen] = useState(false)

  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Launch panel
      </button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
      >
        <button type="button">Panel action</button>
      </ResponsivePanel>
    </>
  )
}

function ControlledPanelWithFinalFocus() {
  const [open, setOpen] = useState(false)
  const finalFocus = useRef<HTMLButtonElement | null>(null)

  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Launch routed panel
      </button>
      <button ref={finalFocus} type="button">Explicit return target</button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
        finalFocus={finalFocus}
      >
        Panel body
      </ResponsivePanel>
    </>
  )
}

function ControlledPanelWithRemountedFinalFocus() {
  const [open, setOpen] = useState(false)
  const [routeVersion, setRouteVersion] = useState(0)

  return (
    <>
      <button
        key={routeVersion}
        id="remounted-route-trigger"
        type="button"
        onClick={() => {
          setOpen(true)
          setRouteVersion((value) => value + 1)
        }}
      >
        Remounted route target
      </button>
      <ResponsivePanel
        open={open}
        onOpenChange={setOpen}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
        finalFocus={() => document.getElementById('remounted-route-trigger')}
      >
        Panel body
      </ResponsivePanel>
    </>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  ;(globalThis as any).matchMedia = __stubPrev_matchMedia
})

describe('ResponsivePanel', () => {
  it('uses an accessible Dialog on desktop and reports close intent', async () => {
    mockViewport(true)
    const onOpenChange = vi.fn()

    render(
      <ResponsivePanel
        open
        onOpenChange={onOpenChange}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
        height="viewport"
      >
        Panel body
      </ResponsivePanel>,
    )

    const panel = await screen.findByRole('dialog', { name: 'Panel title' })
    const backdrop = document.querySelector('[data-slot="responsive-panel-backdrop"]')
    expect(panel.getAttribute('data-slot')).toBe('responsive-panel-dialog')
    expect(panel.className).toContain('h-[calc(100dvh-4rem)]')
    expect(panel.className).toContain('max-w-[90rem]')
    expect(panel.className).toContain('bg-card')
    expect(panel.className).toContain('shadow-[var(--design-shadow-elevated)]')
    expect(panel.className).toContain('shadow-[var(--design-shadow-elevated)]')
    expect(panel.className).toContain('pm-dialog-motion')
    expect(backdrop?.className).toContain('bg-black/50')
    expect(backdrop?.className).toContain('bg-black/50')
    expect(backdrop?.className).toContain('pm-overlay-motion')
    expect(panel.getAttribute('aria-describedby')).not.toBeNull()
    expect(screen.getByText('Panel description')).not.toBeNull()
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.documentElement.style.overflow).toBe('hidden')
    expect(document.body.dataset.modalScrollLock).toBe('true')
    expect(document.querySelector('[data-slot="responsive-panel-viewport"]')?.className)
      .toContain('overflow-hidden')
    const closeButton = screen.getByRole('button', { name: 'Close panel' })
    expect(closeButton.getAttribute('data-slot')).toBe('responsive-panel-close')
    expect(closeButton.textContent).toBe('')
    expect(closeButton.querySelector('svg')).not.toBeNull()
    expect(closeButton.closest('[data-slot="responsive-panel-header"]')).not.toBeNull()
    expect(document.querySelector('[data-slot="responsive-panel-footer"]')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Close panel' }))
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('moves an open dialog portal into and out of the active fullscreen element', async () => {
    mockViewport(true)
    const fullscreenHost = document.createElement('section')
    document.body.append(fullscreenHost)
    let fullscreenElement: Element | null = null
    Object.defineProperty(document, 'fullscreenElement', {
      configurable: true,
      get: () => fullscreenElement,
    })

    const view = render(
      <ResponsivePanel open onOpenChange={vi.fn()} title="Panel title">
        Panel body
      </ResponsivePanel>,
    )

    const dialog = await screen.findByRole('dialog', { name: 'Panel title' })
    expect(fullscreenHost.contains(dialog)).toBe(false)

    act(() => {
      fullscreenElement = fullscreenHost
      document.dispatchEvent(new Event('fullscreenchange'))
    })
    await waitFor(() => {
      expect(fullscreenHost.contains(
        screen.getByRole('dialog', { name: 'Panel title' }),
      )).toBe(true)
    })

    act(() => {
      fullscreenElement = null
      document.dispatchEvent(new Event('fullscreenchange'))
    })
    await waitFor(() => {
      expect(fullscreenHost.contains(
        screen.getByRole('dialog', { name: 'Panel title' }),
      )).toBe(false)
    })

    view.unmount()
    fullscreenHost.remove()
    Reflect.deleteProperty(document, 'fullscreenElement')
  })

  it('does not treat the enter-gate as a close completion', async () => {
    mockViewport(true)
    const onOpenChangeComplete = vi.fn()

    function RoutedPanel() {
      const [open, setOpen] = useState(true)
      return (
        <ResponsivePanel
          open={open}
          onOpenChange={setOpen}
          onOpenChangeComplete={onOpenChangeComplete}
          title="Panel title"
          description="Panel description"
          closeLabel="Close panel"
        >
          Panel body
        </ResponsivePanel>
      )
    }

    render(<RoutedPanel />)

    await screen.findByRole('dialog', { name: 'Panel title' })
    expect(onOpenChangeComplete).not.toHaveBeenCalledWith(false)

    fireEvent.click(screen.getByRole('button', { name: 'Close panel' }))
    await waitFor(() => expect(onOpenChangeComplete).toHaveBeenCalledWith(false))
  })

  it('keeps create-task body until the close motion finishes', async () => {
    mockViewport(true)

    function CreateTaskPanel() {
      const [editing, setEditing] = useState(false)
      return (
        <>
          <button type="button" onClick={() => setEditing(true)}>新建任务</button>
          <ResponsivePanel
            open={Boolean(editing)}
            onOpenChange={setEditing}
            title="新建全局任务"
            closeLabel="取消"
          >
            {editing ? <form id="work-items-create-form">任务表单</form> : null}
          </ResponsivePanel>
        </>
      )
    }

    render(<CreateTaskPanel />)
    fireEvent.click(screen.getByRole('button', { name: '新建任务' }))
    await screen.findByRole('dialog', { name: '新建全局任务' })
    expect(document.getElementById('work-items-create-form')).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: '取消' }))
    await waitFor(() => expect(document.getElementById('work-items-create-form')).toBeNull())
  })

  it('lets onCloseRequest keep the panel open for unsaved intercept', async () => {
    mockViewport(true)
    const onOpenChange = vi.fn()
    const onCloseRequest = vi.fn(() => false)

    render(
      <ResponsivePanel
        open
        onOpenChange={onOpenChange}
        onCloseRequest={onCloseRequest}
        title="Panel title"
        closeLabel="Close panel"
      >
        Panel body
      </ResponsivePanel>,
    )

    await screen.findByRole('dialog', { name: 'Panel title' })
    fireEvent.click(screen.getByRole('button', { name: 'Close panel' }))
    expect(onCloseRequest).toHaveBeenCalledTimes(1)
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog', { name: 'Panel title' })).not.toBeNull()
  })

  it('restores background scrolling after the final open panel unmounts', async () => {
    mockViewport(true)
    document.body.style.overflow = 'auto'
    document.documentElement.style.overflow = 'scroll'
    const first = render(
      <ResponsivePanel open onOpenChange={vi.fn()} title="First" description="First" closeLabel="Close first">
        First body
      </ResponsivePanel>,
    )
    const second = render(
      <ResponsivePanel open onOpenChange={vi.fn()} title="Second" description="Second" closeLabel="Close second">
        Second body
      </ResponsivePanel>,
    )

    await waitFor(() => {
      expect(document.body.style.overflow).toBe('hidden')
      expect(document.documentElement.style.overflow).toBe('hidden')
    })
    first.unmount()
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.documentElement.style.overflow).toBe('hidden')
    second.unmount()
    expect(document.body.style.overflow).toBe('auto')
    expect(document.documentElement.style.overflow).toBe('scroll')
    expect(document.body.dataset.modalScrollLock).toBeUndefined()
  })

  it('uses an accessible Drawer at narrow widths', async () => {
    mockViewport(false)

    render(
      <ResponsivePanel
        open
        onOpenChange={vi.fn()}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
        height="viewport"
      >
        Panel body
      </ResponsivePanel>,
    )

    const panel = await screen.findByRole('dialog', { name: 'Panel title' })
    expect(panel.getAttribute('data-slot')).toBe('responsive-panel-drawer')
    expect(panel.className).toContain('h-[90dvh]')
    expect(panel.className).toContain('pm-drawer-motion')
    expect(panel.getAttribute('aria-describedby')).not.toBeNull()
  })

  it('gates an initially-open panel for one paint so enter motion can start', async () => {
    mockViewport(true)
    render(
      <ResponsivePanel
        open
        onOpenChange={vi.fn()}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
      >
        Panel body
      </ResponsivePanel>,
    )

    expect(screen.queryByRole('dialog', { name: 'Panel title' })).toBeNull()
    expect(await screen.findByRole('dialog', { name: 'Panel title' })).not.toBeNull()
  })

  it('follows controlled state, closes on Escape, and restores focus', async () => {
    mockViewport(true)
    render(<ControlledPanel />)

    const trigger = screen.getByRole('button', { name: 'Launch panel' })
    trigger.focus()
    fireEvent.click(trigger)

    const panel = screen.getByRole('dialog', { name: 'Panel title' })
    expect(panel).not.toBeNull()

    fireEvent.keyDown(document, { key: 'Escape' })

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Panel title' })).toBeNull()
      expect(document.activeElement).toBe(trigger)
    })
  })

  it('restores focus to an explicit route trigger target', async () => {
    mockViewport(false)
    render(<ControlledPanelWithFinalFocus />)

    fireEvent.click(screen.getByRole('button', { name: 'Launch routed panel' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close panel' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Panel title' })).toBeNull()
      expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Explicit return target' }))
    })
  })

  it('resolves a remounted route trigger when the panel closes', async () => {
    mockViewport(true)
    render(<ControlledPanelWithRemountedFinalFocus />)

    const previousTarget = screen.getByRole('button', { name: 'Remounted route target' })
    fireEvent.click(previousTarget)
    const remountedTarget = document.getElementById('remounted-route-trigger') as HTMLButtonElement
    expect(remountedTarget).not.toBe(previousTarget)

    fireEvent.click(screen.getByRole('button', { name: 'Close panel' }))

    await waitFor(() => {
      expect(document.activeElement).toBe(remountedTarget)
    })
  })

  it('keeps the open container and child state stable across a breakpoint change', async () => {
    const viewport = controlledViewport(true)
    render(
      <ResponsivePanel
        open
        onOpenChange={vi.fn()}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
      >
        <input aria-label="Panel draft" defaultValue="" />
      </ResponsivePanel>,
    )

    const draft = await screen.findByRole('textbox', { name: 'Panel draft' }) as HTMLInputElement
    fireEvent.change(draft, { target: { value: 'keep me' } })
    act(() => viewport.setDesktop(false))

    expect(screen.getByRole('dialog').getAttribute('data-slot')).toBe('responsive-panel-dialog')
    expect(screen.getByRole('textbox', { name: 'Panel draft' })).toBe(draft)
    expect(draft.value).toBe('keep me')
  })

  it.each([
    { isDesktop: true, slot: 'responsive-panel-dialog' },
    { isDesktop: false, slot: 'responsive-panel-drawer' },
  ])('rejects every close intent while pending in $slot', async ({ isDesktop, slot }) => {
    mockViewport(isDesktop)
    const onOpenChange = vi.fn()
    const panel = (
      <ResponsivePanel
        open
        pending
        onOpenChange={onOpenChange}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
      >
        Panel body
      </ResponsivePanel>
    )
    const { rerender } = render(panel)

    const dialog = await screen.findByRole('dialog', { name: 'Panel title' })
    expect(dialog.getAttribute('data-slot')).toBe(slot)
    expect(dialog.getAttribute('aria-busy')).toBe('true')
    expect((screen.getByRole('button', { name: 'Close panel' }) as HTMLButtonElement).disabled).toBe(true)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onOpenChange).not.toHaveBeenCalled()

    rerender(
      <ResponsivePanel
        open
        onOpenChange={onOpenChange}
        title="Panel title"
        description="Panel description"
        closeLabel="Close panel"
      >
        Panel body
      </ResponsivePanel>,
    )

    expect((screen.getByRole('button', { name: 'Close panel' }) as HTMLButtonElement).disabled).toBe(false)
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })
})

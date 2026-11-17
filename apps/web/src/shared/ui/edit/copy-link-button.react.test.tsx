import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import {
  clearPMHostData,
  subscribePMHostData,
  type PMHostData,
} from '@/shared/runtime'
import { CopyLinkButton, resolveInternalShareHref } from './copy-link-button'

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
const originalExecCommand = Object.getOwnPropertyDescriptor(document, 'execCommand')

afterEach(() => {
  clearPMHostData()
  delete window.microApp
  window.history.replaceState({}, '', '/')
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard)
  else Reflect.deleteProperty(navigator, 'clipboard')
  if (originalExecCommand) Object.defineProperty(document, 'execCommand', originalExecCommand)
  else Reflect.deleteProperty(document, 'execCommand')
})

describe('CopyLinkButton', () => {
  it('copies an internal path through the current One2All host base path', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    const hostData: PMHostData = {
      contractVersion: 1,
      embedded: true,
      parentOrigin: 'https://one2all.example',
      auth: {
        token: 'memory-only-token',
        user: { id: 47, username: 'teo', role: 'developer' },
      },
      workspace: { id: 2 },
      route: {
        path: '/workspace/quality-management',
        fullPath: '/workspace/quality-management?bug_id=4764&mode=edit',
      },
    }
    window.microApp = {
      addDataListener: (listener, autoTrigger) => {
        if (autoTrigger) listener(hostData)
      },
      dispatch: vi.fn(),
    }
    subscribePMHostData(vi.fn())
    window.history.replaceState(
      {},
      '',
      '/one2all/workspace/quality-management?bug_id=4764&mode=edit',
    )

    render(<CopyLinkButton path="/workspace/documents/31?mode=view" />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.copyLink }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(
      'https://one2all.example/one2all/workspace/documents/31?mode=view',
    ))
  })

  it('copies the current standalone task URL', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    window.history.replaceState({}, '', '/workspace/work-items?task_id=31&mode=view')
    render(<CopyLinkButton path="/workspace/work-items?task_id=31&mode=view" />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.copyLink }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(window.location.href))
    expect(screen.getByRole('button', { name: copy.ui.linkCopied })).not.toBeNull()
  })

  it('rejects an external supplied path', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    window.history.replaceState({}, '', '/workspace/quality-management?bug_id=31')
    render(<CopyLinkButton path="https://example.test/workspace/work-items/31" />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.copyLink }))
    await waitFor(() => expect(screen.getByRole('button', { name: copy.ui.copyLinkFailed })).not.toBeNull())
    expect(writeText).not.toHaveBeenCalled()
  })

  it('falls back to the selection copy path for an internal HTTP deployment', async () => {
    // happy-dom 的 clipboard 挂在原型上且不可 delete，按 jsdom 形态置空实例属性。
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })
    const execCommand = vi.fn().mockReturnValue(true)
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: execCommand,
    })
    render(<CopyLinkButton path="/workspace/quality-management/bugs/31" />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.copyLink }))
    expect(await screen.findByRole('button', { name: copy.ui.linkCopied })).not.toBeNull()
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(document.querySelector('textarea[aria-hidden="true"]')).toBeNull()
  })

  it('resolves a standalone internal path independently of the current route', () => {
    window.history.replaceState({}, '', '/workspace/quality-management?bug_id=31')
    expect(resolveInternalShareHref('/workspace/documents/42#selection')).toBe(
      `${window.location.origin}/workspace/documents/42#selection`,
    )
  })

  it('keeps the compact link action accessible while copying the same safe URL', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    window.history.replaceState({}, '', '/workspace/documents/42')
    render(<CopyLinkButton path="/workspace/documents/42#comment-17" iconOnly />)

    fireEvent.click(screen.getByRole('button', { name: copy.ui.copyLink }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(
      `${window.location.origin}/workspace/documents/42#comment-17`,
    ))
    expect(screen.getByRole('button', { name: copy.ui.linkCopied })).not.toBeNull()
  })
})

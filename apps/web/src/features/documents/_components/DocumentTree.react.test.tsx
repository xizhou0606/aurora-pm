import { createEvent, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { DocumentTreeControl, DocumentTreeViewState } from '../hooks'
import type { DocumentNode } from '../model'
import { DocumentTree } from './DocumentTree'
import { DocumentRailToggle } from './DocumentRailToggle'

function node(overrides: Partial<DocumentNode> = {}): DocumentNode {
  return {
    id: 19,
    workspaceId: 7,
    spaceId: 2,
    type: 'document',
    title: '平台 ADR',
    sortOrder: 1,
    documentId: 11,
    systemManaged: false,
    favorited: false,
    createdBy: 3,
    updatedBy: 3,
    createdAt: '2026-09-04T00:00:00Z',
    updatedAt: '2026-09-04T00:00:00Z',
    ...overrides,
  }
}

function controls(): DocumentTreeControl {
  return {
    context: {
      openInNewTab: vi.fn(),
      beginShare: vi.fn(),
      copyLink: vi.fn().mockResolvedValue(undefined),
      beginCopy: vi.fn(),
      beginShortcut: vi.fn(),
      togglePin: vi.fn().mockResolvedValue(undefined),
      toggleFavorite: vi.fn().mockResolvedValue(undefined),
      beginTransferOwner: vi.fn(),
      updateDestination: vi.fn(),
      updateOwner: vi.fn(),
      submit: vi.fn().mockResolvedValue(undefined),
      close: vi.fn(),
    },
    markdownImport: {
      importFile: vi.fn().mockResolvedValue(undefined),
      dismiss: vi.fn(),
    },
    toggleFolder: vi.fn(),
    openDocument: vi.fn(),
    beginCreate: vi.fn(),
    beginRename: vi.fn(),
    renameNode: vi.fn().mockResolvedValue(true),
    beginMove: vi.fn(),
    moveNode: vi.fn().mockResolvedValue(undefined),
    beginArchive: vi.fn().mockResolvedValue(undefined),
    updateDialogTitle: vi.fn(),
    updateMoveTarget: vi.fn(),
    submitDialog: vi.fn().mockResolvedValue(undefined),
    closeDialog: vi.fn(),
  }
}

function ready(nodes: readonly DocumentNode[]): Extract<DocumentTreeViewState, { status: 'ready' }> {
  return {
    status: 'ready',
    space: {
      id: 2,
      workspaceId: 7,
      name: '文档',
      isDefault: true,
      createdAt: '2026-09-04T00:00:00Z',
      updatedAt: '2026-09-04T00:00:00Z',
    },
    nodes,
    rows: nodes.map((item) => ({
      node: item,
      depth: 0,
      hasChildren: false,
      expanded: false,
      selected: false,
      canMoveUp: false,
      canMoveDown: false,
    })),
    refresh: 'idle',
    dialog: { kind: 'closed' },
    reorderError: false,
    capabilities: {
      canCreate: true,
      canImportMarkdown: true,
      canImportMarkdownAssets: true,
      canCopy: true,
      canCreateShortcut: true,
      canUpdatePreferences: true,
      canTransferOwnership: true,
      canRename: true,
      canMove: true,
      canArchive: true,
    },
    context: {
      dialog: { kind: 'closed' },
      operationError: false,
      linkCopied: false,
      owners: { status: 'empty' },
      access: { view: { status: 'loading' }, onCloseRequest: () => true },
    },
    markdownImport: { status: 'idle' },
  }
}

describe('DocumentTree context actions', () => {
  it('places collapse beside folder creation in the existing header', () => {
    const collapse = vi.fn()
    render(<DocumentTree state={ready([])} control={controls()}
      headerAction={<DocumentRailToggle collapsed={false} directory label={DOCUMENTS_COPY.tree.collapsePanel} onClick={collapse} />} />)
    const button = screen.getByRole('button', { name: DOCUMENTS_COPY.tree.collapsePanel })
    expect(button.parentElement).toBe(screen.getByRole('button', { name: DOCUMENTS_COPY.tree.createFolder }).parentElement)
    fireEvent.click(button)
    expect(collapse).toHaveBeenCalledTimes(1)
  })
  it('renders foreign resources only in their expanded folder slot without creating document nodes', () => {
    const folder = node({ id: 30, type: 'folder', documentId: undefined, title: 'Folder' })
    const value = ready([folder])
    if (value.status !== 'ready') throw new Error('fixture')
    const slot = vi.fn(location => <li>{location.parentId === 30 ? 'folder resource' : 'root resource'}</li>)
    value.rows = value.rows.map(row => ({ ...row, expanded: true }))
    const { rerender } = render(<DocumentTree state={value} control={controls()} resourceEntries={slot} />)
    expect(screen.getByText('folder resource')).toBeTruthy()
    expect(slot).toHaveBeenCalledWith(expect.objectContaining({ parentId: 30, depth: 1, folders: [{ id: 30, name: 'Folder' }] }))
    rerender(<DocumentTree state={{ ...value, rows: value.rows.map(row => ({ ...row, expanded: false })) }} control={controls()} resourceEntries={slot} />)
    expect(screen.queryByText('folder resource')).toBeNull()
    expect(screen.getByText('root resource')).toBeTruthy()
  })
  it('routes Markdown and ZIP selection through the import workflow', async () => {
    const control = controls()
    render(<DocumentTree state={ready([])} control={control} />)
    const file = new File(['# ADR'], 'adr.md', { type: 'text/markdown' })
    fireEvent.change(screen.getByLabelText(DOCUMENTS_COPY.markdownImport.inputLabel), {
      target: { files: [file] },
    })
    await waitFor(() => expect(control.markdownImport.importFile).toHaveBeenCalledWith(file))
  })

  it('opens the same capability-first menu from right click and the row ellipsis at 390px', async () => {
    const documentNode = node()
    const control = controls()
    const { rerender } = render(
      <div className="w-[390px]"><DocumentTree state={ready([documentNode])} control={control} /></div>,
    )
    const title = screen.getByRole('button', { name: DOCUMENTS_COPY.tree.open(documentNode.title) })
    fireEvent.contextMenu(title)
    expect(await screen.findByRole('menuitem', { name: DOCUMENTS_COPY.tree.openNewTab })).not.toBeNull()
    fireEvent.click(screen.getByRole('menuitem', { name: DOCUMENTS_COPY.tree.copy }))
    expect(control.context.beginCopy).toHaveBeenCalledWith(documentNode)

    rerender(<div className="w-[390px]"><DocumentTree state={ready([documentNode])} control={control} /></div>)
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.tree.more(documentNode.title) }))
    expect(await screen.findByRole('menuitem', { name: DOCUMENTS_COPY.tree.share })).not.toBeNull()
    fireEvent.click(screen.getByRole('menuitem', { name: DOCUMENTS_COPY.tree.transferOwner }))
    expect(control.context.beginTransferOwner).toHaveBeenCalledWith(documentNode)
  })

  it('supports ContextMenu keyboard focus and restores focus after Escape', async () => {
    const documentNode = node()
    const control = controls()
    render(<DocumentTree state={ready([documentNode])} control={control} />)
    const title = screen.getByRole('button', { name: DOCUMENTS_COPY.tree.open(documentNode.title) })
    title.focus()
    fireEvent.keyDown(title, { key: 'ContextMenu' })
    const first = await screen.findByRole('menuitem', { name: DOCUMENTS_COPY.tree.openNewTab })
    await waitFor(() => expect(document.activeElement).toBe(first))
    fireEvent.keyDown(first, { key: 'Escape' })
    await waitFor(() => expect(document.activeElement).toBe(title))
  })

  it('locks system favorites and keeps favorite references read-only', async () => {
    const favorites = node({
      id: 1,
      type: 'folder',
      title: '收藏',
      documentId: undefined,
      systemManaged: true,
      systemKind: 'favorites_folder',
    })
    const reference = node({
      id: 2,
      type: 'shortcut',
      title: '平台 ADR',
      shortcutTargetNodeId: 19,
      systemManaged: true,
      systemKind: 'favorite_reference',
    })
    const control = controls()
    render(<DocumentTree state={ready([favorites, reference])} control={control} />)
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.tree.more(favorites.title) })).toBeNull()
    expect(screen.getAllByRole('button', { name: DOCUMENTS_COPY.tree.createDocument })).toHaveLength(1)

    fireEvent.contextMenu(screen.getByRole('button', { name: DOCUMENTS_COPY.tree.open(reference.title) }))
    expect(await screen.findByRole('menuitem', { name: DOCUMENTS_COPY.tree.copyLink })).not.toBeNull()
    expect(screen.queryByRole('menuitem', { name: DOCUMENTS_COPY.tree.copy })).toBeNull()
    expect(screen.queryByRole('menuitem', { name: DOCUMENTS_COPY.tree.archive(reference.title) })).toBeNull()
  })
})

it.each(['document', 'folder'] as const)('drags a %s into a folder', async type => {
  const source = node({ id: 1, type, title: '来源' })
  const destination = node({ id: 2, type: 'folder', title: '目标' })
  const control = controls()
  render(<DocumentTree state={ready([source, destination])} control={control} />)
  const items = screen.getAllByRole('listitem')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  const handle = screen.getAllByRole('button', { name: type === 'folder' ? '展开文件夹：来源' : '打开文档：来源' }).at(-1)!
  expect(handle.draggable).toBe(true)
  fireEvent.dragStart(handle, { dataTransfer })
  fireEvent.dragOver(items[1], { dataTransfer })
  fireEvent.drop(items[1], { dataTransfer })
  expect(control.moveNode).toHaveBeenCalledWith(1, 2)
})

it('rejects self, descendant and external drops and permits moving back to root', () => {
  const source = node({ id: 1, type: 'folder', title: '来源', parentId: 9 })
  const child = node({ id: 2, type: 'folder', title: '子目录', parentId: 1 })
  const control = controls()
  render(<DocumentTree state={ready([source, child])} control={control} />)
  const items = screen.getAllByRole('listitem')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  fireEvent.drop(items[1], { dataTransfer })
  for (const target of items) {
    fireEvent.dragStart(items[0], { dataTransfer })
    fireEvent.dragOver(target, { dataTransfer })
    fireEvent.drop(target, { dataTransfer })
  }
  expect(control.moveNode).not.toHaveBeenCalled()
  fireEvent.dragStart(items[0], { dataTransfer })
  const root = screen.getByText(DOCUMENTS_COPY.move.dropRoot)
  fireEvent.dragOver(root, { dataTransfer })
  fireEvent.drop(root, { dataTransfer })
  expect(control.moveNode).toHaveBeenCalledWith(1, undefined)
})

it('does not allow dragging without move permission or dragging system folders', () => {
  const control = controls()
  const state = ready([node(), node({ id: 2, type: 'folder', systemManaged: true })])
  state.capabilities.canMove = false
  render(<DocumentTree state={state} control={control} />)
  for (const item of screen.getAllByRole('listitem')) expect(item.draggable).toBe(false)
  expect(control.moveNode).not.toHaveBeenCalled()
})


it.each(['before', 'after'] as const)('reorders %s tied zero positions using a server-owned anchor', position => {
  const source = node({ id: 1, title: '来源', sortOrder: 0 })
  const destination = node({ id: 2, title: '目标', sortOrder: 0 })
  const control = controls()
  render(<DocumentTree state={ready([source, destination])} control={control} />)
  const items = screen.getAllByRole('listitem')
  vi.spyOn(items[1], 'getBoundingClientRect').mockReturnValue({ top: 100, height: 40 } as DOMRect)
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  fireEvent.dragStart(screen.getByRole('button', { name: DOCUMENTS_COPY.tree.open(source.title) }), { dataTransfer })
  for (const name of ['dragOver', 'drop'] as const) {
    const event = createEvent[name](items[1], { dataTransfer })
    Object.defineProperty(event, 'clientY', { value: position === 'before' ? 101 : 139 })
    fireEvent(items[1], event)
    expect(event.defaultPrevented).toBe(true)
  }
  expect(control.moveNode).toHaveBeenCalledWith(1, undefined, position === 'before' ? { beforeNodeId: 2 } : { afterNodeId: 2 })
  expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.tree.moveUp(source.title) })).toBeNull()
  expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.tree.moveDown(source.title) })).toBeNull()
})

it('keeps the root drop slot below the list and only reveals it during an internal drag', () => {
  const source = node({ id: 1, parentId: 9 })
  render(<DocumentTree state={ready([source])} control={controls()} />)
  const root = screen.getByText(DOCUMENTS_COPY.move.dropRoot)
  const list = screen.getByRole('list')
  const row = screen.getByRole('listitem')
  expect(list.nextElementSibling).toBe(root)
  expect(root.classList.contains('invisible')).toBe(true)
  expect(root.getAttribute('aria-hidden')).toBe('true')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  fireEvent.dragStart(row, { dataTransfer })
  expect(list.nextElementSibling).toBe(root)
  expect(root.classList.contains('invisible')).toBe(false)
  expect(root.getAttribute('aria-hidden')).toBe('false')
  fireEvent.dragEnd(row)
  expect(root.classList.contains('invisible')).toBe(true)
})

it('does not clear the row highlight when crossing its title or icon', () => {
  const source = node({ id: 1, title: '来源' })
  const destination = node({ id: 2, type: 'folder', title: '目标' })
  render(<DocumentTree state={ready([source, destination])} control={controls()} />)
  const rows = screen.getAllByRole('listitem')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  fireEvent.dragStart(rows[0], { dataTransfer })
  fireEvent.dragOver(rows[1], { dataTransfer })
  const event = createEvent.dragLeave(rows[1])
  Object.defineProperty(event, 'relatedTarget', { value: rows[1].querySelector('button') })
  fireEvent(rows[1], event)
  expect(rows[1].classList.contains('ring-primary')).toBe(true)
  fireEvent.dragLeave(rows[1])
  expect(rows[1].classList.contains('ring-primary')).toBe(false)
})

it('rejects a node denied move access even when the space allows moving', () => {
  const source = node({ id: 1, canMove: false })
  const destination = node({ id: 2, type: 'folder' })
  const control = controls()
  render(<DocumentTree state={ready([source, destination])} control={control} />)
  const rows = screen.getAllByRole('listitem')
  expect(rows[0].draggable).toBe(false)
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }
  fireEvent.dragStart(rows[0], { dataTransfer })
  fireEvent.drop(rows[1], { dataTransfer })
  expect(control.moveNode).not.toHaveBeenCalled()
})

it('accepts native bubbling over a folder title using the row bounds after leaving the source', () => {
  const source = node({ id: 1, title: '来源' })
  const destination = node({ id: 2, type: 'folder', title: '目标' })
  const control = controls()
  render(<DocumentTree state={ready([source, destination])} control={control} />)
  const rows = screen.getAllByRole('listitem')
  const handle = screen.getByRole('button', { name: DOCUMENTS_COPY.tree.open(source.title) })
  const target = screen.getAllByRole('button', { name: DOCUMENTS_COPY.tree.expand(destination.title) }).at(-1)!
  vi.spyOn(rows[1], 'getBoundingClientRect').mockReturnValue({ top: 100, height: 40 } as DOMRect)
  // The hit element's own box must not decide whether the enclosing folder accepts a drop.
  vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({ top: 110, height: 10 } as DOMRect)
  const dataTransfer = new DataTransfer()
  vi.spyOn(dataTransfer, 'setData')
  vi.spyOn(dataTransfer, 'setDragImage')
  fireEvent(handle, new DragEvent('dragstart', { bubbles: true, cancelable: true, dataTransfer }))
  expect(dataTransfer.setData).toHaveBeenCalledTimes(1)
  expect(dataTransfer.effectAllowed).toBe('move')
  fireEvent.dragLeave(rows[0])
  // A native drag gesture shares one transfer object from dragstart through drop.
  for (const name of ['dragover', 'drop']) {
    const event = new DragEvent(name, { bubbles: true, cancelable: true, dataTransfer, clientY: 120 })
    fireEvent(target, event)
    expect(event.defaultPrevented).toBe(true)
  }
  expect(dataTransfer.dropEffect).toBe('move')
  expect(control.moveNode).toHaveBeenCalledTimes(1)
  expect(control.moveNode).toHaveBeenCalledWith(1, 2)
})

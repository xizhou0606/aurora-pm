import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { PMScopeOptions } from '@/shared/api'
import { QUALITY_COPY } from '@/shared/copy'
import { emptyDraft } from '../../model'
import { Form } from './Form'

const scope: PMScopeOptions = {
  projectSets: [{ id: 1, label: 'Project set' }],
  projects: [{ id: 11, label: 'Project', parentId: 1 }],
  requirements: [{ id: 101, label: 'Requirement', parentId: 11 }],
}

describe('quality Form material paste', () => {
  it('edits a stored JSON resolution note through the rich-text control', async () => {
    const resolutionNote = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '11' }] }],
    })
    render(
      <Form
        initial={{
          ...emptyDraft(),
          requirementId: 101,
          title: 'Resolved issue',
          status: 'review',
          resolutionReason: 'fixed',
          resolutionNote,
        }}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        allowStatusEdit
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={vi.fn()}
        onUpload={vi.fn()}
      />,
    )

    const editor = await screen.findByRole('textbox', {
      name: QUALITY_COPY.lifecycle.resolutionNote,
    })
    expect(editor.tagName).not.toBe('TEXTAREA')
    expect(editor.textContent).toBe('11')
    expect(screen.queryByText(resolutionNote)).toBeNull()
  })

  it('places the assignee in basic information before the defect type', () => {
    render(
      <Form
        initial={emptyDraft()}
        scope={scope}
        users={{
          status: 'ready',
          rows: [{ id: 42, name: 'Teo' }],
        }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={vi.fn()}
        onUpload={vi.fn()}
      />,
    )

    const assignee = screen.getByRole('combobox', { name: QUALITY_COPY.form.assignee })
    const defectType = screen.getByRole('combobox', { name: QUALITY_COPY.form.type })
    const basicSection = screen.getByRole('heading', {
      name: QUALITY_COPY.form.basic,
    }).closest('section')
    const sceneSection = screen.getByRole('heading', {
      name: QUALITY_COPY.form.scene,
    }).closest('section')

    expect(basicSection?.contains(assignee)).toBe(true)
    expect(sceneSection?.contains(assignee)).toBe(false)
    expect(Boolean(assignee.compareDocumentPosition(defectType) & Node.DOCUMENT_POSITION_FOLLOWING))
      .toBe(true)
  })

  it('defaults to test and creates a trimmed workspace environment from the manager', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    const createSceneOption = vi.fn(async ({
      kind,
      name,
    }: {
      kind: 'environment' | 'operating_system' | 'browser'
      name: string
    }) => ({
      id: 2,
      kind,
      name,
      builtIn: false,
      legacy: false,
    }))
    render(
      <Form
        initial={{ ...emptyDraft(), requirementId: 101, title: 'Environment catalog' }}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        sceneOptions={{
          environment: {
            status: 'ready',
            rows: [
              { name: 'test', builtIn: true, legacy: false },
              { id: 1, name: 'staging', builtIn: false, legacy: false },
            ],
          },
          operating_system: { status: 'ready', rows: [] },
          browser: { status: 'ready', rows: [] },
        }}
        sceneOptionMutations={{
          canCreate: true,
          canDelete: (option) => Boolean(option.id && !option.builtIn && !option.legacy && !option.usageCount),
          createState: { status: 'idle', action: 'create_scene_option' },
          deleteState: { status: 'idle', action: 'delete_scene_option' },
          create: createSceneOption,
          remove: vi.fn(),
          reset: vi.fn(),
        }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={onSubmit}
        onUpload={vi.fn()}
      />,
    )

    expect(screen.getByRole('combobox', { name: QUALITY_COPY.form.environment }).textContent)
      .toContain('test')
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptions.environment.manage,
    }))
    await screen.findByRole('dialog', {
      name: QUALITY_COPY.form.sceneOptions.environment.dialogTitle,
    })
    expect(screen.getByRole('button', { name: 'staging' })).toBeTruthy()

    const input = screen.getByRole('textbox', {
      name: QUALITY_COPY.form.sceneOptions.environment.namePlaceholder,
    })
    fireEvent.change(input, { target: { value: '  Pre Release  ' } })
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.sceneOptionCreate }))
    await waitFor(() => expect(createSceneOption).toHaveBeenCalledWith({
      kind: 'environment',
      name: 'Pre Release',
    }))
    await waitFor(() => expect(screen.queryByRole('dialog', {
      name: QUALITY_COPY.form.sceneOptions.environment.dialogTitle,
    })).toBeNull())

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.save }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      foundEnvironment: 'Pre Release',
    })))
  })

  it('rejects duplicate and illegal environment names before calling the API', async () => {
    const createSceneOption = vi.fn()
    render(
      <Form
        initial={emptyDraft()}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        sceneOptions={{
          environment: {
            status: 'ready',
            rows: [{ name: 'test', builtIn: true, legacy: false }],
          },
          operating_system: { status: 'ready', rows: [] },
          browser: { status: 'ready', rows: [] },
        }}
        sceneOptionMutations={{
          canCreate: true,
          canDelete: (option) => Boolean(option.id && !option.builtIn && !option.legacy && !option.usageCount),
          createState: { status: 'idle', action: 'create_scene_option' },
          deleteState: { status: 'idle', action: 'delete_scene_option' },
          create: createSceneOption,
          remove: vi.fn(),
          reset: vi.fn(),
        }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={vi.fn()}
        onUpload={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptions.environment.manage,
    }))
    const input = await screen.findByRole('textbox', {
      name: QUALITY_COPY.form.sceneOptions.environment.namePlaceholder,
    })
    fireEvent.change(input, { target: { value: ' TEST ' } })
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.sceneOptionCreate }))
    expect(await screen.findByText(QUALITY_COPY.form.sceneOptions.environment.duplicate)).toBeTruthy()
    expect(createSceneOption).not.toHaveBeenCalled()

    fireEvent.change(input, { target: { value: 'prod@cn' } })
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.sceneOptionCreate }))
    expect(await screen.findByText(QUALITY_COPY.form.sceneOptionNameInvalid)).toBeTruthy()
    expect(createSceneOption).not.toHaveBeenCalled()
  })

  it('manages operating-system options and blocks deletion when a value is referenced', async () => {
    const remove = vi.fn().mockResolvedValue({ kind: 'operating_system', id: 2 })
    render(
      <Form
        initial={{ ...emptyDraft(), operatingSystem: 'macOS' }}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        sceneOptions={{
          environment: {
            status: 'ready',
            rows: [{ name: 'test', builtIn: true, legacy: false, usageCount: 0 }],
          },
          operating_system: {
            status: 'ready',
            rows: [
              { id: 1, name: 'macOS', builtIn: false, legacy: false, usageCount: 8 },
              { id: 2, name: 'Windows 11', builtIn: false, legacy: false, usageCount: 0 },
              { name: 'Linux', builtIn: false, legacy: true, usageCount: 3 },
            ],
          },
          browser: { status: 'ready', rows: [] },
        }}
        sceneOptionMutations={{
          canCreate: true,
          canDelete: (option) => Boolean(option.id && !option.builtIn && !option.legacy && !option.usageCount),
          createState: { status: 'idle', action: 'create_scene_option' },
          deleteState: { status: 'idle', action: 'delete_scene_option' },
          create: vi.fn(),
          remove,
          reset: vi.fn(),
        }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={vi.fn()}
        onUpload={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptions.operating_system.manage,
    }))
    await screen.findByRole('dialog', {
      name: QUALITY_COPY.form.sceneOptions.operating_system.dialogTitle,
    })
    const inUseDelete = screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptionDeleteInUse(8),
    })
    expect(inUseDelete.hasAttribute('disabled')).toBe(true)
    expect(screen.queryByRole('button', {
      name: QUALITY_COPY.form.sceneOptionDelete('Linux'),
    })).toBeNull()

    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptionDelete('Windows 11'),
    }))
    expect(await screen.findByRole('alertdialog', {
      name: QUALITY_COPY.form.sceneOptionDeleteTitle,
    })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.form.sceneOptionDeleteConfirm,
    }))
    await waitFor(() => expect(remove).toHaveBeenCalledWith({
      kind: 'operating_system',
      option: expect.objectContaining({ id: 2, name: 'Windows 11' }),
    }))
  })

  it('keeps result and material sections ordered and uploads pasted files from reproduction', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    const onUpload = vi.fn(async (_requirementId: number, file: File) => ({
      url: `https://cdn.example.test/${encodeURIComponent(file.name)}`,
    }))
    render(
      <Form
        initial={{ ...emptyDraft(), requirementId: 101, title: 'Pasted materials' }}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={onSubmit}
        onUpload={onUpload}
      />,
    )

    const reproduction = await screen.findByRole('textbox', { name: QUALITY_COPY.form.reproduction })
    const resultsSection = screen.getByText(QUALITY_COPY.form.results).closest('section')
    const materialsSection = screen.getByRole('region', { name: QUALITY_COPY.form.materials })
    const image = new File(['image'], 'proof.png', { type: 'image/png' })
    const document = new File(['rows'], 'tasks.csv', { type: 'text/csv' })
    const clipboardData = (files: File[]) => ({
      files,
      getData: () => '',
      setData: () => undefined,
      types: [],
    })

    expect(resultsSection).not.toBeNull()
    expect(resultsSection!.compareDocumentPosition(materialsSection) & Node.DOCUMENT_POSITION_FOLLOWING)
      .toBeTruthy()
    expect(screen.queryByRole('textbox', { name: QUALITY_COPY.form.expected })).toBeNull()
    expect(screen.queryByRole('textbox', { name: QUALITY_COPY.form.actual })).toBeNull()

    fireEvent.paste(reproduction, { clipboardData: clipboardData([image, document]) })

    await waitFor(() => expect(onUpload).toHaveBeenCalledTimes(2))
    expect(onUpload.mock.calls.map(([requirementId, file]) => [requirementId, file.name]))
      .toEqual([[101, 'proof.png'], [101, 'tasks.csv']])
    await screen.findByRole('img', { name: 'proof.png' })
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.save }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      reproductionSteps: expect.stringContaining('https://cdn.example.test/proof.png'),
      reproductionMaterials: [{ url: 'https://cdn.example.test/tasks.csv' }],
    })))
  })

  it('uploads pasted images before a requirement is selected', async () => {
    const onUpload = vi.fn(async (_requirementId: number, file: File) => ({
      url: `https://cdn.example.test/${encodeURIComponent(file.name)}`,
    }))
    render(
      <Form
        initial={emptyDraft()}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onUpload={onUpload}
      />,
    )

    const reproduction = await screen.findByRole('textbox', { name: QUALITY_COPY.form.reproduction })
    const image = new File(['image'], 'before-requirement.png', { type: 'image/png' })
    fireEvent.paste(reproduction, {
      clipboardData: { files: [image], getData: () => '', setData: () => undefined, types: [] },
    })

    await waitFor(() => expect(onUpload).toHaveBeenCalledWith(0, image))
    await screen.findByRole('img', { name: 'before-requirement.png' })
  })

  it('preserves hidden expected and actual values when editing an existing issue', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(
      <Form
        initial={{
          ...emptyDraft(),
          requirementId: 101,
          title: 'Existing issue',
          expectedResult: 'Existing expected result',
          actualResult: 'Existing actual result',
        }}
        scope={scope}
        users={{ status: 'ready', rows: [] }}
        mutation={{ status: 'idle', action: 'edit' }}
        uploadState={{ status: 'idle', action: 'upload' }}
        canUpload
        submitLabel={QUALITY_COPY.form.save}
        onSubmit={onSubmit}
        onUpload={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.save }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      expectedResult: 'Existing expected result',
      actualResult: 'Existing actual result',
    })))
  })
})

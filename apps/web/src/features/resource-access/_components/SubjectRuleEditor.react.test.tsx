import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'

import { unavailableSubjectCapabilities } from '../model'
import { SubjectRuleEditor } from './SubjectRuleEditor'

describe('SubjectRuleEditor', () => {
  it('fails closed and explains why organization rules cannot be created', () => {
    const capabilities = unavailableSubjectCapabilities()
    capabilities.user = { available: true }
    render(
      <SubjectRuleEditor
        rules={[]}
        disabled={false}
        onEdit={vi.fn()}
        control={{
          kind: 'organization_unit',
          query: '',
          candidates: [],
          capabilities,
          status: 'ready',
          hasMore: false,
          loadingMore: false,
          onKindChange: vi.fn(),
          onQueryChange: vi.fn(),
          onSelect: vi.fn(),
          onAdd: vi.fn(),
        }}
      />,
    )

    expect(screen.getByText(copy.subject.organizationUnavailable)).toBeTruthy()
    expect(screen.getByRole('combobox', { name: copy.subject.kindLabel }).textContent).toContain(copy.subject.kinds.organization_unit)
    expect(screen.getByText(copy.subject.unavailable)).toBeTruthy()
    expect(screen.getByRole('combobox', { name: copy.subject.searchLabel }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByRole('button', { name: copy.subject.add }).hasAttribute('disabled')).toBe(true)
    expect(screen.getByText(copy.subject.emptyRules)).toBeTruthy()
  })
})

import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, mock } from 'bun:test'

import { REPORTING_COPY } from '@/shared/copy'

import type { ReportingDirectory, ReportingMember } from '../model/types'
import { MemberDirectory } from './MemberDirectory'

const member: ReportingMember = {
  id: 101, username: 'alpha', displayName: '张三', groupName: '研发组', roleNames: ['后端'], departmentId: 'dev',
}
const directory: ReportingDirectory = {
  groups: [{ id: 21, code: 'dev', name: '研发组', members: [member] }],
  ungrouped: [], total: 1,
}

describe('reporting directory controls', () => {
  it('collapses independently, selects the group while reopening, and preserves member identity', () => {
    const onSelectMember = mock(() => {})
    const onSelectGroup = mock(() => {})
    render(<MemberDirectory state={{ status: 'ready', data: directory }} selected={{ kind: 'member', id: member.id }} onSelectMember={onSelectMember} onSelectGroup={onSelectGroup} onSelectDepartment={() => {}} />)
    const row = screen.getByTestId('reporting-member-row')
    expect(row.getAttribute('aria-pressed')).toBe('true')
    fireEvent.click(row)
    expect(onSelectMember).toHaveBeenCalledWith(member)
    const toggle = screen.getByTestId('reporting-group-toggle-21')
    toggle.focus()
    expect(document.activeElement).toBe(toggle)
    fireEvent.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(toggle.getAttribute('aria-label')).toBe(REPORTING_COPY.directory.expandGroup('研发组'))
    expect(screen.queryByTestId('reporting-member-row')).toBeNull()
    expect(onSelectGroup).not.toHaveBeenCalled()
    fireEvent.click(screen.getByTestId('reporting-group-row-21'))
    expect(onSelectGroup).toHaveBeenCalledWith(21)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByTestId('reporting-member-row')).toBeTruthy()
    expect(screen.getByTestId('reporting-group-row-0').hasAttribute('disabled')).toBe(true)
  })

  it('preserves department selection and collapse state without invoking group selection', () => {
    const onSelectDepartment = mock(() => {})
    const onSelectGroup = mock(() => {})
    const data: ReportingDirectory = {
      ...directory,
      departments: [{ id: 5, openId: 'dev', parentId: '0', name: '研发部门', order: 1, memberCount: 1 }],
    }
    render(<MemberDirectory state={{ status: 'ready', data }} selected={{ kind: 'department', id: 5 }} onSelectMember={() => {}} onSelectGroup={onSelectGroup} onSelectDepartment={onSelectDepartment} />)
    const row = screen.getByTestId('reporting-dept-row-dev')
    expect(row.getAttribute('aria-pressed')).toBe('true')
    const toggle = screen.getByTestId('reporting-dept-toggle-dev')
    fireEvent.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByTestId('reporting-member-row')).toBeNull()
    expect(onSelectDepartment).not.toHaveBeenCalled()
    row.focus()
    expect(document.activeElement).toBe(row)
    fireEvent.click(row)
    expect(onSelectDepartment).toHaveBeenCalledWith(5)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByTestId('reporting-member-row').getAttribute('aria-pressed')).toBe('false')
    expect(onSelectGroup).not.toHaveBeenCalled()
  })
})

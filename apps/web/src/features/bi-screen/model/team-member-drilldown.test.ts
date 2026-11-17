import { describe, expect, it } from 'bun:test'

import type { TeamMemberDrilldownItem } from '../types'
import {
	filterTeamMemberDrilldownItems,
	isTeamMemberTaskOverdue,
} from './team-member-drilldown'

function item(overrides: Partial<TeamMemberDrilldownItem>): TeamMemberDrilldownItem {
	return {
		id: 1,
		title: '明细',
		status: 'draft',
		created_at: '2026-08-01T00:00:00Z',
		...overrides,
	}
}

describe('BI team member drilldown filters', () => {
	it('shows unfinished and overdue development tasks by default', () => {
		const now = new Date('2026-08-21T00:00:00Z').getTime()
		const rows = [
			item({ id: 1, status: 'in_dev' }),
			item({ id: 2, status: 'completed', due_at: '2026-08-20T00:00:00Z', completed_at: '2026-08-20T12:00:00Z' }),
			item({ id: 3, status: 'completed', due_at: '2026-08-20T00:00:00Z', completed_at: '2026-08-19T12:00:00Z' }),
		]

		expect(filterTeamMemberDrilldownItems({
			items: rows, role: 'development', itemType: 'task', showAll: false, now,
		}).map((row) => row.id)).toEqual([1, 2])
		expect(isTeamMemberTaskOverdue(rows[1], now)).toBe(true)
	})

	it('prefers the server overdue fact and falls back to the effective inherited deadline', () => {
		const now = new Date('2026-08-21T00:00:00Z').getTime()
		expect(isTeamMemberTaskOverdue(item({
			status: 'completed', overdue: false,
			due_at: '2026-08-20T00:00:00Z', completed_at: '2026-08-21T00:00:00Z',
		}), now)).toBe(false)
		expect(isTeamMemberTaskOverdue(item({
			status: 'in_dev', effective_due_at: '2026-08-20T00:00:00Z', due_at_source: 'parent',
		}), now)).toBe(true)
	})

	it('shows only P0 and P1 development bugs by default', () => {
		const rows = [
			item({ id: 1, severity: 'p0' }),
			item({ id: 2, severity: 'P1' }),
			item({ id: 3, severity: 'p2' }),
		]
		expect(filterTeamMemberDrilldownItems({
			items: rows, role: 'development', itemType: 'bug', showAll: false,
		}).map((row) => row.id)).toEqual([1, 2])
	})

	it('shows only unfinished testing bugs by default and restores all rows on demand', () => {
		const rows = [item({ id: 1, status: 'review' }), item({ id: 2, status: 'done' })]
		expect(filterTeamMemberDrilldownItems({
			items: rows, role: 'testing', itemType: 'bug', showAll: false,
		}).map((row) => row.id)).toEqual([1])
		expect(filterTeamMemberDrilldownItems({
			items: rows, role: 'testing', itemType: 'bug', showAll: true,
		})).toEqual(rows)
	})
})

import { describe, expect, it } from 'bun:test'

import { normalizeReport, normalizeRun } from './normalizers'

describe('normalizeReport role focus facts', () => {
  it('carries bug_reports and requirements through snake_case', () => {
    const report = normalizeReport({
      target: { kind: 'member', id: 101, name: '赵六' },
      period: { type: 'weekly', start: '2026-08-31', end: '2026-09-07', anchor_date: '2026-09-06' },
      generated_at: '2026-09-07T10:00:00Z',
      tasks: {},
      bugs: {},
      bug_reports: { total: 12, p0: 2, p1: 4, p2: 6 },
      requirements: { total: 8, created_in_period: 3, done: 2, acceptance: 1, development: 2, pending: 3 },
    })
    expect(report.bugReports).toEqual({ total: 12, p0: 2, p1: 4, p2: 6 })
    expect(report.requirements).toEqual({
      total: 8, createdInPeriod: 3, done: 2, acceptance: 1, development: 2, pending: 3,
    })
  })

  it('carries ui_bugs for design members', () => {
    const report = normalizeReport({
      target: { kind: 'member', id: 103, name: '周九' },
      period: { type: 'weekly', start: '2026-08-31', end: '2026-09-07', anchor_date: '2026-09-06' },
      generated_at: '2026-09-07T10:00:00Z',
      tasks: {},
      bugs: {},
      ui_bugs: { resolved: 4, open: 2, overdue: 1 },
    })
    expect(report.uiBugs).toEqual({ resolved: 4, open: 2, overdue: 1 })
  })

  it('keeps role focus facts absent when the payload omits them', () => {
    const report = normalizeReport({
      target: { kind: 'member', id: 102, name: '张三' },
      period: { type: 'daily', start: '2026-09-07', end: '2026-09-08', anchor_date: '2026-09-07' },
      generated_at: '2026-09-07T10:00:00Z',
      tasks: {},
      bugs: {},
    })
    expect(report.bugReports).toBeUndefined()
    expect(report.requirements).toBeUndefined()
  })

  it('normalizes per-member rollup focus facts for team rankings', () => {
    const report = normalizeReport({
      target: { kind: 'group', id: 31, name: '测试组' },
      period: { type: 'weekly', start: '2026-08-31', end: '2026-09-07', anchor_date: '2026-09-06' },
      generated_at: '2026-09-07T10:00:00Z',
      tasks: {},
      bugs: {},
      bug_reports: { total: 2, p0: 1, p1: 1, p2: 0 },
      members: [
        { member: { id: 1, username: 'a', display_name: '赵六' }, tasks: {}, bugs: {}, bug_reports: { total: 2, p0: 1, p1: 1, p2: 0 } },
        { member: { id: 2, username: 'b', display_name: '孙八' }, tasks: {}, bugs: {}, requirements: { total: 1, created_in_period: 0, done: 1, acceptance: 0, development: 0, pending: 0 } },
      ],
    })
    expect(report.members?.[0]?.bugReports?.total).toBe(2)
    expect(report.members?.[1]?.requirements?.done).toBe(1)
  })
})

describe('normalizeRun overall score', () => {
  it('clamps and rounds the AI score', () => {
    expect(normalizeRun({ id: 1, status: 'completed', overall_score: 86.4 }).overallScore).toBe(86)
    expect(normalizeRun({ id: 1, status: 'completed', overall_score: 250 }).overallScore).toBe(100)
    expect(normalizeRun({ id: 1, status: 'completed', overall_score: -5 }).overallScore).toBe(0)
  })

  it('keeps the score absent for legacy runs', () => {
    expect(normalizeRun({ id: 1, status: 'completed' }).overallScore).toBeUndefined()
    expect(normalizeRun({ id: 1, status: 'completed', overall_score: 'high' }).overallScore).toBeUndefined()
  })
})

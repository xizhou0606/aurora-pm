import { describe, expect, it } from 'bun:test'

import { parseReleaseNotes } from './parse-release-notes'

const CHANGELOG = `# 更新日志

本文件归档 PM 项目的用户可感知变更，按版本倒序维护。

## [1.1.0] - 2026-09-03

### 更新摘要

- 摘要一
- 摘要二

### 新增

- 新增一

### 修复

- 修复一
- 修复二

## [CLI-0.4.0] - 2026-09-01

### 更新摘要

- CLI 摘要

## [2.0.0]

### 调整

- 无日期版本条目
`

describe('parseReleaseNotes', () => {
  it('parses versions, dates, summaries, and detail sections newest first', () => {
    const entries = parseReleaseNotes(CHANGELOG)

    expect(entries.map((entry) => entry.version)).toEqual(['1.1.0', 'CLI-0.4.0', '2.0.0'])
    expect(entries.map((entry) => entry.date)).toEqual([
      '2026-09-03',
      '2026-09-01',
      '',
    ])
    expect(entries[0].summary).toEqual(['摘要一', '摘要二'])
    expect(entries[0].sections).toEqual([
      { title: '新增', items: ['新增一'] },
      { title: '修复', items: ['修复一', '修复二'] },
    ])
  })

  it('keeps entries without a summary or with an omitted date', () => {
    const entries = parseReleaseNotes(CHANGELOG)

    expect(entries[1].summary).toEqual(['CLI 摘要'])
    expect(entries[1].sections).toEqual([])
    expect(entries[2].summary).toEqual([])
    expect(entries[2].sections).toEqual([{ title: '调整', items: ['无日期版本条目'] }])
  })

  it('ignores content before the first version heading', () => {
    const entries = parseReleaseNotes(CHANGELOG)

    expect(entries).toHaveLength(3)
    expect(JSON.stringify(entries)).not.toContain('归档')
  })

  it('returns an empty list for markdown without release blocks', () => {
    expect(parseReleaseNotes('')).toEqual([])
    expect(parseReleaseNotes('只有介绍段落，没有版本区块。')).toEqual([])
  })
})

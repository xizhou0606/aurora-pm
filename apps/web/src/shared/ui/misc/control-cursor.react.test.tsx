import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { Button } from '../primitives/button'
import { Checkbox } from '../primitives/checkbox'
import { Input } from '../primitives/input'

describe('shared control cursor affordances', () => {
  it('uses a pointer for active buttons and a blocked cursor when disabled', () => {
    render(
      <>
        <Button type="button">保存</Button>
        <Button type="button" disabled>删除</Button>
      </>,
    )

    expect(screen.getByRole('button', { name: '保存' }).className).toContain('cursor-pointer')
    expect(screen.getByRole('button', { name: '删除' }).className).toContain('disabled:cursor-not-allowed')
  })

  it('uses a pointer for native date-like inputs without changing text inputs', () => {
    render(
      <>
        <Input aria-label="日期" type="date" />
        <Input aria-label="标题" type="text" />
        <Input aria-label="禁用日期" type="date" disabled />
      </>,
    )

    expect(screen.getByLabelText('日期').className).toContain('cursor-pointer')
    expect(screen.getByLabelText('标题').className).not.toContain('cursor-pointer')
    expect(screen.getByLabelText('禁用日期').className).toContain('disabled:cursor-not-allowed')
  })

  it('uses the same active and disabled cursor contract for checkboxes', () => {
    render(
      <>
        <Checkbox aria-label="选择当前行" />
        <Checkbox aria-label="不可选择" disabled />
      </>,
    )

    expect(screen.getByRole('checkbox', { name: '选择当前行' }).className).toContain('cursor-pointer')
    expect(screen.getByRole('checkbox', { name: '不可选择' }).className).toContain('disabled:cursor-not-allowed')
  })
})

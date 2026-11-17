import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { RequirementRoleOwnerEditor } from './requirement-role-owners'

const poolCopy = copy.aiPM.requirementPool

const people = [
  { id: 1, username: 'bill', display_name: 'Bill' },
  { id: 2, username: 'zixuan', display_name: '子轩' },
]

describe('RequirementRoleOwnerEditor', () => {
  it('keeps the assignment trigger free of nested buttons', () => {
    const { container } = render(
      <RequirementRoleOwnerEditor
        requirement={{
          id: 224,
          title: '示例需求',
          product_owner_user_ids: [1, 2],
        }}
        field="product_owner_user_ids"
        label={poolCopy.roleProduct}
        people={people}
        editable
        pending={false}
        onSave={async () => true}
      />,
    )

    const trigger = screen.getByRole('button', {
      name: poolCopy.assignRole(poolCopy.roleProduct, '示例需求'),
    })
    expect(trigger.querySelector('[data-slot="user-face-stack"]')).not.toBeNull()
    expect(container.querySelectorAll('button button')).toHaveLength(0)
  })
})

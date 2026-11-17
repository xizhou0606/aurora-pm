import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'bun:test'
import { GuideActions } from './GuideActions'
afterEach(cleanup)
it('shows membership states without rendering application buttons or querying per level', () => {
 render(<GuideActions memberships={[{kind:'workspace',id:2,status:'joined'},{kind:'plan_unit',id:49,status:'joined'}]} />)
 expect(screen.getAllByRole('status')).toHaveLength(2)
 expect(screen.queryAllByRole('button')).toHaveLength(0)
})
it('only exposes applications for levels that are not joined or pending', () => {
 render(<GuideActions memberships={[{kind:'workspace',id:2,status:'joined'},{kind:'plan_unit',id:49,status:'requestable'}]} />)
 expect(screen.getAllByRole('button')).toHaveLength(1)
 expect(screen.getByRole('button',{name:'申请加入项目/版本'})).toBeTruthy()
})

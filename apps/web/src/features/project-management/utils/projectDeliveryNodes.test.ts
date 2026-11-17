import { describe, expect, it } from 'bun:test'
import type {
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
} from '@/features/project-management/api'
import {
  availableProjectDeliveryNodeLabels,
  normalizeProjectDeliveryNodeLabels,
  normalizeProjectDeliveryNodes,
  projectDeliveryNodeLabel,
} from '@/features/project-management/utils/projectDeliveryNodes'

const labels: ScrumProjectDeliveryNodeLabel[] = [
  { id: 31, name: '灰度发布', system_key: null, sort_order: 50 },
  { id: 12, name: '设计稿定稿节点', system_key: 'design_freeze', sort_order: 20 },
  { id: 41, name: '已停用节点', system_key: null, sort_order: 60, deleted: true },
  { id: 11, name: '产品评审节点', system_key: 'product_review', sort_order: 10 },
]

describe('project delivery node utilities', () => {
	it('keeps temporary client-key labels and nodes before a project is saved', () => {
		const localLabels = [
			{ client_key: 'product_review', name: '产品评审节点', system_key: 'product_review', sort_order: 10 },
			{ client_key: 'custom_1', name: '上线确认', system_key: null, sort_order: 20 },
		] as ScrumProjectDeliveryNodeLabel[]
		const normalizedLabels = normalizeProjectDeliveryNodeLabels(localLabels)
		expect(normalizedLabels.map((item) => item.client_key)).toEqual(['product_review', 'custom_1'])

		const nodes = normalizeProjectDeliveryNodes([
			{ label_client_key: 'custom_1', owner_user_id: 11 },
		] as ScrumProjectDeliveryNode[], normalizedLabels)
		expect(nodes).toEqual([{
			label_client_key: 'custom_1',
			label_name: '上线确认',
			label_deleted: false,
			type: 'custom:custom_1',
			due_at: null,
			actual_completed_at: null,
			owner_user_id: 11,
		}])
		expect(availableProjectDeliveryNodeLabels(normalizedLabels, nodes).map((item) => item.client_key)).toEqual(['product_review'])
	})

	it('normalizes and orders project-set labels by sort order then id', () => {
    expect(normalizeProjectDeliveryNodeLabels([
      ...labels,
      { id: 0, name: '无效', sort_order: 0 },
      { id: 52, name: '  上线确认  ', sort_order: 50 },
    ]).map((item) => [item.id, item.name])).toEqual([
      [11, '产品评审节点'],
      [12, '设计稿定稿节点'],
      [31, '灰度发布'],
      [52, '上线确认'],
      [41, '已停用节点'],
    ])
  })

  it('normalizes nodes by label id and keeps deleted historical nodes', () => {
    const nodes: ScrumProjectDeliveryNode[] = [
      { label_id: 41, label_name: '已停用节点', label_deleted: true, due_at: null, owner_user_id: null },
      { label_id: 31, due_at: '2026-07-28T18:00:00+08:00', owner_user_id: 11 },
      { label_id: 31, due_at: null, owner_user_id: null },
      { label_id: 0, label_name: '无效节点', due_at: null, owner_user_id: null },
    ]

    expect(normalizeProjectDeliveryNodes(nodes, labels)).toEqual([
      {
        label_id: 31,
        label_name: '灰度发布',
        label_deleted: false,
        type: 'custom:31',
        due_at: '2026-07-28T18:00:00+08:00',
        actual_completed_at: null,
        owner_user_id: 11,
      },
      {
        label_id: 41,
        label_name: '已停用节点',
        label_deleted: true,
        type: 'custom:41',
        due_at: null,
        actual_completed_at: null,
        owner_user_id: null,
      },
    ])
  })

  it('maps legacy fixed node types to the current project-set catalog', () => {
    const normalized = normalizeProjectDeliveryNodes([
      { type: 'design_freeze', due_at: '2026-07-18T18:00:00+08:00', owner_user_id: null },
    ], labels)

    expect(normalized).toEqual([
      {
        label_id: 12,
        label_name: '设计稿定稿节点',
        label_deleted: false,
        type: 'design_freeze',
        due_at: '2026-07-18T18:00:00+08:00',
        actual_completed_at: null,
        owner_user_id: null,
      },
    ])
  })

  it('filters selected and deleted labels from available choices', () => {
    const selected = normalizeProjectDeliveryNodes([
      { label_id: 11, label_name: '产品评审节点', type: 'product_review' },
      { label_id: 41, label_name: '已停用节点', label_deleted: true },
    ], labels)

    expect(availableProjectDeliveryNodeLabels(labels, selected).map((item) => item.id)).toEqual([12, 31])
    expect(projectDeliveryNodeLabel(selected[1], labels)).toBe('已停用节点')
  })
})

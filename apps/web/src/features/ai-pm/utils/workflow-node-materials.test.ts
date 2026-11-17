import { describe, expect, it } from 'bun:test'

import type { WorkflowInstance, WorkflowNodeArtifact } from '../model/types'
import {
  appendUniqueDesignImage,
  attachableDesignURL,
  designFileMaterialURLs,
  designMaterialAttachURL,
  designMaterialDisplayName,
  designMaterialIsOnNode,
  designMaterialName,
  isDesignLinkURL,
  mergeDesignMaterialURLs,
  parseDesignLinkDrafts,
  persistedDesignLinkURL,
  persistedDesignLinkURLs,
  resolveNodeDesignMaterialURLs,
  selectEffectiveTestCaseArtifacts,
} from '../utils/workflow-node-materials'

const instance = (overrides: Partial<WorkflowInstance> = {}): WorkflowInstance => ({
  id: 77,
  template_id: 5,
  template_name: '需求生命周期',
  template_version: 3,
  status: 'running',
  subject_type: 'requirement',
  subject_id: 42,
  current_node_ids: ['visual-design'],
  current_nodes: [],
  timeline_nodes: [
    { id: 'start', type: 'start', name: '开始', status: 'completed' },
    { id: 'interaction-design', type: 'human_task', name: '交互设计', status: 'completed', config: { require_design_image: true } },
    { id: 'visual-design', type: 'human_task', name: '视觉设计', status: 'active', config: { require_design_image: true } },
    { id: 'skipped-design', type: 'human_task', name: '未选中设计', status: 'skipped', config: { require_design_image: true } },
    { id: 'end', type: 'end', name: '结束', status: 'pending' },
  ],
  definition_edges: [],
  node_runs: [],
  approvals: [],
  transition_history: [
    { id: 3, node_id: 'interaction-design', node_name: '交互设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '交互', materials: [{ kind: 'design_image', url: 'https://design.example.test/interaction.png' }], occurred_at: '2026-08-09T07:00:00Z' },
    { id: 4, node_id: 'visual-design', node_name: '视觉设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '视觉', materials: [{ kind: 'design_image', url: 'https://design.example.test/visual-draft.png' }], occurred_at: '2026-08-09T08:00:00Z' },
    { id: 5, node_id: 'visual-design', node_name: '视觉设计', node_type: 'human_task', action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '视觉', materials: [{ kind: 'design_image', url: 'https://design.example.test/visual.png' }], occurred_at: '2026-08-09T09:00:00Z' },
    { id: 6, node_id: 'skipped-design', node_name: '未选中设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '其他', materials: [{ kind: 'design_image', url: 'https://design.example.test/skipped.png' }], occurred_at: '2026-08-09T09:30:00Z' },
  ],
  completed_steps: 2,
  total_steps: 4,
  progress: 50,
  started_at: '2026-08-07T00:00:00Z',
  ...overrides,
})

describe('workflow-node-materials', () => {
  it('keeps every design image on the node that produced it', () => {
    const current = instance()
    expect(resolveNodeDesignMaterialURLs(current, 'interaction-design', 'https://design.example.test/global.png')).toEqual([
      'https://design.example.test/interaction.png',
    ])
    expect(resolveNodeDesignMaterialURLs(current, 'visual-design', 'https://design.example.test/global.png')).toEqual([
      'https://design.example.test/visual-draft.png',
      'https://design.example.test/visual.png',
    ])
    expect(resolveNodeDesignMaterialURLs(current, 'skipped-design', 'https://design.example.test/global.png')).toEqual([
      'https://design.example.test/skipped.png',
    ])
  })

  it('does not copy an ambiguous legacy URL onto a second design node', () => {
    const current = instance({
      transition_history: [
        { id: 3, node_id: 'interaction-design', node_name: '交互设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '交互', materials: [{ kind: 'design_image', url: 'https://design.example.test/interaction.png' }], occurred_at: '2026-08-09T07:00:00Z' },
      ],
    })
    expect(resolveNodeDesignMaterialURLs(current, 'visual-design', 'https://design.example.test/global.png')).toEqual([])
  })

  it('lists every uploaded test-case file and hides AI files once a manual upload exists', () => {
    const artifacts: WorkflowNodeArtifact[] = [
      { id: 12, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: 'AI自动生成.csv', content_type: 'text/csv', size: 4096, url: 'https://cdn.example.test/ai.csv', uploader_user_id: 8, created_at: '2026-08-09T08:02:00Z' },
      { id: 11, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '人工确认用例.xlsx', content_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 8192, url: 'https://cdn.example.test/manual.xlsx', uploader_user_id: 8, created_at: '2026-08-09T08:01:00Z' },
      { id: 14, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '边界用例.csv', content_type: 'text/csv', size: 1024, url: 'https://cdn.example.test/manual-2.csv', uploader_user_id: 8, created_at: '2026-08-09T08:03:00Z' },
    ]
    expect(selectEffectiveTestCaseArtifacts(artifacts).map((artifact) => artifact.name)).toEqual([
      '人工确认用例.xlsx',
      '边界用例.csv',
    ])
  })

  it('falls back to every AI file when the node has no manual upload', () => {
    const artifacts: WorkflowNodeArtifact[] = [
      { id: 13, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: '旧版AI用例.csv', content_type: 'text/csv', size: 2048, url: 'https://cdn.example.test/ai-old.csv', uploader_user_id: 8, created_at: '2026-08-09T08:00:00Z' },
      { id: 14, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: '最新版AI用例.csv', content_type: 'text/csv', size: 4096, url: 'https://cdn.example.test/ai-latest.csv', uploader_user_id: 8, created_at: '2026-08-09T08:02:00Z' },
    ]
    expect(selectEffectiveTestCaseArtifacts(artifacts).map((artifact) => artifact.name)).toEqual([
      '旧版AI用例.csv',
      '最新版AI用例.csv',
    ])
  })

  it('appends a newly uploaded design image without replacing earlier files', () => {
    const next = appendUniqueDesignImage(
      {},
      'visual-design',
      [{ url: 'https://design.example.test/visual-draft.png', name: 'visual-draft.png' }],
      { url: 'https://design.example.test/visual.png', name: 'visual.png' },
    )
    expect(next['visual-design']?.map((item) => item.url)).toEqual([
      'https://design.example.test/visual-draft.png',
      'https://design.example.test/visual.png',
    ])
  })

  it('merges requirement library design URLs ahead of node history without duplicates', () => {
    expect(mergeDesignMaterialURLs(
      ['https://design.example.test/visual.png', 'https://www.figma.com/design/early/PM'],
      ['https://www.figma.com/design/early/PM', 'https://www.figma.com/design/extra/PM'],
    )).toEqual([
      'https://www.figma.com/design/early/PM',
      'https://www.figma.com/design/extra/PM',
      'https://design.example.test/visual.png',
    ])
  })

  it('attaches library files with the stored origin URL instead of the download path', () => {
    const row = {
      url: '/api/v1/platform/requirements/42/materials/9/download',
      origin_url: 'https://cdn.example.test/checkout.png',
      name: 'checkout.png',
    }
    expect(designMaterialAttachURL(row)).toBe('https://cdn.example.test/checkout.png')
    expect(designMaterialIsOnNode(row, [])).toBe(false)
    expect(designMaterialIsOnNode(row, ['https://cdn.example.test/checkout.png'])).toBe(true)
    expect(designMaterialIsOnNode(row, [row.url])).toBe(true)
  })

  it('turns a library download path into an absolute URL the node action can accept', () => {
    expect(attachableDesignURL(
      '/api/v1/platform/requirements/42/materials/9/download',
      'http://127.0.0.1:5175',
    )).toBe('http://127.0.0.1:5175/api/v1/platform/requirements/42/materials/9/download')
  })

  it('splits pasted design links so a node can bind more than one URL at once', () => {
    expect(parseDesignLinkDrafts(
      'https://www.figma.com/design/abc/apps\nhttps://www.figma.com/developers/apps111',
    )).toEqual([
      'https://www.figma.com/design/abc/apps',
      'https://www.figma.com/developers/apps111',
    ])
  })

  it('lists every saved design link instead of collapsing to the latest Figma URL', () => {
    const rows = [
      { url: 'https://www.figma.com/developers/apps111', origin_url: 'https://www.figma.com/developers/apps111', content_type: 'text/uri-list', name: 'apps111' },
      { url: '/api/v1/platform/requirements/42/materials/9/download', origin_url: 'https://cdn.example.test/checkout.png', name: 'checkout.png' },
      { url: 'https://www.figma.com/design/abc123/PM', origin_url: 'https://www.figma.com/design/abc123/PM', content_type: 'text/uri-list', name: 'PM' },
    ]
    expect(persistedDesignLinkURLs(rows)).toEqual([
      'https://www.figma.com/developers/apps111',
      'https://www.figma.com/design/abc123/PM',
    ])
    expect(persistedDesignLinkURL(rows)).toBe('https://www.figma.com/design/abc123/PM')
  })

  it('echoes the latest Figma design URL into the node link field and keeps images as files', () => {
    const rows = [
      { url: 'https://figma.com/developers/apps', origin_url: 'https://figma.com/developers/apps', content_type: 'text/uri-list', name: 'apps' },
      { url: '/api/v1/platform/requirements/42/materials/9/download', origin_url: 'https://cdn.example.test/checkout.png', name: 'checkout.png' },
      { url: 'https://www.figma.com/design/abc123/PM', origin_url: 'https://www.figma.com/design/abc123/PM', content_type: 'text/uri-list', name: 'PM' },
    ]
    expect(isDesignLinkURL('https://www.figma.com/design/abc123/PM')).toBe(true)
    expect(isDesignLinkURL('https://cdn.example.test/checkout.png')).toBe(false)
    expect(persistedDesignLinkURL(rows, [], 'https://figma.com/developers/apps')).toBe('https://www.figma.com/design/abc123/PM')
    expect(designFileMaterialURLs(rows, ['https://design.example.test/visual.png'])).toEqual([
      'https://cdn.example.test/checkout.png',
      'https://design.example.test/visual.png',
    ])
  })

  it('echoes a saved HTTP design link when no Figma file URL exists', () => {
    expect(persistedDesignLinkURL(
      [{ url: 'https://www.figma.com/developers/apps', content_type: 'text/uri-list', name: 'apps' }],
      [],
      'https://figma.com/developers/apps',
    )).toBe('https://www.figma.com/developers/apps')
  })

  it('does not treat download, apps, or api path segments as a design file name', () => {
    expect(designMaterialDisplayName(
      'http://127.0.0.1:5175/api/v1/platform/requirements/42/materials/9/download',
      '设计稿',
    )).toBe('设计稿')
    expect(designMaterialDisplayName('http://127.0.0.1:5175/apps', '设计稿')).toBe('设计稿')
    expect(designMaterialName(
      attachableDesignURL('/api/v1/platform/requirements/42/materials/9/download'),
      [{ url: '/api/v1/platform/requirements/42/materials/9/download', name: '收银台效果图' }],
      '设计稿',
    )).toBe('收银台效果图')
  })
})

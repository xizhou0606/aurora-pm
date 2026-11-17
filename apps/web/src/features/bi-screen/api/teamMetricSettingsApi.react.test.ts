import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'
import {
  clearRuntimeSession,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'

import {
  putTeamMetricSettings,
  requestTeamMetricSettings,
} from './api'
import type { TeamMetricSettings } from '../types'

const settings: TeamMetricSettings = {
  version: 2,
  can_manage: true,
  configured: true,
  valid: true,
  requirement_workflow_template_id: 12,
  requirement_workflow_template_name: '需求评审',
  requirement_workflow_published_version: 3,
  approval_node_id: 'product-review',
  approval_node_name: '产品评审',
  options: [{
    template_id: 12,
    template_name: '需求评审',
    published_version: 3,
    approval_nodes: [{ node_id: 'product-review', node_name: '产品评审' }],
  }],
}

describe('BI team metric settings network adapter', () => {
  const originalAdapter = platformApi.defaults.adapter

  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('bi-token')
    setRuntimeWorkspace(23)
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm.example.test',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('loads the workspace-scoped workflow and approval-node options', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return { data: { data: settings }, status: 200, statusText: 'OK', headers: {}, config }
    }

    await expect(requestTeamMetricSettings()).resolves.toEqual(settings)
    expect(captured?.method).toBe('get')
    expect(captured?.url).toBe('/executive-dashboard/bi-screen/team-metric-settings')
    expect(captured?.headers.Authorization).toBe('Bearer bi-token')
    expect(captured?.headers['X-Workspace-ID']).toBe('23')
  })

  it('saves an optimistic-concurrency version with the selected node', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: { ...settings, version: 3 } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const body = {
      expected_version: 2,
      requirement_workflow_template_id: 12,
      approval_node_id: 'product-review',
    }

    await expect(putTeamMetricSettings(body)).resolves.toMatchObject({ version: 3 })
    expect(captured?.method).toBe('put')
    expect(captured?.url).toBe('/executive-dashboard/bi-screen/team-metric-settings')
    expect(JSON.parse(String(captured?.data))).toEqual(body)
  })

  it('rejects a malformed settings contract', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: { ...settings, options: [{ template_id: 12 }] } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    await expect(requestTeamMetricSettings()).rejects.toMatchObject({
      code: 'BI_TEAM_METRIC_SETTINGS_INVALID',
    })
  })
})

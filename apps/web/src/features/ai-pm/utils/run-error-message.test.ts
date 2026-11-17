import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { aiRunErrorMessage } from '../utils/run-error-message'

describe('aiRunErrorMessage', () => {
  it('maps internal AI failures to safe actionable copy', () => {
    expect(aiRunErrorMessage('AI runtime returned status 502: test case output did not satisfy fixed QA rules'))
      .toBe(copy.aiPM.runs.errorInvalidOutput)
    expect(aiRunErrorMessage('AI runtime returned status 502: context canceled'))
      .toBe(copy.aiPM.runs.errorInterrupted)
    expect(aiRunErrorMessage('context deadline exceeded'))
      .toBe(copy.aiPM.runs.errorTimeout)
    expect(aiRunErrorMessage('provider unavailable: status 503'))
      .toBe(copy.aiPM.runs.errorProvider)
    expect(aiRunErrorMessage('test case model response was not a complete valid JSON artifact'))
      .toBe(copy.aiPM.runs.errorInvalidOutput)
    expect(aiRunErrorMessage('test case input exceeds supported batching limits; split the plan unit into smaller runs'))
      .toBe(copy.aiPM.runs.errorInputTooLarge)
    expect(aiRunErrorMessage('internal implementation detail'))
      .toBe(copy.aiPM.runs.errorUnknown)
    expect(aiRunErrorMessage('飞书需求正文未读取，无法执行本次 AI 分析'))
      .toBe(copy.aiPM.runs.errorSourceUnavailable)
    expect(aiRunErrorMessage('飞书需求正文为空，无法执行本次 AI 分析'))
      .toBe(copy.aiPM.runs.errorSourceUnavailable)
  })
})

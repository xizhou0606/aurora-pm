import { describe, expect, it } from 'bun:test'

import { captureResultMessage } from './capture-message'

describe('capture result message', () => {
  it('does not let iframe transport metadata overwrite the result envelope', () => {
    expect(captureResultMessage({
      source: 'pm-design-review-extension-frame',
      type: 'PM_DESIGN_REVIEW_FRAME_READY',
      request_id: 'capture-1',
      screenshot: 'data:image/png;base64,AA==',
    })).toMatchObject({
      source: 'pm-design-review-extension',
      type: 'PM_DESIGN_REVIEW_CAPTURE_RESULT',
      request_id: 'capture-1',
    })
  })
})

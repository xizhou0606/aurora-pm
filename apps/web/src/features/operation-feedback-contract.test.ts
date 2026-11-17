import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { describe, expect, it } from 'bun:test'

import { callArguments, callCallee, is, lineAt, objectProperties, parseGate, walkNode } from '@/shared/testing/oxc-gates'

const MUTATION_FILES = [
  'src/features/project-management/hooks/useProjectMutations.ts',
  'src/features/project-management/hooks/useContentMutations.ts',
  'src/features/project-management/hooks/useDeliveryMutations.ts',
  'src/features/ai-pm/api/api.ts',
] as const

function mutationCallsWithoutFeedback(path: string): number[] {
  const content = readFileSync(path, 'utf8')
  const program = parseGate(path, content)
  const missing: number[] = []
  walkNode(program, (node) => {
    if (!is.callExpression(node) || callCallee(node) !== 'useMutation') return
    const options = callArguments(node)[0]
    const hasFeedback = Boolean(options && is.objectLiteral(options) && objectProperties(options).some(({ name, value }) => (
      name === 'meta' && is.callExpression(value) && callCallee(value) === 'operationFeedback'
    )))
    if (!hasFeedback) missing.push(lineAt(content, node.start))
  })
  return missing
}

describe('project and requirement API feedback contract', () => {
  it('requires every write mutation to declare success and failure feedback', () => {
    const missing = MUTATION_FILES.flatMap((relativePath) => {
      const path = join(process.cwd(), relativePath)
      return mutationCallsWithoutFeedback(path).map((line) => `${relativePath}:${line}`)
    })
    expect(missing).toEqual([])
  })
})

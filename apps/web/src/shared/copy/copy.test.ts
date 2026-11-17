import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'

import { describe, expect, it } from 'bun:test'

import {
  childArray,
  childField,
  identifierName,
  is,
  lineAndCharacterAt,
  parseGate,
  type GateNode,
  walkNode,
} from '@/shared/testing/oxc-gates'

import { copy } from './index'

const sourceRoot = join(process.cwd(), 'src')
const userFacingAttributes = new Set([
  'alt',
  'aria-description',
  'aria-label',
  'aria-roledescription',
  'placeholder',
  'title',
])
const userFacingProperties = new Set([
  'ariaLabel',
  'caption',
  'children',
  'closeLabel',
  'description',
  'emptyText',
  'errorTitle',
  'header',
  'label',
  'loadingLabel',
  'placeholder',
  'refreshingLabel',
  'retryLabel',
  'title',
])

function sourceFiles(relativeRoot = '.'): string[] {
  const absoluteRoot = join(sourceRoot, relativeRoot)
  const files: string[] = []

  function visit(directory: string) {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name)
      if (entry.isDirectory()) visit(path)
      else if (/\.(css|ts|tsx)$/.test(entry.name) && !/\.(?:react-)?test\./.test(entry.name)) {
        files.push(path)
      }
    }
  }

  visit(absoluteRoot)
  return files
}

function literalValue(node: GateNode | undefined, content: string): string | undefined {
  if (!node) return undefined
  if (is.stringLiteral(node)) return String((node as unknown as { value: unknown }).value)
  if (is.templateLiteral(node)) {
    const quasis = childArray(node, 'quasis').map((quasi) => (
      String((childField(quasi, 'value') as unknown as { cooked?: string } | undefined)?.cooked ?? '')
    ))
    const expressions = childArray(node, 'expressions')
    let value = quasis[0] ?? ''
    expressions.forEach((_expression, index) => {
      value += '${…}' + (quasis[index + 1] ?? '')
    })
    return value
  }
  if (is.binaryExpression(node) && (node as unknown as { operator: string }).operator === '+') {
    const fragments = [
      literalValue(childField(node, 'left'), content),
      literalValue(childField(node, 'right'), content),
    ].filter((value): value is string => value !== undefined)
    return fragments.length ? fragments.join('${…}') : undefined
  }
  if (is.jsxExpressionContainer(node)) return literalValue(childField(node, 'expression'), content)
  return undefined
}

function visibleCopyViolations(file: string, content = readFileSync(file, 'utf8')): string[] {
  const source = parseGate(file, content)
  const violations: string[] = []

  function report(node: GateNode, value: string) {
    if (!value.trim()) return
    const { line, character } = lineAndCharacterAt(content, node.start)
    violations.push(
      `${relative(sourceRoot, file)}:${line}:${character} ${JSON.stringify(value)}`,
    )
  }

  walkNode(source, (node, parent) => {
    if (is.jsxText(node)) {
      report(node, String((node as unknown as { value: unknown }).value))
    }

    if (is.jsxAttribute(node)) {
      const name = identifierName(childField(node, 'name')) ?? ''
      if (userFacingAttributes.has(name) || userFacingProperties.has(name)) {
        const value = literalValue(childField(node, 'value'), content)
        if (value !== undefined) report(node, value)
      }
    }

    if (
      is.jsxExpressionContainer(node) &&
      (parent?.type === 'JSXElement' || parent?.type === 'JSXFragment')
    ) {
      const value = literalValue(childField(node, 'expression'), content)
      if (value !== undefined) report(node, value)
    }

    if (is.objectProperty(node)) {
      const name = identifierName(childField(node, 'key')) ?? ''
      if (userFacingProperties.has(name)) {
        const value = literalValue(childField(node, 'value'), content)
        if (value !== undefined) report(node, value)
      }
    }
  })
  return violations
}

describe('React migration copy and framework token boundaries', () => {
  it('matches the project-management tab and activity headings to the Vue baseline', () => {
    expect(copy.projectManagement.tabs.activity).toBe('动态')
    expect(copy.projectManagement.activity.heading).toBe('动态')
    expect(copy.projectManagement.activity.errorTitle).toBe('活动记录加载失败')
  })

  it('keeps JSX text and user-facing attributes or props in the typed registry', () => {
    const productionTsx = sourceFiles().filter((file) => file.endsWith('.tsx'))
    const violations = productionTsx.flatMap((file) => visibleCopyViolations(file))

    expect(violations).toEqual([])
  })

  it('detects direct, component-prop, and interpolated inline copy', () => {
    const fixture = [
      'const id = 7;',
      'const view = <ResponsivePanel description="Inline description" closeLabel="Close">',
      '  Visible child',
      '  <span title={`Details ${id}`}>Nested text</span>',
      '</ResponsivePanel>;',
    ].join('\n')
    const violations = visibleCopyViolations(join(sourceRoot, '__copy-fixture.tsx'), fixture).join('\n')

    expect(violations).toContain('Inline description')
    expect(violations).toContain('Close')
    expect(violations).toContain('Visible child')
    expect(violations).toContain('Details ${…}')
    expect(violations).toContain('Nested text')
  })

  it('does not introduce Element Plus variables into React surfaces', () => {
    const productionTsx = sourceFiles().filter((file) => file.endsWith('.tsx'))
    const reactSources = [
      ...productionTsx,
      ...sourceFiles('shared'),
    ]
    const violations = [...new Set(reactSources)].filter((file) => {
      const content = readFileSync(file, 'utf8')
      return content.includes('--el-') && !content.includes('@ts-nocheck')
    })
    expect(violations).toEqual([])
  })
})

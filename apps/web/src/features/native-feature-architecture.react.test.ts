import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'

import { describe, expect, it } from 'bun:test'

import {
  childArray,
  childField,
  GateTypeIndex,
  expressionName,
  identifierName,
  importDeclarationView,
  is,
  parseGate,
  type GateNode,
  type GateSource,
  walkNode,
} from '@/shared/testing/oxc-gates'

const FEATURE_NAMES = [
  'inbox',
  'work-items',
  'bi-screen',
  'quality-management',
  'design-management',
  'bitables',
  'resource-access',
] as const

const FORBIDDEN_BOOLEAN_PROPS = new Set([
  'loading',
  'error',
  'refreshing',
  'saving',
  'moving',
  'deleting',
  'pending',
])
const FORBIDDEN_QUERY_IMPORTS = new Set([
  'useQuery',
  'useQueries',
  'useSuspenseQuery',
  'useSuspenseQueries',
  'useInfiniteQuery',
  'useMutation',
  'useQueryClient',
  'QueryClient',
])
const FORBIDDEN_QUERY_CACHE_CALLS = new Set([
  'invalidateQueries',
  'removeQueries',
  'resetQueries',
  'setQueryData',
  'setQueriesData',
])
const FORBIDDEN_TRANSPORT_CALLS = new Set([
  'fetch',
  'requestPage',
  'requestJson',
])
const FORBIDDEN_HOST_BUSINESS_SYMBOLS = new Set([
  'downloadHostPlatform',
  'hostPlatformApi',
  'requestHostPlatform',
  'requestHostPlatformPage',
  'requestHostScopeOptions',
  'requestHostUsers',
  'requestPMGlobalWorkItemsPage',
  'uploadHostPlatform',
])
const FORBIDDEN_WORK_ITEMS_ENDPOINTS = [
  '/global-work-items',
  '/tasks/batch-update',
  '/tasks/create-options',
  '/tasks/test-domain',
  '/time-logs',
] as const
const FORBIDDEN_WORK_ITEMS_SOURCE_PATTERNS = [
  /\/tasks\/\$\{[^}]+\}\/links(?:[`'"/?]|$)/,
  /\/tasks\/\$\{[^}]+\}\/time-logs(?:[`'"/?]|$)/,
] as const
const DERIVED_STATUS_KEYS = new Set([
  'idle',
  'loading',
  'empty',
  'error',
  'ready',
  'refreshing',
  'stale_error',
  'loadingMore',
  'hasMore',
])
const FORBIDDEN_QUALITY_NATIVE_CONTROLS = new Set([
  'button',
  'input',
  'select',
  'textarea',
  'table',
  'thead',
  'tbody',
  'tr',
  'th',
  'td',
])

/** Files that intentionally import API clients for React context injection. */
const TRANSPORT_ALLOWLIST = new Set([
  'src/features/bi-screen/components/environment.tsx',
  'src/features/quality-management/components/environment.tsx',
  'src/features/inbox/components/environment.tsx',
])

/** Components that intentionally accept lifecycle boolean props for form submission state. */
const LIFECYCLE_BOOLEAN_ALLOWLIST = new Set([
  'src/features/quality-management/components/form/ResolutionFields.tsx:ResolutionFields.pending',
  'src/features/quality-management/components/form/BasicSection.tsx:BasicSection.pending',
])

const CSS_SELECTOR_ALLOWLIST: Readonly<Record<string, Readonly<Record<string, string>>>> = {
  'work-items/components/collection/daypilot.css': {
    '.work-items-daypilot .scheduler_default_main': 'Maps the DayPilot Lite Scheduler root to PM design tokens.',
    '.work-items-daypilot .scheduler_default_corner': 'Normalizes the third-party corner header surface.',
    '.work-items-daypilot .scheduler_default_rowheader': 'Normalizes the third-party row-header surface.',
    '.work-items-daypilot .scheduler_default_timeheadercol': 'Normalizes the third-party day header surface.',
    '.work-items-daypilot .scheduler_default_timeheadergroup': 'Normalizes the third-party month header surface.',
    '.work-items-daypilot .scheduler_default_scrollable': 'Contains Scheduler-owned horizontal and vertical scrolling.',
    '.work-items-daypilot .scheduler_default_rowheader_inner': 'Removes default padding around PM task header markup.',
    '.work-items-daypilot .scheduler_default_rowheader.work-items-daypilot-group': 'Distinguishes PM-owned flat owner group rows without using Pro tree APIs.',
    '.work-items-daypilot-corner': 'Lays out the PM number and task-description header inside DayPilot cornerHtml.',
    '.work-items-daypilot-corner > span': 'Continues the PM number-column divider through DayPilot cornerHtml.',
    '.work-items-daypilot-group-header': 'Lays out PM owner grouping inside DayPilot row-header HTML.',
    '.work-items-daypilot-task-header': 'Lays out PM task numbering and description inside DayPilot row-header HTML.',
    '.work-items-daypilot-index': 'Renders the PM task index column inside DayPilot row-header HTML.',
    '.work-items-daypilot-description': 'Contains PM task key/title overflow inside DayPilot row-header HTML.',
    '.work-items-daypilot-description > strong': 'Styles the task key inside DayPilot row-header HTML.',
    '.work-items-daypilot-description > span': 'Truncates the task title inside DayPilot row-header HTML.',
    '.work-items-daypilot .scheduler_default_event.work-items-daypilot-event': 'Allows the PM task label to extend past the DayPilot duration box.',
    '.work-items-daypilot .work-items-daypilot-event .scheduler_default_event_inner': 'Replaces only the DayPilot event chrome with PM duration-bar markup.',
    '.work-items-daypilot-event-content': 'Positions PM duration and title markup inside a DayPilot event.',
    '.work-items-daypilot-duration': 'Restores the reference green duration bar inside DayPilot event DOM.',
    '.work-items-daypilot-event-title': 'Places the task label after the DayPilot-owned duration width.',
  },
}

const sourceRoot = join(process.cwd(), 'src')
const featureRoot = join(sourceRoot, 'features')

function featureDirectory(name: typeof FEATURE_NAMES[number]): string {
  return join(featureRoot, name)
}

function isTestSource(fileName: string): boolean {
  return /(?:\.react-test|\.test|\.spec)\.[cm]?[jt]sx?$/.test(fileName)
}

function isWithin(fileName: string, directory: string): boolean {
  return fileName === directory || fileName.startsWith(`${directory}/`)
}

function productionSourceFiles(directory: string): string[] {
  const files: string[] = []
  const visit = (current: string) => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      const path = join(current, entry.name)
      if (entry.isDirectory()) visit(path)
      else if (/\.[cm]?[jt]sx?$/.test(entry.name) && !isTestSource(path)) files.push(path)
    }
  }
  visit(directory)
  return files
}

/** tsconfig include 的字面等价：src 下全部 ts/tsx/d.ts（含测试，测试由各用例自行过滤）。 */
function projectSources(): GateSource[] {
  const files: string[] = []
  const visit = (directory: string) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name)
      if (entry.isDirectory()) visit(path)
      else if (/\.[cm]?[jt]sx?$/.test(entry.name)) files.push(path)
    }
  }
  visit(sourceRoot)
  return files.map((fileName) => {
    const content = readFileSync(fileName, 'utf8')
    return { fileName, content, program: parseGate(fileName, content) }
  })
}

function sourceLabel(fileName: string): string {
  return relative(process.cwd(), fileName)
}

function capitalized(name: string): boolean {
  return /^[A-Z]/.test(name)
}

function hookReturnTypeReference(node: GateNode): boolean {
  return is.typeReference(node) &&
    (expressionName(childField(node, 'typeName')) ?? '') === 'ReturnType' &&
    Boolean(childArray(node, 'typeArguments').some((argument) => (
      is.typeQuery(argument) &&
      /^use[A-Z0-9_$][A-Za-z0-9_$]*$/.test(
        (expressionName(childField(argument, 'exprName')) ?? '').split('.').at(-1) ?? '',
      )
    )))
}

function forbiddenLifecycleBoolean(name: string): boolean {
  const normalized = name.toLowerCase()
  return FORBIDDEN_BOOLEAN_PROPS.has(normalized) ||
    /^is.*(?:loading|error|refreshing|saving|moving|deleting|pending)$/.test(normalized)
}

function componentFunctions(source: GateSource): Array<{
  name: string
  parameter: GateNode
}> {
  const components: Array<{ name: string; parameter: GateNode }> = []
  const add = (name: string, callable: GateNode) => {
    const parameter = childArray(callable, 'params')[0]
    if (parameter) components.push({ name, parameter })
  }
  const wrapped = (node: GateNode): GateNode | undefined => {
    if (is.arrowFunction(node) || is.functionExpression(node)) return node
    if (
      is.callExpression(node) &&
      (expressionName(childField(node, 'callee')) === 'memo' ||
        expressionName(childField(node, 'callee')) === 'forwardRef')
    ) {
      const candidate = childArray(node, 'arguments')[0]
      if (candidate && (is.arrowFunction(candidate) || is.functionExpression(candidate))) {
        return candidate
      }
      return undefined
    }
    return undefined
  }
  walkNode(source.program, (node, parent) => {
    if (
      is.functionDeclaration(node) &&
      capitalized(identifierName(childField(node, 'id')) ?? '')
    ) add(identifierName(childField(node, 'id')) ?? 'default', node)
    if (
      parent?.type === 'ExportDefaultDeclaration' &&
      is.functionDeclaration(node) &&
      !childField(node, 'id')
    ) add('default', node)
    if (
      is.variableDeclaration(node) &&
      is.identifier(childField(node, 'id')) &&
      capitalized(identifierName(childField(node, 'id')) ?? '') &&
      childField(node, 'init')
    ) {
      const initializer = childField(node, 'init')
      const callable = initializer ? wrapped(initializer) : undefined
      if (callable) add(identifierName(childField(node, 'id')) ?? '', callable)
    }
  })
  return components
}

function apiModuleSpecifier(value: string): boolean {
  return /(?:^|\/)api(?:\/|$)/.test(value) ||
    /(?:^|\/)(?:client|resources|contracts|normalizers)$/.test(value)
}

function callName(expression: GateNode): string | undefined {
  if (is.identifier(expression)) return identifierName(expression)
  if (is.propertyAccess(expression)) return identifierName(childField(expression, 'property'))
  return undefined
}

function guardedSurface(fileName: string, root: string): boolean {
  return isWithin(fileName, join(root, 'components')) ||
    isWithin(fileName, join(root, 'routes')) ||
    fileName === join(root, 'Page.tsx')
}

function cssSelectors(content: string): string[] {
  const withoutComments = content.replace(/\/\*[\s\S]*?\*\//g, '')
  const selectors: string[] = []
  for (const match of withoutComments.matchAll(/([^{}]+)\{/g)) {
    const block = match[1]?.trim()
    if (!block || block.startsWith('@')) continue
    selectors.push(...block.split(',').map((selector) => selector.trim()).filter(Boolean))
  }
  return selectors
}

describe('native feature architecture gate', () => {
  const sources = projectSources()
  const sourcesByFile = new Map(sources.map((source) => [source.fileName, source]))
  const typeIndex = new GateTypeIndex(sources)

  it('keeps Quality form and table surfaces on shared UI primitives', () => {
    const violations: string[] = []
    const root = featureDirectory('quality-management')
    for (const source of sources) {
      if (isTestSource(source.fileName) || !isWithin(source.fileName, root)) continue
      walkNode(source.program, (node) => {
        if (is.jsxOpeningElement(node) || is.jsxSelfClosingElement(node)) {
          const tag = identifierName(childField(node, 'name')) ?? ''
          if (FORBIDDEN_QUALITY_NATIVE_CONTROLS.has(tag)) {
            violations.push(`${sourceLabel(source.fileName)}:${tag}`)
          }
        }
      })
    }
    expect(violations).toEqual([])
  })

  it('keeps transport, Query ownership, payload mapping, and cache control out of routes and components', () => {
    const violations: string[] = []
    for (const source of sources) {
      if (isTestSource(source.fileName) || TRANSPORT_ALLOWLIST.has(sourceLabel(source.fileName))) continue
      const root = FEATURE_NAMES.map(featureDirectory).find((candidate) => guardedSurface(source.fileName, candidate))
      if (!root) continue
      const label = sourceLabel(source.fileName)
      walkNode(source.program, (node) => {
        if (is.importDeclaration(node)) {
          const { moduleSpecifier, importKind, defaultName, namespaceName, named } = importDeclarationView(node)
          if (importKind !== 'type') {
            if (defaultName === 'platformApi' || apiModuleSpecifier(moduleSpecifier)) {
              violations.push(`${label}:direct transport ${defaultName}`)
            }
            if (namespaceName &&
              (apiModuleSpecifier(moduleSpecifier) || moduleSpecifier === '@tanstack/react-query')) {
              violations.push(`${label}:direct transport/query namespace ${namespaceName}`)
            }
            for (const specifier of named) {
              if (specifier.isTypeOnly) continue
              if (specifier.local === 'platformApi' || apiModuleSpecifier(moduleSpecifier)) {
                violations.push(`${label}:direct API adapter ${specifier.local}`)
              }
              if (moduleSpecifier === '@tanstack/react-query' && FORBIDDEN_QUERY_IMPORTS.has(specifier.imported)) {
                violations.push(`${label}:direct Query owner ${specifier.imported}`)
              }
            }
          }
        }
        if (is.callExpression(node)) {
          const name = callName(node)
          if (name && FORBIDDEN_QUERY_CACHE_CALLS.has(name)) {
            violations.push(`${label}:direct Query cache call ${name}`)
          }
          if (name && FORBIDDEN_TRANSPORT_CALLS.has(name)) {
            violations.push(`${label}:direct transport call ${name}`)
          }
          if (
            (source.fileName.endsWith('/Page.tsx') || source.fileName.includes('/routes/')) &&
            name &&
            /^(?:parse|normalize|map|resolve).*(?:capabilit|permission)/i.test(name)
          ) violations.push(`${label}:page-owned capability parsing ${name}`)
        }
        if (
          (source.fileName.endsWith('/Page.tsx') || source.fileName.includes('/routes/')) &&
          is.variableDeclaration(node) &&
          is.identifier(childField(node, 'id')) &&
          /(?:payload|requestBody|mutationBody)$/i.test(identifierName(childField(node, 'id')) ?? '') &&
          is.objectLiteral(childField(node, 'init'))
        ) violations.push(`${label}:page-owned payload mapping ${identifierName(childField(node, 'id'))}`)
      })
    }
    expect(violations).toEqual([])
  })

  it('forbids lifecycle boolean bags, whole screen models, and hook return types in component props', () => {
    const violations: string[] = []
    for (const source of sources) {
      if (isTestSource(source.fileName) || !FEATURE_NAMES.some((name) => (
        isWithin(source.fileName, join(featureDirectory(name), 'components'))
      ))) continue
      for (const component of componentFunctions(source)) {
        const label = sourceLabel(source.fileName)
        const parameterType = (() => {
          const annotation = childField(component.parameter, 'typeAnnotation')
          return annotation ? childField(annotation, 'typeAnnotation') : undefined
        })()
        if (parameterType && typeIndex.containsHookReturnType(parameterType, source.fileName)) {
          violations.push(`${label}:${component.name}:ReturnType<typeof use...>`)
        }
        for (const property of typeIndex.propertiesOf(parameterType, source.fileName)) {
          if (
            forbiddenLifecycleBoolean(property.name) &&
            typeIndex.booleanLike(property.typeNode, source.fileName)
          ) {
            if (!LIFECYCLE_BOOLEAN_ALLOWLIST.has(`${label}:${component.name}.${property.name}`)) {
              violations.push(`${label}:${component.name}.${property.name}`)
            }
          }
          for (const item of property.declarations) {
            if (typeIndex.containsHookReturnType(item, source.fileName)) {
              violations.push(`${label}:${component.name}.${property.name}:ReturnType<typeof use...>`)
            }
          }
          if (
            /^(?:model|pageModel|screenModel)$/i.test(property.name) &&
            typeIndex.referencesTypeMatching(
              property.typeNode,
              source.fileName,
              /(?:Page|Screen|Workspace|Feature)Model\b/,
            )
          ) violations.push(`${label}:${component.name}.${property.name}:whole page model`)
        }
      }
    }
    expect(violations).toEqual([])
  })

  it('keeps derived lifecycle and capability enumerations with model or hook owners', () => {
    const violations: string[] = []
    for (const source of sources) {
      if (isTestSource(source.fileName) || !FEATURE_NAMES.some((name) => (
        isWithin(source.fileName, join(featureDirectory(name), 'components'))
      ))) continue
      walkNode(source.program, (node) => {
        if (is.arrayLiteral(node)) {
          const keys = childArray(node, 'elements').flatMap((element) => (
            is.stringLiteralLike(element) &&
            DERIVED_STATUS_KEYS.has(String((element as unknown as { value: unknown }).value))
              ? [String((element as unknown as { value: unknown }).value)]
              : []
          ))
          if (new Set(keys).size >= 2) {
            violations.push(`${sourceLabel(source.fileName)}:component-owned status keys ${keys.join(',')}`)
          }
        }
      })
    }
    expect(violations).toEqual([])
  })

  it('forbids feature-owned Zustand stores', () => {
    const violations: string[] = []
    for (const name of FEATURE_NAMES) {
      for (const file of productionSourceFiles(featureDirectory(name))) {
        const source = sourcesByFile.get(file)
        if (!source) continue
        for (const statement of childArray(source.program, 'body')) {
          if (
            is.importDeclaration(statement) &&
            (() => {
              const specifier = String((statement as unknown as { source: unknown }).source ?? '').replace(/^['"]|['"]$/g, '')
              return specifier === 'zustand' || specifier.startsWith('zustand/')
            })()
          ) violations.push(`${sourceLabel(file)}:feature-owned Zustand dependency`)
        }
      }
    }
    expect(violations).toEqual([])
  })

  it('keeps native business traffic on the PM API contract', () => {
    const violations: string[] = []
    for (const file of productionSourceFiles(sourceRoot)) {
      const source = sourcesByFile.get(file)
      if (!source) continue
      const label = sourceLabel(file)
      walkNode(source.program, (node) => {
        if (is.identifier(node) && FORBIDDEN_HOST_BUSINESS_SYMBOLS.has(identifierName(node) ?? '')) {
          violations.push(`${label}:legacy host business transport ${identifierName(node)}`)
        }
        if (is.stringLiteralLike(node) && String((node as unknown as { value: unknown }).value).includes('/global-work-items')) {
          violations.push(`${label}:legacy PM global work-items endpoint`)
        }
      })
    }
    for (const name of FEATURE_NAMES) {
      for (const file of productionSourceFiles(featureDirectory(name))) {
        const source = sourcesByFile.get(file)
        if (!source) continue
        const label = sourceLabel(file)
        if (name === 'work-items') {
          for (const pattern of FORBIDDEN_WORK_ITEMS_SOURCE_PATTERNS) {
            if (pattern.test(source.content)) {
              violations.push(`${label}:unsupported templated task endpoint ${pattern.source}`)
            }
          }
        }
        walkNode(source.program, (node) => {
          if (name === 'work-items' && is.stringLiteralLike(node)) {
            const text = String((node as unknown as { value: unknown }).value)
            const endpoint = FORBIDDEN_WORK_ITEMS_ENDPOINTS.find((item) => text.includes(item))
            if (endpoint) violations.push(`${label}:unsupported endpoint ${endpoint}`)
            if (text === 'one2all' && !label.endsWith('work-items/model/utils.ts')) {
              violations.push(`${label}:legacy one2all authority outside route compatibility parser`)
            }
          }
        })
      }
    }
    expect(violations).toEqual([])
  })

  it('requires consumers outside a feature to use its public index', () => {
    const violations: string[] = []
    for (const source of sources) {
      if (!isWithin(source.fileName, sourceRoot) || isTestSource(source.fileName)) continue
      for (const statement of childArray(source.program, 'body')) {
        if (!is.importDeclaration(statement)) continue
        const specifier = String((statement as unknown as { source: unknown }).source ?? '').replace(/^['"]|['"]$/g, '')
        for (const name of FEATURE_NAMES) {
          const root = featureDirectory(name)
          if (isWithin(source.fileName, root)) continue
          if (specifier.startsWith(`@/features/${name}/`)) {
            violations.push(`${sourceLabel(source.fileName)}:deep import ${specifier}`)
          }
          if (specifier.startsWith('.')) {
            const target = resolve(dirname(source.fileName), specifier)
            if (isWithin(target, root) && target !== root) {
              violations.push(`${sourceLabel(source.fileName)}:deep relative import ${specifier}`)
            }
          }
        }
      }
    }
    expect(violations).toEqual([])
  })

  it('allows only explicitly audited third-party or complex residual CSS selectors', () => {
    const violations: string[] = []
    for (const name of FEATURE_NAMES) {
      const root = featureDirectory(name)
      const visit = (directory: string) => {
        for (const entry of readdirSync(directory, { withFileTypes: true })) {
          const path = join(directory, entry.name)
          if (entry.isDirectory()) visit(path)
          else if (entry.name.endsWith('.css')) {
            const cssPath = relative(featureRoot, path)
            const allowlist = CSS_SELECTOR_ALLOWLIST[cssPath] ?? {}
            for (const selector of cssSelectors(readFileSync(path, 'utf8'))) {
              if (!allowlist[selector]) violations.push(`${cssPath}:${selector}`)
            }
          }
        }
      }
      visit(root)
    }
    expect(violations).toEqual([])
  })

  it('keeps TSX inline styles to CSS custom properties for runtime values', () => {
    const violations: string[] = []
    for (const name of FEATURE_NAMES) {
      const root = featureDirectory(name)
      for (const file of productionSourceFiles(root).filter((item) => item.endsWith('.tsx'))) {
        const source = sourcesByFile.get(file)
        if (!source) continue
        const featurePath = relative(featureRoot, file)
        walkNode(source.program, (node) => {
          if (is.jsxAttribute(node) && identifierName(childField(node, 'name')) === 'style') {
            const container = childField(node, 'value')
            const expression = container && is.jsxExpressionContainer(container)
              ? childField(container, 'expression')
              : undefined
            if (!expression || !is.objectLiteral(expression)) {
              violations.push(`${featurePath}:non-literal inline style`)
            } else {
              for (const property of childArray(expression, 'properties')) {
                const rawName = (() => {
                  const key = childField(property, 'key')
                  if (!key) return ''
                  if (key.type === 'Identifier' || key.type === 'JSXIdentifier') {
                    return (key as unknown as { name: string }).name
                  }
                  if (key.type === 'Literal') return String((key as unknown as { value: unknown }).value)
                  return ''
                })()
                // 运行时几何值必须走 CSS 自定义属性（style={{ '--x': v }} + 工具类消费），
                // 具体样式一律用 Tailwind 类（FRONTEND_ARCHITECTURE.md §12.8）
                if (!rawName.startsWith('--')) {
                  violations.push(`${featurePath}:inline style ${rawName || 'spread/computed'}`)
                }
              }
            }
          }
        })
      }
    }
    expect(violations).toEqual([])
  })

  it('keeps DayPilot behind the Gantt-only lazy boundary', () => {
    const workItemsRoot = featureDirectory('work-items')
    const ganttEntry = join(workItemsRoot, 'components/collection/Gantt.tsx')
    const scheduler = join(workItemsRoot, 'components/collection/GanttScheduler.tsx')
    const imports: string[] = []

    for (const file of productionSourceFiles(workItemsRoot)) {
      if (readFileSync(file, 'utf8').includes('@daypilot/daypilot-lite-react')) {
        imports.push(relative(workItemsRoot, file))
      }
    }

    expect(readFileSync(ganttEntry, 'utf8')).toContain("lazy(() => import('./GanttScheduler').then((m) => ({ default: m.GanttScheduler }))")
    expect(imports).toEqual(['components/collection/GanttScheduler.tsx'])
    expect(readFileSync(scheduler, 'utf8')).toContain('eventMoveHandling="Update"')
    expect(readFileSync(scheduler, 'utf8')).toContain('eventResizeHandling="Update"')
    expect(readFileSync(scheduler, 'utf8')).toContain('args.preventDefault()')
    expect(readFileSync(scheduler, 'utf8')).toContain('void onUpdate(row.item, patch)')
    expect(readFileSync(scheduler, 'utf8')).toContain('timeRangeSelectedHandling="Disabled"')
  })
})

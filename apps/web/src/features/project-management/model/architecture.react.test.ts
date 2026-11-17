import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'

import { describe, expect, it } from 'bun:test'

import {
  childArray,
  childField,
  expressionName,
  GateTypeIndex,
  identifierName,
  is,
  lineAndCharacterAt,
  parseGate,
  type GateNode,
  type GateSource,
  walkNode,
} from '@/shared/testing/oxc-gates'

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
  'useMutation',
  'useQueryClient',
])

const FORBIDDEN_QUERY_CACHE_CALLS = new Set([
  'invalidateQueries',
  'removeQueries',
])

function capitalized(name: string): boolean {
  return /^[A-Z]/.test(name)
}

function isTestSource(fileName: string): boolean {
  return /(?:\.react-test|\.test|\.spec)\.[cm]?[jt]sx?$/.test(fileName)
}

function hookReturnTypeReference(node: GateNode): boolean {
  if (
    is.typeReference(node) &&
    expressionName(childField(node, 'typeName')) === 'ReturnType' &&
    childArray(node, 'typeArguments').some((argument) => (
      is.typeQuery(argument) &&
      /^use[A-Z0-9_$][A-Za-z0-9_$]*$/.test(
        (expressionName(childField(argument, 'exprName')) ?? '').split('.').at(-1) ?? '',
      )
    ))
  ) {
    return true
  }
  return false
}

function callName(expression: GateNode): string | undefined {
  if (is.identifier(expression)) return identifierName(expression)
  if (is.propertyAccess(expression)) return identifierName(childField(expression, 'property'))
  return undefined
}

/** tsconfig include 的字面等价：src 下全部 ts/tsx/d.ts（含测试，测试由各用例自行过滤）。 */
function projectSources(): GateSource[] {
  const sourceRoot = join(process.cwd(), 'src')
  const files: string[] = []
  const visit = (directory: string) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name)
      if (entry.isDirectory()) visit(path)
      else if (/\.[cm]?[jt]sx?$/.test(entry.name)) files.push(path)
    }
  }
  visit(sourceRoot)
  return files.map((fileName) => ({
    fileName,
    content: readFileSync(fileName, 'utf8'),
    program: parseGate(fileName, readFileSync(fileName, 'utf8')),
  }))
}

function componentFunctions(source: GateSource): Array<{
  name: string
  parameter: GateNode
}> {
  const components: Array<{ name: string; parameter: GateNode }> = []
  const addComponent = (
    name: string,
    callable: GateNode,
  ) => {
    const parameter = childArray(callable, 'params')[0]
    if (parameter) components.push({ name, parameter })
  }
  const wrappedCallable = (node: GateNode): GateNode | undefined => {
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
    }
    return undefined
  }
  walkNode(source.program, (node, parent) => {
    if (
      is.functionDeclaration(node) &&
      capitalized(identifierName(childField(node, 'id')) ?? '')
    ) {
      addComponent(identifierName(childField(node, 'id')) ?? 'default', node)
    }
    if (
      parent?.type === 'ExportDefaultDeclaration' &&
      is.functionDeclaration(node) &&
      !childField(node, 'id')
    ) {
      addComponent('default', node)
    }
    if (
      is.variableDeclaration(node) &&
      is.identifier(childField(node, 'id')) &&
      capitalized(identifierName(childField(node, 'id')) ?? '') &&
      childField(node, 'init')
    ) {
      const initializer = childField(node, 'init')
      const callable = initializer ? wrappedCallable(initializer) : undefined
      if (callable) addComponent(identifierName(childField(node, 'id')) ?? '', callable)
    }
  })
  return components
}

function parameterTypeNode(parameter: GateNode): GateNode | undefined {
  const annotation = childField(parameter, 'typeAnnotation')
  return annotation ? childField(annotation, 'typeAnnotation') : undefined
}

describe('P7 project-management architecture gate', () => {
  const sources = projectSources()

  it('keeps legacy plan-unit route and copy terms behind the Router compatibility parser', () => {
    const sourceRoot = join(process.cwd(), 'src')
    const compatibilitySource = join(sourceRoot, 'app/router/search.ts')
      const compatibilitySources = new Set([
        compatibilitySource,
        join(sourceRoot, 'app/router/search-utils.ts'),
        join(sourceRoot, 'shared/router/search.ts'),
        join(sourceRoot, 'shared/router/search-utils.ts'),
        join(sourceRoot, 'shared/router/search-quality.ts'),
      ])
    const violations: string[] = []

    for (const source of sources) {
      if (!source.fileName.startsWith(sourceRoot) || isTestSource(source.fileName)) continue
      const sourceName = relative(process.cwd(), source.fileName)
      if (
        !compatibilitySources.has(source.fileName) &&
        source.content.includes("'plan_units'")
      ) {
        violations.push(`${sourceName}:legacy plan_units route literal`)
      }
      if (source.fileName.endsWith('.tsx') && source.content.includes('计划单元')) {
        violations.push(`${sourceName}:legacy user-visible plan-unit copy`)
      }
    }

    expect(violations).toEqual([])
  })

  it('forbids lifecycle boolean bags and hook return types in component props', () => {
    const typeIndex = new GateTypeIndex(sources)
    const componentRoot = join(
      process.cwd(),
      'src/features/project-management/components',
    )
    const violations: string[] = []

    for (const source of sources) {
      if (
        !source.fileName.startsWith(componentRoot) ||
        !source.fileName.endsWith('.tsx') ||
        isTestSource(source.fileName)
      ) {
        continue
      }
      for (const component of componentFunctions(source)) {
        const sourceName = relative(process.cwd(), source.fileName)
        const parameterType = parameterTypeNode(component.parameter)
        if (parameterType && typeIndex.containsHookReturnType(parameterType, source.fileName)) {
          violations.push(`${sourceName}:${component.name}:ReturnType<typeof use...>`)
        }
        for (const property of typeIndex.propertiesOf(parameterType, source.fileName)) {
          if (forbiddenLifecycleBoolean(property.name)) {
            if (typeIndex.booleanLike(property.typeNode, source.fileName)) {
              violations.push(`${sourceName}:${component.name}.${property.name}`)
            }
          }
          for (const declaration of property.declarations) {
            if (typeIndex.containsHookReturnType(declaration, source.fileName)) {
              violations.push(
                `${sourceName}:${component.name}.${property.name}:ReturnType<typeof use...>`,
              )
            }
          }
        }
      }
    }

    expect(violations).toEqual([])
  })

  it('keeps transport, mutation, and query-cache ownership out of components and routes', () => {
    const guardedRoots = [
      join(process.cwd(), 'src/features/project-management/components'),
      join(process.cwd(), 'src/features/project-management/routes'),
    ]
    const violations: string[] = []

    for (const source of sources) {
      if (
        !guardedRoots.some((root) => source.fileName.startsWith(root)) ||
        isTestSource(source.fileName)
      ) {
        continue
      }
      const sourceName = relative(process.cwd(), source.fileName)
      walkNode(source.program, (node) => {
        if (is.importDeclaration(node)) {
          const { moduleSpecifier, importKind, defaultName, namespaceName, named } = importView(node)
          if (importKind !== 'type') {
            if (defaultName === 'platformApi' || apiModuleSpecifier(moduleSpecifier)) {
              violations.push(`${sourceName}:direct import ${defaultName}`)
            }
            if (
              namespaceName &&
              (apiModuleSpecifier(moduleSpecifier) ||
                moduleSpecifier === '@tanstack/react-query')
            ) {
              violations.push(
                `${sourceName}:direct transport/query namespace ${namespaceName}`,
              )
            }
            for (const specifier of named) {
              if (specifier.isTypeOnly) continue
              if (specifier.local === 'platformApi') {
                violations.push(`${sourceName}:direct import platformApi`)
              }
              if (apiModuleSpecifier(moduleSpecifier)) {
                violations.push(`${sourceName}:direct API adapter ${specifier.local}`)
              }
              if (
                moduleSpecifier === '@tanstack/react-query' &&
                FORBIDDEN_QUERY_IMPORTS.has(specifier.imported)
              ) {
                violations.push(`${sourceName}:direct query hook ${specifier.imported}`)
              }
            }
          }
        }
        if (is.callExpression(node)) {
          const name = callName(node)
          if (name && FORBIDDEN_QUERY_CACHE_CALLS.has(name)) {
            violations.push(`${sourceName}:direct query cache call ${name}`)
          }
        }
      })
    }

    expect(violations).toEqual([])
  })

  it('forbids feature-owned Zustand stores in model and hooks', () => {
    const guardedRoots = [
      join(process.cwd(), 'src/features/project-management/model'),
      join(process.cwd(), 'src/features/project-management/hooks'),
    ]
    const violations: string[] = []

    for (const source of sources) {
      if (
        !guardedRoots.some((root) => source.fileName.startsWith(root)) ||
        isTestSource(source.fileName)
      ) {
        continue
      }
      const sourceName = relative(process.cwd(), source.fileName)
      for (const statement of childArray(source.program, 'body')) {
        if (
          is.importDeclaration(statement) &&
          (() => {
            const specifier = String((statement as unknown as { source: unknown }).source ?? '').replace(/^['"]|['"]$/g, '')
            return specifier === 'zustand' || specifier.startsWith('zustand/')
          })()
        ) {
          violations.push(`${sourceName}:feature-owned Zustand dependency`)
        }
      }
    }

    expect(violations).toEqual([])
  })

  it('keeps people selection and preview on shared UI components', () => {
    const componentRoot = join(
      process.cwd(),
      'src/features/project-management/components',
    )
    const sharedPeopleComponents = new Set([
      'MultiUserSelect',
      'UserDirectoryMultiSelect',
      'UserDirectorySelect',
      'UserFaceStack',
      'UserPreview',
      'UserSelect',
    ])
    const legacyPrivatePeopleComponents = [
      'OwnerCombobox',
      'DeveloperMultiSelect',
      'UserMultiPicker',
    ]
    const violations: string[] = []

    for (const source of sources) {
      if (
        !source.fileName.startsWith(componentRoot) ||
        !source.fileName.endsWith('.tsx') ||
        isTestSource(source.fileName)
      ) continue
      const sourceName = relative(process.cwd(), source.fileName)
      for (const legacyName of legacyPrivatePeopleComponents) {
        if (source.content.includes(legacyName)) {
          violations.push(`${sourceName}:private people component ${legacyName}`)
        }
      }
      walkNode(source.program, (node) => {
        if (!is.importDeclaration(node)) return
        const { moduleSpecifier, named } = importView(node)
        for (const specifier of named) {
          if (
            sharedPeopleComponents.has(specifier.imported) &&
            moduleSpecifier !== '@/shared/ui'
          ) {
            violations.push(`${sourceName}:${specifier.imported} imported from ${moduleSpecifier}`)
          }
        }
      })
    }

    expect(violations).toEqual([])
  })

  it('routes project and requirement controls through shared UI primitives', () => {
    const guardedRoots = [
      join(process.cwd(), 'src/features/project-management'),
      join(process.cwd(), 'src/features/ai-pm'),
    ]
    const nativeControls = new Set([
      'button',
      'dialog',
      'input',
      'select',
      'textarea',
    ])
    const nativeControlAllowlist = new Set([
      'src/features/ai-pm/components/RequirementPoolTable/DeliverableComponents.tsx:107:native <input>',
    ])
    const violations: string[] = []

    for (const source of sources) {
      if (
        !guardedRoots.some((root) => source.fileName.startsWith(root)) ||
        !source.fileName.endsWith('.tsx') ||
        isTestSource(source.fileName)
      ) continue
      const sourceName = relative(process.cwd(), source.fileName)
      walkNode(source.program, (node) => {
        if (
          (is.jsxOpeningElement(node) || is.jsxSelfClosingElement(node)) &&
          /^[a-z]+$/.test(identifierName(childField(node, 'name')) ?? '') &&
          nativeControls.has(identifierName(childField(node, 'name')) ?? '')
        ) {
          const { line } = lineAndCharacterAt(source.content, node.start)
          const violationKey = `${sourceName}:${line}:native <${identifierName(childField(node, 'name'))}>`
          if (!nativeControlAllowlist.has(violationKey)) {
            violations.push(violationKey)
          }
        }
      })
    }

    expect(violations).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// 内部：ESLint 形态插件的临时视图（importDeclarationView 定义于文件尾部前向引用）
// ---------------------------------------------------------------------------

function importView(node: GateNode): {
  moduleSpecifier: string
  importKind: 'value' | 'type'
  defaultName: string | undefined
  namespaceName: string | undefined
  named: Array<{ imported: string; local: string; isTypeOnly: boolean }>
} {
  const record = node as Record<string, unknown>
  const moduleSpecifier = String(record.source ?? '').replace(/^['"]|['"]$/g, '')
  const importKind = record.importKind === 'type' ? 'type' : 'value'
  const specifiers = Array.isArray(record.specifiers) ? (record.specifiers as GateNode[]) : []
  let defaultName: string | undefined
  let namespaceName: string | undefined
  const named: Array<{ imported: string; local: string; isTypeOnly: boolean }> = []
  for (const specifier of specifiers) {
    if (specifier.type === 'ImportDefaultSpecifier') {
      const local = childField(specifier, 'local')
      defaultName = local ? identifierName(local) : undefined
    } else if (specifier.type === 'ImportNamespaceSpecifier') {
      const local = childField(specifier, 'local')
      namespaceName = local ? identifierName(local) : undefined
    } else if (specifier.type === 'ImportNamedSpecifier') {
      const imported = childField(specifier, 'imported')
      const local = childField(specifier, 'local')
      const importedName = (imported ? identifierName(imported) : undefined) ?? (local ? identifierName(local) : undefined) ?? ''
      named.push({
        imported: importedName,
        local: local ? identifierName(local) ?? '' : '',
        isTypeOnly: (specifier as Record<string, unknown>).importKind === 'type',
      })
    }
  }
  return { moduleSpecifier, importKind, defaultName, namespaceName, named }
}

function apiModuleSpecifier(value: string): boolean {
  return /(?:^|\/)api(?:\/|$)/.test(value)
}

function forbiddenLifecycleBoolean(name: string): boolean {
  const normalized = name.toLowerCase()
  return FORBIDDEN_BOOLEAN_PROPS.has(normalized) ||
    /^is.*(?:loading|error|refreshing|saving|moving|deleting|pending)$/.test(normalized)
}

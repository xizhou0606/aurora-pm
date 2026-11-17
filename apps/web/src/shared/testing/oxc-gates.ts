import { parseSync } from 'oxc-parser'

/**
 * 架构门禁的 AST 基础设施：基于 oxc-parser（Rust 原生解析，零 typescript 包依赖）。
 *
 * 背景：TS7 的 npm 包不再提供经典编译 API（createSourceFile / TypeChecker 等），
 * 而架构门禁测试只依赖「解析 → 遍历 → 类型守卫」与「类型别名/接口的字面追溯」，
 * 全部可由 oxc AST 表达。本模块是门禁测试与解析器之间唯一的边界。
 */

export type GateNode = {
  type: string
  start: number
  end: number
  [key: string]: unknown
}

export type GateSource = {
  fileName: string
  content: string
  program: GateNode
}

export function parseGate(fileName: string, content: string): GateNode {
  const { program, errors } = parseSync(fileName, content)
  if (errors.length > 0) throw new Error(`parse failed for ${fileName}: ${errors[0]?.message ?? 'unknown'}`)
  return program as unknown as GateNode
}

/** 深度优先遍历整棵 AST；parent 用于「表达式是否直接位于 JSX 子级」类判定。 */
export function walkNode(
  node: unknown,
  visit: (node: GateNode, parent: GateNode | null) => void,
  parent: GateNode | null = null,
): void {
  if (Array.isArray(node)) {
    node.forEach((item) => walkNode(item, visit, parent))
    return
  }
  if (!node || typeof node !== 'object') return
  const candidate = node as Record<string, unknown>
  if (typeof candidate.type !== 'string') return
  const gateNode = candidate as GateNode
  visit(gateNode, parent)
  for (const [key, value] of Object.entries(gateNode)) {
    if (key === 'type' || key === 'start' || key === 'end') continue
    walkNode(value, visit, gateNode)
  }
}

export type GateSourceLike = {
  fileName: string
  content: string
  program: GateNode
}

export function walkSources(sources: readonly GateSourceLike[], visit: (node: GateNode, source: GateSourceLike) => void): void {
  for (const source of sources) walkNode(source.program, (node) => visit(node, source))
}

// ---------------------------------------------------------------------------
// 类型守卫：oxc 节点 type 与 TS 经典守卫的对应关系
// ---------------------------------------------------------------------------

const GUARDS = {
  importDeclaration: 'ImportDeclaration',
  stringLiteral: 'Literal',
  stringLiteralLike: 'Literal',
  identifier: 'Identifier',
  callExpression: 'CallExpression',
  variableDeclaration: 'VariableDeclaration',
  exportDefault: 'ExportDefaultDeclaration',
  functionDeclaration: 'FunctionDeclaration',
  arrowFunction: 'ArrowFunctionExpression',
  functionExpression: 'FunctionExpression',
  jsxAttribute: 'JSXAttribute',
  jsxExpressionContainer: 'JSXExpressionContainer',
  jsxOpeningElement: 'JSXOpeningElement',
  jsxSelfClosingElement: 'JSXSelfClosingElement',
  jsxElement: 'JSXElement',
  jsxFragment: 'JSXFragment',
  objectLiteral: 'ObjectExpression',
  typeReference: 'TSTypeReference',
  typeQuery: 'TSTypeQuery',
  typeAlias: 'TSTypeAliasDeclaration',
  interfaceDeclaration: 'TSInterfaceDeclaration',
  propertyAccess: 'MemberExpression',
  namedImports: 'ImportNamedSpecifier',
  namespaceImport: 'ImportNamespaceSpecifier',
  arrayLiteral: 'ArrayExpression',
  expressionStatement: 'ExpressionStatement',
  jsxText: 'JSXText',
  objectProperty: 'Property',
  templateLiteral: 'TemplateLiteral',
  noSubstitutionTemplate: 'Literal',
  binaryExpression: 'BinaryExpression',
} as const

export type GuardName = keyof typeof GUARDS

function guard(guardName: GuardName, node: GateNode | null | undefined): boolean {
  if (!node || typeof node !== 'object') return false
  const type = (node as GateNode).type
  if (type !== GUARDS[guardName]) return false
  // Literal 需要按字面量种类细分（TS 的 isStringLiteral / isNoSubstitutionTemplateLiteral）
  if (type === 'Literal') {
    const value = (node as GateNode & { value?: unknown }).value
    if (guardName === 'stringLiteral') return typeof value === 'string'
    if (guardName === 'noSubstitutionTemplate') return value === undefined
  }
  return true
}

export const is = {
  importDeclaration: (node: GateNode | null | undefined) => guard('importDeclaration', node),
  stringLiteral: (node: GateNode | null | undefined) => guard('stringLiteral', node),
  stringLiteralLike: (node: GateNode | null | undefined) => guard('stringLiteralLike', node),
  identifier: (node: GateNode | null | undefined) => guard('identifier', node),
  callExpression: (node: GateNode | null | undefined) => guard('callExpression', node),
  variableDeclaration: (node: GateNode | null | undefined) => guard('variableDeclaration', node),
  exportDefault: (node: GateNode | null | undefined) => guard('exportDefault', node),
  functionDeclaration: (node: GateNode | null | undefined) => guard('functionDeclaration', node),
  arrowFunction: (node: GateNode | null | undefined) => guard('arrowFunction', node),
  functionExpression: (node: GateNode | null | undefined) => guard('functionExpression', node),
  jsxAttribute: (node: GateNode | null | undefined) => guard('jsxAttribute', node),
  jsxExpressionContainer: (node: GateNode | null | undefined) => guard('jsxExpressionContainer', node),
  jsxOpeningElement: (node: GateNode | null | undefined) => guard('jsxOpeningElement', node),
  jsxSelfClosingElement: (node: GateNode | null | undefined) => guard('jsxSelfClosingElement', node),
  jsxElement: (node: GateNode | null | undefined) => guard('jsxElement', node),
  jsxFragment: (node: GateNode | null | undefined) => guard('jsxFragment', node),
  objectLiteral: (node: GateNode | null | undefined) => guard('objectLiteral', node),
  typeReference: (node: GateNode | null | undefined) => guard('typeReference', node),
  typeQuery: (node: GateNode | null | undefined) => guard('typeQuery', node),
  typeAlias: (node: GateNode | null | undefined) => guard('typeAlias', node),
  interfaceDeclaration: (node: GateNode | null | undefined) => guard('interfaceDeclaration', node),
  propertyAccess: (node: GateNode | null | undefined) => guard('propertyAccess', node),
  namedImportSpecifier: (node: GateNode | null | undefined) => guard('namedImports', node),
  namespaceImportSpecifier: (node: GateNode | null | undefined) => guard('namespaceImport', node),
  arrayLiteral: (node: GateNode | null | undefined) => guard('arrayLiteral', node),
  templateLiteral: (node: GateNode | null | undefined) => guard('templateLiteral', node),
  noSubstitutionTemplate: (node: GateNode | null | undefined) => guard('noSubstitutionTemplate', node),
  binaryExpression: (node: GateNode | null | undefined) => guard('binaryExpression', node),
  expressionStatement: (node: GateNode | null | undefined) => guard('expressionStatement', node),
  jsxText: (node: GateNode | null | undefined) => guard('jsxText', node),
  objectProperty: (node: GateNode | null | undefined) => guard('objectProperty', node),
}

export function isIdentifierNamed(node: GateNode | null | undefined, name: string): boolean {
  return is.identifier(node) && (node as unknown as { name: string }).name === name
}

/** 标识符 / 属性访问链的可读名（a.b.c → 'a.b.c'）。 */
export function expressionName(node: GateNode | null | undefined): string | undefined {
  if (!node) return undefined
  if (is.identifier(node)) return (node as unknown as { name: string }).name
  if (is.propertyAccess(node)) {
    const self = node as unknown as { object: GateNode; property: GateNode }
    const object = expressionName(self.object)
    const property = expressionName(self.property)
    return object && property ? `${object}.${property}` : undefined
  }
  return undefined
}

/** 节点在源码里的文本（按 start/end 切片），供 getText 语义替代。 */
export function nodeText(node: GateNode, content: string): string {
  if (typeof node.start !== 'number' || typeof node.end !== 'number') return ''
  return content.slice(node.start, node.end)
}

/** ImportDeclaration 的结构化视图（default/namespace/named + importKind）。 */
export function importDeclarationView(node: GateNode): {
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
      const local = (specifier as Record<string, unknown>).local as GateNode
      defaultName = (local as unknown as { name: string }).name
    } else if (specifier.type === 'ImportNamespaceSpecifier') {
      const local = (specifier as Record<string, unknown>).local as GateNode
      namespaceName = (local as unknown as { name: string }).name
    } else if (specifier.type === 'ImportNamedSpecifier') {
      const recordSpecifier = specifier as Record<string, unknown>
      const importedNode = recordSpecifier.imported as GateNode | undefined
      const localNode = recordSpecifier.local as GateNode
      const imported = importedNode && is.identifier(importedNode)
        ? (importedNode as unknown as { name: string }).name
        : (localNode as unknown as { name: string }).name
      named.push({
        imported,
        local: (localNode as unknown as { name: string }).name,
        isTypeOnly: recordSpecifier.importKind === 'type',
      })
    }
  }
  return { moduleSpecifier, importKind, defaultName, namespaceName, named }
}

/** ObjectExpression 的属性列表（key 名 + value 节点）。 */
export function objectProperties(node: GateNode): Array<{ name: string; value: GateNode }> {
  const properties = (node as Record<string, unknown>).properties
  if (!Array.isArray(properties)) return []
  const result: Array<{ name: string; value: GateNode }> = []
  for (const property of properties as GateNode[]) {
    const name = propertyName(property)
    const value = (property as Record<string, unknown>).value as GateNode | undefined
    if (name && value) result.push({ name, value })
  }
  return result
}

/** CallExpression 的实参列表。 */
export function callArguments(node: GateNode): GateNode[] {
  const arguments_ = (node as Record<string, unknown>).arguments
  return Array.isArray(arguments_) ? (arguments_ as GateNode[]) : []
}

/** 调用表达式的可读名（useMutation / api.request）。 */
export function callCallee(node: GateNode): string | undefined {
  return expressionName((node as Record<string, unknown>).callee as GateNode)
}

/** JSXAttribute 的属性名（a / aria-label / data-x）。 */
export function jsxAttributeName(node: GateNode): string {
  const name = (node as Record<string, unknown>).name as GateNode | undefined
  if (!name) return ''
  if (name.type === 'JSXIdentifier') return (name as unknown as { name: string }).name
  if (name.type === 'JSXNamespacedName') {
    const namespaced = name as unknown as { namespace: { name: string }; name: { name: string } }
    return `${namespaced.namespace.name}:${namespaced.name.name}`
  }
  return ''
}

/** 偏移 → 行号（1 起）。 */
export function lineAt(content: string, offset: number): number {
  return lineAndCharacterAt(content, offset).line
}

/** 偏移 → 行号 / 列号（均 1 起），等价 TS 的 getLineAndCharacterOfPosition。 */
export function lineAndCharacterAt(content: string, offset: number): { line: number; character: number } {
  const bounded = Math.min(Math.max(offset, 0), content.length)
  let line = 1
  let lineStart = 0
  for (let index = 0; index < bounded; index += 1) {
    if (content.charCodeAt(index) === 10) {
      line += 1
      lineStart = index + 1
    }
  }
  return { line, character: bounded - lineStart + 1 }
}

/** 子节点访问（无则 undefined）。 */
export function childField(node: GateNode, key: string): GateNode | undefined {
  const value = (node as Record<string, unknown>)[key]
  return value && typeof value === 'object' && !Array.isArray(value) ? value as GateNode : undefined
}

/** 子节点数组访问（无则空数组）。 */
export function childArray(node: GateNode, key: string): GateNode[] {
  const value = (node as Record<string, unknown>)[key]
  return Array.isArray(value) ? value as GateNode[] : []
}

/** Identifier / JSXIdentifier 节点的名称。 */
export function identifierName(node: GateNode | null | undefined): string | undefined {
  if (!node) return undefined
  const type = node.type
  if (type === 'Identifier' || type === 'JSXIdentifier') {
    return (node as unknown as { name: string }).name
  }
  return undefined
}

// ---------------------------------------------------------------------------
// 类型索引：checker 的字面替代（类型别名 / 接口追溯、props 枚举、booleanLike）
// ---------------------------------------------------------------------------

type TypeDeclaration = {
  fileName: string
  node: GateNode
  members: GateNode[]
}

type ImportBinding = { moduleSpecifier: string; exportedName: string }

const PRIMITIVE_TYPES = new Set([
  'TSBooleanKeyword', 'TSUndefinedKeyword', 'TSNullKeyword',
  'TSStringKeyword', 'TSNumberKeyword', 'TSAnyKeyword', 'TSUnknownKeyword',
  'TSVoidKeyword', 'TSNeverKeyword', 'TSSymbolKeyword', 'TSBigIntKeyword',
  'TSObjectType', 'TSThisType', 'TSLiteralType',
])

function propertyName(property: GateNode): string | undefined {
  const key = (property as Record<string, unknown>).key
  if (!key || typeof key !== 'object') return undefined
  const keyNode = key as GateNode
  if (keyNode.type === 'Identifier') return keyNode.name as string
  if (keyNode.type === 'Literal') return String((keyNode as unknown as { value: unknown }).value)
  return undefined
}

function typeAnnotationOf(member: GateNode): GateNode | undefined {
  const annotation = (member as Record<string, unknown>).typeAnnotation
  if (!annotation || typeof annotation !== 'object') return undefined
  const annotated = annotation as GateNode
  return annotated.type === 'TSTypeAnnotation'
    ? (annotated.typeAnnotation as GateNode)
    : annotated
}

function declaredMembers(node: GateNode): GateNode[] {
  if (node.type === 'TSTypeLiteral' || node.type === 'TSInterfaceDeclaration') {
    const body = (node as Record<string, unknown>).body as GateNode | undefined
    const members = (body as Record<string, unknown> | undefined)?.body
    return Array.isArray(members) ? (members as GateNode[]) : []
  }
  return []
}

/** 全量源码上的类型声明索引：checker 符号解析的字面替代。 */
export class GateTypeIndex {
  private readonly declarations = new Map<string, TypeDeclaration[]>()
  private readonly exportsByFile = new Map<string, Map<string, string>>()
  private readonly importsByFile = new Map<string, Map<string, ImportBinding>>()

  constructor(sources: readonly GateSourceLike[]) {
    for (const source of sources) this.indexFile(source)
  }

  private indexFile(source: GateSourceLike): void {
    const fileName = source.fileName
    if (!this.importsByFile.has(fileName)) this.importsByFile.set(fileName, new Map())
    if (!this.exportsByFile.has(fileName)) this.exportsByFile.set(fileName, new Map())
    const imports = this.importsByFile.get(fileName)
    const exports = this.exportsByFile.get(fileName)

    walkNode(source.program, (node, parent) => {
      if (is.importDeclaration(node)) {
        const specifier = String((node as Record<string, unknown>).source ?? '')
          .replace(/^['"]|['"]$/g, '')
        const clause = (node as Record<string, unknown>).importClause as GateNode | undefined
        if (!clause) return
        const defaultName = (clause as Record<string, unknown>).name as GateNode | undefined
        if (defaultName && is.identifier(defaultName)) {
          imports?.set((defaultName as unknown as { name: string }).name, { moduleSpecifier: specifier, exportedName: 'default' })
        }
        const named = (clause as Record<string, unknown>).namedBindings
        if (named && typeof named === 'object') {
          const specifiers = (named as Record<string, unknown>).specifiers
          if (Array.isArray(specifiers)) {
            for (const item of specifiers as GateNode[]) {
              const local = (item as Record<string, unknown>).local as GateNode | undefined
              const imported = (item as Record<string, unknown>).imported as GateNode | undefined
              if (!local || !is.identifier(local)) continue
              const exportedName = imported && is.identifier(imported)
                ? (imported as unknown as { name: string }).name
                : (local as unknown as { name: string }).name
              imports?.set((local as unknown as { name: string }).name, { moduleSpecifier: specifier, exportedName })
            }
          }
        }
        return
      }
      if ((is.typeAlias(node) || is.interfaceDeclaration(node)) &&
        (parent?.type === 'Program' || parent?.type === 'ExportNamedDeclaration')) {
        const id = (node as Record<string, unknown>).id as GateNode | undefined
        if (!id || !is.identifier(id)) return
        const name = (id as unknown as { name: string }).name
        const list = this.declarations.get(name) ?? []
        list.push({ fileName, node, members: declaredMembers(node) })
        this.declarations.set(name, list)
        exports?.set(name, name)
      }
    })
  }

  private declarationFor(name: string, fileName: string, visitedFiles: Set<string>): TypeDeclaration | undefined {
    const own = (this.declarations.get(name) ?? []).find((item) => item.fileName === fileName)
    if (own) return own
    if (visitedFiles.has(fileName)) return undefined
    visitedFiles.add(fileName)
    const binding = this.importsByFile.get(fileName)?.get(name)
    if (!binding) return undefined
    const targetFile = this.resolveModule(fileName, binding.moduleSpecifier)
    if (!targetFile) return undefined
    const exported = this.exportsByFile.get(targetFile)?.get(binding.exportedName)
    if (exported && exported !== name) return this.declarationFor(exported, targetFile, visitedFiles)
    return (this.declarations.get(binding.exportedName) ?? []).find((item) => item.fileName === targetFile)
      ?? (this.declarations.get(binding.exportedName) ?? []).at(-1)
  }

  private resolveModule(fileName: string, moduleSpecifier: string): string | undefined {
    if (!moduleSpecifier.startsWith('.')) return undefined
    const base = fileName.slice(0, fileName.lastIndexOf('/'))
    const parts = `${base}/${moduleSpecifier}`.split('/')
    const stack: string[] = []
    for (const part of parts) {
      if (part === '.' || part === '') continue
      if (part === '..') stack.pop()
      else stack.push(part)
    }
    const withoutExtension = stack.join('/')
    for (const candidate of [withoutExtension, `${withoutExtension}.ts`, `${withoutExtension}.tsx`, `${withoutExtension}/index.ts`]) {
      if (this.importsByFile.has(candidate)) return candidate
    }
    return undefined
  }

  private resolvedTypeNodes(
    typeNode: GateNode,
    fileName: string,
    visited: Set<string>,
  ): GateNode[] {
    const key = `${fileName}:${typeNode.start}:${typeNode.type}`
    if (visited.has(key)) return []
    visited.add(key)
    if (typeNode.type === 'TSParenthesizedType') {
      const inner = (typeNode as Record<string, unknown>).typeAnnotation as GateNode | undefined
      return inner ? this.resolvedTypeNodes(inner, fileName, visited) : []
    }
    if (typeNode.type === 'TSTypeReference') {
      const name = expressionName((typeNode as Record<string, unknown>).typeName as GateNode)
      if (!name) return []
      const declaration = this.declarationFor(name, fileName, new Set())
      if (declaration) return this.resolvedTypeNodes(declaration.node, declaration.fileName, visited)
      return [typeNode]
    }
    if (typeNode.type === 'TSTypeAliasDeclaration') {
      const annotation = (typeNode as Record<string, unknown>).typeAnnotation as GateNode | undefined
      return annotation ? this.resolvedTypeNodes(annotation, fileName, visited) : []
    }
    if (typeNode.type === 'TSUnionType' || typeNode.type === 'TSIntersectionType') {
      const members = childArray(typeNode, 'types').flatMap((member) => this.resolvedTypeNodes(member, fileName, visited))
      return members
    }
    return [typeNode]
  }

  /** 参数类型节点 → 可枚举的属性（名 + 类型节点 + 属性声明节点）。 */
  propertiesOf(typeNode: GateNode | undefined, fileName: string): Array<{
    name: string
    typeNode: GateNode | undefined
    declarations: GateNode[]
  }> {
    if (!typeNode) return []
    const properties: Array<{
      name: string
      typeNode: GateNode | undefined
      declarations: GateNode[]
    }> = []
    const seen = new Set<string>()
    const visited = new Set<string>()
    const visitType = (node: GateNode) => {
      for (const resolved of this.resolvedTypeNodes(node, fileName, visited)) {
        for (const member of declaredMembers(resolved)) {
          const name = propertyName(member)
          if (!name) continue
          const key = `${resolved.type}:${member.start}:${name}`
          if (seen.has(key)) continue
          seen.add(key)
          properties.push({ name, typeNode: typeAnnotationOf(member), declarations: [member] })
        }
      }
    }
    visitType(typeNode)
    return properties
  }

  /** 属性 / 参数类型是否为布尔或 boolean 联合（含 undefined/null 成员）。 */
  booleanLike(typeNode: GateNode | undefined, fileName: string): boolean {
    if (!typeNode) return false
    const resolved = this.resolvedTypeNodes(typeNode, fileName, new Set([`p:${typeNode.start}`]))
    if (resolved.length === 0) return false
    const primitives = resolved.filter((node) => PRIMITIVE_TYPES.has(node.type))
    if (primitives.length !== resolved.length) return false
    const meaningful = primitives.filter((node) => node.type !== 'TSUndefinedKeyword' && node.type !== 'TSNullKeyword')
    if (meaningful.length === 0) return false
    return meaningful.every((node) => node.type === 'TSBooleanKeyword')
  }

  /** 解析后的类型树里是否出现匹配正则的类型引用（FeatureModel 等）。 */
  referencesTypeMatching(typeNode: GateNode | undefined, fileName: string, pattern: RegExp): boolean {
    if (!typeNode) return false
    const found: string[] = []
    const rootVisited = this.resolvedTypeNodes(typeNode, fileName, new Set())
    const visitResolved = (node: GateNode) => {
      if (node.type === 'TSTypeReference') {
        const name = expressionName((node as Record<string, unknown>).typeName as GateNode)
        if (name && pattern.test(name)) found.push(name)
      }
      for (const [key, value] of Object.entries(node)) {
        if (key === 'type' || key === 'start' || key === 'end') continue
        if (Array.isArray(value)) value.forEach((item) => {
          if (item && typeof item === 'object' && typeof (item as GateNode).type === 'string') visitResolved(item as GateNode)
        })
        else if (value && typeof value === 'object' && typeof (value as GateNode).type === 'string') visitResolved(value as GateNode)
      }
    }
    for (const resolved of rootVisited) visitResolved(resolved)
    return found.length > 0
  }

  /** 类型节点（含别名/接口追溯）里是否引用 ReturnType<typeof use...>。 */
  containsHookReturnType(typeNode: GateNode, fileName: string): boolean {
    let found = false
    const visited = new Set<string>()
    const visit = (node: GateNode) => {
      if (found) return
      const visitKey = `${fileName}:${node.start}:${node.type}`
      if (visited.has(visitKey)) return
      visited.add(visitKey)
      if (node.type === 'TSTypeReference') {
        const name = expressionName((node as Record<string, unknown>).typeName as GateNode)
        if (name === 'ReturnType') {
          const parameters = (node as Record<string, unknown>).typeArguments as
            { typeArguments?: GateNode[]; params?: GateNode[] } | undefined
          const arguments_ = parameters?.typeArguments ?? parameters?.params ?? []
          const hookQuery = arguments_.some((argument) => {
            const query = argument as GateNode
            if (query.type !== 'TSTypeQuery') return false
            const expression = expressionName((query as Record<string, unknown>).exprName as GateNode) ?? ''
            return /^use[A-Z0-9_$][A-Za-z0-9_$]*$/.test(expression.split('.').at(-1) ?? '')
          })
          if (hookQuery) {
            found = true
            return
          }
        }
      }
      for (const [key, value] of Object.entries(node)) {
        if (key === 'type' || key === 'start' || key === 'end') continue
        if (Array.isArray(value)) value.forEach((item) => {
          if (item && typeof item === 'object' && typeof (item as GateNode).type === 'string') visit(item as GateNode)
        })
        else if (value && typeof value === 'object' && typeof (value as GateNode).type === 'string') visit(value as GateNode)
      }
    }
    visit(typeNode)
    if (found) return true
    // 追溯类型别名 / 接口：属性声明的类型可能经由别名引用 ReturnType<typeof use...>
    const resolved = this.resolvedTypeNodes(typeNode, fileName, visited)
    for (const resolvedNode of resolved) {
      if (resolvedNode.type !== 'TSTypeAliasDeclaration' && resolvedNode.type !== 'TSInterfaceDeclaration') continue
      if (visitWithIndex(this, resolvedNode, fileName, visited)) return true
    }
    return false
  }
}

function visitWithIndex(
  index: GateTypeIndex,
  node: GateNode,
  fileName: string,
  visited: Set<string>,
): boolean {
  if (index.containsHookReturnType(node, fileName)) return true
  for (const [key_, value] of Object.entries(node)) {
    if (key_ === 'type' || key_ === 'start' || key_ === 'end') continue
    const candidates = Array.isArray(value) ? value : [value]
    for (const item of candidates) {
      if (item && typeof item === 'object' && typeof (item as GateNode).type === 'string') {
        if (visitWithIndex(index, item as GateNode, fileName, visited)) return true
      }
    }
  }
  return false
}

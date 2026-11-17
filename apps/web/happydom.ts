import { JSDOM } from "jsdom"
import { mock } from 'bun:test'
import { GlobalRegistrator } from "@happy-dom/global-registrator"

// 1) 全局测试 DOM：happy-dom（官方 global-registrator）。
//    曾整体试过 jsdom 全局（与迁移前 vitest 等价），但 NewReviewWorkbench
//    等重组件在 --parallel worker 中 OOM（SIGKILL）并卡死协调器，故保留
//    happy-dom 作全局环境。
GlobalRegistrator.register({ url: "http://localhost:3000" })

// happy-dom omits draggable reflection and aliases DragEvent to Event. Native
// dragging needs both the attribute and MouseEvent coordinates; keep this
// compatibility in the test environment, not in application drag handlers.
if (!('draggable' in HTMLElement.prototype)) {
  Object.defineProperty(HTMLElement.prototype, 'draggable', {
    configurable: true,
    get(this: HTMLElement) {
      const value = this.getAttribute('draggable')?.toLowerCase()
      if (value === 'true') return true
      if (value === 'false') return false
      return this.localName === 'img' || (this.localName === 'a' && this.hasAttribute('href'))
    },
    set(this: HTMLElement, value: boolean) {
      this.setAttribute('draggable', String(Boolean(value)))
    },
  })
}

if (!(new DragEvent('drag') instanceof MouseEvent)) {
  class TestDragEvent extends MouseEvent {
    readonly dataTransfer: DataTransfer | null

    constructor(type: string, init: DragEventInit = {}) {
      super(type, init)
      this.dataTransfer = init.dataTransfer ?? null
    }
  }
  for (const target of [globalThis, window]) {
    Object.defineProperty(target, 'DragEvent', { configurable: true, writable: true, value: TestDragEvent })
  }
}

// jsdom（及浏览器）暴露 window.origin；happy-dom 20 没有，会让
// `event.origin !== window.origin` 之类守卫恒真拒绝。从 location 派生。
if ((globalThis as unknown as { origin?: string }).origin === undefined) {
  Object.defineProperty(globalThis, 'origin', {
    configurable: true,
    get: () => (globalThis as unknown as { location: { origin: string } }).location.origin,
  })
}

// 2) happy-dom 的 getElementsByTagNameNS("*", name) 会跳过无命名空间元素
//    （浏览器会返回），transfer 模块的 XML 解析依赖该行为。沿实例原型链恢复平价。
type NSLookup = (namespace: string, name: string) => NodeList
type NSHolder = { getElementsByTagNameNS?: NSLookup }

function patchWildcardNSLookup(instance: object): void {
  let current: object | null = instance
  while (current) {
    const holder = current as NSHolder
    const original = Object.getOwnPropertyDescriptor(holder, 'getElementsByTagNameNS')?.value
    if (typeof original === 'function' && !original.__browserParity) {
      const patched = function (this: Element, namespace: string, name: string): NodeList {
        const matches = original.call(this, namespace, name)
        if (namespace === '*' && matches.length === 0) return this.getElementsByTagName(name)
        return matches
      }
      ;(patched as NSLookup & { __browserParity?: boolean }).__browserParity = true
      Object.defineProperty(holder, 'getElementsByTagNameNS', { configurable: true, writable: true, value: patched })
    }
    current = Object.getPrototypeOf(current)
  }
}

const parityProbeDocument = new DOMParser().parseFromString('<probe />', 'application/xml')
patchWildcardNSLookup(parityProbeDocument)
if (parityProbeDocument.documentElement) patchWildcardNSLookup(parityProbeDocument.documentElement)

// 3) happy-dom 的 DataTransfer.setDragImage 直接 throw（浏览器为标准 API，no-op 即可），
//    拖拽预览依赖该调用不抛错。
const dataTransferProto = (globalThis as unknown as { DataTransfer?: { prototype: { setDragImage?: (...args: unknown[]) => void } } }).DataTransfer?.prototype
if (dataTransferProto && typeof dataTransferProto.setDragImage === 'function') {
  dataTransferProto.setDragImage = function setDragImage(): void {}
}

// 4) DOMPurify 专属：DOMPurify 的官方支持 DOM 是 jsdom/linkedom 类完整实现，
//    happy-dom 下它会剥离全部元素节点。DOMPurify 官方用法是
//    `createDOMPurify(window)`——这里仅在其模块初始化期间把全局 window/document
//    换成 jsdom 实例，让默认实例绑定 jsdom，随后立即恢复 happy-dom 测试环境。
//    DOMPurify 只产出字符串（不回注节点），两套 DOM 不会交叉。
//    用同步 require：preload 必须在任何测试前同步完成全部补丁安装。
const happyWindow = globalThis.window
const happyDocument = globalThis.document
const purifierDom = new JSDOM('<!doctype html><html><head></head><body></body></html>', {
  url: 'http://localhost:3000',
})
;(globalThis as unknown as { window: unknown }).window = purifierDom.window
;(globalThis as unknown as { document: unknown }).document = purifierDom.window.document
const DOMPurify = require('dompurify')
;(globalThis as unknown as { window: unknown }).window = happyWindow
;(globalThis as unknown as { document: unknown }).document = happyDocument
// 双模块上下文：require 与 ESM import 拿到的实例不同。把 jsdom 绑定的实例
// 通过官方 mock.module 注册为 dompurify 模块，测试内 import 才能拿到正确实例。
mock.module('dompurify', () => ({ ...DOMPurify, default: DOMPurify }))

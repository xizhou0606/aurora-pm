import { afterAll, beforeAll } from 'bun:test'
import { JSDOM } from 'jsdom'

// Happy DOM does not apply CSSStyleRule.style mutations to getComputedStyle.
// CSSOM-sensitive suites use jsdom locally instead of emulating a CSS engine.
// This environment is for imperative editor/CSSOM tests, not React suites whose
// Testing Library screen and event constructors already bind to the preload DOM.
export function useJsdomEnvironment() {
  const names = [
    'window', 'document', 'navigator', 'Node', 'Text', 'Element', 'HTMLElement',
    'Document', 'DocumentFragment', 'DOMParser', 'DOMRect', 'Range',
    'Event', 'MouseEvent', 'KeyboardEvent', 'MutationObserver', 'getComputedStyle',
  ] as const
  const previous = new Map<string, PropertyDescriptor | undefined>()
  let dom: JSDOM | undefined

  beforeAll(() => {
    dom = new JSDOM('<!doctype html><html><head></head><body></body></html>', {
      url: 'http://localhost:3000',
      pretendToBeVisual: true,
    })
    for (const name of names) {
      previous.set(name, Object.getOwnPropertyDescriptor(globalThis, name))
      const value = name === 'getComputedStyle'
        ? dom.window.getComputedStyle.bind(dom.window)
        : dom.window[name]
      Object.defineProperty(globalThis, name, { configurable: true, writable: true, value })
    }
  })

  afterAll(() => {
    try {
      dom?.window.close()
    } finally {
      for (const [name, descriptor] of previous) {
        if (descriptor) Object.defineProperty(globalThis, name, descriptor)
        else Reflect.deleteProperty(globalThis, name)
      }
      previous.clear()
    }
  })
}

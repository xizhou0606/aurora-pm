// 允许 style 对象承载 CSS 自定义属性（shadcn/no-inline-styles 的动态值模式：
// style={{ '--x': v }} + className="w-(--x)"）。React 类型默认不收录 `--` 键，
// 按 React 官方方式对 CSSProperties 做模块增强。
import 'react'

declare module 'react' {
  interface CSSProperties {
    [key: `--${string}`]: string | number | undefined
  }
}

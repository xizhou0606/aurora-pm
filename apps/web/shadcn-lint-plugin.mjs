// oxlint 的 jsPlugins 要求模块本身即 ESLint 插件对象；
// @shadcn/lint 的插件挂在具名导出 plugin 下，这里做一层薄壳转发。
export { plugin as default } from '@shadcn/lint'

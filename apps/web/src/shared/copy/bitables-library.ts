export const BITABLE_LIBRARY_COPY = {
  permissions: '权限管理',
  more: (name: string) => `${name}的更多操作`, open: '在新标签页打开', share: '分享', copyLink: '复制链接',
  duplicateName: (name: string) => `${name} 副本`, favoritesFolder: '收藏',
  duplicate: '创建副本', shortcut: '创建快捷方式', pin: '置顶', unpin: '取消置顶', favorite: '收藏', unfavorite: '取消收藏',
  owner: '转移所有者', move: '移动', rename: '重命名', remove: '删除',
  name: '名称', destination: '目标文件夹', root: '根目录', ownerLabel: '新所有者',
  save: '确认', cancel: '取消', retry: '重试确认',
  copied: '链接已复制', copyFailed: '无法自动复制，请重试。',
  deleteTitle: '删除多维表格', deleteDescription: (name: string) => `删除“${name}”后将无法继续打开该表格及其记录。请确认操作。`,
  deleteShortcutTitle: '删除快捷方式', deleteShortcutDescription: '仅删除此快捷方式，不删除原表格。',
  error: '操作失败，请刷新后重试。', unknown: '提交结果尚未确认。请重试确认原操作，不要重复提交。',
  foldersUnavailable: '当前无法读取文档文件夹，仅可选择根目录。',
} as const

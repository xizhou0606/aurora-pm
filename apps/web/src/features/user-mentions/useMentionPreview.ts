import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import { searchPlatformUsers } from '@/shared/api'

export function useMentionPreview(workspaceId: number) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const clear = () => { clearTimeout(timer) }
    const over = (event: Event) => {
      if (!(event.target instanceof Element)) return
      if (event.target.closest('[data-user-mention-card]')) { clear(); return }
      const mention = event.target.closest<HTMLElement>('[data-comment-mention],.wt-cmt-at')
      if (mention) { clear(); setAnchor(mention) }
    }
    const out = (event: Event) => {
      const next = (event as MouseEvent).relatedTarget
      if (next instanceof Element && next.closest('[data-comment-mention],.wt-cmt-at,[data-user-mention-card]')) return
      clear(); timer = setTimeout(() => setAnchor(null), 180)
    }
    const close = () => { clear(); setAnchor(null) }
    const key = (event: KeyboardEvent) => { if (event.key === 'Escape') close() }
    document.addEventListener('pointerover',over); document.addEventListener('pointerout',out)
    document.addEventListener('focusin',over); document.addEventListener('focusout',out)
    document.addEventListener('keydown',key); document.addEventListener('scroll',close,true)
    return () => {
      clear(); document.removeEventListener('pointerover',over); document.removeEventListener('pointerout',out)
      document.removeEventListener('focusin',over); document.removeEventListener('focusout',out)
      document.removeEventListener('keydown',key); document.removeEventListener('scroll',close,true)
    }
  }, [])
  const id = Number(anchor?.dataset.userId || anchor?.getAttribute('id'))
  const username = anchor?.dataset.userUsername || anchor?.getAttribute('username') || ''
  const validId = Number.isSafeInteger(id) && id > 0
  const query = useQuery({
    queryKey: pmKeys.resource(workspaceId > 0 ? workspaceId : 1, 'workspace', 'user-mention', validId ? id : username),
    enabled: !!anchor && workspaceId > 0 && (validId || !!username), staleTime: 120000, retry:false,
    queryFn: async ({signal}) => {
      const page = await searchPlatformUsers(validId ? {ids:[id]} : {username,exact:true},signal)
      const matches = page.items.filter(user=>validId ? user.id === id : user.username.toLowerCase() === username.toLowerCase())
      return matches.length === 1 ? matches[0] : null
    },
  })
  const [loadingVisible, setLoadingVisible] = useState(false)
  useEffect(() => {
    setLoadingVisible(false)
    if (!query.isFetching) return
    const timer = setTimeout(() => setLoadingVisible(true), 200)
    return () => clearTimeout(timer)
  }, [query.isFetching, anchor])
  return {loadingVisible,anchor, close:()=>setAnchor(null), user:query.data, loading:query.isFetching, label:anchor?.textContent?.replace(/^@/,'') || ''}
}

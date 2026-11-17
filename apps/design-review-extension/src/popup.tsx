import { Camera, CheckCircle2, ExternalLink, LoaderCircle, ScanSearch, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import type { CaptureTask } from './types'
import './popup.css'

type PopupTask = CaptureTask & { on_target_tab: boolean }

function Popup() {
  const [task, setTask] = useState<PopupTask | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    void chrome.runtime.sendMessage({ type: 'PM_DESIGN_REVIEW_GET_STATUS' }).then((response: { ok: boolean; task?: PopupTask; error?: string }) => {
      if (!response.ok) setError(response.error ?? '无法读取采集状态')
      setTask(response.task ?? null)
      setLoading(false)
    })
  }, [])

  const run = async (type: 'PM_DESIGN_REVIEW_COMPLETE_CAPTURE' | 'PM_DESIGN_REVIEW_FOCUS_TARGET' | 'PM_DESIGN_REVIEW_FOCUS_WORKBENCH') => {
    setLoading(true)
    setError('')
    const response = await chrome.runtime.sendMessage({ type }) as { ok: boolean; error?: string }
    if (!response.ok) {
      setError(response.error ?? '采集失败')
      setLoading(false)
    } else window.close()
  }

  return <main>
    <header><span className="logo"><ScanSearch size={18} /></span><div><strong>PM 设计走查</strong><small>页面证据采集器 v{chrome.runtime.getManifest().version}</small></div></header>
    {loading ? <section className="status"><LoaderCircle className="spin" size={16} /><div><b>正在读取页面状态</b><p>请稍候。</p></div></section> : task ? <>
      <section className="status active"><Camera size={16} /><div><b>{task.state === 'awaiting_normalization' ? '正在生成归一化方案' : task.on_target_tab ? '目标页面已就绪' : '采集任务等待中'}</b><p>{task.state === 'awaiting_normalization' ? '请返回 PM 工作台等待处理，目标页会在完成或失败后自动恢复。' : task.on_target_tab ? `确认页面状态后，将以 ${task.viewport.width} x ${task.viewport.height} 采集。` : '目标页面已在另一个标签页中打开。'}</p></div></section>
      {error ? <p className="error"><TriangleAlert size={14} />{error}</p> : null}
      <button className="primary" onClick={() => void run(task.state === 'awaiting_normalization' ? 'PM_DESIGN_REVIEW_FOCUS_WORKBENCH' : task.on_target_tab ? 'PM_DESIGN_REVIEW_COMPLETE_CAPTURE' : 'PM_DESIGN_REVIEW_FOCUS_TARGET')}>
        {task.state === 'awaiting_normalization' ? <><ExternalLink size={14} />返回工作台</> : task.on_target_tab ? <><Camera size={14} />采集并返回工作台</> : <><ExternalLink size={14} />前往目标页面</>}
      </button>
      <p className="hint">采集期间会临时冻结动效和调整视口，完成或失败后自动恢复。登录及页面交互仍由你手动完成。</p>
    </> : <section className="status success"><CheckCircle2 size={16} /><div><b>扩展已启用</b><p>请在 PM 设计管理工作台中发起网页采集。</p></div></section>}
    <footer>不读取 Cookie、浏览器存储、输入历史或网络请求</footer>
  </main>
}

createRoot(document.getElementById('root')!).render(<Popup />)

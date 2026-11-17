import { useEffect, useId, useMemo, useRef, useState } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { readThemePaint, useAppearance } from '../theme/appearance'
import { blockAt, clampDuration, offsetOf, type Block } from './ai-loading/bot/cycles'
import { NOTIF_BLUE } from './ai-loading/bot/decor'
import { BotEngine, type BotFrame } from './ai-loading/bot/engine'
import { DEMI_VIEWBOX, RAYON } from './ai-loading/bot/repere'
import { mixHex } from './ai-loading/bot/skins'
import { POSES } from './ai-loading/bot/states'

/**
 * 1:1 复刻上游 README 第一支 demo（docs/demo.gif）：idle（黑球+白眼）→
 * wink（眨眼）→ orbit（彩环绕球）→ burst（迸散重组）。块时长按 demo.gif
 * 逐帧量出（状态边界 1.5 / 3.1 / 6.1s，全长 9.7s ≈ 243 帧 @25fps），不是
 * 各状态的默认持长；GIF 由同一引擎导出，逐帧一致。
 */
const LOADING_CYCLE: Block[] = (
  [
    ['idle', 1.5],
    ['wink', 1.6],
    ['orbit', 3.0],
    ['burst', 3.6],
  ] as const
).map(([state, duration]) => ({ state, duration: clampDuration(state, duration) }))

const HEX = /^#[0-9a-f]{6}$/i

/** 引擎的 mixHex 只吃 #rrggbb；主题 token 将来换成其他色彩空间时退回安全默认色。 */
function hexPaint(value: string, fallback: string): string {
  return HEX.test(value) ? value : fallback
}

type AiLoadingProps = {
  /** 渲染边长，px。彩环最远到 1.4 倍体半径，小尺寸下主要读得出体的形变。 */
  size?: number
  className?: string
  /** 无障碍名称；传 null 表示纯装饰（旁边已有等价状态文字时）。 */
  label?: string | null
  /**
   * 冻结在蒙太奇的这一秒渲染单帧，不跑时钟（与上游同名 API 同义）：测试、
   * 截图对比和「海报帧」都用它；动画实例不要传。
   */
  frozenAt?: number
}

/**
 * AI 等待动画：1:1 复刻上游 README 的 demo——黑色形体在实测姿态间变形
 * （idle 黑球 → wink 眨眼 → orbit 彩环绕球 → burst 迸散），眼睛独立游移。
 * 引擎见 `./ai-loading-bot/`（移植自 jeremy-prt/bloub，纯函数于时间），这里
 * 只负责驱动时钟与渲染 SVG。
 *
 * 颜色跟随亮暗主题：墨色取 `--foreground`，眼洞透出的底色取 `--card`；彩环
 * 是实测的固定色轮，两种主题下都成立。`prefers-reduced-motion` 下静止在
 * idle 姿态（demo 的海报帧），不再跑时钟。
 */
function AiLoading({ size = 24, className, label = copy.ui.aiThinking, frozenAt }: AiLoadingProps) {
  const { resolved } = useAppearance()
  // 主题解析态变化时才重读 token：readThemePaint 读 documentElement 的计算值
  const paint = useMemo(() => readThemePaint(), [resolved])
  const ink = hexPaint(paint.foreground, '#151820')
  const paper = hexPaint(paint.card, '#ffffff')
  const uid = useId().replace(/[^a-zA-Z0-9-]/g, '')
  const maskId = `${uid}-bot-mask`
  // 蒙太奇游标：引擎只在块边界被 setState，回卷（新一轮）走 reset
  const playedRef = useRef(-1)

  const [engine] = useState(() => new BotEngine(RAYON, 'idle'))
  const [frame, setFrame] = useState<BotFrame>(() => engine.sample(POSES.idle))

  const [reduced, setReduced] = useState(
    () => window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches ?? false,
  )

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (!media) return
    const sync = () => setReduced(media.matches)
    media.addEventListener('change', sync)
    return () => media.removeEventListener('change', sync)
  }, [])

  // 把蒙太奇走到时刻 t（跨块时按上游 rendAt 的规则日期化 setState）并采样。
  const frameAt = (t: number): BotFrame => {
    const { index } = blockAt(LOADING_CYCLE, t)
    if (index !== playedRef.current) {
      const block = LOADING_CYCLE[index]!
      const at = offsetOf(LOADING_CYCLE, index)
      // 回卷到新一轮时清掉前态重放：否则循环首帧会熔上一轮的末态
      if (index < playedRef.current) engine.reset(block.state, at)
      else engine.setState(block.state, at)
      playedRef.current = index
    }
    return engine.sample(t)
  }

  useEffect(() => {
    if (frozenAt !== undefined) {
      setFrame(frameAt(frozenAt))
      return
    }
    if (reduced) {
      // 静止帧取 idle 姿态（黑球+白眼），即 demo 的海报帧
      setFrame(frameAt(POSES.idle))
      return
    }
    // 场景时钟按帧差推进并设上限：标签页隐藏后回来不跳帧。
    // 绘制节流到 ~30fps：批量队列可能同屏 5-10 个实例，60fps 的 React 重渲染
    // 会吃满主线程，loader 不需要这个顺滑度。
    let raf = 0
    let last = 0
    let painted = 0
    let clock = 0
    const tick = (ms: number) => {
      raf = requestAnimationFrame(tick)
      clock += last ? Math.min((ms - last) / 1000, 0.064) : 0
      last = ms
      if (ms - painted < 32) return
      painted = ms
      setFrame(frameAt(clock))
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [engine, reduced, frozenAt])

  const dotFill = (dot: BotFrame['dots'][number]) =>
    dot.color ?? (dot.depth === undefined ? ink : mixHex(paper, ink, dot.depth))

  const dots = frame.dots.map((dot, i) =>
    dot.d ? (
      <path
        key={`d${i}`}
        d={dot.d}
        fill={dotFill(dot)}
        opacity={dot.opacity}
        transform={`translate(${dot.x} ${dot.y}) rotate(${dot.rot ?? 0}) scale(${RAYON})`}
      />
    ) : (
      <circle key={`d${i}`} cx={dot.x} cy={dot.y} r={dot.r} fill={dotFill(dot)} opacity={dot.opacity} />
    ),
  )

  return (
    <svg
      data-slot="ai-loading"
      width={size}
      height={size}
      viewBox={`${-DEMI_VIEWBOX} ${-DEMI_VIEWBOX} ${DEMI_VIEWBOX * 2} ${DEMI_VIEWBOX * 2}`}
      role={label ? 'img' : undefined}
      aria-label={label ?? undefined}
      aria-hidden={label ? undefined : true}
      className={cn('shrink-0', className)}
    >
      <defs>
        {/* 眼睛是体上的真孔：滑向边缘时被轮廓自动裁掉 */}
        <mask id={maskId} maskUnits="userSpaceOnUse" x={-DEMI_VIEWBOX} y={-DEMI_VIEWBOX} width={DEMI_VIEWBOX * 2} height={DEMI_VIEWBOX * 2}>
          <path d={frame.bodyPath} fill="#fff" />
          {frame.eyes.map((eye, i) => (
            <path key={i} d={eye.d} transform={eye.matrix} opacity={eye.alpha} fill="#000" />
          ))}
          {frame.notch ? <circle cx={frame.notch.x} cy={frame.notch.y} r={frame.notch.r} fill="#000" /> : null}
        </mask>
        {frame.arcs.map((arc) => (
          <linearGradient
            key={arc.id}
            id={`${uid}-${arc.id}`}
            gradientUnits="userSpaceOnUse"
            x1={arc.grad.x1}
            y1={arc.grad.y1}
            x2={arc.grad.x2}
            y2={arc.grad.y2}
          >
            {arc.grad.stops.map((stopColor, i) => (
              <stop key={i} offset={i / (arc.grad.stops.length - 1)} stopColor={stopColor} />
            ))}
          </linearGradient>
        ))}
      </defs>

      {/* 环带后半：先画，被体遮住才读得出环绕 */}
      <g fill="none" strokeLinecap="round">
        {frame.arcs.map((arc) => (
          <path
            key={`b${arc.id}`}
            d={arc.back}
            stroke={`url(#${uid}-${arc.id})`}
            strokeWidth={arc.width}
            opacity={arc.opacity}
          />
        ))}
      </g>

      {frame.dotsBehind ? dots : null}

      <g opacity={frame.bodyAlpha}>
        {/* 不透明底衬出体的精确轮廓：眼洞透出的是纸色而非环带或粒子 */}
        <path data-slot="ai-loading-paper" d={frame.bodyPath} fill={paper} />
        <g mask={`url(#${maskId})`}>
          <rect data-slot="ai-loading-ink" x={-DEMI_VIEWBOX} y={-DEMI_VIEWBOX} width={DEMI_VIEWBOX * 2} height={DEMI_VIEWBOX * 2} fill={ink} />
        </g>
      </g>

      {!frame.dotsBehind ? dots : null}

      {frame.notif ? <circle cx={frame.notif.x} cy={frame.notif.y} r={frame.notif.r} fill={NOTIF_BLUE} /> : null}

      <g fill="none" strokeLinecap="round">
        {frame.arcs.map((arc) => (
          <path
            key={`f${arc.id}`}
            d={arc.front}
            stroke={`url(#${uid}-${arc.id})`}
            strokeWidth={arc.width}
            opacity={arc.opacity}
          />
        ))}
      </g>
    </svg>
  )
}

type AiLoadingOverlayProps = {
  /** 居中展示的提示文字，同时是无障碍名称；传 null 只留动画 */
  label?: string | null
  /** 居中 blob 的边长，px */
  size?: number
  className?: string
}

/**
 * antd Spin 式的等待：`fixed` 盖满整个视口，永远相对 body 垂直水平居中，
 * 背后整页压暗模糊——容器多高、滚到哪里都不影响居中位置。行内按钮、状态行
 * 仍用 `<AiLoading>`。
 */
function AiLoadingOverlay({ label = copy.ui.aiThinking, size = 48, className }: AiLoadingOverlayProps) {
  return (
    <div
      data-slot="ai-loading-overlay"
      role="status"
      aria-busy="true"
      aria-label={label ?? undefined}
      className={cn('fixed inset-0 z-30 grid place-items-center bg-background/75 px-6', className)}
    >
      <div className="flex flex-col items-center gap-2.5 text-center">
        <AiLoading size={size} label={null} />
        {label ? <span className="max-w-sm text-xs font-medium leading-5 text-muted-foreground">{label}</span> : null}
      </div>
    </div>
  )
}

export { AiLoading, AiLoadingOverlay }
export type { AiLoadingProps, AiLoadingOverlayProps }

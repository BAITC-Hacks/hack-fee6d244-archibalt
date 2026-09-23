import { useEffect, useRef } from 'react'
import { cx } from './cx'
import { levelLabels, type LevelKind } from './Badge'

const SEGMENTS: { from: number; to: number; level: LevelKind }[] = [
  { from: 0, to: 40, level: 'draft' }, { from: 40, to: 70, level: 'working' }, { from: 70, to: 90, level: 'ready' }, { from: 90, to: 100, level: 'priority' },
]

/** Число, которое плавно доезжает до значения: пишет в textContent в requestAnimationFrame, без перерисовки React. */
export function AnimatedNumber({ value, duration = 700, className }: { value: number; duration?: number; className?: string }) {
  const node = useRef<HTMLSpanElement>(null)
  const shown = useRef(value)
  useEffect(() => {
    const el = node.current
    if (!el) return
    const start = shown.current
    if (start === value || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) { shown.current = value; el.textContent = String(value); return }
    let frame = 0; const began = performance.now()
    const tick = (now: number) => {
      const t = Math.min((now - began) / duration, 1)
      shown.current = Math.round(start + (value - start) * (1 - Math.pow(1 - t, 3)))
      el.textContent = String(shown.current)
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [value, duration])
  return <span ref={node} className={className}>{shown.current}</span>
}

/** Шкала 0–100 с сегментами уровней 40 / 70 / 90. */
export function Meter({ value, label = 'Готовность задачи', showValue = true }: { value: number; label?: string; showValue?: boolean }) {
  const score = Math.max(0, Math.min(100, Math.round(value)))
  const current = SEGMENTS.find(seg => score < seg.to) ?? SEGMENTS[SEGMENTS.length - 1]
  return <div className="ui-meter">
    {showValue && <div className="ui-meter-head"><AnimatedNumber className="ui-meter-value" value={score} /><span className="ui-meter-of">из 100</span></div>}
    <div className="ui-meter-track" role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={score} aria-valuetext={`${score} из 100, ${levelLabels[current.level]}`}>
      {SEGMENTS.map(seg => {
        const fill = Math.max(0, Math.min(1, (score - seg.from) / (seg.to - seg.from)))
        return <span key={seg.level} className="ui-meter-seg" style={{ flex: seg.to - seg.from }}><span className="ui-meter-fill" style={{ width: `${fill * 100}%` }} /></span>
      })}
    </div>
    <div className="ui-meter-scale" aria-hidden="true">
      {SEGMENTS.map(seg => <span key={seg.level} style={{ flex: seg.to - seg.from }} className={cx(seg === current && 'is-current')} title={levelLabels[seg.level]}>{seg.from}</span>)}
    </div>
  </div>
}

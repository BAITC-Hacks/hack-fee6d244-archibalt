import { useEffect, useRef } from 'react'

/**
 * Живой фон блока: три мягких пятна в цветах kit плывут по keyframes (transform/opacity),
 * слой смещается от прокрутки (параллакс 0.15). При reduced-motion — статичный градиент.
 */
export function AnimatedBackground({ intensity = 'full' }: { intensity?: 'full' | 'soft' }) {
  const node = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = node.current
    if (!el || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
    let frame = 0
    const update = () => {
      frame = 0
      const rect = el.parentElement?.getBoundingClientRect()
      if (!rect || rect.bottom < 0 || rect.top > window.innerHeight) return
      el.style.setProperty('--parallax', `${Math.round(-rect.top * 0.15)}px`)
    }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(update) }
    update()
    window.addEventListener('scroll', schedule, { passive: true })
    return () => { window.removeEventListener('scroll', schedule); cancelAnimationFrame(frame) }
  }, [])
  return <div ref={node} className={`anim-bg anim-bg-${intensity}`} aria-hidden="true">
    <span className="blob blob-a" /><span className="blob blob-b" /><span className="blob blob-c" />
  </div>
}

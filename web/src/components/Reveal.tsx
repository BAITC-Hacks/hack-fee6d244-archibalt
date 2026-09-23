import { useEffect, useRef, useState, type ReactNode } from 'react'

/** Секция плавно проявляется, когда входит в экран (один раз). Без IntersectionObserver и при reduced-motion — сразу видна. */
export function Reveal({ children, className = '', as: Tag = 'section', id }: { children: ReactNode; className?: string; as?: 'section' | 'div'; id?: string }) {
  const node = useRef<HTMLElement>(null)
  const [shown, setShown] = useState(() => typeof IntersectionObserver === 'undefined' || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
  useEffect(() => {
    if (shown || !node.current) return
    const observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setShown(true); observer.disconnect() } }, { rootMargin: '0px 0px -8% 0px' })
    observer.observe(node.current)
    return () => observer.disconnect()
  }, [shown])
  return <Tag ref={node as never} id={id} className={`reveal${shown ? ' is-shown' : ''}${className ? ` ${className}` : ''}`}>{children}</Tag>
}

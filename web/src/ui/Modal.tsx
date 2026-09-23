import { useEffect, useId, useRef, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { cx } from './cx'
import { CloseIcon } from './icons'

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])'
let openCount = 0

interface ModalProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
  size?: 'md' | 'lg'
  initialFocus?: RefObject<HTMLElement>
}

/** Диалог в портале: фокус-ловушка, Esc и клик по подложке закрывают, фокус возвращается на место. */
export function Modal({ open, onClose, title, description, children, footer, size = 'md', initialFocus }: ModalProps) {
  const titleId = useId(); const descId = useId()
  const panel = useRef<HTMLDivElement>(null)
  const closeRef = useRef(onClose); closeRef.current = onClose
  const focusRef = useRef(initialFocus); focusRef.current = initialFocus

  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    openCount += 1; document.body.classList.add('ui-scroll-lock')
    const first = focusRef.current?.current || panel.current?.querySelector('.ui-modal-body')?.querySelector<HTMLElement>(FOCUSABLE) || panel.current
    first?.focus()
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') { event.preventDefault(); closeRef.current(); return }
      if (event.key !== 'Tab' || !panel.current) return
      const items = Array.from(panel.current.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(node => node.offsetParent !== null)
      if (!items.length) { event.preventDefault(); return }
      const head = items[0]; const tail = items[items.length - 1]
      if (event.shiftKey && (document.activeElement === head || document.activeElement === panel.current)) { event.preventDefault(); tail.focus() }
      else if (!event.shiftKey && document.activeElement === tail) { event.preventDefault(); head.focus() }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      openCount = Math.max(0, openCount - 1)
      if (!openCount && !document.querySelector('.agent-overlay')) document.body.classList.remove('ui-scroll-lock')
      previous?.focus?.()
    }
  }, [open])

  if (!open) return null
  return createPortal(
    <div className="ui-modal-overlay" onMouseDown={event => { if (event.target === event.currentTarget) onClose() }}>
      <div ref={panel} className={cx('ui-modal', size === 'lg' && 'ui-modal-lg')} role="dialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={description ? descId : undefined} tabIndex={-1}>
        <div className="ui-modal-head">
          <div><h2 className="ui-modal-title" id={titleId}>{title}</h2>{description && <p className="ui-modal-desc" id={descId}>{description}</p>}</div>
          <button type="button" className="ui-modal-close" aria-label="Закрыть" onClick={onClose}><CloseIcon /></button>
        </div>
        {children && <div className="ui-modal-body">{children}</div>}
        {footer && <div className="ui-modal-foot">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}

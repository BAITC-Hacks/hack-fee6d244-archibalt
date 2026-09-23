import { useEffect, useId, useRef, useState } from 'react'

interface MenuItem { label: string; onSelect: () => void }

/** Блок сессии в шапке: кружок с инициалом, подпись, меню по клику. */
export function SessionMenu({ kind, title, label, items }: { kind: string; title: string; label: string; items: MenuItem[] }) {
  const menuId = useId()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null); const trigger = useRef<HTMLButtonElement>(null); const first = useRef<HTMLButtonElement>(null)
  useEffect(() => { if (open) first.current?.focus() }, [open])
  useEffect(() => {
    if (!open) return
    const onDown = (event: MouseEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { setOpen(false); trigger.current?.focus(); return }
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      const list = Array.from(root.current?.querySelectorAll<HTMLButtonElement>('[role=menuitem]') ?? [])
      const index = list.indexOf(document.activeElement as HTMLButtonElement)
      event.preventDefault(); list[(index + (event.key === 'ArrowDown' ? 1 : list.length - 1)) % list.length]?.focus()
    }
    document.addEventListener('mousedown', onDown); document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open])
  const initial = (label.replace(/^(Демо-)?команда\s+/i, '').trim()[0] || '•').toUpperCase()
  return <div className="session" ref={root}>
    <button ref={trigger} type="button" className="session-trigger" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuId : undefined} aria-label={`${title}: ${label}`} onClick={() => setOpen(value => !value)}>
      <span className={`session-avatar session-${kind}`} aria-hidden="true">{initial}</span>
      <span className="session-name">{label}</span>
      <span className="ui-select-chevron" aria-hidden="true" />
    </button>
    {open && <div className="session-menu" id={menuId} role="menu" aria-label={title}>
      <p className="session-menu-head"><small>{title}</small><strong>{label}</strong></p>
      {items.map((item, index) => <button key={item.label} ref={index === 0 ? first : undefined} type="button" role="menuitem" className="session-menu-item" onClick={() => { setOpen(false); item.onSelect() }}>{item.label}</button>)}
    </div>}
  </div>
}

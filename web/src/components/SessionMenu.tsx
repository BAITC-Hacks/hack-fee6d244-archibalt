import { useEffect, useId, useRef, useState } from 'react'
import type { Team } from '../api'
import { useToast } from '../ui'

/** Блок сессии команды в шапке: инициал, название, меню с выходом. */
export function SessionMenu({ team, onLogout }: { team: Team; onLogout: () => Promise<void> }) {
  const toast = useToast()
  const menuId = useId()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null); const trigger = useRef<HTMLButtonElement>(null); const item = useRef<HTMLButtonElement>(null)
  useEffect(() => { if (open) item.current?.focus() }, [open])
  useEffect(() => {
    if (!open) return
    const onDown = (event: MouseEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') { setOpen(false); trigger.current?.focus() } }
    document.addEventListener('mousedown', onDown); document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open])
  const initial = (team.name.replace(/^(Демо-)?команда\s+/i, '').trim()[0] || team.name[0] || 'К').toUpperCase()
  return <div className="session" ref={root}>
    <button ref={trigger} type="button" className="session-trigger" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuId : undefined} onClick={() => setOpen(value => !value)}>
      <span className="session-avatar" aria-hidden="true">{initial}</span>
      <span className="session-name">{team.name}</span>
      <span className="ui-select-chevron" aria-hidden="true" />
    </button>
    {open && <div className="session-menu" id={menuId} role="menu" aria-label="Команда">
      <p className="session-menu-head"><small>Вы вошли как команда</small><strong>{team.name}</strong></p>
      <button ref={item} type="button" role="menuitem" className="session-menu-item" onClick={() => { setOpen(false); void onLogout().then(() => toast.show('Вы вышли из команды')) }}>Выйти</button>
    </div>}
  </div>
}

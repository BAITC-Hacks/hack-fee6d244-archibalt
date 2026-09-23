import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { cx } from './cx'

export interface SelectOption { value: string; label: string }
interface SelectProps {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  id?: string
  invalid?: boolean
  disabled?: boolean
  required?: boolean
  'aria-label'?: string
  'aria-describedby'?: string
}

/** Выпадающий список на кнопке: роли listbox/option, клавиши ↑ ↓ Home End Enter Esc. */
export function Select({ value, onChange, options, placeholder = 'Выберите', id, invalid, disabled, 'aria-label': ariaLabel, 'aria-describedby': describedBy }: SelectProps) {
  const auto = useId()
  const baseId = id || auto
  const listId = `${baseId}-list`
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const list = useRef<HTMLUListElement>(null)
  const selectedIndex = options.findIndex(option => option.value === value)
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined

  function show(index = selectedIndex >= 0 ? selectedIndex : 0) { if (disabled || !options.length) return; setActive(index); setOpen(true) }
  function hide(focusTrigger = true) { setOpen(false); if (focusTrigger) trigger.current?.focus() }
  function choose(index: number) { const option = options[index]; if (option) onChange(option.value); hide() }

  useEffect(() => { if (open) list.current?.focus() }, [open])
  useEffect(() => { if (open) list.current?.querySelector(`[data-index="${active}"]`)?.scrollIntoView({ block: 'nearest' }) }, [open, active])
  useEffect(() => {
    if (!open) return
    const onDown = (event: MouseEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  function onTriggerKey(event: KeyboardEvent) {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
      event.preventDefault()
      show(event.key === 'ArrowUp' ? Math.max(selectedIndex - 1, 0) : event.key === 'ArrowDown' && selectedIndex >= 0 ? Math.min(selectedIndex + 1, options.length - 1) : undefined)
    }
  }
  function onListKey(event: KeyboardEvent) {
    const last = options.length - 1
    switch (event.key) {
      case 'ArrowDown': event.preventDefault(); setActive(index => Math.min(index + 1, last)); break
      case 'ArrowUp': event.preventDefault(); setActive(index => Math.max(index - 1, 0)); break
      case 'Home': event.preventDefault(); setActive(0); break
      case 'End': event.preventDefault(); setActive(last); break
      case 'Enter': case ' ': event.preventDefault(); choose(active); break
      case 'Escape': event.preventDefault(); event.stopPropagation(); hide(); break
      case 'Tab': hide(false); break
      default: {
        if (event.key.length !== 1) return
        const letter = event.key.toLowerCase()
        const next = options.findIndex((option, index) => index > active && option.label.toLowerCase().startsWith(letter))
        const found = next >= 0 ? next : options.findIndex(option => option.label.toLowerCase().startsWith(letter))
        if (found >= 0) setActive(found)
      }
    }
  }

  return <div className="ui-select" ref={root}>
    <button ref={trigger} id={baseId} type="button" className="ui-select-trigger" disabled={disabled}
      aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? listId : undefined} aria-label={ariaLabel} aria-describedby={describedBy} aria-invalid={invalid || undefined}
      onClick={() => (open ? hide() : show())} onKeyDown={onTriggerKey}>
      <span className={cx('ui-select-value', !selected && 'is-placeholder')}>{selected ? selected.label : placeholder}</span>
      <span className="ui-select-chevron" aria-hidden="true" />
    </button>
    {open && <ul ref={list} id={listId} className="ui-select-list" role="listbox" tabIndex={-1} aria-labelledby={baseId} aria-activedescendant={`${baseId}-opt-${active}`} onKeyDown={onListKey}>
      {options.map((option, index) => <li key={option.value || '__empty'} id={`${baseId}-opt-${index}`} data-index={index} role="option" aria-selected={option.value === value}
        className={cx('ui-select-option', index === active && 'is-active')} onMouseEnter={() => setActive(index)} onMouseDown={event => event.preventDefault()} onClick={() => choose(index)}>{option.label}</li>)}
    </ul>}
  </div>
}

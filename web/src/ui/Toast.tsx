import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { CloseIcon } from './icons'

export type ToastTone = 'info' | 'success' | 'error'
interface ToastItem { id: number; message: string; tone: ToastTone }
interface ToastApi { show: (message: string, tone?: ToastTone) => void; success: (message: string) => void; error: (message: string) => void }

const ToastContext = createContext<ToastApi | null>(null)

/** Всплывающие уведомления вместо alert. Одна область aria-live на всё приложение. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const nextId = useRef(1)
  const dismiss = useCallback((id: number) => setItems(list => list.filter(item => item.id !== id)), [])
  const show = useCallback((message: string, tone: ToastTone = 'info') => {
    const id = nextId.current++
    setItems(list => [...list.slice(-2), { id, message, tone }])
    window.setTimeout(() => dismiss(id), tone === 'error' ? 8000 : 5000)
  }, [dismiss])
  const api = useMemo<ToastApi>(() => ({ show, success: message => show(message, 'success'), error: message => show(message, 'error') }), [show])
  return <ToastContext.Provider value={api}>
    {children}
    <ol className="ui-toasts" aria-live="polite" aria-relevant="additions">
      {items.map(item => <li key={item.id} className={`ui-toast ui-toast-${item.tone}`} role={item.tone === 'error' ? 'alert' : 'status'}>
        <p>{item.message}</p>
        <button type="button" aria-label="Скрыть уведомление" onClick={() => dismiss(item.id)}><CloseIcon /></button>
      </li>)}
    </ol>
  </ToastContext.Provider>
}

export function useToast(): ToastApi {
  const api = useContext(ToastContext)
  if (!api) throw new Error('useToast нужен внутри ToastProvider')
  return api
}

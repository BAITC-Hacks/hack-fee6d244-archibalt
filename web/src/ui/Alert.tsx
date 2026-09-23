import type { ReactNode } from 'react'
import { cx } from './cx'

/** Встроенное сообщение рядом с формой: ошибка, успех, подсказка. */
export function Alert({ tone = 'info', title, children, className }: { tone?: 'info' | 'success' | 'warning' | 'error'; title?: string; children?: ReactNode; className?: string }) {
  return <div className={cx('ui-alert', `ui-alert-${tone}`, className)} role={tone === 'error' ? 'alert' : 'status'}>
    {title && <strong>{title}</strong>}
    {children && <div>{children}</div>}
  </div>
}

import { cx } from './cx'

export function Spinner({ size = 'md', label }: { size?: 'sm' | 'md' | 'lg'; label?: string }) {
  return <span className={cx('ui-spinner', size !== 'md' && `ui-spinner-${size}`)} role={label ? 'status' : undefined} aria-label={label} aria-hidden={label ? undefined : true} />
}

export function Loading({ children = 'Загружаем данные…' }: { children?: string }) {
  return <div className="container ui-loading" role="status"><Spinner />{children}</div>
}

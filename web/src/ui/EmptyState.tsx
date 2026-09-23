import type { ReactNode } from 'react'
import { cx } from './cx'
import { InboxIcon } from './icons'

export function EmptyState({ title, children, action, slim }: { title: string; children?: ReactNode; action?: ReactNode; slim?: boolean }) {
  return <div className={cx('ui-empty', slim && 'ui-empty-slim')}>
    <span className="ui-empty-mark"><InboxIcon /></span>
    <h3>{title}</h3>
    {children && <p>{children}</p>}
    {action && <div className="ui-empty-action">{action}</div>}
  </div>
}

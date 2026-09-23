import type { ReactNode } from 'react'
import { cx } from './cx'

export type LevelKind = 'draft' | 'working' | 'ready' | 'priority'
export type ProposalKind = 'new' | 'selected' | 'accepted' | 'declined' | 'rejected' | 'on_hold'
export type BadgeKind = LevelKind | ProposalKind

export const levelLabels: Record<LevelKind, string> = { draft: 'требует уточнения', working: 'рабочая', ready: 'готовая', priority: 'приоритетная' }
export const proposalLabels: Record<ProposalKind, string> = { new: 'на рассмотрении', selected: 'выбрана', accepted: 'принята командой', declined: 'команда отказалась', rejected: 'отклонена', on_hold: 'отложена' }
export const badgeLabel = (kind: BadgeKind): string => (kind in levelLabels ? levelLabels[kind as LevelKind] : proposalLabels[kind as ProposalKind]) ?? kind

export function Badge({ kind, children, className }: { kind: BadgeKind; children?: ReactNode; className?: string }) {
  return <span className={cx('ui-badge', `ui-badge-${kind}`, className)}>{children ?? badgeLabel(kind)}</span>
}

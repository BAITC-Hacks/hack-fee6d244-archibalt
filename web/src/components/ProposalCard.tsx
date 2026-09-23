import { useState } from 'react'
import { api, json, withAuth, type Proposal, type Team } from '../api'
import { useSession } from '../session'
import { Alert, Badge, Button, ConfirmDialog, useToast } from '../ui'
import { ProposalChat } from './ProposalChat'

type Action = 'select' | 'reject' | 'hold' | 'confirm-stage' | 'accept' | 'decline'
const DECIDED: Proposal['status'][] = ['selected', 'accepted', 'declined', 'rejected']
const done: Record<Action, string> = { select: 'Команда выбрана. Ждём её подтверждения.', reject: 'Предложение отклонено', hold: 'Предложение отложено', 'confirm-stage': 'Этап подтверждён, команде начислены баллы', accept: 'Проект принят. Можно начинать работу.', decline: 'Вы отказались от проекта' }
/** Статус словами: кто что решил и чего ждём. */
const STATUS: Record<Proposal['status'], string> = { new: 'На рассмотрении', selected: 'Выбрана, ждём ответа команды', accepted: 'Принят обеими сторонами', declined: 'Команда отказалась', rejected: 'Отклонена', on_hold: 'Отложена' }
const date = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })

/** Отклики одной структуры: команда · идея · план · срок и ссылка · статус и действие. */
export function ProposalTable({ proposals, canDecide, teams = [], update }: { proposals: Proposal[]; canDecide: boolean; teams?: Team[]; update: () => void }) {
  const skills = new Map(teams.map(team => [team.id, [...team.skills, ...team.tech].slice(0, 3)]))
  return <div className="proposal-table" role="table" aria-label="Предложения команд">
    <div className="proposal-row proposal-head" role="row">
      <span role="columnheader">Команда</span><span role="columnheader">Идея</span><span role="columnheader">План</span><span role="columnheader">Срок</span><span role="columnheader">Ссылка</span><span role="columnheader">Статус</span><span role="columnheader">Действие</span>
    </div>
    {proposals.map(proposal => <ProposalCard key={proposal.id} proposal={proposal} canDecide={canDecide} skills={skills.get(proposal.team.id) ?? []} update={update} />)}
  </div>
}

export function ProposalCard({ proposal, canDecide, skills = [], update }: { proposal: Proposal; canDecide: boolean; skills?: string[]; update: () => void }) {
  const toast = useToast(); const { business, team, explain } = useSession()
  const [busy, setBusy] = useState<Action | ''>(''); const [error, setError] = useState(''); const [ask, setAsk] = useState<'reject' | 'decline' | ''>(''); const [chat, setChat] = useState(false)
  const ownTeam = Boolean(team && team.team.id === proposal.team.id)
  const role = ownTeam && !canDecide ? 'team' : 'business'
  const decided = DECIDED.includes(proposal.status)
  async function decision(action: Action, token = role === 'team' ? team?.token : business?.token) {
    setBusy(action); setError('')
    try {
      await api<Proposal>(`/proposals/${proposal.id}/${action}`, withAuth(token, json('POST')))
      setAsk(''); toast.success(done[action]); update()
    } catch (err) { setAsk(''); setError(explain(err, role, fresh => void decision(action, fresh))) } finally { setBusy('') }
  }
  const showChat = canDecide || ownTeam
  return <div className="proposal-item" role="rowgroup">
    <div className="proposal-row" role="row">
      <div role="cell" className="proposal-team"><strong>{proposal.team.name}</strong>{skills.length > 0 && <small>{skills.join(' · ')}</small>}<small>{date(proposal.created_at)}</small></div>
      <div role="cell" className="proposal-text"><span className="proposal-label">Идея</span><p>{proposal.idea}</p></div>
      <div role="cell" className="proposal-text"><span className="proposal-label">План</span><p>{proposal.plan}</p></div>
      <div role="cell" className="proposal-when"><span className="proposal-label">Срок</span><span>{proposal.deadline}</span></div>
      <div role="cell" className="proposal-link"><span className="proposal-label">Ссылка</span><a href={proposal.link} target="_blank" rel="noopener noreferrer">Материалы ↗</a></div>
      <div role="cell" className="proposal-status"><span className="proposal-label">Статус</span><Badge kind={proposal.status}>{STATUS[proposal.status]}</Badge>{proposal.stage_confirmed && <small className="confirmed-stage">Этап подтверждён</small>}</div>
      <div role="cell" className="proposal-actions">
        {canDecide && <>
          <Button variant="primary" size="sm" disabled={decided || Boolean(busy)} loading={busy === 'select'} onClick={() => void decision('select')}>Выбрать</Button>
          <div className="proposal-quiet">
            <Button variant="ghost" size="sm" disabled={decided || proposal.status === 'on_hold' || Boolean(busy)} loading={busy === 'hold'} onClick={() => void decision('hold')}>Отложить</Button>
            <Button variant="ghost" size="sm" disabled={decided || Boolean(busy)} onClick={() => setAsk('reject')}>Отклонить</Button>
          </div>
          {(proposal.status === 'selected' || proposal.status === 'accepted') && !proposal.stage_confirmed && <Button variant="secondary" size="sm" loading={busy === 'confirm-stage'} disabled={Boolean(busy)} onClick={() => void decision('confirm-stage')}>Подтвердить этап</Button>}
        </>}
        {ownTeam && !canDecide && proposal.status === 'selected' && <>
          <Button variant="primary" size="sm" loading={busy === 'accept'} disabled={Boolean(busy)} onClick={() => void decision('accept')}>Принять проект</Button>
          <Button variant="ghost" size="sm" disabled={Boolean(busy)} onClick={() => setAsk('decline')}>Отказаться</Button>
        </>}
        {!showChat && <small className="proposal-nobody">Решение за заявителем</small>}
        {showChat && <Button variant="ghost" size="sm" aria-expanded={chat} onClick={() => setChat(value => !value)}>{chat ? 'Скрыть переписку' : `Обсудить${proposal.messages_count ? ` (${proposal.messages_count})` : ''}`}</Button>}
      </div>
    </div>
    {error && <Alert tone="error" className="proposal-alert">{error}</Alert>}
    {chat && showChat && <ProposalChat proposalId={proposal.id} token={role === 'team' ? team?.token : business?.token} me={role} />}
    <ConfirmDialog open={ask === 'reject'} title={`Отклонить предложение команды ${proposal.team.name}?`} confirmLabel="Отклонить" tone="danger" busy={busy === 'reject'} onCancel={() => setAsk('')} onConfirm={() => void decision('reject')}>
      <p className="dialog-text">Команда увидит статус «отклонена». Выбрать это предложение потом не получится.</p>
    </ConfirmDialog>
    <ConfirmDialog open={ask === 'decline'} title="Отказаться от проекта?" confirmLabel="Отказаться" tone="danger" busy={busy === 'decline'} onCancel={() => setAsk('')} onConfirm={() => void decision('decline')}>
      <p className="dialog-text">Заявитель увидит статус «команда отказалась». Принять проект после отказа не получится.</p>
    </ConfirmDialog>
  </div>
}

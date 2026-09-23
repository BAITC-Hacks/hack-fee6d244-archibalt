import { useState } from 'react'
import { api, json, type Proposal } from '../api'
import { errorText } from '../fields'
import { Alert, Badge, Button, ConfirmDialog, useToast } from '../ui'

type Action = 'select' | 'reject' | 'confirm-stage'
const DECIDED: Proposal['status'][] = ['selected', 'accepted', 'declined', 'rejected']

export function ProposalCard({ proposal, canDecide, update }: { proposal: Proposal; canDecide: boolean; update: () => void }) {
  const toast = useToast()
  const [busy, setBusy] = useState<Action | ''>(''); const [error, setError] = useState(''); const [askReject, setAskReject] = useState(false)
  const decided = DECIDED.includes(proposal.status)
  async function decision(action: Action) {
    setBusy(action); setError('')
    try {
      await api<Proposal>(`/proposals/${proposal.id}/${action}`, json('POST'))
      setAskReject(false)
      toast.success(action === 'select' ? `Команда ${proposal.team.name} выбрана` : action === 'reject' ? 'Предложение отклонено' : 'Этап подтверждён, команде начислены баллы')
      update()
    } catch (err) { setAskReject(false); setError(errorText(err)) } finally { setBusy('') }
  }
  return <article className="proposal">
    <div className="proposal-top"><h3>{proposal.team.name}</h3><Badge kind={proposal.status} /></div>
    <div className="proposal-detail"><strong>Идея</strong><p>{proposal.idea}</p></div>
    <div className="proposal-detail"><strong>План</strong><p>{proposal.plan}</p></div>
    <div className="proposal-meta"><span>Срок: {proposal.deadline}</span><a href={proposal.link} target="_blank" rel="noopener noreferrer">Прототип и материалы ↗</a></div>
    {error && <Alert tone="error" className="proposal-alert">{error}</Alert>}
    {canDecide && <>
      <div className="decision-actions">
        <Button variant="primary" size="sm" disabled={decided || Boolean(busy)} loading={busy === 'select'} onClick={() => void decision('select')}>Выбрать команду</Button>
        <Button variant="ghost" size="sm" disabled={decided || Boolean(busy)} onClick={() => setAskReject(true)}>Отклонить</Button>
      </div>
      {(proposal.status === 'selected' || proposal.status === 'accepted') && !proposal.stage_confirmed && <div className="stage-action"><Button variant="secondary" size="sm" loading={busy === 'confirm-stage'} disabled={Boolean(busy)} onClick={() => void decision('confirm-stage')}>Подтвердить этап работы команды</Button></div>}
      {proposal.stage_confirmed && <p className="confirmed-stage">Этап подтверждён, баллы команде начислены</p>}
      <ConfirmDialog open={askReject} title={`Отклонить предложение команды ${proposal.team.name}?`} confirmLabel="Отклонить" tone="danger" busy={busy === 'reject'} onCancel={() => setAskReject(false)} onConfirm={() => void decision('reject')}>
        <p className="dialog-text">Команда увидит статус «отклонена». Выбрать это предложение после отказа уже не получится.</p>
      </ConfirmDialog>
    </>}
  </article>
}

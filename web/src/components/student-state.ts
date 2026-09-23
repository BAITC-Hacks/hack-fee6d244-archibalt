import type { Proposal } from '../api'

export interface StudentProposal extends Proposal { task: { id: number; title: string } }
export interface StudentNotice { key: string; title: string; detail: string; taskId: number; proposalId: number }
export function studentNotices(proposals: StudentProposal[], incoming: Record<number, number>): StudentNotice[] {
  const notices: StudentNotice[] = []
  for (const p of proposals) {
    const base = { detail: p.task?.title || `Задача №${p.task_id}`, taskId: p.task_id, proposalId: p.id }
    if (p.status === 'selected') notices.push({ ...base, key: `selected:${p.id}`, title: 'Бизнес выбрал вашу команду — подтвердите участие' })
    if (p.status === 'rejected') notices.push({ ...base, key: `rejected:${p.id}`, title: 'Бизнес отклонил предложение' })
    if (p.status === 'on_hold') notices.push({ ...base, key: `on_hold:${p.id}`, title: 'Бизнес отложил решение по отклику' })
    if (p.stage_confirmed) notices.push({ ...base, key: `stage:${p.id}`, title: 'Этап подтверждён — начислено 10 баллов' })
    if (incoming[p.id]) notices.push({ ...base, key: `message:${p.id}:${incoming[p.id]}`, title: 'Сообщение от заказчика' })
  }
  return notices.reverse()
}

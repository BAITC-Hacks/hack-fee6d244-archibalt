import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, withToken, type Task, type Team } from '../api'
import { ProposalTable } from '../components/ProposalCard'
import { plural } from '../fields'
import { useSession } from '../session'
import { Alert, Badge, Button, ButtonLink, EmptyState, Loading, Spinner } from '../ui'
import { useLoad } from '../useLoad'
import { useStartChat } from './TaskNew'

const date = (iso: string) => new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })

/** «Мои задачи» заявителя: задачи с местом в каталоге и отклики с решениями. */
export function MyTasks() {
  const startChat = useStartChat()
  const { business, checking, requestLogin, explain } = useSession()
  const { data: teams } = useLoad<Team[]>('/teams', [])
  const [tasks, setTasks] = useState<Task[] | null>(null); const [error, setError] = useState(''); const [refreshing, setRefreshing] = useState(false)
  const token = business?.token
  const load = useCallback(async () => {
    if (!token) return
    setRefreshing(true)
    try { const me = await api<{ tasks: Task[] }>('/business/me', withToken(token)); setTasks(me.tasks); setError('') }
    catch (err) { setError(explain(err, 'business')) } finally { setRefreshing(false) }
  }, [token, explain])
  useEffect(() => { void load() }, [load, business?.taskIds])

  if (checking) return <Loading />
  if (!business) return <div className="container flow-page"><EmptyState title="Войдите как заявитель" action={<Button variant="primary" onClick={() => requestLogin('business')}>Войти</Button>}>Здесь собраны ваши задачи и отклики команд на них.</EmptyState></div>
  if (tasks === null) return error ? <div className="container flow-page"><Alert tone="error">{error}</Alert></div> : <Loading />
  const total = tasks.reduce((sum, task) => sum + (task.proposals?.length ?? 0), 0)
  return <div className="container flow-page my-tasks">
    <div className="section-heading">
      <div><p className="eyebrow">Для заявителя</p><h1>Мои задачи</h1><p className="lead">{tasks.length} {plural(tasks.length, 'задача', 'задачи', 'задач')} · {total} {plural(total, 'отклик', 'отклика', 'откликов')}. Решение по каждому отклику принимаете вы.</p></div>
      {refreshing && <span className="catalog-refresh-inline" role="status"><Spinner size="sm" /> Обновляем…</span>}
    </div>
    {error && <Alert tone="error">{error}</Alert>}
    {tasks.length === 0 ? <EmptyState title="У вас пока нет задач" action={<Button variant="primary" aria-haspopup="dialog" onClick={() => startChat()}>Обсудить задачу</Button>}>Опишите первую задачу, и команды смогут предложить решения.</EmptyState>
      : <div className="my-task-list">{tasks.map(task => <section className="detail-card my-task" key={task.id}>
        <div className="my-task-head">
          <div className={`score-tile score-${task.level}`}><strong>{task.score}</strong><span>из 100</span></div>
          <div className="my-task-info">
            <div className="task-meta"><Badge kind={task.level} /><span>{task.confirmed && task.rank ? `#${task.rank} из ${task.catalog_size ?? '—'} в каталоге` : 'не опубликована'}</span><span>создана {date(task.created_at)}</span></div>
            <h2><Link to={`/task/${task.id}`}>{task.fields.title || 'Задача без названия'}</Link></h2>
          </div>
          <div className="my-task-actions"><ButtonLink size="sm" to={`/task/${task.id}/edit`}>Дополнить</ButtonLink><ButtonLink size="sm" variant="ghost" to={`/task/${task.id}`}>Открыть</ButtonLink></div>
        </div>
        {task.proposals?.length ? <ProposalTable proposals={task.proposals} teams={teams} canDecide update={() => void load()} /> : <p className="section-note">{task.confirmed ? 'Откликов пока нет. Задача открыта всем командам.' : 'Задача ещё не опубликована: подтвердите карточку, и команды её увидят.'}</p>}
      </section>)}</div>}
  </div>
}

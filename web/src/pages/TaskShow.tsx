import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, json, withToken, type Proposal, type Task, type Team } from '../api'
import { PageError } from '../components/PageState'
import { ProposalTable } from '../components/ProposalCard'
import { RatingPanel } from '../components/RatingPanel'
import { fieldSpecs, type Mode } from '../fields'
import { useSession } from '../session'
import { Alert, Badge, Button, ButtonLink, EmptyState, Field, Input, Loading, Spinner, Textarea, useToast } from '../ui'
import { useLoad } from '../useLoad'

export function TaskShow({ mode }: { mode: Mode }) {
  const { id } = useParams(); const toast = useToast(); const { data: task, setData: setTask, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const { data: teams } = useLoad<Team[]>('/teams', [])
  const { team, checking, requestLogin, explain, isOwner } = useSession()
  const [idea, setIdea] = useState(''); const [plan, setPlan] = useState(''); const [deadline, setDeadline] = useState(''); const [link, setLink] = useState(''); const [busy, setBusy] = useState(false); const [submitError, setSubmitError] = useState('')
  async function refresh() { try { setTask(await api<Task>(`/tasks/${id}`)) } catch (err) { setSubmitError(explain(err, 'team')) } }
  async function send(token: string) {
    setBusy(true); setSubmitError('')
    try {
      await api<Proposal>(`/tasks/${id}/proposals`, withToken(token, json('POST', { idea, plan, deadline, link })))
      toast.success('Предложение отправлено. Заявитель увидит его в списке предложений.')
      setIdea(''); setPlan(''); setDeadline(''); setLink(''); await refresh()
    } catch (err) { setSubmitError(explain(err, 'team', fresh => void send(fresh))) } finally { setBusy(false) }
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    if (!idea.trim() || !plan.trim() || !deadline.trim() || !link.trim()) { setSubmitError('Заполните все четыре поля: идею, план, срок и ссылку.'); return }
    if (!team) { requestLogin('team', token => void send(token)); return }
    void send(team.token)
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  const owner = isOwner(task.id)
  return <div className="container detail-page">
    <Link className="back-link" to="/">← Каталог задач</Link>
    <div className="detail-header">
      <div><div className="task-meta"><span>{task.industry || 'Без темы'}</span><Badge kind={task.level} />{owner && <span className="owner-mark">Ваша задача</span>}</div><h1>{task.fields.title || 'Задача без названия'}</h1><p>{task.fields.need || task.draft_text}</p></div>
      <div className="detail-score"><strong>{task.score}</strong><span>из 100 · готовность задачи</span></div>
    </div>
    <div className="detail-grid">
      <div>
        <section className="detail-card">
          <div className="section-heading compact"><div><p className="eyebrow">Описание</p><h2>О задаче</h2></div>{owner && <ButtonLink to={`/task/${id}/edit`}>Дополнить задачу</ButtonLink>}</div>
          <dl className="task-fields">{fieldSpecs.filter(spec => spec.key !== 'title').map(spec => <div key={spec.key}><dt>{spec.label}</dt><dd>{task.fields[spec.key] || <span className="not-filled">Пока не указано</span>}</dd></div>)}</dl>
        </section>
        <section className="detail-card proposals-section" id="proposals">
          <div className="section-heading compact"><div><p className="eyebrow">Открытый выбор</p><h2>Предложения команд <span className="heading-count">{task.proposals?.length || 0}</span></h2></div></div>
          {owner && <p className="section-note">Автоматического назначения нет: команду выбираете вы.</p>}
          {task.proposals?.length
            ? <ProposalTable proposals={task.proposals} teams={teams} canDecide={owner && mode === 'business'} update={() => void refresh()} />
            : <EmptyState slim title="Предложений пока нет">Задача открыта всем командам. Первый отклик появится здесь.</EmptyState>}
        </section>
      </div>
      <aside className="detail-aside">
        {mode === 'team' && <section className="respond-card" id="respond">
          <p className="eyebrow">Для студенческих команд</p>
          <h2>Предложить решение</h2>
          {checking ? <p className="respond-who"><Spinner size="sm" /> Проверяем вход…</p>
            : team ? <p className="respond-who">Предложение уйдёт от команды <strong>{team.team.name}</strong>.</p>
            : <div className="respond-who"><p>Опишите идею, а при отправке мы попросим email или телефон команды.</p></div>}
          <form onSubmit={submit} noValidate>
            {submitError && <Alert tone="error">{submitError}</Alert>}
            <Field label="Идея решения" required><Textarea minRows={3} value={idea} onChange={event => setIdea(event.target.value)} placeholder="Что предлагаете сделать?" /></Field>
            <Field label="План работы" required><Textarea minRows={3} value={plan} onChange={event => setPlan(event.target.value)} placeholder="Какие шаги выполните?" /></Field>
            <Field label="Срок" required><Input value={deadline} onChange={event => setDeadline(event.target.value)} placeholder="Например, 2 недели" /></Field>
            <Field label="Ссылка на прототип или материалы" required><Input type="url" inputMode="url" value={link} onChange={event => setLink(event.target.value)} placeholder="https://..." /></Field>
            <Button type="submit" variant="primary" size="lg" block loading={busy}>{busy ? 'Отправляем…' : 'Отправить предложение'}</Button>
            <p className="field-help">Число предложений не ограничено. Отправка не назначает команду автоматически.</p>
          </form>
        </section>}
        <RatingPanel task={task} />
      </aside>
    </div>
  </div>
}

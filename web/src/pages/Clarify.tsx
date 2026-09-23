import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, json, withAuth, type Task } from '../api'
import { PageError } from '../components/PageState'
import { ratingKey } from '../fields'
import { useSession } from '../session'
import { Alert, Button, Field, Loading, Meter, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export function Clarify() {
  const { id } = useParams(); const navigate = useNavigate(); const { business, explain } = useSession()
  const { data: task, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const [answers, setAnswers] = useState<Record<string, string>>({}); const [step, setStep] = useState(0); const [busy, setBusy] = useState(false); const [submitError, setSubmitError] = useState('')
  const field = useRef<HTMLTextAreaElement>(null)
  useEffect(() => { if (task) setAnswers(Object.fromEntries(task.questions.map(q => [String(q.id), q.answer || '']))) }, [task])
  useEffect(() => { field.current?.focus() }, [step])

  async function send(token = business?.token) {
    setBusy(true); setSubmitError('')
    try { await api<Task>(`/tasks/${id}/answers`, withAuth(token, json('POST', { answers }))); navigate(`/task/${id}/edit`) }
    catch (err) { setSubmitError(explain(err, 'business', fresh => void send(fresh))) } finally { setBusy(false) }
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />

  const total = task.questions.length
  const q = task.questions[Math.min(step, total - 1)]
  const last = step >= total - 1
  const miss = q ? task.missing.find(item => item.key === ratingKey(q.field_key)) : undefined
  const gain = miss?.gain; const hint = miss?.hint
  const answered = task.questions.filter(item => (answers[String(item.id)] || '').trim()).length
  function next(event?: FormEvent) { event?.preventDefault(); if (last) void send(); else setStep(value => value + 1) }
  function skip() { if (q) setAnswers(current => ({ ...current, [String(q.id)]: '' })); next() }

  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 2 из 3</p><h1>Добавим детали, важные команде</h1><p className="lead">Ответьте по своему опыту. Если чего-то пока нет, пропустите вопрос — AI не будет придумывать факты за вас.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={next}>
        {submitError && <Alert tone="error">{submitError}</Alert>}
        <div className="source-box"><span>Ваше исходное описание</span><p>{task.draft_text}</p></div>
        {q ? <>
          <div className="question-progress" aria-hidden="true">{task.questions.map((item, index) => <span key={item.id} className={index < step ? 'is-done' : index === step ? 'is-current' : ''} />)}</div>
          <Field key={q.id} className="question-field" label={<><span className="question-number">Вопрос {step + 1} из {total}</span><span>{q.text}</span></>} hint={hint}>
            <Textarea ref={field} value={answers[String(q.id)] || ''} disabled={busy} onChange={event => setAnswers({ ...answers, [String(q.id)]: event.target.value })} placeholder="Коротко, своими словами" />
          </Field>
          <div className="question-actions">
            {step > 0 && <Button variant="ghost" disabled={busy} onClick={() => setStep(value => value - 1)}>← Назад</Button>}
            <Button variant="secondary" disabled={busy} onClick={skip}>Пропустить{gain ? <span className="skip-cost">−{gain} к готовности</span> : null}</Button>
            <Button type="submit" variant="primary" loading={busy && last}>{last ? (busy ? 'Собираем карточку…' : 'Собрать карточку →') : 'Дальше →'}</Button>
          </div>
        </> : <Button type="submit" variant="primary" size="lg" className="form-submit" loading={busy}>Собрать карточку →</Button>}
      </form>
      <aside className="side-note">
        <p className="eyebrow">Сейчас</p>
        <Meter value={task.score} label="Готовность задачи сейчас" />
        <p className="clarify-count">Отвечено {answered} из {total}. Каждый ответ добавляет сведения в карточку и баллы к готовности.</p>
        <p className="mode-note">Режим: {task.ai_mode === 'openai' ? 'AI' : 'локальные вопросы'}. <Link to="/ai">Как это работает</Link></p>
      </aside>
    </div>
  </div>
}

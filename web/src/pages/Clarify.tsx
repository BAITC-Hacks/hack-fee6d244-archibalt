import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, json, type Task } from '../api'
import { PageError } from '../components/PageState'
import { errorText } from '../fields'
import { Alert, Button, Field, Loading, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export function Clarify() {
  const { id } = useParams(); const navigate = useNavigate(); const { data: task, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const [answers, setAnswers] = useState<Record<string, string>>({}); const [busy, setBusy] = useState(false); const [submitError, setSubmitError] = useState('')
  useEffect(() => { if (task) setAnswers(Object.fromEntries(task.questions.map(q => [String(q.id), q.answer || '']))) }, [task])
  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); setSubmitError(''); try { await api<Task>(`/tasks/${id}/answers`, json('POST', { answers })); navigate(`/task/${id}/edit`) } catch (err) { setSubmitError(errorText(err)) } finally { setBusy(false) } }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 2 из 3</p><h1>Добавим детали, важные команде</h1><p className="lead">Ответьте по своему опыту. Если чего-то пока нет, оставьте поле пустым — AI не будет придумывать факты за вас.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={submit}>
        {submitError && <Alert tone="error">{submitError}</Alert>}
        <div className="source-box"><span>Ваше исходное описание</span><p>{task.draft_text}</p></div>
        {task.questions.map((q, index) => <Field key={q.id} className="question-field" label={<><span className="question-number">{String(index + 1).padStart(2, '0')}</span><span>{q.text}</span></>}>
          <Textarea value={answers[String(q.id)] || ''} disabled={busy} onChange={event => setAnswers({ ...answers, [String(q.id)]: event.target.value })} placeholder="Коротко, своими словами" />
        </Field>)}
        <Button type="submit" variant="primary" size="lg" className="form-submit" loading={busy}>{busy ? 'Собираем карточку…' : 'Собрать карточку →'}</Button>
      </form>
      <aside className="side-note"><span className="side-note-icon">AI</span><h2>Вопросы под вашу задачу</h2><p>Они помогают раскрыть контекст, данные и результат, чтобы студенты могли предложить выполнимый план.</p><p className="mode-note">Режим: {task.ai_mode === 'openai' ? 'AI' : 'локальные вопросы'}. <Link to="/ai">Как это работает</Link></p></aside>
    </div>
  </div>
}

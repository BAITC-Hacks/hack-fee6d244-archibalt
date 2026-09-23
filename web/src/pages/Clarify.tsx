import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, json, withAuth, type Task } from '../api'
import { PageError } from '../components/PageState'
import { ratingKey } from '../fields'
import { useSession } from '../session'
import type { FieldKey } from '../api'
import { Alert, Button, Field, Loading, Meter, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export function Clarify() {
  const { id } = useParams(); const navigate = useNavigate(); const { business, explain } = useSession()
  const { data: task, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const [answers, setAnswers] = useState<Record<string, string>>({}); const [step, setStep] = useState(0); const [busy, setBusy] = useState(false); const [submitError, setSubmitError] = useState('')
  const field = useRef<HTMLTextAreaElement>(null)
  useEffect(() => { if (task) setAnswers(Object.fromEntries(task.questions.map(q => [String(q.id), q.answer || '']))) }, [task])
  useEffect(() => { field.current?.focus({ preventScroll: true }) }, [step])

  async function send(token = business?.token) {
    setBusy(true); setSubmitError('')
    try { await api<Task>(`/tasks/${id}/answers`, withAuth(token, json('POST', { answers }))); navigate(`/task/${id}/edit`) }
    catch (err) { setSubmitError(explain(err, 'business', fresh => void send(fresh))) } finally { setBusy(false) }
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />

  const total = task.questions.length
  const review = step >= total
  const q = review ? undefined : task.questions[step]
  const gainOf = (key: FieldKey) => task.missing.find(item => item.key === ratingKey(key))?.gain
  const gain = q ? gainOf(q.field_key) : undefined
  const asked = q ? splitExample(q.text, q.field_key) : undefined
  const answered = task.questions.filter(item => (answers[String(item.id)] || '').trim()).length
  function next(event?: FormEvent) { event?.preventDefault(); if (review) void send(); else setStep(value => value + 1) }
  function skip() { if (q) setAnswers(current => ({ ...current, [String(q.id)]: '' })); next() }

  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 2 из 3</p><h1>Добавим детали, важные команде</h1><p className="lead">Ответьте по своему опыту. Если чего-то пока нет, пропустите вопрос — AI не будет придумывать факты за вас.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={next}>
        {submitError && <Alert tone="error">{submitError}</Alert>}
        <div className="source-box"><span>Ваше исходное описание</span><p>{task.draft_text}</p></div>
        {total > 0 && <div className="question-progress" aria-hidden="true">{task.questions.map((item, index) => <span key={item.id} className={index < step ? 'is-done' : index === step ? 'is-current' : ''} />)}</div>}
        {q && asked ? <div className="question-step" key={q.id}>
          <Field className="question-field" label={<><span className="question-number">Вопрос {step + 1} из {total}</span><span>{asked.question}</span></>} hint={<>Пример ответа: {asked.example}</>}>
            <Textarea ref={field} value={answers[String(q.id)] || ''} disabled={busy} onChange={event => setAnswers({ ...answers, [String(q.id)]: event.target.value })} onKeyDown={event => { if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) next() }} placeholder="Коротко, своими словами" />
          </Field>
          <div className="question-actions">
            {step > 0 && <Button variant="ghost" onClick={() => setStep(value => value - 1)}>← Назад</Button>}
            <Button variant="secondary" onClick={skip}>{gain ? `Пропустить (−${gain} к готовности)` : 'Пропустить'}</Button>
            <Button type="submit" variant="primary">{step === total - 1 ? 'К сводке ответов →' : 'Дальше →'}</Button>
          </div>
        </div> : <div className="question-step question-review" key="review">
          <p className="question-number">{total ? 'Проверьте ответы' : 'Вопросов нет'}</p>
          {total > 0 ? <ol className="answer-summary">{task.questions.map((item, index) => {
            const text = (answers[String(item.id)] || '').trim(); const cost = gainOf(item.field_key)
            return <li key={item.id}>
              <div><strong>{splitExample(item.text, item.field_key).question}</strong>{text ? <p>{text}</p> : <p className="answer-skipped">Пропущено{cost ? ` · −${cost} к готовности` : ''}. Поле останется пустым, AI его не заполнит.</p>}</div>
              <Button variant="ghost" size="sm" disabled={busy} onClick={() => setStep(index)}>Изменить</Button>
            </li>
          })}</ol> : <p className="field-help">Карточка соберётся из исходного описания.</p>}
          <div className="question-actions">
            {total > 0 && <Button variant="ghost" disabled={busy} onClick={() => setStep(total - 1)}>← Назад</Button>}
            <Button type="submit" variant="primary" size="lg" loading={busy}>{busy ? 'AI собирает карточку…' : 'Собрать карточку →'}</Button>
          </div>
        </div>}
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

/** Пример ответа под вопросом: из скобок «(например, …)» в тексте вопроса, иначе — типовой по полю. */
const EXAMPLES: Partial<Record<FieldKey, string>> = {
  context: 'заявки приходят в WhatsApp и Excel, за неделю теряем 5–10 обращений',
  need: 'видеть все заявки в одном месте и кто за них отвечает',
  users: 'администраторы центра и руководитель',
  data: 'выгрузка Excel за полгода, около 1 200 строк',
  constraints: 'пилот за 4 недели, без платных сервисов',
  expected_result: 'рабочий прототип и короткая инструкция',
  success_criteria: 'ни одна заявка не теряется в течение месяца',
  contact: 'email или телефон ответственного',
  interaction_format: 'созвон раз в неделю и ответы в чате',
}
function splitExample(text: string, key: FieldKey) {
  const match = text.match(/\s*\((?:например|к примеру|пример)[,:]?\s*([^)]+)\)\s*/i)
  if (match) return { question: text.replace(match[0], ' ').replace(/\s+([?.!])/g, '$1').trim(), example: match[1].trim() }
  return { question: text, example: EXAMPLES[key] ?? 'пара предложений своими словами' }
}

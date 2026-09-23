import { useEffect, useState } from 'react'
import type { FieldKey } from '../api'
import { useNavigate, useParams } from 'react-router-dom'
import { api, json, withAuth, type Fields, type Task } from '../api'
import { PageError } from '../components/PageState'
import { RatingPanel } from '../components/RatingPanel'
import { fieldSpecs, ratingKey } from '../fields'
import { useSession } from '../session'
import { Alert, Badge, Button, Field, Input, Loading, Meter, Textarea, useToast } from '../ui'
import { useLoad } from '../useLoad'
import './TaskEdit.css'

const sections: { id: string; title: string; description: string; fields: FieldKey[] }[] = [
  { id: 'brief', title: 'Суть задачи', description: 'Помогите команде понять, что нужно сделать и для кого.', fields: ['title', 'need', 'context', 'users'] },
  { id: 'result', title: 'Результат и условия', description: 'Опишите, что получите в конце и как проверите работу.', fields: ['expected_result', 'success_criteria', 'data', 'constraints'] },
  { id: 'connection', title: 'Связь с командой', description: 'Оставьте рабочий контакт и договоритесь, как будете общаться.', fields: ['contact', 'interaction_format'] },
]
const labels: Partial<Record<FieldKey, string>> = {
  title: 'Название задачи', need: 'Что нужно сделать', context: 'Что происходит сейчас', users: 'Кто будет пользоваться решением',
  expected_result: 'Что команда должна передать', success_criteria: 'Как вы примете результат', constraints: 'Сроки и ограничения',
  contact: 'Рабочий контакт', interaction_format: 'Как будете общаться',
}

export function TaskEdit() {
  const { id } = useParams(); const navigate = useNavigate(); const toast = useToast(); const { business, explain } = useSession(); const { data: task, setData: setTask, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const [fields, setFields] = useState<Fields>({} as Fields); const [busy, setBusy] = useState<'' | 'save' | 'confirm'>(''); const [submitError, setSubmitError] = useState(''); const [delta, setDelta] = useState<{ from: number; to: number } | null>(null)
  useEffect(() => { if (task) setFields(task.fields) }, [task])
  const dirty = Boolean(task && fieldSpecs.some(({ key }) => (fields[key] || '') !== (task.fields[key] || '')))
  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  async function save(token = business?.token) {
    setBusy('save'); setSubmitError('')
    try {
      const updated = await api<Task>(`/tasks/${id}/fields`, withAuth(token, json('PUT', { fields })))
      const from = updated.previous_score ?? task.score
      setDelta({ from, to: updated.score }); setTask(updated)
      toast.success(updated.score > from ? `Рейтинг вырос: ${from} → ${updated.score}` : 'Черновик сохранён, оценка обновлена.')
    } catch (err) { setSubmitError(explain(err, 'business', fresh => void save(fresh))) } finally { setBusy('') }
  }
  async function publish(token = business?.token) {
    setBusy('confirm'); setSubmitError('')
    try {
      const saved = await api<Task>(`/tasks/${id}/fields`, withAuth(token, json('PUT', { fields })))
      setTask(saved)
      const done = await api<Task>(`/tasks/${id}/confirm`, withAuth(token, json('POST')))
      toast.success(done.rank ? `Задача опубликована: место #${done.rank} из ${done.catalog_size ?? done.rank} в каталоге.` : 'Задача опубликована и появилась в каталоге.')
      navigate(`/task/${id}`)
    } catch (err) { setSubmitError(explain(err, 'business', fresh => void publish(fresh))) } finally { setBusy('') }
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  const filled = fieldSpecs.filter(({ key }) => fields[key]?.trim()).length
  return <div className="container task-editor">
    <header className="task-editor-head">
      <div className="task-editor-meta"><span>Задача №{task.id}</span><span className={task.confirmed ? 'is-published' : ''}>{task.confirmed ? 'В каталоге' : 'Черновик · ещё не опубликован'}</span><span>{task.industry}</span></div>
      <h1>{task.confirmed ? 'Обновите вашу задачу' : 'Проверьте задачу перед публикацией'}</h1>
      <p>Поправьте описание, чтобы команда поняла вас правильно. Когда всё готово — опубликуйте задачу.</p>
    </header>
    <nav className="task-editor-nav" aria-label="Разделы редактора">
      {sections.map((section, index) => <a key={section.id} href={`#edit-${section.id}`}><span>{String(index + 1).padStart(2, '0')}</span>{section.title}<small>{section.fields.filter(key => fields[key]?.trim()).length}/{section.fields.length}</small></a>)}
    </nav>
    <div className="task-editor-layout">
      <form id="task-edit-form" className="task-editor-form" onSubmit={event => { event.preventDefault(); void save() }}>
        {sections.map((section, index) => <section className="task-editor-section" id={`edit-${section.id}`} key={section.id} aria-labelledby={`edit-heading-${section.id}`}>
          <header><span className="task-editor-step">{String(index + 1).padStart(2, '0')}</span><div><h2 id={`edit-heading-${section.id}`}>{section.title}</h2><p>{section.description}</p></div></header>
          <fieldset disabled={Boolean(busy)}>
            <legend className="ui-visually-hidden">{section.title}</legend>
            {section.fields.map(key => {
              const spec = fieldSpecs.find(item => item.key === key)!
              const hint = key === 'contact' ? 'Этот контакт увидят команды. Укажите рабочий email, телефон или Telegram.' : spec.hint
              return <Field key={key} id={key} className="form-field" label={<>{labels[key] || spec.label}{!fields[key]?.trim() && <span className="task-editor-empty">Не заполнено</span>}</>} hint={hint}>
                {key === 'title' || key === 'contact'
                  ? <Input value={fields[key] || ''} onChange={event => setFields(current => ({ ...current, [key]: event.target.value }))} placeholder={key === 'contact' ? 'name@company.kz или @username' : 'Например, автоматизировать сбор заявок'} />
                  : <Textarea minRows={3} value={fields[key] || ''} onChange={event => setFields(current => ({ ...current, [key]: event.target.value }))} placeholder={spec.hint} />}
              </Field>
            })}
          </fieldset>
        </section>)}
      </form>
      <aside className="task-editor-aside" aria-label="Готовность и публикация">
        <section className="task-editor-score">
          <div className="task-editor-score-head"><h2>Насколько ясна задача</h2><Badge kind={task.level} /></div>
          <Meter value={task.score} label="Готовность описания после сохранения" />
          <p className="task-editor-score-note">{dirty ? 'Есть правки. Сохраните черновик, чтобы обновить оценку.' : 'Оценка последней сохранённой версии.'}</p>
          {delta && delta.to !== delta.from && <p className="task-editor-delta" role="status">{delta.from} → {delta.to} баллов после сохранения</p>}
          <div className="task-editor-completeness"><span>Заполнено полей</span><strong>{filled} из {fieldSpecs.length}</strong></div>
          {task.missing.length > 0 && <div className="task-editor-improve"><h3>Что стоит уточнить</h3>{task.missing.slice(0, 3).map(item => <button type="button" key={item.key} onClick={() => focusField(fields, item.key)}><span>{item.label}</span><strong>+{item.gain} <span aria-hidden="true">↗</span></strong></button>)}</div>}
          <p className="task-editor-small">Можно публиковать с любой оценкой. Чем понятнее задача, тем проще команде предложить решение.</p>
          <details className="task-editor-rating"><summary>Как считается оценка</summary><RatingPanel task={task} preliminary={!task.confirmed} delta={delta} onPick={key => focusField(fields, key)} /></details>
        </section>
        <section className="task-editor-next"><h2>Что будет после публикации</h2><ol><li>Задача появится в каталоге.</li><li>Команды предложат свои решения.</li><li>Вы выберете, с кем работать.</li></ol></section>
      </aside>
    </div>
    <div className="task-editor-dock">
      {submitError && <Alert tone="error">{submitError}</Alert>}
      <div className="task-editor-dock-row"><div className="task-editor-save-state" role="status"><strong>{busy ? (busy === 'save' ? 'Сохраняем черновик…' : 'Публикуем задачу…') : dirty ? 'Есть несохранённые изменения' : 'Все изменения сохранены'}</strong><span>{task.confirmed ? 'Сохранение черновика снимет задачу с публикации.' : 'В каталоге появится только после публикации.'}</span></div><div className="task-editor-buttons">
        <Button type="submit" form="task-edit-form" variant="secondary" loading={busy === 'save'} disabled={busy === 'confirm'}>{task.confirmed ? 'Снять с публикации' : 'Сохранить черновик'}</Button>
        <Button variant="primary" loading={busy === 'confirm'} disabled={busy === 'save'} onClick={() => void publish()}>{busy === 'confirm' ? 'Публикуем…' : task.confirmed ? 'Обновить в каталоге' : 'Опубликовать задачу'}<span aria-hidden="true">↗</span></Button>
      </div></div>
    </div>
  </div>
}

/** Подсказка рейтинга ведёт к первому подходящему полю, пустое — в приоритете. */
function focusField(fields: Fields, key: string) {
  const candidates = fieldSpecs.map(spec => spec.key).filter(field => ratingKey(field) === key)
  const target = candidates.find(field => !fields[field]?.trim()) ?? candidates[0]
  const node = target && document.getElementById(target)
  if (!node) return
  node.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'center' })
  window.setTimeout(() => node.focus({ preventScroll: true }), 250)
  node.closest('.form-field')?.classList.add('is-flash'); window.setTimeout(() => node.closest('.form-field')?.classList.remove('is-flash'), 1400)
}

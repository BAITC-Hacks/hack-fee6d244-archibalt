import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, json, withAuth, type Fields, type Task } from '../api'
import { PageError } from '../components/PageState'
import { RatingPanel } from '../components/RatingPanel'
import { fieldSpecs, ratingKey } from '../fields'
import { useSession } from '../session'
import { Alert, Button, Field, Input, Loading, Textarea, useToast } from '../ui'
import { useLoad } from '../useLoad'

export function TaskEdit() {
  const { id } = useParams(); const navigate = useNavigate(); const toast = useToast(); const { business, explain } = useSession(); const { data: task, setData: setTask, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const [fields, setFields] = useState<Fields>({} as Fields); const [busy, setBusy] = useState<'' | 'save' | 'confirm'>(''); const [submitError, setSubmitError] = useState(''); const [delta, setDelta] = useState<{ from: number; to: number } | null>(null)
  useEffect(() => { if (task) setFields(task.fields) }, [task])
  async function save(token = business?.token) {
    setBusy('save'); setSubmitError('')
    try {
      const updated = await api<Task>(`/tasks/${id}/fields`, withAuth(token, json('PUT', { fields })))
      const from = updated.previous_score ?? task.score
      setDelta({ from, to: updated.score }); setTask(updated)
      toast.success(updated.score > from ? `Рейтинг вырос: ${from} → ${updated.score}` : 'Дополнения сохранены, рейтинг пересчитан.')
    } catch (err) { setSubmitError(explain(err, 'business', fresh => void save(fresh))) } finally { setBusy('') }
  }
  async function publish(token = business?.token) {
    setBusy('confirm'); setSubmitError('')
    try {
      await api<Task>(`/tasks/${id}/fields`, withAuth(token, json('PUT', { fields })))
      const done = await api<Task>(`/tasks/${id}/confirm`, withAuth(token, json('POST')))
      toast.success(done.rank ? `Задача опубликована: место #${done.rank} из ${done.catalog_size ?? done.rank} в каталоге.` : 'Задача опубликована и появилась в каталоге.')
      navigate(`/task/${id}`)
    } catch (err) { setSubmitError(explain(err, 'business', fresh => void publish(fresh))) } finally { setBusy('') }
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  return <div className="container flow-page edit-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 3 из 3</p><h1>Проверьте карточку и рейтинг</h1><p className="lead">Все поля можно изменить. Сохраните дополнения, чтобы увидеть новый предварительный балл, затем подтвердите публикацию.</p></div>
    <div className="edit-grid">
      <form className="paper-form edit-form" onSubmit={event => { event.preventDefault(); void save() }}>
        {submitError && <Alert tone="error">{submitError}</Alert>}
        <div className="form-section-title"><h2>Карточка задачи</h2><span>Все сведения редактируемы</span></div>
        {fieldSpecs.map(spec => <Field key={spec.key} id={spec.key} className="form-field" label={spec.label} hint={missingHint(task, spec.key)}>
          {spec.key === 'title' || spec.key === 'contact'
            ? <Input value={fields[spec.key] || ''} onChange={event => setFields({ ...fields, [spec.key]: event.target.value })} placeholder={spec.hint} />
            : <Textarea minRows={2} value={fields[spec.key] || ''} onChange={event => setFields({ ...fields, [spec.key]: event.target.value })} placeholder={spec.hint} />}
        </Field>)}
        <div className="form-actions">
          <Button type="submit" variant="secondary" loading={busy === 'save'} disabled={busy === 'confirm'}>Сохранить и пересчитать</Button>
          <Button variant="primary" loading={busy === 'confirm'} disabled={busy === 'save'} onClick={() => void publish()}>{busy === 'confirm' ? 'Публикуем…' : 'Подтвердить и опубликовать →'}</Button>
        </div>
        <p className="field-help">Задача появится в каталоге только после вашего подтверждения.</p>
      </form>
      <RatingPanel task={task} preliminary={!task.confirmed} delta={delta} />
    </div>
  </div>
}

function missingHint(task: Task, field: string) {
  const miss = task.missing.find(item => item.key === ratingKey(field))
  return miss && (field !== 'contact' || !task.fields.contact) ? `+${miss.gain} к готовности: ${miss.hint}` : undefined
}

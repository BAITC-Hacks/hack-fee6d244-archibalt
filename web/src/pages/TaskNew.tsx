import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, json, type CatalogResponse, type Task } from '../api'
import { errorText } from '../fields'
import { Alert, Button, Chips, Field, Input, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export function TaskNew() {
  const navigate = useNavigate(); const [draft, setDraft] = useState(''); const [industry, setIndustry] = useState(''); const [busy, setBusy] = useState(false); const [draftError, setDraftError] = useState(''); const [error, setError] = useState('')
  const { data } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })
  async function submit(event: FormEvent) { event.preventDefault(); if (!draft.trim()) { setDraftError('Опишите задачу хотя бы одним предложением.'); return } setBusy(true); setError(''); try { const task = await api<Task>('/tasks', json('POST', { draft_text: draft, industry })); navigate(`/task/${task.id}/clarify`) } catch (err) { setError(errorText(err)) } finally { setBusy(false) } }
  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 1 из 3</p><h1>С какой задачей нужна помощь?</h1><p className="lead">Напишите как есть. Достаточно пары предложений — дальше мы зададим вопросы, которые помогут команде понять задачу.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={submit} noValidate>
        {error && <Alert tone="error">{error}</Alert>}
        <Field label="Краткое описание" required error={draftError || undefined} hint="Не придумывайте недостающие данные: их можно добавить после уточнения.">
          <Textarea minRows={5} value={draft} disabled={busy} onChange={event => { setDraft(event.target.value); setDraftError('') }} placeholder="Например: клиенты долго ждут ответ на повторяющиеся вопросы, а наша команда поддержки не успевает." />
        </Field>
        <Field label="Тема" optional>
          <Input value={industry} disabled={busy} onChange={event => setIndustry(event.target.value)} placeholder="Например, образование" />
        </Field>
        <Chips label="Темы из каталога" options={data.industries} value={industry} onPick={setIndustry} />
        <Button type="submit" variant="primary" size="lg" className="form-submit" loading={busy}>{busy ? 'Подбираем вопросы…' : 'Продолжить к вопросам →'}</Button>
      </form>
      <aside className="side-note"><span className="side-note-icon">?</span><h2>Что будет дальше</h2><ol><li>Мы зададим не менее трёх уточняющих вопросов.</li><li>Вы проверите и отредактируете карточку.</li><li>После вашего подтверждения задача появится в каталоге.</li></ol><p>Команду выбираете только вы.</p></aside>
    </div>
  </div>
}

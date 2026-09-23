import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api, json, withToken, type CatalogResponse, type Task } from '../api'
import { readDraft, saveDraft } from '../fields'
import { useSession } from '../session'
import { Alert, Button, Chips, Field, Input, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export const DRAFT_PLACEHOLDER = 'Языковой центр вырос из Excel, заявки теряются в WhatsApp, руководству не видна картина'

export function TaskNew() {
  const navigate = useNavigate(); const location = useLocation()
  const { business, requestLogin, refreshBusiness, explain } = useSession()
  const initial = useRef(readDraft())
  const passed = (location.state as { draft?: string } | null)?.draft
  const [draft, setDraft] = useState(passed ?? initial.current.text); const [industry, setIndustry] = useState(initial.current.industry)
  const [busy, setBusy] = useState(false); const [draftError, setDraftError] = useState(''); const [industryError, setIndustryError] = useState(''); const [error, setError] = useState(''); const [savedAt, setSavedAt] = useState(false)
  const { data } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })

  useEffect(() => { const timer = window.setTimeout(() => { saveDraft(draft, industry); setSavedAt(Boolean(draft || industry)) }, 400); return () => window.clearTimeout(timer) }, [draft, industry])

  async function create(token: string) {
    setBusy(true); setError('')
    try {
      const task = await api<Task>('/tasks', withToken(token, json('POST', { draft_text: draft, industry })))
      saveDraft('', ''); await refreshBusiness(token)
      navigate(`/task/${task.id}/clarify`)
    } catch (err) { setError(explain(err, 'business', create)) } finally { setBusy(false) }
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    let ok = true
    if (!draft.trim()) { setDraftError('Опишите задачу хотя бы одним предложением.'); ok = false }
    if (!industry.trim()) { setIndustryError('Укажите тему: по ней команды находят задачи.'); ok = false }
    if (!ok) return
    if (!business) { requestLogin('business', create); return }
    void create(business.token)
  }
  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса · шаг 1 из 3</p><h1>С какой задачей нужна помощь?</h1><p className="lead">Напишите как есть. Достаточно пары предложений — дальше мы зададим вопросы, которые помогут команде понять задачу.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={submit} noValidate>
        {error && <Alert tone="error">{error}</Alert>}
        <Field label="Краткое описание" required error={draftError || undefined} hint={<DraftHint saved={savedAt} />}>
          <Textarea minRows={5} value={draft} disabled={busy} onChange={event => { setDraft(event.target.value); setDraftError('') }} placeholder={DRAFT_PLACEHOLDER} />
        </Field>
        <Field label="Тема" required error={industryError || undefined}>
          <Input value={industry} disabled={busy} onChange={event => { setIndustry(event.target.value); setIndustryError('') }} placeholder="Например, образование" />
        </Field>
        <Chips label="Темы из каталога" options={data.industries} value={industry} onPick={value => { setIndustry(value); setIndustryError('') }} />
        <Button type="submit" variant="primary" size="lg" className="form-submit" loading={busy}>{busy ? 'AI читает задачу…' : 'Продолжить к вопросам →'}</Button>
        {!business && <p className="field-help">При отправке попросим email или телефон: так вы вернётесь к задаче и увидите отклики.</p>}
      </form>
      <aside className="side-note"><span className="side-note-icon">?</span><h2>Что будет дальше</h2><ol><li>Мы зададим не менее трёх уточняющих вопросов.</li><li>Вы проверите и отредактируете карточку.</li><li>После вашего подтверждения задача появится в каталоге.</li></ol><p>Команду выбираете только вы.</p></aside>
    </div>
  </div>
}

/** Подсказка под полем черновика: правило AI и тихая отметка автосохранения (место под неё зарезервировано). */
export function DraftHint({ saved }: { saved: boolean }) {
  return <span className="draft-hint"><span>Пишите своими словами. Сведения, которых нет, AI не придумает.</span><span className={`draft-saved${saved ? ' is-on' : ''}`} role="status">{saved ? 'Черновик сохранён' : ''}</span></span>
}

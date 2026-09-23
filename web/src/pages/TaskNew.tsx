import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { useAgentChat } from '../components/AgentChat'
import { readDraft, saveDraft } from '../fields'
import { Button, Chips, Field, Textarea } from '../ui'
import { useLoad } from '../useLoad'

export const DRAFT_PLACEHOLDER = 'Языковой центр вырос из Excel, заявки теряются в WhatsApp, руководству не видна картина'
/** Бэкенд требует непустую отрасль; до чата её не спрашиваем. */
export const DEFAULT_INDUSTRY = 'Другое'

/** Сразу открыть диалог с агентом, вход не нужен: он понадобится только при сборке карточки. Пустой текст — агент сам спросит суть в чате. */
export function useStartChat() {
  const { openAgentChat } = useAgentChat()
  return (draft = '', industry = readDraft().industry) => {
    const text = draft.trim()
    if (text) saveDraft(text, industry)
    openAgentChat({ draftText: text, industry: industry.trim() || DEFAULT_INDUSTRY })
  }
}

export function TaskNew() {
  const location = useLocation(); const navigate = useNavigate(); const startChat = useStartChat()
  const initial = useRef(readDraft())
  const passed = (location.state as { draft?: string } | null)?.draft
  const [draft, setDraft] = useState(passed ?? initial.current.text); const [industry, setIndustry] = useState(initial.current.industry)
  const [draftError, setDraftError] = useState(''); const [savedAt, setSavedAt] = useState(false)
  const { data } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })

  useEffect(() => { const timer = window.setTimeout(() => { saveDraft(draft, industry); setSavedAt(Boolean(draft || industry)) }, 400); return () => window.clearTimeout(timer) }, [draft, industry])

  // Пришли с черновиком в state — сразу в диалог; state чистим, чтобы «назад» не открывал его снова.
  const autostarted = useRef(false)
  useEffect(() => {
    if (autostarted.current || !passed?.trim()) return
    autostarted.current = true
    navigate(location.pathname, { replace: true, state: null })
    startChat(passed, industry)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function submit(event?: FormEvent) {
    event?.preventDefault()
    if (!draft.trim()) { setDraftError('Опишите задачу хотя бы одним предложением.'); return }
    startChat(draft, industry)
  }
  return <div className="container flow-page">
    <div className="flow-head"><p className="eyebrow">Для бизнеса</p><h1>С какой задачей нужна помощь?</h1><p className="lead">Напишите как есть. Достаточно пары предложений — дальше агент задаст 3–5 вопросов в диалоге и соберёт карточку.</p></div>
    <div className="flow-layout">
      <form className="paper-form" onSubmit={submit} noValidate>
        <Field label="Опишите задачу своими словами" error={draftError || undefined} hint={<DraftHint saved={savedAt} />}>
          <Textarea minRows={5} value={draft} onChange={event => { setDraft(event.target.value); setDraftError('') }}
            onKeyDown={event => { if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) submit() }} placeholder={DRAFT_PLACEHOLDER} />
        </Field>
        <Chips label="Тема, необязательно" options={data.industries} value={industry} onPick={value => setIndustry(current => current === value ? '' : value)} />
        <Button type="submit" variant="primary" size="lg" className="form-submit">Начать разговор →</Button>
      </form>
      <aside className="side-note"><span className="side-note-icon">?</span><h2>Что будет дальше</h2><ol><li>Агент задаст 3–5 уточняющих вопросов.</li><li>Вы проверите и отредактируете карточку.</li><li>После вашего подтверждения задача появится в каталоге.</li></ol><p>Команду выбираете только вы.</p></aside>
    </div>
  </div>
}

/** Подсказка под полем черновика: правило AI и тихая отметка автосохранения (место под неё зарезервировано). */
export function DraftHint({ saved }: { saved: boolean }) {
  return <span className="draft-hint"><span>Пишите своими словами. Сведения, которых нет, AI не придумает.</span><span className={`draft-saved${saved ? ' is-on' : ''}`} role="status">{saved ? 'Черновик сохранён' : ''}</span></span>
}

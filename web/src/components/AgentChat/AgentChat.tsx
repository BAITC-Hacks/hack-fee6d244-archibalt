import { useEffect, useId, useLayoutEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { ApiError, type Fields } from '../../api'
import { errorText, fieldSpecs, ratingKey, saveDraft } from '../../fields'
import { useSession } from '../../session'
import { AnimatedNumber, Button, ConfirmDialog, LogoMark, Meter, Textarea, useToast } from '../../ui'
import { cx } from '../../ui/cx'
import { CloseIcon } from '../../ui/icons'
import { emptyCard } from './transport'
import type { AgentChatStart, AgentQuestion, ChatSession, Step, Transport } from './types'
import './AgentChat.css'

type Msg =
  | { id: number; kind: 'agent'; text: string }
  | { id: number; kind: 'user'; text: string; skipped?: boolean }
  | { id: number; kind: 'question'; q: AgentQuestion; state: 'open' | 'answered' | 'skipped'; picked?: string[] }
  | { id: number; kind: 'error'; text: string; detail?: string; retry: () => void; resolved?: boolean }
  | { id: number; kind: 'final'; reason: string; canMore: boolean; closed?: boolean }
type NewMsg = Msg extends infer M ? (M extends Msg ? Omit<M, 'id'> : never) : never

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'
const MAX_QUESTIONS = 5
const clip = (text: string, max = 120) => { const clean = text.trim().replace(/\s+/g, ' '); return clean.length > max ? `${clean.slice(0, max).trimEnd()}…` : clean }
const reducedMotion = () => Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
/** Варианты ответа по типу вопроса; для «да/нет» без подсказок — стандартная пара. */
const optionsOf = (q: AgentQuestion) => (q.input_type === 'yes_no' && (q.suggestions?.length ?? 0) < 2 ? ['Да', 'Нет'] : q.suggestions ?? [])

/** Модалка диалога с агентом: слева лента вопросов с интерактивными ответами, справа живая карточка задачи. */
export function AgentChat({ start, transport, onClose }: { start: AgentChatStart; transport: Transport; onClose: () => void }) {
  const navigate = useNavigate(); const toast = useToast(); const { business, explain, refreshBusiness } = useSession()
  const titleId = useId(); const descId = useId(); const cardId = useId()
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [pending, setPending] = useState(false)
  const [building, setBuilding] = useState(false)
  const [card, setCard] = useState<Fields>(emptyCard)
  const [score, setScore] = useState(0)
  const [flash, setFlash] = useState<Record<string, number>>({})
  const [text, setText] = useState('')
  const [multi, setMulti] = useState<string[]>([])
  const [confirm, setConfirm] = useState(false)
  const [cardOpen, setCardOpen] = useState(false)
  const session = useRef<ChatSession | null>(null)
  const token = useRef(business?.token || '')
  if (business?.token) token.current = business.token
  const cardRef = useRef(card)
  const askedRef = useRef(0)
  const nextId = useRef(1)
  const panel = useRef<HTMLDivElement>(null); const feed = useRef<HTMLDivElement>(null); const input = useRef<HTMLTextAreaElement>(null)

  const push = (...items: NewMsg[]) => setMsgs(list => [...list, ...items.map(item => ({ ...item, id: nextId.current++ }) as Msg)])
  const patch = (id: number, change: (msg: Msg) => Msg) => setMsgs(list => list.map(msg => (msg.id === id ? change(msg) : msg)))

  const lastQuestion = [...msgs].reverse().find((msg): msg is Extract<Msg, { kind: 'question' }> => msg.kind === 'question')
  const active = lastQuestion?.state === 'open' && !pending ? lastQuestion : undefined
  const gainOf = (q: AgentQuestion) => session.current?.missing.find(item => item.key === ratingKey(q.field_key))?.gain

  /** Выполнить запрос к агенту: «печатает…», на 401 — вход и повтор, на сбой — сообщение с кнопкой «Повторить». */
  async function run(action: (token: string) => Promise<void>) {
    setPending(true)
    try { await action(token.current) }
    catch (err) {
      if (err instanceof ApiError && err.status === 401) push({ kind: 'agent', text: explain(err, 'business', fresh => { token.current = fresh; void run(action) }) })
      else push({ kind: 'error', text: 'Не получилось получить ответ. Повторить?', detail: err instanceof ApiError ? errorText(err) : undefined, retry: () => void run(action) })
    } finally { setPending(false) }
  }

  function apply(step: Step, options: { first?: boolean; reason?: string } = {}) {
    const prev = cardRef.current
    const changed = fieldSpecs.map(spec => spec.key).filter(key => (prev[key] || '') !== (step.card[key] || ''))
    cardRef.current = step.card; setCard(step.card); setScore(step.score)
    if (!options.first && changed.length) setFlash(current => ({ ...current, ...Object.fromEntries(changed.map(key => [key, (current[key] || 0) + 1])) }))
    askedRef.current = step.asked
    if (step.question && !step.done) { setMulti([]); push({ kind: 'question', q: step.question, state: 'open' }) }
    else push({ kind: 'final', reason: options.reason || step.reason || 'Основное уже есть. Можно собирать карточку.', canMore: !options.reason && step.asked < MAX_QUESTIONS && session.current?.mode !== 'legacy' })
  }

  // Старт: новый диалог по черновику или продолжение задачи. Ref защищает от двойного эффекта StrictMode.
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    if ('taskId' in start) {
      push({ kind: 'agent', text: 'Продолжим с того места, где остановились.' })
      void run(async t => {
        const s = await transport.resume(start.taskId, t); session.current = s
        push(...s.history.flatMap((q): NewMsg[] => [{ kind: 'question', q, state: q.answer ? 'answered' : 'skipped', picked: q.answer ? q.answer.split('; ') : [] }, q.answer ? { kind: 'user', text: q.answer } : { kind: 'user', text: 'Пропустить', skipped: true }]))
        apply(s.first, { first: true })
      })
    } else {
      push({ kind: 'agent', text: `Понял: «${clip(start.draftText)}». Задам 3–4 вопроса.` })
      void run(async t => {
        const s = await transport.start(start, t); session.current = s
        saveDraft('', ''); if (s.mode !== 'mock') void refreshBusiness(t)
        apply(s.first, { first: true })
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function answer(value: string, picked?: string[]) {
    const q = active; const s = session.current
    if (!q || !s) return
    const clean = value.trim()
    patch(q.id, msg => ({ ...msg, state: clean ? 'answered' : 'skipped', picked }) as Msg)
    push(clean ? { kind: 'user', text: clean } : { kind: 'user', text: 'Пропустить', skipped: true })
    setText(''); setMulti([])
    void run(async t => apply(await transport.next(s, clean, t)))
  }

  function submit(event?: FormEvent) {
    event?.preventDefault()
    if (!active) return
    const typed = text.trim()
    if (active.q.input_type === 'multi') { const all = [...multi, ...(typed ? [typed] : [])]; if (all.length) answer(all.join('; '), multi); return }
    if (typed) answer(typed, optionsOf(active.q).filter(option => option === typed))
    else input.current?.focus()
  }

  function build(final: Extract<Msg, { kind: 'final' }>) {
    const s = session.current
    if (!s || building) return
    void run(async t => {
      setBuilding(true)
      try {
        const task = await transport.finish(s, t)
        patch(final.id, msg => ({ ...msg, closed: true }) as Msg)
        if (s.mode !== 'mock') { void refreshBusiness(t); navigate(`/task/${task.id}/edit`) }
        onClose()
        toast.success(`Карточка собрана: ${task.score}/100`)
      } finally { setBuilding(false) }
    })
  }

  function askMore(final: Extract<Msg, { kind: 'final' }>) {
    const s = session.current
    if (!s) return
    patch(final.id, msg => ({ ...msg, closed: true }) as Msg)
    push({ kind: 'user', text: 'Спросить ещё' })
    const before = askedRef.current
    void run(async t => {
      const step = await transport.next(s, undefined, t)
      apply(step, step.done && step.asked <= before ? { reason: 'Больше вопросов нет: всё важное уже есть. Можно собирать карточку.' } : {})
    })
  }

  function requestClose() {
    if (confirm) return
    if (session.current && session.current.mode !== 'mock') setConfirm(true)
    else onClose()
  }
  const closeRef = useRef(requestClose); closeRef.current = requestClose

  // Фокус-ловушка и Esc — только когда наш диалог верхний (над ним может открыться вход или подтверждение).
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    document.body.classList.add('ui-scroll-lock')
    panel.current?.focus()
    function onKey(event: globalThis.KeyboardEvent) {
      const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]')
      if (!panel.current || dialogs[dialogs.length - 1] !== panel.current) return
      if (event.key === 'Escape') { event.preventDefault(); closeRef.current(); return }
      if (event.key !== 'Tab') return
      const items = Array.from(panel.current.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(node => node.offsetParent !== null)
      if (!items.length) return
      const head = items[0]; const tail = items[items.length - 1]
      if (event.shiftKey && (document.activeElement === head || document.activeElement === panel.current)) { event.preventDefault(); tail.focus() }
      else if (!event.shiftKey && document.activeElement === tail) { event.preventDefault(); head.focus() }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      if (!document.querySelector('.ui-modal-overlay')) document.body.classList.remove('ui-scroll-lock')
      previous?.focus?.()
    }
  }, [])

  // Новый вопрос — фокус в поле ввода; новое сообщение — плавная прокрутка к концу ленты.
  useEffect(() => { if (active) input.current?.focus({ preventScroll: true }) }, [active?.id]) // eslint-disable-line react-hooks/exhaustive-deps
  useLayoutEffect(() => { feed.current?.scrollTo({ top: feed.current.scrollHeight, behavior: reducedMotion() ? 'auto' : 'smooth' }) }, [msgs.length, pending])

  function chipKeys(event: KeyboardEvent<HTMLElement>) {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key]
    if (step === undefined && event.key !== 'Home' && event.key !== 'End') return
    const items = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'))
    const index = items.indexOf(document.activeElement as HTMLButtonElement)
    if (index < 0) return
    event.preventDefault()
    const to = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (index + (step ?? 0) + items.length) % items.length
    items[to].focus()
  }

  function renderQuestion(msg: Extract<Msg, { kind: 'question' }>) {
    const { q } = msg; const open = msg === active
    const type = q.input_type || 'text'; const options = optionsOf(q)
    const pressed = (option: string) => (open ? (type === 'multi' ? multi.includes(option) : type === 'text' && text.trim() === option) : Boolean(msg.picked?.includes(option)))
    function pick(option: string) {
      if (type === 'text') { setText(option); input.current?.focus() }
      else if (type === 'multi') setMulti(list => (list.includes(option) ? list.filter(item => item !== option) : [...list, option]))
      else answer(option, [option])
    }
    return <div className="agent-bubble agent-bubble-agent">
      <p>{q.text}</p>
      {options.length > 0 && <div className={cx('agent-options', type === 'yes_no' && 'agent-options-yn')} role="group" aria-label={type === 'text' ? 'Подсказки: подставить в поле' : 'Варианты ответа'} onKeyDown={chipKeys}>
        {options.map(option => <button key={option} type="button" className="ui-chip agent-chip" aria-pressed={pressed(option)} disabled={!open} onClick={() => pick(option)}>{option}</button>)}
      </div>}
      {type === 'text' && options.length > 0 && <p className="agent-note">Подсказка подставится в поле, её можно поправить.</p>}
      {type === 'multi' && <div className="agent-multi-foot">
        <span className="agent-note">{open ? 'Можно выбрать несколько и дописать своё в поле ниже.' : ' '}</span>
        <Button size="sm" variant="primary" disabled={!open || (!multi.length && !text.trim())} onClick={() => submit()}>Готово</Button>
      </div>}
    </div>
  }

  function renderMsg(msg: Msg) {
    if (msg.kind === 'user') return <div className="agent-bubble agent-bubble-user"><p className={cx(msg.skipped && 'is-skipped')}>{msg.skipped ? 'Пропущено' : msg.text}</p></div>
    if (msg.kind === 'question') return renderQuestion(msg)
    if (msg.kind === 'error') return <div className="agent-bubble agent-bubble-agent agent-bubble-error">
      <p>{msg.text}</p>{msg.detail && <p className="agent-note">{msg.detail}</p>}
      <div className="agent-actions"><Button size="sm" disabled={msg.resolved || pending} onClick={() => { patch(msg.id, m => ({ ...m, resolved: true }) as Msg); msg.retry() }}>Повторить</Button></div>
    </div>
    if (msg.kind === 'final') return <div className="agent-bubble agent-bubble-agent">
      <p>{msg.reason}</p>
      <div className="agent-actions">
        <Button variant="primary" loading={building && !msg.closed} disabled={msg.closed || (pending && !building)} onClick={() => build(msg)}>{building ? 'Собираю карточку…' : 'Собрать карточку'}</Button>
        {msg.canMore && <Button variant="secondary" disabled={msg.closed || pending} onClick={() => askMore(msg)}>Спросить ещё</Button>}
      </div>
    </div>
    return <div className="agent-bubble agent-bubble-agent"><p>{msg.text}</p></div>
  }

  const skipGain = active ? gainOf(active.q) : undefined
  const placeholder = !active ? (pending ? 'Агент думает…' : 'Вопросов сейчас нет') : active.q.input_type === 'multi' ? 'Своё, если нет в списке' : active.q.input_type === 'choice' || active.q.input_type === 'yes_no' ? 'Или ответьте своими словами' : 'Коротко, своими словами'
  const filled = fieldSpecs.filter(spec => card[spec.key]?.trim()).length

  return createPortal(<div className="agent-overlay" onMouseDown={event => { if (event.target === event.currentTarget) requestClose() }}>
    <div ref={panel} className="agent-dialog" role="dialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={descId} tabIndex={-1}>
      <header className="agent-head">
        <span className="agent-avatar agent-avatar-lg"><LogoMark size={22} /></span>
        <div><h2 id={titleId}>Уточняем задачу</h2><p id={descId}>Отвечайте коротко. Чего нет — пропускайте: агент не придумывает факты.</p></div>
        <button type="button" className="ui-modal-close" aria-label="Закрыть" onClick={requestClose}><CloseIcon /></button>
      </header>
      <div className="agent-body">
        <section className="agent-chat" aria-label="Диалог с агентом">
          <div ref={feed} className="agent-feed" role="log" aria-live="polite" aria-relevant="additions">
            {msgs.map(msg => <div key={msg.id} className={cx('agent-msg', msg.kind === 'user' ? 'agent-msg-user' : 'agent-msg-agent')}>
              {msg.kind !== 'user' && <span className="agent-avatar" aria-hidden="true"><LogoMark size={18} /></span>}
              {renderMsg(msg)}
            </div>)}
            {pending && <div className="agent-msg agent-msg-agent">
              <span className="agent-avatar" aria-hidden="true"><LogoMark size={18} /></span>
              <div className="agent-bubble agent-bubble-agent agent-typing"><span className="agent-sr">Агент печатает…</span><i /><i /><i /></div>
            </div>}
          </div>
          <form className="agent-composer" onSubmit={submit}>
            <Textarea ref={input} minRows={1} value={text} aria-label="Ваш ответ" placeholder={placeholder}
              onChange={event => setText(event.target.value)}
              onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); submit() } }} />
            <div className="agent-composer-row">
              <Button variant="ghost" size="sm" disabled={!active} onClick={() => answer('')}>
                Пропустить{skipGain ? <span className="agent-cost">−{skipGain} к готовности</span> : null}
              </Button>
              <span className="agent-keys" aria-hidden="true">Enter — отправить, Shift+Enter — новая строка</span>
              <Button type="submit" variant="primary" size="sm" disabled={!active || (!text.trim() && !(active.q.input_type === 'multi' && multi.length))}>Отправить</Button>
            </div>
          </form>
        </section>
        <aside className={cx('agent-card', cardOpen && 'is-open')} aria-label="Карточка задачи">
          <div className="agent-card-head">
            <h3>Карточка задачи</h3>
            <span className="agent-card-mini"><AnimatedNumber value={score} />/100 · {filled} из {fieldSpecs.length}</span>
            <button type="button" className="agent-card-toggle" aria-expanded={cardOpen} aria-controls={cardId} onClick={() => setCardOpen(value => !value)}>{cardOpen ? 'Свернуть' : 'Показать'}</button>
          </div>
          <div className="agent-card-body" id={cardId}>
            <Meter value={score} label="Готовность карточки, предварительно" />
            <p className="agent-note">Предварительно. Точную оценку посчитаем, когда соберём карточку.</p>
            <dl className="agent-fields">
              {fieldSpecs.map(spec => <div key={`${spec.key}:${flash[spec.key] || 0}`} className={cx('agent-field', Boolean(flash[spec.key]) && 'is-flash')}>
                <dt>{spec.label}</dt><dd className={cx(!card[spec.key]?.trim() && 'is-empty')}>{card[spec.key]?.trim() || 'не указано'}</dd>
              </div>)}
            </dl>
          </div>
        </aside>
      </div>
    </div>
    <ConfirmDialog open={confirm} title="Закрыть диалог?" confirmLabel="Закрыть" cancelLabel="Продолжить" onCancel={() => setConfirm(false)}
      onConfirm={() => { setConfirm(false); onClose(); navigate('/business') }}>
      <p className="agent-confirm">Ответы сохранены, можно вернуться позже. Задача останется в разделе «Мои задачи».</p>
    </ConfirmDialog>
  </div>, document.body)
}

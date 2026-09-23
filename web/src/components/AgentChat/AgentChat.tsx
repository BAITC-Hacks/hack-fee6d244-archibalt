import { useEffect, useId, useLayoutEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { ApiError, type Fields } from '../../api'
import { capitalize, errorText, fieldSpecs, plural, ratingKey, saveDraft } from '../../fields'
import { useSession } from '../../session'
import { AnimatedNumber, Button, ConfirmDialog, LogoMark, Meter, Textarea, useToast } from '../../ui'
import { cx } from '../../ui/cx'
import { CloseIcon } from '../../ui/icons'
import { emptyCard } from './transport'
import type { AgentChatStart, AgentQuestion, ChatSession, ResultOption, Step, Transport } from './types'
import './AgentChat.css'

type Msg =
  | { id: number; kind: 'agent'; text: string }
  | { id: number; kind: 'user'; text: string; skipped?: boolean }
  | { id: number; kind: 'question'; q: AgentQuestion; state: 'open' | 'answered' | 'skipped'; picked?: string[] }
  | { id: number; kind: 'error'; text: string; detail?: string; retry: () => void; resolved?: boolean }
  | { id: number; kind: 'options'; options: ResultOption[]; canMore: boolean; chosen?: number | 'own' }
  | { id: number; kind: 'final'; reason: string; canMore: boolean; closed?: boolean }
  | { id: number; kind: 'visual'; state: 'loading' | 'ready' | 'error'; src?: string; mock?: boolean; error?: string }
type NewMsg = Msg extends infer M ? (M extends Msg ? Omit<M, 'id'> : never) : never

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'
const MAX_QUESTIONS = 5
const clip = (text: string, max = 120) => { const clean = text.trim().replace(/\s+/g, ' '); return clean.length > max ? `${clean.slice(0, max).trimEnd()}…` : clean }
/** Причина от AI бывает со строчной и без точки. */
const sentence = (text: string) => { const clean = capitalize(text.trim()); return /[.!?…]$/.test(clean) ? clean : `${clean}.` }
const reducedMotion = () => Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
/** Варианты ответа по типу вопроса; для «да/нет» без подсказок — стандартная пара. */
const optionsOf = (q: AgentQuestion) => {
  if (q.input_type === 'yes_no') return (q.suggestions?.length ?? 0) >= 2 ? q.suggestions! : ['Да', 'Нет']
  if (q.suggestions?.length) return q.suggestions
  const example = splitExample(q.text).example
  return example && (q.input_type ?? 'text') === 'text' ? [capitalize(example)] : []
}
/** «(например: …)» из текста вопроса выносим в подсказку, чтобы вопрос читался коротко. */
function splitExample(text: string) {
  const match = text.match(/\s*\((?:например|к примеру|пример)[,:]?\s*([^)]+)\)\s*/i)
  if (!match) return { question: text, example: '' }
  return { question: text.replace(match[0], ' ').replace(/\s+([?.!])/g, '$1').trim(), example: match[1].replace(/^[\s"“”«»]+|[\s"“”«»]+$/g, '') }
}

/** Модалка диалога с агентом: слева лента вопросов с интерактивными ответами, справа живая карточка задачи. */
export function AgentChat({ start, transport, onClose }: { start: AgentChatStart; transport: Transport; onClose: () => void }) {
  const navigate = useNavigate(); const toast = useToast(); const { business, explain, refreshBusiness } = useSession()
  const titleId = useId(); const descId = useId(); const cardId = useId()
  const [msgs, setMsgs] = useState<Msg[]>([])
  const [pending, setPending] = useState(false)
  const [building, setBuilding] = useState(false)
  const [drawing, setDrawing] = useState(false)
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
  const offered = useRef(false)
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

  /** Живая карточка: обновить поля и балл, подсветить изменившиеся поля. */
  function preview(next: Fields, value: number, flashChanges = true) {
    const prev = cardRef.current
    const changed = fieldSpecs.map(spec => spec.key).filter(key => (prev[key] || '') !== (next[key] || ''))
    cardRef.current = next; setCard(next); setScore(value)
    if (flashChanges && changed.length) setFlash(current => ({ ...current, ...Object.fromEntries(changed.map(key => [key, (current[key] || 0) + 1])) }))
  }

  /** Шаг диалога: следующий вопрос или финал; перед первым финалом — варианты первого результата, если сервер их даёт. */
  async function apply(step: Step, token: string, options: { first?: boolean; reason?: string } = {}) {
    preview(step.card, step.score, !options.first)
    askedRef.current = step.asked
    if (step.question && !step.done) { setMulti([]); push({ kind: 'question', q: step.question, state: 'open' }); return }
    const reason = sentence(options.reason || step.reason || 'Основное уже есть. Можно собирать карточку.')
    const canMore = !options.reason && step.asked < MAX_QUESTIONS && session.current?.mode !== 'legacy'
    const s = session.current
    if (s && !offered.current && !options.reason) {
      const list = await transport.resultOptions(s, token)
      offered.current = true
      if (list.length) { push({ kind: 'agent', text: reason }, { kind: 'options', options: list, canMore }); return }
    }
    push({ kind: 'final', reason, canMore })
  }

  function chooseResult(msg: Extract<Msg, { kind: 'options' }>, index: number) {
    const s = session.current; const option = msg.options[index]
    if (!s || !option || pending) return
    patch(msg.id, m => ({ ...m, chosen: index }) as Msg)
    push({ kind: 'user', text: `Выбираю: ${option.title}` })
    void run(async t => {
      await transport.applyResult(s, index, t)
      s.chosen = index
      const prev = cardRef.current
      const bonus = s.missing.filter(item => (item.key === 'expected_result' && !prev.expected_result?.trim()) || (item.key === 'success_criteria' && !prev.success_criteria?.trim())).reduce((sum, item) => sum + item.gain, 0)
      preview({ ...prev, expected_result: option.result, success_criteria: option.check }, Math.min(100, score + bonus))
      push({ kind: 'final', reason: 'Записал результат и критерии успеха в карточку. Можно собирать.', canMore: msg.canMore })
      showVisual() // концепт рисуется сам после выбора; «Собрать карточку» его не ждёт
    })
  }

  /** Концепт выбранного результата: необязательный шаг, не блокирует «Собрать карточку»; сбой — сообщение с «Повторить». */
  function showVisual() {
    const s = session.current
    if (!s || !transport.visual || drawing) return
    const id = nextId.current++ // id сразу: push назначает его лениво, а patch по готовности должен его найти
    setMsgs(list => [...list, { id, kind: 'visual', state: 'loading' }])
    setDrawing(true)
    transport.visual(s, token.current)
      .then(v => patch(id, m => ({ ...m, state: 'ready', src: v.src, mock: v.mock }) as Msg))
      .catch(err => patch(id, m => ({ ...m, state: 'error', error: err instanceof ApiError ? errorText(err) : err instanceof Error ? err.message : '' }) as Msg))
      .finally(() => setDrawing(false))
  }

  function ownResult(msg: Extract<Msg, { kind: 'options' }>) {
    patch(msg.id, m => ({ ...m, chosen: 'own' }) as Msg)
    push({ kind: 'user', text: 'Свой вариант' }, { kind: 'final', reason: 'Хорошо. Результат и критерии успеха допишете в редакторе карточки. Можно собирать.', canMore: msg.canMore })
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
        await apply(s.first, t, { first: true })
      })
    } else {
      push({ kind: 'agent', text: `Понял: «${clip(start.draftText)}». Задам 3–5 вопросов.` })
      void run(async t => {
        const s = await transport.start(start, t); session.current = s
        saveDraft('', ''); if (s.mode !== 'mock') void refreshBusiness(t)
        await apply(s.first, t, { first: true })
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
    void run(async t => apply(await transport.next(s, clean, t), t))
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
      await apply(step, t, step.done && step.asked <= before ? { reason: 'Больше вопросов нет: всё важное уже есть. Можно собирать карточку.' } : {})
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
  // Финал — фокус на «Собрать карточку», чтобы с клавиатуры не искать кнопку.
  const lastMsg = msgs[msgs.length - 1]
  useEffect(() => { if ((lastMsg?.kind === 'final' && !lastMsg.closed) || (lastMsg?.kind === 'visual' && lastMsg.state === 'loading')) feed.current?.querySelector<HTMLButtonElement>('.agent-final-main')?.focus({ preventScroll: true }) }, [lastMsg])
  useLayoutEffect(() => {
    const box = feed.current; if (!box) return
    // Длинное сообщение (варианты результата) показываем с начала, остальное — докручиваем до конца.
    const last = box.querySelector<HTMLElement>('.agent-msg:last-child')
    const top = last && last.offsetHeight > box.clientHeight - 40 ? last.offsetTop - 12 : box.scrollHeight
    box.scrollTo({ top, behavior: reducedMotion() ? 'auto' : 'smooth' })
  }, [msgs.length, pending])

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
      <p>{splitExample(q.text).question}</p>
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
    if (msg.kind === 'options') return <div className="agent-bubble agent-bubble-agent agent-bubble-wide">
      <p>Вот {msg.options.length === 2 ? 'два варианта' : `${msg.options.length} варианта`} первого результата, который команда сможет сделать и который вы сможете проверить.</p>
      <ul className="agent-results">
        {msg.options.map((option, index) => <li key={index} className={cx('agent-result', msg.chosen === index && 'is-chosen', msg.chosen !== undefined && msg.chosen !== index && 'is-muted')}>
          <div className="agent-result-head"><h4>{option.title}</h4>{option.weeks > 0 && <span className="agent-result-weeks" title="Оценка AI, не обязательство">~{option.weeks} {plural(option.weeks, 'неделя', 'недели', 'недель')} · оценка AI</span>}</div>
          <dl>
            <div><dt>Результат</dt><dd>{option.result}</dd></div>
            <div><dt>Как проверим</dt><dd>{option.check}</dd></div>
            {option.needs && <div><dt>Что нужно от вас</dt><dd>{option.needs}</dd></div>}
          </dl>
          <Button size="sm" variant={msg.chosen === index ? 'primary' : 'secondary'} disabled={msg.chosen !== undefined || pending} aria-pressed={msg.chosen === index} onClick={() => chooseResult(msg, index)}>{msg.chosen === index ? 'Выбрано' : 'Выбрать'}</Button>
        </li>)}
      </ul>
      <div className="agent-actions"><Button size="sm" variant="ghost" disabled={msg.chosen !== undefined || pending} onClick={() => ownResult(msg)}>Свой вариант</Button></div>
    </div>
    if (msg.kind === 'final') return <div className="agent-bubble agent-bubble-agent">
      <p>{msg.reason}</p>
      <div className="agent-actions">
        <Button variant="primary" className="agent-final-main" loading={building && !msg.closed} disabled={msg.closed || (pending && !building)} onClick={() => build(msg)}>{building ? 'Собираю карточку…' : 'Собрать карточку'}</Button>
        {msg.canMore && <Button variant="secondary" disabled={msg.closed || pending} onClick={() => askMore(msg)}>Спросить ещё</Button>}
      </div>
    </div>
    if (msg.kind === 'visual') {
      if (msg.state === 'loading') return <div className="agent-bubble agent-bubble-agent agent-visual-loading"><p>Рисую концепт…</p><span className="agent-typing" aria-hidden="true"><i /><i /><i /></span></div>
      if (msg.state === 'error') return <div className="agent-bubble agent-bubble-agent agent-bubble-error">
        <p>Не получилось нарисовать пример. Карточка не изменилась, собрать её можно и без картинки.</p>{msg.error && <p className="agent-note">{msg.error}</p>}
        <div className="agent-actions"><Button size="sm" disabled={drawing} onClick={showVisual}>Повторить</Button></div>
      </div>
      return <div className="agent-bubble agent-bubble-agent agent-bubble-wide agent-visual">
        <img src={msg.src} alt="Визуальный концепт выбранного результата" />
        <p className="agent-note">Концепт для обсуждения, не обещание объёма · оценка AI{msg.mock && <span className="agent-visual-mock">mock</span>}</p>
        <div className="agent-actions"><Button size="sm" variant="ghost" disabled={drawing} onClick={showVisual}>Другой вариант</Button></div>
      </div>
    }
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

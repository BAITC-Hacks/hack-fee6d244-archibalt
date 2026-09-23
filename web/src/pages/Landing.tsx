import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import type { AiInfo, CatalogResponse } from '../api'
import { AnimatedBackground } from '../components/AnimatedBackground'
import { useExample } from '../components/BeforeAfter'
import { ScrollStory } from '../components/ScrollStory'
import { Reveal } from '../components/Reveal'
import { plural, readDraft, saveDraft, type Mode } from '../fields'
import { ButtonLink, Textarea } from '../ui'
import { useLoad } from '../useLoad'
import { DRAFT_PLACEHOLDER } from './TaskNew'

const MIN_DRAFT = 10

const COMPARE: { row: string; cells: [string, string, string, string] }[] = [
  { row: 'Уточняющие вопросы по пробелам', cells: ['Да, 3–4 по вашему тексту', 'Нет, одинаковые поля для всех', 'Да, но без структуры', 'Нет'] },
  { row: 'Объяснение балла готовности', cells: ['Да, 7 показателей с +N', 'Нет', 'Нет', 'Нет'] },
  { row: 'Защита от выдуманных фактов', cells: ['Да, проверка по вашему тексту', 'Не требуется', 'Нет', 'Не требуется'] },
  { row: 'Открытый каталог для команд', cells: ['Да, сортировка по баллу', 'Нет', 'Нет', 'Нет'] },
  { row: 'Выбор команды вручную', cells: ['Да, решение за вами', 'Нет', 'Нет', 'Да, в переписке'] },
  { row: 'Переписка по каждому отклику', cells: ['Да, у отклика', 'Нет', 'Нет', 'Да, всё в одном чате'] },
]
const YES = /^Да/

const FAQ = [
  { q: 'Нужна ли регистрация?', a: 'Нет. Вход по email или телефону и одноразовому коду в момент, когда он нужен. В демо код всегда 000000.' },
  { q: 'Кто выбирает команду?', a: 'Только вы. Автоматического назначения нет: вы сравниваете отклики и выбираете вручную, команда подтверждает, что берёт проект.' },
  { q: 'Может ли AI что-то выдумать в карточке?', a: 'Карточка собирается только из вашего описания и ответов. Проверка удаляет сведения, которых в них нет, а пустое поле остаётся пустым. Последний вызов AI открыт на странице «Как работает AI».' },
  { q: 'Сколько это стоит?', a: 'Для участников AI Sana бесплатно.' },
  { q: 'Можно ли откликнуться на слабую задачу?', a: 'Да. Задача с баллом 0–39 видна в каталоге с пометкой «требует уточнения» и принимает отклики, как любая другая.' },
]

/** Главная: одно поле для задачи, пример «до/после», как работает, сравнение, FAQ. */
export function Landing({ setMode }: { setMode: (mode: Mode) => void }) {
  const navigate = useNavigate()
  const [draft, setDraft] = useState(() => readDraft().text)
  const ready = draft.trim().length >= MIN_DRAFT
  const { data: catalog } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })
  const { data: ai } = useLoad<AiInfo | null>('/ai', null)
  const example = useExample()
  const first = useRef(true)

  useEffect(() => {
    if (first.current) { first.current = false; return }
    const timer = window.setTimeout(() => saveDraft(draft, readDraft().industry), 400)
    return () => window.clearTimeout(timer)
  }, [draft])

  function submit(event?: FormEvent) {
    event?.preventDefault()
    if (!ready) return
    setMode('business'); saveDraft(draft, readDraft().industry)
    navigate('/task/new', { state: { draft } })
  }

  const stats = catalog.stats
  const taskCount = stats?.tasks ?? catalog.tasks.length
  const rankNow = example.task?.rank || catalog.tasks.findIndex(task => task.id === 1) + 1 || undefined
  const rankAfter = 1 + catalog.tasks.filter(task => task.id !== 1 && task.score > example.after).length
  const questionCount = ai?.last_call?.questions?.length

  return <div className="landing">
    <section className="landing-hero">
      <AnimatedBackground />
      <div className="container landing-hero-grid">
      <div className="landing-hero-copy">
        <h1>Опишите проблему. <em>Получите решения.</em></h1>
        <p className="lead">Archibalt превращает сырой запрос бизнеса в понятную задачу для студенческих команд. AI задаёт вопросы и не выдумывает факты, оценка готовности 0–100 объясняет, что добавить.</p>
        <form className="hero-input" onSubmit={submit}>
          <Textarea aria-label="Опишите задачу своими словами" minRows={3} value={draft} onChange={event => setDraft(event.target.value)}
            onKeyDown={event => { if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) submit() }} placeholder={DRAFT_PLACEHOLDER} />
          <button type="submit" className={`ui-btn ui-btn-primary hero-send${ready ? ' is-ready' : ''}`} tabIndex={ready ? 0 : -1} aria-hidden={!ready}>Отправить →</button>
        </form>
        <p className="hero-micro">AI задаст 3–4 вопроса и соберёт карточку. Ничего не придумает.</p>
        <div className="hero-secondary">
          <Link className="text-link" to="/catalog" onClick={() => setMode('team')}>Смотреть каталог задач</Link>
          {taskCount > 0 && <span className="hero-proof">{taskCount} {plural(taskCount, 'задача', 'задачи', 'задач')} в каталоге{stats ? ` · ${stats.proposals} ${plural(stats.proposals, 'отклик', 'отклика', 'откликов')}` : ''}{stats?.teams ? ` · ${stats.teams} ${plural(stats.teams, 'команда', 'команды', 'команд')}` : ''} · запуск одной командой</span>}
        </div>
      </div>
      <div className="hero-visual">
        <div className="hero-score glass" role="img" aria-label={`Пример: готовность задачи ${example.score} из 100 после ответов становится ${example.after}`}>
          <HeroRing from={example.score} to={example.after} />
          <p className="hero-score-title">{example.title}</p>
          <p className="hero-score-note">пример из каталога</p>
        </div>
        <span className="metric-chip glass chip-a"><strong>{example.score} → {example.after}</strong> готовность</span>
        {rankNow ? <span className="metric-chip glass chip-b"><strong>#{rankNow} → #{rankAfter}</strong> в каталоге</span> : null}
        <span className="metric-chip glass chip-c"><strong>{questionCount ? `${questionCount} ${plural(questionCount, 'вопрос', 'вопроса', 'вопросов')}` : '3–4 вопроса'}</strong> · около 3 минут</span>
        <span className="metric-chip glass chip-d"><strong>0</strong> выдуманных фактов</span>
      </div>
    </div></section>

    <ScrollStory example={example} catalog={catalog.tasks} />

    <Reveal className="container landing-section">
      <div className="facts">
        <div><strong>{example.breakdown.length || 7}</strong><p>показателей в открытой формуле рейтинга</p></div>
        <div><strong>0–100</strong><p>балл готовности и место в каталоге</p></div>
        <div><strong>≥3</strong><p>уточняющих вопроса по вашему тексту</p></div>
        <div><strong>2</strong><p>подтверждения: вы выбираете команду, команда принимает проект</p></div>
      </div>
    </Reveal>

    <Reveal className="container landing-section">
      <p className="eyebrow">Сравнение</p>
      <h2>Чем это отличается от привычных способов</h2>
      <div className="compare-wrap"><table className="compare">
        <thead><tr><th scope="col"><span className="visually-hidden">Возможность</span></th><th scope="col" className="is-us">Archibalt</th><th scope="col">Обычная анкета</th><th scope="col">Чат с LLM</th><th scope="col">Excel и WhatsApp</th></tr></thead>
        <tbody>{COMPARE.map(line => <tr key={line.row}><th scope="row">{line.row}</th>{line.cells.map((cell, index) => <td key={index} className={`${index === 0 ? 'is-us ' : ''}${YES.test(cell) ? 'is-yes' : 'is-no'}`}>{cell}</td>)}</tr>)}</tbody>
      </table></div>
    </Reveal>

    <Reveal className="container landing-section landing-faq">
      <p className="eyebrow">Вопросы</p>
      <h2>Коротко о главном</h2>
      <div className="faq">{FAQ.map(item => <details key={item.q}><summary>{item.q}</summary><p>{item.a}</p></details>)}</div>
    </Reveal>

    <Reveal className="container landing-section">
      <div className="final-cta">
        <AnimatedBackground intensity="soft" />
        <div><h2>Первая задача — за три минуты</h2><p>Опишите проблему, ответьте на вопросы и посмотрите, какой балл получит карточка.</p></div>
        <div className="final-cta-actions">
          <ButtonLink variant="primary" size="lg" to="/task/new" onClick={() => setMode('business')}>Описать задачу</ButtonLink>
          <Link className="text-link" to="/catalog">Смотреть каталог</Link>
        </div>
      </div>
      <p className="final-note">Пример «до/после» — демо-задача <Link to="/task/1">«{example.title}»</Link>. Балл «после» показан для случая, когда недостающие поля заполнены.</p>
    </Reveal>
  </div>
}

/** Кольцо готовности в hero: один раз доезжает от «было» до «стало» после загрузки. */
function HeroRing({ from, to }: { from: number; to: number }) {
  const [value, setValue] = useState(from)
  const started = useRef(false)
  useEffect(() => {
    if (started.current) return
    started.current = true
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) { setValue(to); return }
    let frame = 0; let begin = 0
    const tick = (now: number) => {
      if (!begin) begin = now
      const t = Math.min(1, (now - begin) / 1600)
      setValue(Math.round(from + (to - from) * (1 - Math.pow(1 - t, 3))))
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    const delay = window.setTimeout(() => { frame = requestAnimationFrame(tick) }, 700)
    return () => { window.clearTimeout(delay); cancelAnimationFrame(frame); started.current = false }
  }, [from, to])
  const r = 70; const length = 2 * Math.PI * r
  return <span className={`hero-ring${value >= 90 ? ' is-top' : ''}`}>
    <svg viewBox="0 0 160 160" aria-hidden="true"><circle cx="80" cy="80" r={r} className="hero-ring-track" /><circle cx="80" cy="80" r={r} className="hero-ring-arc" strokeDasharray={`${(value / 100) * length} ${length}`} /></svg>
    <span className="hero-ring-value"><strong>{value}</strong><small>из 100</small></span>
  </span>
}

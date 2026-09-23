import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { AnimatedBackground } from '../components/AnimatedBackground'
import { BeforeAfter, useExample } from '../components/BeforeAfter'
import { Reveal } from '../components/Reveal'
import { capitalize, plural, type Mode } from '../fields'
import { Button, ButtonLink } from '../ui'
import { useLoad } from '../useLoad'
import { useStartChat } from './TaskNew'

const STEPS = [
  { title: 'Обсудите проблему', text: 'AI уточнит, что происходит сейчас и какой результат нужен.' },
  { title: 'Подтвердите ТЗ', text: 'Проверьте описание и критерии успеха перед публикацией.' },
  { title: 'Получите предложения', text: 'Опубликованная задача доступна всем студенческим командам.' },
  { title: 'Выберите команду', text: 'Сравните отклики и решите, с кем продолжить работу.' },
]

const FAQ = [
  { q: 'Нужен готовый документ с ТЗ?', a: 'Нет. Начните с проблемы. AI поможет собрать черновик, который вы сможете исправить перед публикацией.' },
  { q: 'Кто выбирает исполнителей?', a: 'Вы. Можно выбрать одну, несколько или ни одной команды.' },
  { q: 'Низкий рейтинг мешает откликнуться?', a: 'Нет. Все опубликованные задачи доступны командам. Балл показывает, какие сведения ещё стоит уточнить.' },
  { q: 'Когда понадобится вход?', a: 'При сборке карточки и публикации задачи. Так вы сможете вернуться к ней и увидеть отклики.' },
]

/** Главная: hero, четыре шага, готовность задачи с примером «было/стало», FAQ и CTA. */
export function Landing({ setMode }: { setMode: (mode: Mode) => void }) {
  const startChat = useStartChat()
  const { data: catalog } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })
  const example = useExample()

  /** Чат открывается сразу, без текста: суть агент спросит первым сообщением (черновик подставится в поле). */
  function discuss() {
    setMode('business')
    startChat()
  }

  const rankNow = example.task?.rank || catalog.tasks.findIndex(task => task.id === 1) + 1 || undefined
  const rankAfter = 1 + catalog.tasks.filter(task => task.id !== 1 && task.score > example.after).length
  const top = catalog.tasks.slice(0, 3)
  const GAP: Record<string, string> = { data: 'данные', expected_result: 'результат', success_criteria: 'критерии', constraints: 'сроки', business_link: 'контакт', users: 'пользователи' }
  const gapList = example.added.map(item => GAP[item.key]).filter(Boolean)
  const gaps = gapList.length > 1 ? `${gapList.slice(0, -1).join(', ')} и ${gapList[gapList.length - 1]}` : gapList[0] || 'данные и сроки'

  return <div className="landing">
    <section className="landing-hero">
      <AnimatedBackground />
      <div className="container landing-hero-grid">
      <div className="landing-hero-copy">
        <h1>От проблемы бизнеса — <em>к решению со студенческой командой.</em></h1>
        <p className="lead">AI поможет разобраться в задаче и составить понятное ТЗ. Студенческие команды смогут предложить решения, а вы выберете, с кем работать.</p>
        <div className="hero-secondary">
          <Button variant="primary" size="lg" onClick={discuss}>Обсудить задачу</Button>
          <ButtonLink variant="secondary" size="lg" to="/catalog" onClick={() => setMode('team')}>Найти задачу</ButtonLink>
        </div>
        <p className="hero-micro">Можно начать без готового ТЗ.</p>
      </div>
      <div className="hero-visual">
        <div className="hero-score glass">
          <HeroRing from={example.score} to={example.after} />
          <dl className="hero-ba">
            <div><dt>Было {example.score}</dt><dd>не указаны {gaps}</dd></div>
            <div className="is-after"><dt>Стало {example.after}</dt><dd>{rankAfter ? `#${rankAfter} в каталоге` : 'выше в каталоге'}</dd></div>
          </dl>
          <p className="hero-score-note">пример: {example.title.toLowerCase()}</p>
          <span className="metric-chip glass chip-a"><strong>{example.score} → {example.after}</strong> готовность</span>
          {rankNow ? <span className="metric-chip glass chip-b"><strong>#{rankNow} → #{rankAfter}</strong> место после ответов</span> : null}
        </div>
        {top.length > 0 && <div className="hero-catalog glass">
          <p className="hero-catalog-head"><span>Открытый каталог</span><Link to="/catalog">Все задачи →</Link></p>
          <ol>{top.map(task => <li key={task.id}>
            <strong className={`hero-catalog-score score-${task.level}`}>{task.score}</strong>
            <Link to={`/task/${task.id}`}>{capitalize(task.fields.title.replace(/^Демо:\s*/, ''))}</Link>
            <span>{task.proposals_count ? `${task.proposals_count} ${plural(task.proposals_count, 'отклик', 'отклика', 'откликов')}` : 'без откликов'}</span>
          </li>)}</ol>
        </div>}
      </div>
    </div></section>

    <Reveal className="container landing-section" id="how">
      <p className="eyebrow">Как это работает</p>
      <h2>От разговора — к выбору команды</h2>
      <ol className="landing-steps">{STEPS.map((step, index) => <li key={step.title}><span className="landing-step-num">{index + 1}</span><h3>{step.title}</h3><p>{step.text}</p></li>)}</ol>
    </Reveal>

    <Reveal className="container landing-section landing-readiness">
      <div className="landing-readiness-copy">
        <p className="eyebrow">Рейтинг готовности</p>
        <h2>Понятно, чего не хватает до старта</h2>
        <p className="lead">Рейтинг 0–100 показывает готовность задачи. Дополняйте сведения и подтверждайте изменения: балл пересчитается, а позиция в каталоге обновится.</p>
        <ul className="landing-notes">
          <li>Откликаться можно на задачи с любым рейтингом.</li>
          <li>Команды получают баллы за этапы, подтверждённые бизнесом.</li>
        </ul>
      </div>
      <div className="landing-readiness-example">
        <p className="eyebrow">Пример улучшения задачи</p>
        <BeforeAfter example={example} />
      </div>
    </Reveal>

    <Reveal className="container landing-section landing-faq" id="faq">
      <p className="eyebrow">Вопросы</p>
      <h2>Коротко о главном</h2>
      <div className="faq">{FAQ.map(item => <details key={item.q}><summary>{item.q}</summary><p>{item.a}</p></details>)}</div>
    </Reveal>

    <Reveal className="container landing-section">
      <div className="final-cta">
        <AnimatedBackground intensity="soft" />
        <div><h2>Начнём с вашей задачи</h2></div>
        <div className="final-cta-actions">
          <Button variant="primary" size="lg" onClick={discuss}>Обсудить задачу</Button>
          <ButtonLink variant="secondary" size="lg" to="/catalog" onClick={() => setMode('team')}>Найти задачу</ButtonLink>
        </div>
      </div>
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

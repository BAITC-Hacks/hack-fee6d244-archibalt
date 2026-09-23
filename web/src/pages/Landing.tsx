import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { AnimatedBackground } from '../components/AnimatedBackground'
import { useExample } from '../components/BeforeAfter'
import { ScrollStory } from '../components/ScrollStory'
import { Reveal } from '../components/Reveal'
import { capitalize, plural, type Mode } from '../fields'
import { Button, ButtonLink } from '../ui'
import { useLoad } from '../useLoad'
import { useStartChat } from './TaskNew'

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

    <ScrollStory example={example} catalog={catalog.tasks} />

    <Reveal className="container landing-section">
      <div className="facts">
        <div><strong><CountUp to={example.breakdown.length || 7} /></strong><p>показателей в открытой формуле рейтинга</p></div>
        <div><strong>0–<CountUp to={100} /></strong><p>балл готовности и место в каталоге</p></div>
        <div><strong>≥<CountUp to={3} /></strong><p>уточняющих вопроса по вашему тексту</p></div>
        <div><strong><CountUp to={2} /></strong><p>подтверждения: вы выбираете команду, команда принимает проект</p></div>
      </div>
    </Reveal>

    <Reveal className="container landing-section" id="compare">
      <p className="eyebrow">Сравнение</p>
      <h2>Чем это отличается от привычных способов</h2>
      <div className="compare-wrap"><table className="compare">
        <thead><tr><th scope="col"><span className="visually-hidden">Возможность</span></th><th scope="col" className="is-us">Archibalt</th><th scope="col">Обычная анкета</th><th scope="col">Чат с LLM</th><th scope="col">Excel и WhatsApp</th></tr></thead>
        <tbody>{COMPARE.map(line => <tr key={line.row}><th scope="row">{line.row}</th>{line.cells.map((cell, index) => <td key={index} className={`${index === 0 ? 'is-us ' : ''}${YES.test(cell) ? 'is-yes' : 'is-no'}`}>{cell}</td>)}</tr>)}</tbody>
      </table></div>
    </Reveal>

    <Reveal className="container landing-section landing-faq" id="faq">
      <p className="eyebrow">Вопросы</p>
      <h2>Коротко о главном</h2>
      <div className="faq">{FAQ.map(item => <details key={item.q}><summary>{item.q}</summary><p>{item.a}</p></details>)}</div>
    </Reveal>

    <Reveal className="container landing-section">
      <div className="final-cta">
        <AnimatedBackground intensity="soft" />
        <div><h2>Первая задача — за три минуты</h2><p>Опишите проблему, ответьте на вопросы и посмотрите, какой балл получит карточка.</p></div>
        <div className="final-cta-actions">
          <ButtonLink variant="primary" size="lg" to="/task/new" onClick={() => setMode('business')}>Обсудить задачу</ButtonLink>
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

/** Число считает от 0 до значения, когда впервые попадает на экран (один раз, ~1 с, easing out). */
function CountUp({ to, duration = 1000 }: { to: number; duration?: number }) {
  const node = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    const el = node.current
    if (!el) return
    if (typeof IntersectionObserver === 'undefined' || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) { el.textContent = String(to); return }
    el.textContent = '0'
    let frame = 0
    const observer = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return
      observer.disconnect()
      const begin = performance.now()
      const tick = (now: number) => {
        const t = Math.min(1, (now - begin) / duration)
        el.textContent = String(Math.round(to * (1 - Math.pow(1 - t, 3))))
        if (t < 1) frame = requestAnimationFrame(tick)
      }
      frame = requestAnimationFrame(tick)
    }, { rootMargin: '0px 0px -10% 0px' })
    observer.observe(el)
    return () => { observer.disconnect(); cancelAnimationFrame(frame) }
  }, [to, duration])
  return <span ref={node} className="count-up">{to}</span>
}

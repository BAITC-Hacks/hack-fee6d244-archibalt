import { useEffect, useRef, useState } from 'react'
import type { Breakdown, Task } from '../api'
import { capitalize } from '../fields'
import { Badge } from '../ui'
import { levelOf, type useExample } from './BeforeAfter'

type Example = ReturnType<typeof useExample>

const STEPS = [
  { title: 'Черновик', text: 'Заявитель пишет как есть, одной-двумя фразами. Такой запрос команде пока непонятен: нет данных, результата и критериев. Готовность — 30 из 100.' },
  { title: 'AI спрашивает', text: 'AI находит пробелы и задаёт вопросы простыми словами. Отвечать можно коротко, любой вопрос можно пропустить: пустое поле AI не заполнит за вас.' },
  { title: 'Формула объясняет', text: 'Семь показателей с весами. Каждый ответ закрывает конкретный показатель, и видно, сколько баллов он добавил.' },
  { title: 'Место в каталоге', text: 'Чем полнее задача, тем выше она в общем каталоге. Команды видят её первой и откликаются, а решение остаётся за заявителем.' },
]

/** Пример диалога для истории: ответы закрывают данные, результат со сроком и критерии. */
const DIALOG = [
  { q: 'Какие данные о посетителях у вас есть?', a: 'Анкеты после событий за год', keys: ['data'] },
  { q: 'Что команда должна передать в конце и к какому сроку?', a: 'Календарь тем и часов, пилот за 4 недели', keys: ['expected_result', 'constraints'] },
  { q: 'Как поймёте, что стало лучше?', a: 'Посещаемость событий выросла на 20%', keys: ['success_criteria'] },
]

const clamp = (value: number, min = 0, max = 1) => Math.min(max, Math.max(min, value))
const ease = (t: number) => 1 - Math.pow(1 - t, 3)
const ROW = 56

/** Прогресс прокрутки секции 0–1: scroll + requestAnimationFrame, без таймеров. */
function useScrollProgress(node: React.RefObject<HTMLElement>) {
  const [progress, setProgress] = useState(0)
  useEffect(() => {
    let frame = 0
    const measure = () => {
      frame = 0
      const el = node.current; if (!el) return
      const rect = el.getBoundingClientRect()
      const total = el.offsetHeight - window.innerHeight
      const next = total > 0 ? clamp(-rect.top / total) : 0
      setProgress(current => (Math.abs(current - next) > 0.0005 ? next : current))
    }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(measure) }
    measure()
    window.addEventListener('scroll', schedule, { passive: true }); window.addEventListener('resize', schedule)
    return () => { window.removeEventListener('scroll', schedule); window.removeEventListener('resize', schedule); cancelAnimationFrame(frame) }
  }, [node])
  return progress
}

/** «Как это работает»: история одной задачи. Слева четыре шага, справа липкий визуал, который меняется от прокрутки. */
export function ScrollStory({ example, catalog }: { example: Example; catalog: Task[] }) {
  const section = useRef<HTMLElement>(null)
  const raw = useScrollProgress(section)
  const reduced = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  const scaled = clamp(raw * STEPS.length, 0, STEPS.length - 0.0001)
  const step = Math.floor(scaled)
  const local = reduced ? 1 : clamp((scaled - step) / 0.8)

  const start = example.score
  const final = example.after
  const breakdown = example.breakdown
  const addedKeys = new Set(example.added.map(item => item.key))
  const fill = step < 2 ? 0 : step === 2 ? ease(local) : 1
  const score = Math.round(start + (final - start) * fill)
  const level = levelOf(score)

  return <>
  <div className="container story-intro" id="how"><p className="eyebrow">Как это работает</p><h2>Одна задача: от черновика до роста в каталоге</h2><p className="lead">Пример: ответы добавляют детали, повышают готовность и помогают команде понять задачу.</p></div>
  <section className="story" ref={section} aria-label="Как это работает: история одной задачи">
    <div className="container story-grid">
      <div className="story-steps">
        {STEPS.map((item, index) => <div key={item.title} className={`story-step${index === step ? ' is-active' : ''}`}>
          <div className="story-step-body">
            <span className="story-step-num">{index + 1}</span>
            <h3>{item.title}</h3>
            <p>{item.text}</p>
          </div>
        </div>)}
      </div>
      <div className="story-sticky">
        <div className="story-stage" data-step={step}>
          <DraftCard example={example} score={score} level={level} compact={step >= 1} hidden={step === 3} />
          <div className={`story-layer story-dialog${step === 1 ? ' is-on' : ''}`} aria-hidden={step !== 1}>
            {DIALOG.map((item, index) => {
              const t = step === 1 ? clamp((local - index / DIALOG.length) * DIALOG.length) : step > 1 ? 1 : 0
              const typed = reduced ? item.q : item.q.slice(0, Math.round(item.q.length * clamp(t / 0.75)))
              return <div key={item.q} className={`bubble-pair${t > 0 ? ' is-on' : ''}`}>
                <p className="bubble">{typed}<span className="caret" aria-hidden="true" style={{ opacity: t > 0 && t < 0.75 ? 1 : 0 }} /></p>
                <span className={`answer-chip${t >= 0.8 ? ' is-on' : ''}`}>{item.a}</span>
              </div>
            })}
          </div>
          <div className={`story-layer story-formula${step === 2 ? ' is-on' : ''}`} aria-hidden={step !== 2}>
            <ul>{breakdown.map((item: Breakdown, index) => {
              const gain = addedKeys.has(item.key) ? item.weight - item.earned : 0
              const rowFill = step < 2 ? 0 : step > 2 ? 1 : clamp(fill * breakdown.length - index * 0.6)
              const earned = item.earned + gain * rowFill
              return <li key={item.key}>
                <span>{item.label}</span>
                <span className="story-bar"><span style={{ transform: `scaleX(${item.weight ? earned / item.weight : 0})` }} /></span>
                <span className={gain ? 'story-gain' : 'story-flat'}>{gain ? `+${gain}` : `${item.earned}/${item.weight}`}</span>
              </li>
            })}</ul>
          </div>
          <MiniCatalog on={step === 3} progress={step === 3 ? ease(local) : step > 3 ? 1 : 0} example={example} catalog={catalog} />
        </div>
      </div>
    </div>
  </section>
  </>
}

function DraftCard({ example, score, level, compact, hidden }: { example: Example; score: number; level: ReturnType<typeof levelOf>; compact: boolean; hidden: boolean }) {
  return <div className={`story-card glass${compact ? ' is-compact' : ''}${hidden ? ' is-hidden' : ''}`} aria-hidden={hidden}>
    <div className="story-card-head">
      <span className="story-card-title">{example.title}</span>
      <span className="story-score"><strong>{score}</strong> из 100</span>
    </div>
    <Badge kind={level} />
    <p className="story-draft">«{example.draft}»</p>
  </div>
}

function MiniCatalog({ on, progress, example, catalog }: { on: boolean; progress: number; example: Example; catalog: Task[] }) {
  const others = catalog.filter(task => task.id !== 1).slice(0, 4).map(task => ({ id: task.id, title: capitalize(task.fields.title.replace(/^Демо:\s*/, '')), score: task.score }))
  const rows = others.length ? others : [{ id: 4, title: 'Поиск учебных материалов', score: 90 }, { id: 3, title: 'Очередь заявок мастерской', score: 70 }, { id: 2, title: 'Планирование доставки', score: 60 }, { id: 5, title: 'Распределение заявок на вторсырьё', score: 42 }]
  const from = rows.length; const target = rows.filter(row => row.score > example.after).length
  const ourY = (from - (from - target) * progress) * ROW
  const place = 1 + rows.filter((_, index) => index * ROW + ROW / 2 < ourY).length
  return <div className={`story-layer story-catalog${on ? ' is-on' : ''}`} aria-hidden={!on}>
    <span className="rank-chip glass">#{from + 1} → #{place}</span>
    <ol className="mini-catalog" style={{ height: (rows.length + 1) * ROW }}>
      {rows.map((row, index) => <li key={row.id} style={{ transform: `translateY(${index * ROW + (index * ROW + ROW / 2 >= ourY ? ROW : 0)}px)` }}>
        <span className="mini-rank">#{index + 1 + (index * ROW + ROW / 2 >= ourY ? 1 : 0)}</span><span className="mini-title">{row.title}</span><strong>{row.score}</strong>
      </li>)}
      <li className="is-ours" style={{ transform: `translateY(${ourY}px)`, transition: 'none' }}>
        <span className="mini-rank">#{place}</span><span className="mini-title">{example.title}</span><strong>{example.after}</strong>
      </li>
    </ol>
  </div>
}

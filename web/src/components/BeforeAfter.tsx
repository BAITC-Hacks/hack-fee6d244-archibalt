import { Link } from 'react-router-dom'
import type { Missing, Task } from '../api'
import { Badge, type LevelKind } from '../ui'
import { useLoad } from '../useLoad'

const EXAMPLE_ID = 1
const TARGET = 90
/** Запасные данные seed-задачи 1, если API не ответил: блок не должен пропадать с первого экрана. */
const FALLBACK = {
  title: 'Планирование событий в районных библиотеках',
  draft: 'Мы проводим события в нескольких районных библиотеках, но плохо понимаем, какие темы и часы удобны посетителям. Хотим что-то для планирования программы.',
  score: 30,
  missing: [
    { key: 'data', label: 'Данные и материалы', gain: 20, hint: '' },
    { key: 'expected_result', label: 'Ожидаемый результат', gain: 15, hint: '' },
    { key: 'success_criteria', label: 'Критерии успеха', gain: 15, hint: '' },
    { key: 'constraints', label: 'Ограничения', gain: 10, hint: '' },
  ] as Missing[],
}

const levelOf = (score: number): LevelKind => (score >= 90 ? 'priority' : score >= 70 ? 'ready' : score >= 40 ? 'working' : 'draft')

/** «До и после» одной seed-задачи: сырой запрос с низким баллом → та же задача после ответов на вопросы. */
export function BeforeAfter() {
  const { data: task } = useLoad<Task | null>(`/tasks/${EXAMPLE_ID}`, null)
  const score = task?.score ?? FALLBACK.score
  const draft = task?.draft_text || FALLBACK.draft
  const title = (task?.fields.title || FALLBACK.title).replace(/^Демо:\s*/, '').replace(/^./, c => c.toUpperCase())
  const added: Missing[] = []
  let after = score
  for (const item of task?.missing ?? FALLBACK.missing) {
    if (after >= TARGET) break
    added.push(item); after += item.gain
  }
  return <aside className="before-after" aria-label="Пример: одна задача до и после уточнения">
    <p className="before-after-caption">Пример из каталога</p>
    <div className="ba-card ba-before">
      <div className="ba-head"><span className="ba-tag">Было</span><span className="ba-score"><strong>{score}</strong> из 100</span></div>
      <Badge kind={levelOf(score)} />
      <p className="ba-draft">«{draft}»</p>
    </div>
    <div className="ba-arrow" aria-hidden="true"><span>ответы на вопросы AI</span></div>
    <div className="ba-card ba-after">
      <div className="ba-head"><span className="ba-tag">Стало</span><span className="ba-score"><strong>{after}</strong> из 100</span></div>
      <Badge kind={levelOf(after)} />
      <p className="ba-title">{title}</p>
      <ul className="ba-added">{added.map(item => <li key={item.key}><span>{item.label}</span><span>+{item.gain}</span></li>)}</ul>
    </div>
    <p className="ba-note">Так выглядит результат, если ответить на вопросы. <Link to={`/task/${EXAMPLE_ID}`}>Открыть задачу</Link></p>
  </aside>
}

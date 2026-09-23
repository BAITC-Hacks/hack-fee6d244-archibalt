import type { Task } from '../api'
import { Badge, Meter } from '../ui'

export function RatingPanel({ task, preliminary = false }: { task: Task; preliminary?: boolean }) {
  return <aside className="rating-panel">
    <p className="eyebrow">{preliminary ? 'Предварительная готовность' : 'Готовность задачи'}</p>
    <Meter value={task.score} label="Рейтинг готовности" />
    <div className="rating-level"><Badge kind={task.level} />{preliminary && <span>предварительно</span>}</div>
    <p className="rating-explain">{preliminary ? 'Сохраните дополнения и подтвердите карточку, чтобы балл учитывался в каталоге.' : 'Рейтинг состоит из подтверждённых сведений.'}</p>
    {task.missing.length > 0 && <div className="rating-next"><strong>Как поднять оценку на +{task.missing[0].gain}</strong><span>{task.missing[0].hint}</span></div>}
    <h2>Из чего состоит рейтинг</h2>
    <ul className="breakdown">{task.breakdown.map(item => <li key={item.key}><div><strong>{item.label}</strong><small>{item.reason}</small></div><span>{item.earned}/{item.weight}</span></li>)}</ul>
    {task.missing.length > 0 && <><h2>Что улучшить дальше</h2><ul className="missing-list">{task.missing.map(item => <li key={item.key}><span>+{item.gain}</span><div><strong>{item.label}</strong><small>{item.hint}</small></div></li>)}</ul></>}
    <p className="rating-footnote">В каталоге задачи сортируются по рейтингу. Задача с рейтингом 0–39 тоже видна, с пометкой «требует уточнения».</p>
  </aside>
}

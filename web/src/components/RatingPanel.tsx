import type { Task } from '../api'
import { Badge, Meter, levelLabels, type LevelKind } from '../ui'

const NEXT: Record<LevelKind, LevelKind | null> = { draft: 'working', working: 'ready', ready: 'priority', priority: null }

/** Строка прогноза: «Сейчас 55 · будете #4 из 7 · до уровня «готовая» не хватает 15». */
export function Forecast({ task }: { task: Task }) {
  const parts = [`Сейчас ${task.score}`]
  if (task.confirmed && task.rank) parts.push(`место #${task.rank} из ${task.catalog_size ?? task.rank}`)
  else if (task.rank_if_confirmed) parts.push(`будете #${task.rank_if_confirmed} из ${(task.catalog_size ?? 0) + 1}`)
  const next = NEXT[task.level]
  if (next && task.next_level_gain) parts.push(`до уровня «${levelLabels[next]}» не хватает ${task.next_level_gain}`)
  return <p className="forecast">{parts.join(' · ')}</p>
}

export function RatingPanel({ task, preliminary = false, delta }: { task: Task; preliminary?: boolean; delta?: { from: number; to: number } | null }) {
  return <aside className="rating-panel">
    <p className="eyebrow">{preliminary ? 'Предварительная готовность' : 'Готовность задачи'}</p>
    <Meter value={task.score} label="Рейтинг готовности" />
    <div className="rating-level"><Badge kind={task.level} />{preliminary && <span>балл видите только вы до подтверждения</span>}</div>
    {delta && delta.to !== delta.from && <p className={`rating-delta${delta.to > delta.from ? ' is-up' : ''}`}>{delta.from} → {delta.to} ({delta.to > delta.from ? '+' : ''}{delta.to - delta.from} за данные)</p>}
    <Forecast task={task} />
    <p className="rating-explain">{preliminary ? 'Сохраните дополнения и подтвердите карточку, чтобы балл учитывался в каталоге.' : 'Рейтинг состоит из подтверждённых сведений.'}</p>
    {task.missing.length > 0 && <div className="rating-next"><strong>Как поднять оценку на +{task.missing[0].gain}</strong><span>{task.missing[0].hint}</span></div>}
    <h2>Из чего состоит рейтинг</h2>
    <ul className="breakdown">{task.breakdown.map(item => <li key={item.key}><div><strong>{item.label}</strong><small>{item.reason}</small></div><span>{item.earned}/{item.weight}</span></li>)}</ul>
    {task.missing.length > 0 && <><h2>Что улучшить дальше</h2><ul className="missing-list">{task.missing.map(item => <li key={item.key}><span>+{item.gain}</span><div><strong>{item.label}</strong><small>{item.hint}</small></div></li>)}</ul></>}
    <p className="rating-footnote">В каталоге задачи сортируются по рейтингу. Задача с рейтингом 0–39 тоже видна, с пометкой «требует уточнения».</p>
  </aside>
}

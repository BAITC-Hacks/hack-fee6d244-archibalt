import { Link, useSearchParams } from 'react-router-dom'
import type { CatalogResponse, Task } from '../api'
import { capitalize, plural, type Mode } from '../fields'
import { Alert, Badge, ButtonLink, Chips, EmptyState, Spinner, levelLabels, type LevelKind } from '../ui'
import { useLoad } from '../useLoad'

const ALL = 'Все'
const HAVE: Record<string, string> = { context_need: 'контекст', data: 'данные', expected_result: 'результат', success_criteria: 'критерии', constraints: 'ограничения', users: 'пользователи', business_link: 'контакт' }
const LEVELS: LevelKind[] = ['draft', 'working', 'ready', 'priority']
const levelName = (level: LevelKind) => capitalize(levelLabels[level])

/** Каталог: сетка карточек, фильтры-чипы; при смене фильтра сетка остаётся на месте и тихо обновляется. */
export function Catalog({ mode, setMode }: { mode: Mode; setMode: (mode: Mode) => void }) {
  const [params, setParams] = useSearchParams()
  const industry = params.get('industry') || ''; const level = params.get('level') || ''
  const query = new URLSearchParams(); if (industry) query.set('industry', industry); if (level) query.set('level', level)
  const { data, loading, refreshing, error } = useLoad<CatalogResponse>(`/tasks${query.size ? `?${query}` : ''}`, { tasks: [], industries: [], levels: [] }, { keepPrevious: true })
  const apply = (next: { industry: string; level: string }) => setParams({ ...(next.industry ? { industry: next.industry } : {}), ...(next.level ? { level: next.level } : {}) }, { replace: true, preventScrollReset: true })
  const filtered = Boolean(industry || level)
  return <section className="container catalog-page">
    <div className="catalog-head">
      <div><h1>Каталог задач</h1><p className="lead">Задачи отсортированы по готовности: чем полнее описание, тем выше место. Задачи с пометкой «требует уточнения» тоже принимают отклики.</p></div>
      {!loading && <p className="catalog-pulse">{pulse(data)}</p>}
    </div>
    <div className="catalog-filters" role="group" aria-label="Фильтры каталога">
      <div className="catalog-filter"><span>Тема</span><Chips label="Тема" options={[ALL, ...data.industries]} value={industry || ALL} onPick={value => apply({ industry: value === ALL ? '' : value, level })} /></div>
      <div className="catalog-filter"><span>Готовность</span><Chips label="Готовность" options={[ALL, ...LEVELS.map(levelName)]} value={level ? levelName(level as LevelKind) : ALL} onPick={value => apply({ industry, level: LEVELS.find(item => levelName(item) === value) ?? '' })} /></div>
      <span className={`catalog-refresh${refreshing ? ' is-on' : ''}`} role="status">{refreshing && <><Spinner size="sm" /> Обновляем</>}</span>
    </div>
    <div className={`catalog-results${refreshing ? ' is-refreshing' : ''}`} aria-busy={loading || refreshing}>
      {error && <Alert tone="error" className="catalog-error">{data.tasks.length ? 'Не удалось обновить список, показываем прежний. Попробуйте ещё раз через минуту.' : 'Не удалось загрузить каталог. Проверьте подключение и обновите страницу.'}</Alert>}
      {loading ? <div className="task-grid" aria-hidden="true">{[0, 1, 2].map(n => <div className="task-card task-card-skeleton" key={n}><span /><span /><span /></div>)}</div>
        : data.tasks.length ? <div className="task-grid">{data.tasks.map(task => <TaskCard key={task.id} task={task} mode={mode} onOpen={() => mode === 'team' && setMode('team')} />)}</div>
        : !error && <EmptyState title={filtered ? 'По этим фильтрам задач нет' : 'В каталоге пока нет задач'} action={filtered
          ? <ButtonLink to="/catalog" replace>Показать все задачи</ButtonLink>
          : <ButtonLink variant="primary" to="/task/new" onClick={() => setMode('business')}>Описать задачу</ButtonLink>}>
          {filtered ? 'Выберите другую тему или уровень готовности.' : 'Опишите первую задачу: после публикации её увидят все команды.'}
        </EmptyState>}
    </div>
  </section>
}

function TaskCard({ task, mode, onOpen }: { task: Task; mode: Mode; onOpen: () => void }) {
  const have = task.breakdown.filter(item => item.weight > 0 && item.earned === item.weight).map(item => HAVE[item.key]).filter(Boolean)
  const proposals = (task as Task & { proposals_count?: number }).proposals_count
  return <article className={`task-card task-card-${task.level}`}>
    <div className="task-card-top">
      <ScoreMedal score={task.score} level={task.level} />
      <div className="task-card-place">{task.rank ? <strong>#{task.rank}</strong> : null}<Badge kind={task.level} /></div>
    </div>
    <h2><Link to={`/task/${task.id}`} onClick={onOpen}>{task.fields.title || 'Задача без названия'}</Link></h2>
    <p className="task-card-industry">{task.industry || 'Без темы'}</p>
    <p className="task-card-have"><span>Есть:</span> {have.length ? have.join(' · ') : '—'}</p>
    <div className="task-card-foot">
      <span>{proposals !== undefined ? `${proposals} ${plural(proposals, 'отклик', 'отклика', 'откликов')}` : ''}</span>
      <ButtonLink size="sm" variant={mode === 'team' ? 'primary' : 'secondary'} to={`/task/${task.id}${mode === 'team' ? '#respond' : ''}`} onClick={onOpen}>{mode === 'team' ? 'Откликнуться' : 'Открыть'}</ButtonLink>
    </div>
  </article>
}

/** Кружок-медаль: дуга по баллу 0–100 и число в центре. */
function ScoreMedal({ score, level }: { score: number; level: string }) {
  const r = 26; const length = 2 * Math.PI * r
  return <span className={`score-medal score-medal-${level}`} role="img" aria-label={`Готовность ${score} из 100`}>
    <svg viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="32" r={r} className="score-medal-track" /><circle cx="32" cy="32" r={r} className="score-medal-arc" strokeDasharray={`${(score / 100) * length} ${length}`} /></svg>
    <strong>{score}</strong>
  </span>
}

function pulse(data: CatalogResponse) {
  const tasks = data.stats?.tasks ?? data.tasks.length
  const parts = [`${tasks} ${plural(tasks, 'задача', 'задачи', 'задач')}`]
  if (data.stats) parts.push(`${data.stats.proposals} ${plural(data.stats.proposals, 'отклик', 'отклика', 'откликов')}`)
  if (data.stats?.teams) parts.push(`${data.stats.teams} ${plural(data.stats.teams, 'команда', 'команды', 'команд')}`)
  return parts.join(' · ')
}

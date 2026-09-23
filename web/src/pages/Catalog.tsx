import { Link, useSearchParams } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { capitalize, plural, type Mode } from '../fields'
import { Alert, Badge, Button, EmptyState, Field, Select, Spinner, levelLabels, type LevelKind } from '../ui'
import { useLoad } from '../useLoad'

export function Catalog(_: { setMode: (mode: Mode) => void }) {
  const [params, setParams] = useSearchParams()
  const industry = params.get('industry') || ''; const level = params.get('level') || ''
  const query = new URLSearchParams(); if (industry) query.set('industry', industry); if (level) query.set('level', level)
  const { data, loading, refreshing, error } = useLoad<CatalogResponse>(`/tasks${query.size ? `?${query}` : ''}`, { tasks: [], industries: [], levels: [] }, { keepPrevious: true })
  const apply = (next: { industry: string; level: string }) => setParams({ ...(next.industry ? { industry: next.industry } : {}), ...(next.level ? { level: next.level } : {}) })
  const industryOptions = [{ value: '', label: 'Все темы' }, ...data.industries.map(value => ({ value, label: value }))]
  const levelOptions = [{ value: '', label: 'Любая' }, ...data.levels.map(item => ({ value: item.key, label: capitalize(levelLabels[item.key as LevelKind] ?? item.label) }))]
  return <>
    <section id="catalog" className="catalog-section container">
      <div className="section-heading">
        <div><p className="eyebrow">Все опубликованные задачи</p><h2>Выберите задачу</h2><p>Задачи с пометкой «требует уточнения» тоже открыты для просмотра и отклика.</p></div>
        <span className="count-pill">{pulse(data)}</span>
      </div>
      <div className="filters" role="group" aria-label="Фильтры каталога">
        <Field label="Тема"><Select value={industry} options={industryOptions} onChange={value => apply({ industry: value, level })} /></Field>
        <Field label="Готовность"><Select value={level} options={levelOptions} onChange={value => apply({ industry, level: value })} /></Field>
        {(industry || level) && <Button variant="ghost" onClick={() => setParams({})}>Сбросить фильтры</Button>}
      </div>
      <div className={`catalog-results${refreshing ? ' is-refreshing' : ''}`} aria-busy={loading || refreshing}>
        {refreshing && <span className="catalog-refresh" role="status"><Spinner size="sm" /> Обновляем список…</span>}
        {error && <Alert tone="error" className="catalog-error">{error}</Alert>}
        {loading ? <div className="task-list" aria-hidden="true">{[0, 1, 2].map(n => <div className="task-row task-skeleton" key={n}><div className="score-tile" /><div className="task-summary"><span /><span /><span /></div></div>)}</div>
          : data.tasks.length ? <div className="task-list">{data.tasks.map((task, index) => <article className="task-row" key={task.id}>
            <div className={`score-tile score-${task.level}`}><strong>{task.score}</strong><span>из 100</span></div>
            <div className="task-summary">
              <div className="task-meta"><span>Место в списке #{index + 1}</span><span>{task.industry || 'Без темы'}</span><Badge kind={task.level} /></div>
              <h3><Link to={`/task/${task.id}`}>{task.fields.title || 'Задача без названия'}</Link></h3>
              <p>{task.fields.need || task.draft_text}</p>
              <div className="task-row-footer"><span>Рейтинг выше — позиция выше</span><Link to={`/task/${task.id}`}>Открыть задачу →</Link></div>
            </div>
          </article>)}</div>
          : !error && <EmptyState title="Задач по этим фильтрам нет" action={<Button onClick={() => setParams({})}>Показать все задачи</Button>}>Попробуйте другую тему или уровень готовности.</EmptyState>}
      </div>
    </section>
  </>
}

function pulse(data: CatalogResponse) {
  const tasks = data.stats?.tasks ?? data.tasks.length
  const parts = [`${tasks} ${plural(tasks, 'задача', 'задачи', 'задач')}`]
  if (data.stats?.proposals) parts.push(`${data.stats.proposals} ${plural(data.stats.proposals, 'отклик', 'отклика', 'откликов')}`)
  return parts.join(' · ')
}

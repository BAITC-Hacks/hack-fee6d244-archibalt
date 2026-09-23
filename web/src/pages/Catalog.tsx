import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import type { CatalogResponse, Task } from '../api'
import { capitalize, plural, type Mode } from '../fields'
import { useSession } from '../session'
import { Alert, Badge, Button, ButtonLink, EmptyState, Input, Select, levelLabels, type LevelKind } from '../ui'
import { CloseIcon } from '../ui/icons'
import { useLoad } from '../useLoad'
import { selectTasks } from './catalog-model'
import './Catalog.css'

const LEVELS: LevelKind[] = ['priority', 'ready', 'working', 'draft']
const RANGES = { priority: '90–100', ready: '70–89', working: '40–69', draft: '0–39' }
const SORTS = [{ value: '', label: 'По готовности' }, { value: 'newest', label: 'Сначала новые' }, { value: 'proposals', label: 'Меньше откликов' }]

export function Catalog({ mode, setMode }: { mode: Mode; setMode: (mode: Mode) => void }) {
  const [params, setParams] = useSearchParams()
  const [filtersOpen, setFiltersOpen] = useState(() => window.matchMedia('(min-width: 801px)').matches)
  const { isOwner } = useSession()
  const { data, loading, error } = useLoad<CatalogResponse>('/tasks', { tasks: [], industries: [], levels: [] })
  const industry = params.get('industry') || '', level = params.get('level') || '', search = params.get('q') || '', sort = params.get('sort') || ''
  const tasks = selectTasks(data.tasks, { industry, level, search, sort })
  const filtered = Boolean(industry || level || search)
  const apply = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value); else next.delete(key)
    setParams(next, { replace: true, preventScrollReset: true })
  }
  const reset = () => setParams(sort ? { sort } : {}, { replace: true, preventScrollReset: true })
  return <section className="container project-catalog">
    <header className="catalog-intro">
      <div><p className="catalog-kicker"><span /> КАТАЛОГ БИЗНЕС-ЗАДАЧ</p><h1>Ваш следующий<br /><em>реальный проект.</em></h1><p className="catalog-description">Найдите задачу по интересам, предложите решение<br className="catalog-desktop-break" /> и создайте полезный продукт вместе с бизнесом.</p></div>
      <div className="catalog-overview" aria-label="Статистика каталога">
        <div><strong>{loading || error ? '—' : data.tasks.length}</strong><span>задач в каталоге</span></div>
        <div><strong>{loading || error ? '—' : data.tasks.filter(task => task.score >= 70).length}</strong><span>с готовностью 70+</span></div>
        <div><strong>{loading || error ? '—' : data.stats?.teams ?? '—'}</strong><span>команд на платформе</span></div>
      </div>
    </header>
    <div className="catalog-searchbar" role="search">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></svg>
      <Input type="search" aria-label="Поиск задач" placeholder="Что вам интересно? Например, доставка или образование" value={search} onChange={event => apply('q', event.target.value)} />
      {search && <Button variant="ghost" aria-label="Очистить поиск" onClick={() => apply('q', '')}><CloseIcon /></Button>}
      <span className="catalog-search-hint">Поиск по задачам</span>
    </div>
    <div className="catalog-workspace">
      <aside className="catalog-sidebar" aria-label="Фильтры каталога">
        <details className="catalog-filter-panel" open={filtersOpen} onToggle={event => setFiltersOpen(event.currentTarget.open)}>
          <summary>Фильтры <span aria-hidden="true">⌄</span></summary>
          <div className="catalog-filter-body">
            <fieldset><legend>Направление</legend>
              {['', ...data.industries].map(value => <label className="catalog-filter-option" key={value}><input type="radio" name="catalog-industry" checked={industry === value} onChange={() => apply('industry', value)} /><span>{value || 'Все направления'}</span><small>{value ? data.tasks.filter(task => task.industry === value).length : data.tasks.length}</small></label>)}
            </fieldset>
            <fieldset><legend>Готовность описания</legend>
              <label className="catalog-filter-option"><input type="radio" name="catalog-level" checked={!level} onChange={() => apply('level', '')} /><span>Любая готовность</span></label>
              {LEVELS.map(value => <label className="catalog-filter-option" key={value}><input type="radio" name="catalog-level" checked={level === value} onChange={() => apply('level', value)} /><span>{capitalize(levelLabels[value])}<small className="catalog-filter-range">{RANGES[value]} баллов</small></span></label>)}
            </fieldset>
            {filtered && <Button variant="ghost" className="catalog-reset" onClick={reset}>Сбросить фильтры</Button>}
          </div>
        </details>
        <div className="catalog-guide"><span className="catalog-guide-mark" aria-hidden="true">↗</span><h2>Что значит рейтинг?</h2><p>Чем больше бизнес рассказал о задаче, тем выше её балл и место в каталоге.</p><strong>Откликнуться можно на любую задачу — даже с низким баллом.</strong></div>
        <Link className="catalog-business-link" to="/task/new" onClick={() => setMode('business')}>Есть задача для команды?<span>Опубликовать задачу →</span></Link>
      </aside>
      <div className="catalog-main">
        <div className="catalog-toolbar"><h2 role="status" aria-live="polite">{loading ? 'Загружаем задачи…' : error ? 'Каталог недоступен' : <>{filtered ? 'Найдено' : 'Все задачи'} <span>{tasks.length}</span></>}</h2><Select aria-label="Сортировка задач" value={SORTS.some(item => item.value === sort) ? sort : ''} options={SORTS} onChange={value => apply('sort', value)} /></div>
        {filtered && <div className="catalog-applied" aria-label="Выбранные фильтры">
          {industry && <button onClick={() => apply('industry', '')} aria-label={`Убрать направление ${industry}`}>{industry}<CloseIcon /></button>}
          {level && <button onClick={() => apply('level', '')} aria-label="Убрать фильтр готовности">{levelLabels[level as LevelKind] || level}<CloseIcon /></button>}
          {search && <button onClick={() => apply('q', '')} aria-label="Убрать поисковый запрос">«{search}»<CloseIcon /></button>}
          <Button variant="ghost" size="sm" onClick={reset}>Сбросить всё</Button>
        </div>}
        <div className="catalog-projects" aria-busy={loading}>
          {error ? <Alert tone="error">Не удалось загрузить каталог. <button className="catalog-retry" onClick={() => window.location.reload()}>Попробовать ещё раз</button></Alert>
            : loading ? <div className="project-grid" aria-hidden="true">{[0, 1, 2, 3].map(n => <div className="project-skeleton" key={n}><span /><span /><span /><span /></div>)}</div>
            : tasks.length ? <div className="project-grid">{tasks.map(task => <TaskCard key={task.id} task={task} respond={mode === 'team' && !isOwner(task.id)} />)}</div>
            : <EmptyState title={filtered ? 'Пока нет подходящих задач' : 'Здесь появятся первые проекты'} action={filtered ? <Button onClick={reset}>Показать все задачи</Button> : <ButtonLink variant="primary" to="/task/new" onClick={() => setMode('business')}>Описать задачу</ButtonLink>}>{filtered ? 'Попробуйте другой запрос или уберите часть фильтров.' : 'Опишите бизнес-задачу, чтобы команды могли предложить решение.'}</EmptyState>}
        </div>
        {!loading && !error && tasks.length > 0 && <p className="catalog-end">{tasks.length} {plural(tasks.length, 'задача', 'задачи', 'задач')}{filtered ? ` из ${data.tasks.length}` : ' в каталоге'}. {filtered && <button onClick={reset}>Показать все</button>}</p>}
      </div>
    </div>
  </section>
}

function TaskCard({ task, respond }: { task: Task; respond: boolean }) {
  return <article className={`project-card project-card-${task.level}`}>
    <div className="project-card-meta"><span className="project-industry">{task.industry || 'Без направления'}</span>{task.rank && <span className="project-rank" title="Место по готовности среди всех задач">№{task.rank} в каталоге</span>}</div>
    <h3><Link to={`/task/${task.id}`}>{task.fields.title || 'Задача без названия'}</Link></h3>
    <p className="project-need">{task.fields.need || task.fields.context || 'Бизнесу нужна помощь в уточнении задачи. Откройте карточку, чтобы узнать больше.'}</p>
    <div className="project-outcome"><span>ОЖИДАЕМЫЙ РЕЗУЛЬТАТ</span><p>{task.fields.expected_result || 'Обсудите результат вместе с бизнесом'}</p></div>
    <div className="project-readiness"><div><span>Готовность описания</span><strong>{task.score}<small> / 100</small></strong></div><meter min="0" max="100" value={task.score} aria-label="Готовность описания">{task.score} из 100</meter><Badge kind={task.level} /></div>
    <div className="project-card-footer"><span>{task.proposals_count === undefined ? 'Отклики открыты' : task.proposals_count === 0 ? 'Станьте первыми' : `${task.proposals_count} ${plural(task.proposals_count, 'отклик', 'отклика', 'откликов')}`}</span><ButtonLink to={`/task/${task.id}${respond ? '#respond' : ''}`} variant={respond ? 'primary' : 'secondary'} aria-label={`${respond ? 'Откликнуться' : 'Подробнее'}: ${task.fields.title || 'Задача без названия'}`}>{respond ? 'Откликнуться' : 'Подробнее'} <span aria-hidden="true">↗</span></ButtonLink></div>
  </article>
}

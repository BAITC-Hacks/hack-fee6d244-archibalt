import { Link, useSearchParams } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { PageError } from '../components/PageState'
import { capitalize, type Mode } from '../fields'
import { Badge, Button, ButtonLink, EmptyState, Field, Loading, Select, levelLabels, type LevelKind } from '../ui'
import { useLoad } from '../useLoad'

export function Catalog({ setMode }: { setMode: (mode: Mode) => void }) {
  const [params, setParams] = useSearchParams()
  const industry = params.get('industry') || ''; const level = params.get('level') || ''
  const query = new URLSearchParams(); if (industry) query.set('industry', industry); if (level) query.set('level', level)
  const { data, loading, error } = useLoad<CatalogResponse>(`/tasks${query.size ? `?${query}` : ''}`, { tasks: [], industries: [], levels: [] })
  const apply = (next: { industry: string; level: string }) => setParams({ ...(next.industry ? { industry: next.industry } : {}), ...(next.level ? { level: next.level } : {}) })
  const industryOptions = [{ value: '', label: 'Все темы' }, ...data.industries.map(value => ({ value, label: value }))]
  const levelOptions = [{ value: '', label: 'Любая' }, ...data.levels.map(item => ({ value: item.key, label: capitalize(levelLabels[item.key as LevelKind] ?? item.label) }))]
  return <>
    <section className="catalog-hero"><div className="container hero-grid">
      <div>
        <p className="eyebrow">Для бизнеса и студенческих команд</p>
        <h1>Задачи бизнеса.<br /><em>Идеи студентов.</em></h1>
        <p className="lead">Напишите, какую проблему хотите решить. Мы поможем уточнить задачу и покажем её готовность по шкале 0–100. Студенты предложат решения, а вы сами выберете команду.</p>
        <div className="hero-actions">
          <ButtonLink variant="primary" size="lg" to="/task/new" onClick={() => setMode('business')}>Бизнесу: описать задачу <span aria-hidden="true">↗</span></ButtonLink>
          <a className="text-link" href="#catalog" onClick={() => setMode('team')}>Команде: выбрать задачу ↓</a>
        </div>
      </div>
      <div className="hero-explainer">
        <div className="process-row"><span>01</span><p>Опишите потребность своими словами</p></div>
        <div className="process-row"><span>02</span><p>Ответьте на вопросы и улучшите оценку</p></div>
        <div className="process-row"><span>03</span><p>Получите идеи команд и выберите сами</p></div>
        <p className="explain-note">Оценка показывает, насколько задача готова к работе.</p>
      </div>
    </div></section>
    <section id="catalog" className="catalog-section container">
      <div className="section-heading">
        <div><p className="eyebrow">Все опубликованные задачи</p><h2>Выберите задачу</h2><p>Задачи с пометкой «требует уточнения» тоже открыты для просмотра и отклика.</p></div>
        <span className="count-pill">{data.tasks.length} в каталоге</span>
      </div>
      <div className="filters" role="group" aria-label="Фильтры каталога">
        <Field label="Тема"><Select value={industry} options={industryOptions} onChange={value => apply({ industry: value, level })} /></Field>
        <Field label="Готовность"><Select value={level} options={levelOptions} onChange={value => apply({ industry, level: value })} /></Field>
        {(industry || level) && <Button variant="ghost" onClick={() => setParams({})}>Сбросить фильтры</Button>}
      </div>
      {loading ? <Loading /> : error ? <PageError message={error} /> : data.tasks.length ? <div className="task-list">{data.tasks.map((task, index) => <article className="task-row" key={task.id}>
        <div className={`score-tile score-${task.level}`}><strong>{task.score}</strong><span>из 100</span></div>
        <div className="task-summary">
          <div className="task-meta"><span>Место в списке #{index + 1}</span><span>{task.industry || 'Без темы'}</span><Badge kind={task.level} /></div>
          <h3><Link to={`/task/${task.id}`}>{task.fields.title || 'Задача без названия'}</Link></h3>
          <p>{task.fields.need || task.draft_text}</p>
          <div className="task-row-footer"><span>Рейтинг выше — позиция выше</span><Link to={`/task/${task.id}`}>Открыть задачу →</Link></div>
        </div>
      </article>)}</div> : <EmptyState title="Задач по этим фильтрам нет" action={<Button onClick={() => setParams({})}>Показать все задачи</Button>}>Попробуйте другую тему или уровень готовности.</EmptyState>}
    </section>
  </>
}

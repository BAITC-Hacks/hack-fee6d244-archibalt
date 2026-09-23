import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import type { CatalogResponse } from '../api'
import { capitalize, plural, readDraft, saveDraft, type Mode } from '../fields'
import { DRAFT_PLACEHOLDER } from './TaskNew'
import { BeforeAfter } from '../components/BeforeAfter'
import { Alert, Badge, Button, EmptyState, Field, Select, Spinner, Textarea, levelLabels, type LevelKind } from '../ui'
import { useLoad } from '../useLoad'

export function Catalog({ setMode }: { setMode: (mode: Mode) => void }) {
  const [params, setParams] = useSearchParams(); const navigate = useNavigate()
  const [draft, setDraft] = useState(() => readDraft().text)
  function startDraft(event: FormEvent) { event.preventDefault(); setMode('business'); saveDraft(draft, readDraft().industry); navigate('/task/new', { state: { draft } }) }
  const industry = params.get('industry') || ''; const level = params.get('level') || ''
  const query = new URLSearchParams(); if (industry) query.set('industry', industry); if (level) query.set('level', level)
  const { data, loading, refreshing, error } = useLoad<CatalogResponse>(`/tasks${query.size ? `?${query}` : ''}`, { tasks: [], industries: [], levels: [] }, { keepPrevious: true })
  const apply = (next: { industry: string; level: string }) => setParams({ ...(next.industry ? { industry: next.industry } : {}), ...(next.level ? { level: next.level } : {}) })
  const industryOptions = [{ value: '', label: 'Все темы' }, ...data.industries.map(value => ({ value, label: value }))]
  const levelOptions = [{ value: '', label: 'Любая' }, ...data.levels.map(item => ({ value: item.key, label: capitalize(levelLabels[item.key as LevelKind] ?? item.label) }))]
  return <>
    <section className="catalog-hero"><div className="container hero-grid">
      <div>
        <p className="eyebrow">Для бизнеса и студенческих команд</p>
        <h1>Из сырого запроса — задача, <em>на которую откликаются команды</em></h1>
        <p className="lead">AI задаёт вопросы и не выдумывает факты. Вы видите оценку готовности 0–100 и что добавить, студенты предлагают решения, команду выбираете вы.</p>
        <form className="hero-draft" onSubmit={startDraft}>
          <label className="hero-draft-label" htmlFor="hero-draft">Опишите задачу своими словами</label>
          <Textarea id="hero-draft" minRows={3} value={draft} onChange={event => setDraft(event.target.value)} placeholder={DRAFT_PLACEHOLDER} />
          <div className="hero-draft-foot">
            <span>3 вопроса → карточка с оценкой готовности → отклики команд · около 3 минут</span>
            <Button type="submit" variant="primary" size="lg">Описать задачу →</Button>
          </div>
        </form>
        <a className="text-link hero-team-link" href="#catalog" onClick={() => setMode('team')}>Я из команды: выбрать задачу ↓</a>
      </div>
      <BeforeAfter />
    </div></section>
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

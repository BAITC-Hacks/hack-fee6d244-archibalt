import { useSearchParams } from 'react-router-dom'
import { PageError } from '../components/PageState'
import type { Team } from '../api'
import { capitalize, plural } from '../fields'
import { Button, ButtonLink, EmptyState, Input, Loading, Select } from '../ui'
import { CloseIcon } from '../ui/icons'
import { useLoad } from '../useLoad'
import { useStartChat } from './TaskNew'
import { selectTeams } from './teams-model'
import './Teams.css'

export function Teams() {
  const startChat = useStartChat()
  const { data, loading, error } = useLoad<Team[]>('/teams', [])
  const [params, setParams] = useSearchParams()
  const search = params.get('q') || '', interest = params.get('interest') || '', sort = params.get('sort') === 'name' ? 'name' : ''
  const teams = selectTeams(data, { search, interest, sort })
  const interests = [...new Set(data.flatMap(team => team.interests))].sort((a, b) => a.localeCompare(b, 'ru'))
  const filtered = Boolean(search || interest)
  const apply = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value); else next.delete(key)
    setParams(next, { replace: true, preventScrollReset: true })
  }
  const reset = () => setParams(sort ? { sort } : {}, { replace: true, preventScrollReset: true })
  if (loading) return <Loading />
  if (error) return <PageError message={error} />

  return <section className="container teams-directory">
    <header className="teams-intro">
      <div>
        <p className="teams-kicker">СООБЩЕСТВО ARCHIBALT</p>
        <h1>Команды для<br /><em>реальных задач.</em></h1>
        <p className="teams-description">Знакомьтесь с участниками: что умеют, чем интересуются и какие технологии используют.</p>
      </div>
      <aside className="teams-start">
        <span className="teams-start-label">ОТ НАВЫКОВ К ПРАКТИКЕ</span>
        <h2>Ваша команда тоже может начать</h2>
        <p>Выберите задачу бизнеса и предложите своё решение. Первый проект начинается с отклика.</p>
        <ButtonLink to="/catalog" variant="primary">Найти задачу <span aria-hidden="true">→</span></ButtonLink>
      </aside>
    </header>

    {data.length > 0 && <>
      <div className="teams-controls" role="search" aria-label="Поиск команд">
        <div className="teams-search-field">
          <label htmlFor="team-search">Название, навык или технология</label>
          <div className="teams-searchbox">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></svg>
            <Input id="team-search" type="search" value={search} onChange={event => apply('q', event.target.value)} placeholder="Например, Python или дизайн" />
            {search && <Button variant="ghost" onClick={() => apply('q', '')} aria-label="Очистить поиск"><CloseIcon /></Button>}
          </div>
        </div>
        <div className="teams-interest-field">
          <label id="team-interest-label" htmlFor="team-interest">Интересы команды</label>
          <Select id="team-interest" aria-label="Интересы команды" value={interest} onChange={value => apply('interest', value)} options={[{ value: '', label: 'Все направления' }, ...interests.map(value => ({ value, label: capitalize(value) }))]} />
        </div>
      </div>
      <div className="teams-results-bar">
        <div className="teams-results-heading"><h2>{filtered ? 'Результаты поиска' : 'Все команды'}</h2><span className="teams-count" role="status" aria-live="polite">{teams.length} {plural(teams.length, 'команда', 'команды', 'команд')}</span></div>
        <div className="teams-sort"><span id="team-sort-label">Порядок</span><Select aria-label="Порядок команд" value={sort} onChange={value => apply('sort', value)} options={[{ value: '', label: 'По баллам' }, { value: 'name', label: 'По названию' }]} /></div>
      </div>
      {filtered && <div className="teams-active-filters">
        {search && <span>Поиск: <strong>{search}</strong></span>}
        {interest && <span>Интерес: <strong>{capitalize(interest)}</strong></span>}
        <Button variant="ghost" size="sm" onClick={reset}>Сбросить фильтры <CloseIcon /></Button>
      </div>}
    </>}

    {teams.length ? <div className="team-directory-grid">{teams.map(team => <article className="team-profile-card" key={team.id} aria-labelledby={`team-name-${team.id}`}>
      <header className="team-profile-head">
        <span className={`team-avatar team-avatar-${team.id % 4}`} aria-hidden="true">{team.name.trim().slice(0, 2).toLocaleUpperCase('ru')}</span>
        <div><span className="team-profile-eyebrow">КОМАНДА</span><h3 id={`team-name-${team.id}`}>{team.name}</h3></div>
      </header>
      <div className="team-profile-section team-profile-skills"><h4>Что умеют</h4>{team.skills.length ? <ul className="team-skill-tags">{team.skills.map((skill, index) => <li key={`${skill}-${index}`}>{capitalize(skill)}</li>)}</ul> : <p className="team-profile-empty">Навыки пока не указаны</p>}</div>
      <div className="team-profile-section"><h4>Технологии</h4>{team.tech.length ? <ul className="team-tech-tags">{team.tech.map((tech, index) => <li key={`${tech}-${index}`}>{tech}</li>)}</ul> : <p className="team-profile-empty">Технологии пока не указаны</p>}</div>
      <div className="team-profile-section team-profile-interests"><h4>Что интересно</h4><p>{team.interests.map(capitalize).join(' · ') || 'Интересы пока не указаны'}</p></div>
      <footer className={`team-profile-progress${team.points > 0 ? ' has-progress' : ''}`}>
        <strong>{team.points}<span>{plural(team.points, 'балл', 'балла', 'баллов')}</span></strong>
        <span>{team.points > 0 ? 'За подтверждённый прогресс' : 'Пока без начисленных баллов'}</span>
      </footer>
    </article>)}</div> : <EmptyState title={filtered ? 'Таких команд пока не нашли' : 'Команд пока нет'} action={filtered ? <Button onClick={reset}>Показать все команды</Button> : <ButtonLink to="/catalog" variant="primary">Перейти к задачам</ButtonLink>}>{filtered ? 'Попробуйте другой навык, технологию или уберите фильтры.' : 'Команда появится здесь после первого входа по коду.'}</EmptyState>}

    <div className="teams-bottom-notes">
      <section className="teams-progress-note"><span className="teams-points-mark">+10</span><div><h2>За реальный прогресс</h2><p>Команда получает 10 баллов, когда бизнес подтверждает этап работы. Начать можно и с нуля баллов.</p></div></section>
      <section className="teams-business-note"><h2>Ищете исполнителей?</h2><p>Опубликуйте задачу, дождитесь откликов и выберите команду по её предложению.</p><Button variant="ghost" aria-haspopup="dialog" onClick={() => startChat()}>Описать задачу <span aria-hidden="true">→</span></Button></section>
    </div>
  </section>
}

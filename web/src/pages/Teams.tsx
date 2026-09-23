import { PageError } from '../components/PageState'
import type { Team } from '../api'
import { EmptyState, Loading } from '../ui'
import { useLoad } from '../useLoad'

export function Teams() {
  const { data, loading, error } = useLoad<Team[]>('/teams', [])
  if (loading) return <Loading />; if (error) return <PageError message={error} />
  return <div className="container flow-page">
    <p className="eyebrow">Открытое сообщество</p><h1>Команды</h1>
    <p className="lead">Любая команда может выбрать задачу в общем каталоге и отправить своё предложение.</p>
    {data.length ? <div className="teams-grid">{data.map(team => <article className="detail-card" key={team.id}>
      <h2>{team.name}</h2>
      <p><strong>Интересы:</strong> {team.interests.join(', ') || 'Не указаны'}</p>
      <p><strong>Навыки:</strong> {team.skills.join(', ') || 'Не указаны'}</p>
      <p><strong>Технологии:</strong> {team.tech.join(', ') || 'Не указаны'}</p>
      <p className="team-points">Баллы за подтверждённый прогресс: {team.points}</p>
    </article>)}</div> : <EmptyState title="Команд пока нет">Команда появится после первого входа по коду.</EmptyState>}
  </div>
}

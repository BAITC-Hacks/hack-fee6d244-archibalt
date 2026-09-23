import type { AiInfo } from '../api'
import { PageError } from '../components/PageState'
import { Alert, Loading } from '../ui'
import { useLoad } from '../useLoad'

export function AiPage() {
  const { data, loading, error } = useLoad<AiInfo>('/ai', null as unknown as AiInfo)
  if (loading) return <Loading />; if (error || !data) return <PageError message={error || 'Информация недоступна.'} />
  return <div className="container flow-page info-page">
    <p className="eyebrow">Прозрачность AI</p><h1>Как мы помогаем уточнить задачу</h1>
    <p className="lead">AI предлагает вопросы и собирает черновик карточки. Каждое поле можно изменить; публикация происходит только после подтверждения человеком.</p>
    <div className="info-grid">
      <section className="detail-card"><h2>Режим работы</h2><p>{data.mode === 'openai' ? 'AI через API' : 'Локальные вопросы без внешнего API'}</p>{data.last_error && <Alert tone="warning" title="Последняя ошибка внешнего AI">{data.last_error}</Alert>}</section>
      <section className="detail-card"><h2>Промпт для вопросов</h2><pre>{data.prompt_questions}</pre></section>
      <section className="detail-card"><h2>Промпт для карточки</h2><pre>{data.prompt_card}</pre></section>
      <section className="detail-card"><h2>Формат ответа</h2><pre>{data.schema_example}</pre></section>
    </div>
  </div>
}

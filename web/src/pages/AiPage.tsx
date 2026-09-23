import type { AiInfo, FieldKey } from '../api'
import { PageError } from '../components/PageState'
import { fieldSpecs } from '../fields'
import { Alert, ButtonLink, EmptyState, Loading } from '../ui'
import { useLoad } from '../useLoad'

const labelOf = (key: string) => fieldSpecs.find(spec => spec.key === key)?.label ?? key
const when = (iso: string) => new Date(iso).toLocaleString('ru-RU', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

/** «Как работает AI»: живой последний вызов (вход → вопросы → карточка → что убрала проверка), промпты свёрнуты. */
export function AiPage() {
  const { data, loading, error } = useLoad<AiInfo | null>('/ai', null)
  if (loading) return <Loading />; if (error || !data) return <PageError message={error || 'Сведения об AI сейчас недоступны. Обновите страницу через минуту.'} />
  const call = data.last_call
  const removed = new Set<string>(call?.removed_by_guard ?? [])
  const cardKeys = fieldSpecs.map(spec => spec.key).filter(key => call && (call.fields[key] || call.raw_fields[key])) as FieldKey[]
  return <div className="container flow-page ai-page">
    <p className="eyebrow">Прозрачность AI</p>
    <h1>Как работает AI</h1>
    <p className="lead">AI задаёт уточняющие вопросы и собирает карточку только из слов заявителя. Затем проверка сравнивает каждое поле с исходным текстом и убирает то, чего в нём не было. Ниже — последний реальный вызов, без пересказа.</p>
    <p className="ai-mode"><span className={`ai-dot${data.mode === 'openai' ? ' is-live' : ''}`} />{data.mode === 'openai' ? 'Работает внешняя модель через API' : 'Локальный режим: вопросы и карточка без внешнего API'}</p>
    {data.last_error && <Alert tone="warning" title="Последняя ошибка внешнего AI">{data.last_error} Сервис переключился на локальный режим, работа не прерывалась.</Alert>}

    {call ? <section className="ai-trace" aria-label="Последний вызов AI">
      <p className="ai-trace-when">Последний вызов: {when(call.at)}. Email и телефоны скрыты.</p>
      <ol className="ai-steps">
        <li><span className="ai-step-num">1</span><div><h2>Вход</h2><p className="ai-muted">Тема: {call.industry || 'не указана'}</p><blockquote>{call.draft}</blockquote></div></li>
        <li><span className="ai-step-num">2</span><div><h2>Вопросы и ответы</h2>
          {call.questions.length ? <ul className="ai-qa">{call.questions.map(q => <li key={q.id}><strong>{q.text}</strong>{q.answer ? <p>{q.answer}</p> : <p className="ai-muted">Пропущено — поле останется пустым.</p>}</li>)}</ul> : <p className="ai-muted">Вопросов не было.</p>}
        </div></li>
        <li><span className="ai-step-num">3</span><div><h2>Карточка</h2>
          {cardKeys.length ? <dl className="ai-card">{cardKeys.map(key => <div key={key} className={removed.has(key) ? 'is-removed' : undefined}>
            <dt>{labelOf(key)}</dt>
            <dd>{removed.has(key) ? <><s>{call.raw_fields[key]}</s><small>Убрано проверкой: этого нет в тексте заявителя.</small></> : call.fields[key]}</dd>
          </div>)}</dl> : <p className="ai-muted">Модель не заполнила ни одного поля.</p>}
          {fieldSpecs.some(spec => !call.fields[spec.key] && !call.raw_fields[spec.key]) && <p className="ai-muted">Пустые поля: {fieldSpecs.filter(spec => !call.fields[spec.key] && !call.raw_fields[spec.key]).map(spec => spec.label.toLowerCase()).join(', ')}. AI не добавил фактов, которых не было.</p>}
        </div></li>
        <li><span className="ai-step-num">4</span><div><h2>Проверка</h2>
          <p>{removed.size ? `Убрано полей: ${removed.size} (${[...removed].map(labelOf).join(', ')}). Остальное подтверждено текстом заявителя.` : 'Проверка ничего не убрала: все поля карточки опираются на текст заявителя.'}</p>
        </div></li>
      </ol>
    </section> : <EmptyState title="Вызовов пока не было" action={<ButtonLink variant="primary" to="/task/new">Обсудить задачу</ButtonLink>}>После запуска сервера AI ещё не собирал карточку. Опишите задачу и ответьте на вопросы — здесь появится её путь.</EmptyState>}

    <section className="ai-prompts">
      <h2>Промпты и формат ответа</h2>
      <details><summary>Промпт для вопросов</summary><pre>{data.prompt_questions}</pre></details>
      <details><summary>Промпт для карточки</summary><pre>{data.prompt_card}</pre></details>
      <details><summary>Формат ответа модели</summary><pre>{data.schema_example}</pre></details>
    </section>
  </div>
}

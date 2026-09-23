import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { api, json, withToken, type Proposal, type Task, type Team } from '../api'
import { PageError } from '../components/PageState'
import { ProposalTable } from '../components/ProposalCard'
import { RatingPanel } from '../components/RatingPanel'
import { type Mode } from '../fields'
import { useSession } from '../session'
import { Alert, Button, ButtonLink, EmptyState, Field, Input, Loading, Spinner, Textarea, useToast } from '../ui'
import { useLoad } from '../useLoad'
import './TaskShow.css'

export function TaskShow({ mode }: { mode: Mode }) {
  const { id } = useParams(); const { hash } = useLocation(); const toast = useToast(); const { data: task, setData: setTask, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const { data: teams } = useLoad<Team[]>('/teams', [])
  const { team, checking, requestLogin, explain, isOwner } = useSession()
  const [idea, setIdea] = useState(''); const [plan, setPlan] = useState(''); const [deadline, setDeadline] = useState(''); const [link, setLink] = useState(''); const [busy, setBusy] = useState(false); const [submitError, setSubmitError] = useState('')
  useEffect(() => {
    if (!loading && hash) document.getElementById(hash.slice(1))?.scrollIntoView({ block: 'start' })
  }, [loading, hash, id])
  async function refresh() { try { setTask(await api<Task>(`/tasks/${id}`)) } catch (err) { setSubmitError(explain(err, 'team')) } }
  async function send(token: string) {
    setBusy(true); setSubmitError('')
    try {
      await api<Proposal>(`/tasks/${id}/proposals`, withToken(token, json('POST', { idea, plan, deadline, link })))
      toast.success('Предложение отправлено. Заявитель увидит его в списке предложений.')
      setIdea(''); setPlan(''); setDeadline(''); setLink(''); await refresh()
    } catch (err) { setSubmitError(explain(err, 'team', fresh => void send(fresh))) } finally { setBusy(false) }
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    if (!idea.trim() || !plan.trim() || !deadline.trim() || !link.trim()) { setSubmitError('Заполните все четыре поля: идею, план, срок и ссылку.'); return }
    if (!team) { requestLogin('team', token => void send(token)); return }
    void send(team.token)
  }
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  const owner = isOwner(task.id)
  const published = task.status === 'published'
  const mine = team ? task.proposals?.find(item => item.team.id === team.team.id && item.status === 'selected') : undefined
  const count = task.proposals?.length || 0
  return <div className="container task-detail">
    <nav className="task-breadcrumb" aria-label="Навигация по странице"><Link to="/catalog">← Все задачи</Link><span aria-hidden="true">/</span><span>{task.industry || 'Проект для команды'}</span></nav>
    <header className="task-intro">
      <div className="task-intro-meta"><span className="task-category">{task.industry || 'Бизнес-задача'}</span><span className={`task-availability${published ? ' is-published' : ''}`}>{published ? 'Принимает предложения' : 'Ещё не опубликована'}</span>{owner && <span className="task-owned">Ваша задача</span>}</div>
      <h1>{task.fields.title || 'Задача без названия'}</h1>
      <p>Изучите задачу и предложите, как ваша команда её решит. Решение о сотрудничестве принимает бизнес.</p>
    </header>
    {mine && <Alert tone="success" className="task-selected" title="Бизнес выбрал вашу команду">Подтвердите участие или откажитесь в своём отклике. <a href="#proposals">Перейти к откликам →</a></Alert>}
    <nav className="task-section-nav" aria-label="Разделы задачи"><a href="#brief">Что сделать</a><a href="#resources">Данные и условия</a><a href="#proposals">Отклики <span>{count}</span></a>{!owner && published && <a className="task-nav-cta" href="#respond">Хочу выполнить <span aria-hidden="true">↗</span></a>}</nav>
    <div className="task-workspace">
      <div className="task-content">
        {task.has_visual && <figure className="task-panel task-visual" style={{ margin: 0 }}>
          <img src={`/api/tasks/${task.id}/visual.png`} alt="Визуальный концепт первого результата" loading="lazy" style={{ display: 'block', width: '100%', maxWidth: 560, height: 'auto', borderRadius: 12 }}
            onError={event => { const figure = event.currentTarget.closest('figure'); if (figure) figure.hidden = true }} />
          <figcaption style={{ marginTop: 8, fontSize: 13, color: 'var(--muted)' }}>Концепт для обсуждения, не обещание объёма · оценка AI</figcaption>
        </figure>}
        <section className="task-panel task-brief" id="brief">
          <div className="task-panel-heading"><span className="task-section-number">01</span><h2>Задача в трёх ответах</h2></div>
          <div className="task-brief-item"><h3>Что нужно сделать</h3><p>{task.fields.need || task.draft_text || 'Бизнес пока не описал работу. Уточните её перед началом.'}</p></div>
          <div className="task-brief-item task-deliverable"><h3><span aria-hidden="true">↗</span> Что передать в конце</h3><p>{task.fields.expected_result || 'Результат пока не определён. Предложите его в отклике и согласуйте с бизнесом.'}</p></div>
          <div className="task-brief-item"><h3>Как поймут, что работа сделана</h3><p>{task.fields.success_criteria || 'Критерии приёмки пока не указаны. Согласуйте их с бизнесом до начала работы.'}</p></div>
        </section>
        <section className="task-panel" id="resources">
          <div className="task-panel-heading"><span className="task-section-number">02</span><h2>С чем предстоит работать</h2></div>
          <dl className="task-info-list">
            <div><dt>Зачем это бизнесу</dt><dd>{task.fields.context || 'Контекст пока не указан.'}</dd></div>
            <div><dt>Кто будет пользоваться</dt><dd>{task.fields.users || 'Пользователей нужно уточнить у бизнеса.'}</dd></div>
            <div><dt>Какие данные уже есть</dt><dd>{task.fields.data || 'Данные не описаны. Уточните, что бизнес сможет предоставить.'}</dd></div>
            <div><dt>Сроки и ограничения</dt><dd>{task.fields.constraints || 'Ограничения не указаны. Срок можно предложить в отклике.'}</dd></div>
          </dl>
        </section>
        <section className="task-panel" id="business-contact">
          <div className="task-panel-heading"><span className="task-section-number">03</span><h2>Как работать с бизнесом</h2></div>
          <dl className="task-info-list"><div><dt>Консультации и обратная связь</dt><dd>{task.fields.interaction_format || 'Формат общения пока не указан. Договоритесь о нём перед стартом.'}</dd></div><div><dt>Контакт по задаче</dt><dd>{task.fields.contact || 'Контакт не указан. Оставьте предложение через форму ниже.'}</dd></div></dl>
        </section>
        <section className="task-panel task-proposals" id="proposals">
          <div className="task-panel-heading"><span className="task-section-number">04</span><h2>Предложения команд <span className="task-count">{count}</span></h2></div>
          <p className="task-panel-note">{owner ? 'Сравните идеи и планы. Вы сами решаете, с кем продолжить работу.' : 'Посмотрите, что предлагают другие. Вы тоже можете откликнуться, даже если бизнес уже выбрал команду.'}</p>
          {task.proposals?.length ? <ProposalTable proposals={task.proposals} teams={teams} canDecide={owner && mode === 'business'} update={() => void refresh()} /> : <EmptyState slim title="Станьте первой командой">Здесь появятся идеи и планы тех, кто хочет решить задачу.</EmptyState>}
        </section>
        {!owner && <section className="task-panel task-response" id="respond">
          <div className="task-panel-heading"><span className="task-section-number">05</span><h2>Откликнуться на задачу</h2></div>
          <p className="task-panel-note">Расскажите, как решите задачу. Готовый продукт для отклика не нужен.</p>
          {!published ? <Alert>Отклики станут доступны после публикации задачи.</Alert> : <>
            <div className="task-response-meta">
              {checking ? <p className="task-response-team"><Spinner size="sm" /> Проверяем вход…</p> : team ? <p className="task-response-team">От команды <strong>{team.team.name}</strong></p> : <p className="task-response-team">Заполните сейчас — войдите при отправке.</p>}
              <span>Все 4 поля обязательны</span>
            </div>
            <form onSubmit={submit} noValidate className="task-response-form">
              {submitError && <Alert tone="error" className="task-form-wide">{submitError}</Alert>}
              <Field className="task-form-wide" label="Ваша идея" required hint="Что вы создадите и какую пользу это принесёт бизнесу."><Textarea minRows={3} value={idea} onChange={event => setIdea(event.target.value)} placeholder="Предлагаем сделать… В результате бизнес сможет…" /></Field>
              <Field className="task-form-wide" label="План работы" required hint="Достаточно основных этапов — подробное техническое задание не нужно."><Textarea minRows={3} value={plan} onChange={event => setPlan(event.target.value)} placeholder={'1. Изучим задачу и данные\n2. Соберём и проверим прототип\n3. Передадим результат бизнесу'} /></Field>
              <Field label="Срок выполнения" required hint="Сколько времени нужно от старта до результата."><Input value={deadline} onChange={event => setDeadline(event.target.value)} placeholder="Например, 3 недели" /></Field>
              <Field label="Ссылка на материалы" required hint="Макет, репозиторий или документ с идеей. Откройте доступ по ссылке."><Input type="url" inputMode="url" value={link} onChange={event => setLink(event.target.value)} placeholder="https://…" /></Field>
              <div className="task-response-submit task-form-wide"><p>Отклик будет виден всем.<br />Начало работы — после согласования с бизнесом.</p><Button type="submit" variant="primary" size="lg" loading={busy}>{busy ? 'Отправляем…' : 'Отправить отклик'} <span aria-hidden="true">→</span></Button></div>
            </form>
          </>}
        </section>}
      </div>
      <aside className="task-sidebar">
        <section className="task-action-panel">
          <p className="task-small-label">{owner ? 'УПРАВЛЕНИЕ ЗАДАЧЕЙ' : 'ОТ ИДЕИ К ПЕРВОМУ ПРОЕКТУ'}</p>
          <h2>{owner ? 'Найдите свою команду' : 'Хотите взяться?'}</h2>
          <p>{owner ? 'Полное описание помогает командам предложить подходящее решение.' : 'Предложите свой подход. Чтобы откликнуться, достаточно идеи, плана, срока и ссылки на материалы.'}</p>
          {owner ? <><ButtonLink variant="primary" block to={`/task/${id}/edit`}>Дополнить задачу</ButtonLink><a className="task-secondary-action" href="#proposals">Посмотреть отклики ({count}) →</a></> : published ? <a className="ui-btn ui-btn-primary ui-btn-block" href="#respond">Хочу выполнить задачу <span aria-hidden="true">↗</span></a> : <Alert>Задача ещё не опубликована</Alert>}
          <ol className="task-start-steps"><li><span>1</span><div><strong>Предложите решение</strong><small>Опишите идею и план в отклике</small></div></li><li><span>2</span><div><strong>Дождитесь выбора бизнеса</strong><small>Бизнес сравнит предложения команд</small></div></li><li><span>3</span><div><strong>Подтвердите участие</strong><small>После выбора примите проект в своём отклике</small></div></li></ol>
        </section>
        <div className="task-readiness-summary"><div><span>Полнота описания</span><strong>{task.score}<small> / 100</small></strong></div><p>Баллы показывают, сколько бизнес рассказал о проекте. Откликнуться можно с любым рейтингом.</p>
          <details className="task-rating-details"><summary>Что заполнено и чего не хватает</summary><RatingPanel task={task} /></details>
        </div>
      </aside>
    </div>
  </div>
}

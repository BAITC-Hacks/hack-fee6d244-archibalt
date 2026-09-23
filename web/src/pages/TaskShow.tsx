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
import { useStartChat } from './TaskNew'
import './TaskShow.css'

export function TaskShow({ mode, setMode }: { mode: Mode; setMode: (mode: Mode) => void }) {
  const student = mode === 'team'
  const startChat = useStartChat()
  const { id } = useParams(); const { hash } = useLocation(); const toast = useToast(); const { data: task, setData: setTask, loading, error } = useLoad<Task>(`/tasks/${id}`, null as unknown as Task)
  const { data: teams } = useLoad<Team[]>('/teams', [])
  const { team, checking, requestLogin, explain, isOwner } = useSession()
  const [idea, setIdea] = useState(''); const [plan, setPlan] = useState(''); const [deadline, setDeadline] = useState(''); const [link, setLink] = useState('')
  const [editIdea, setEditIdea] = useState(''); const [editPlan, setEditPlan] = useState(''); const [editDeadline, setEditDeadline] = useState(''); const [editLink, setEditLink] = useState('')
  const [busy, setBusy] = useState<'quick' | 'full' | 'edit' | ''>(''); const [submitError, setSubmitError] = useState('')
  useEffect(() => {
    if (!loading && hash) document.getElementById(hash.slice(1))?.scrollIntoView({ block: 'start' })
  }, [loading, hash, id, mode])
  useEffect(() => {
    const current = team && task?.proposals?.find(item => item.team.id === team.team.id)
    if (current) {
      setEditIdea(current.idea || ''); setEditPlan(current.plan || ''); setEditDeadline(current.deadline || ''); setEditLink(current.link || '')
    }
  }, [task?.proposals, team?.team.id])
  async function refresh() { try { setTask(await api<Task>(`/tasks/${id}`)) } catch (err) { setSubmitError(explain(err, 'team')) } }
  useEffect(() => {
    if (!/^#proposal-\d+$/.test(hash)) return
    void refresh()
    const onStudentUpdated = () => void refresh()
    window.addEventListener('student-updated', onStudentUpdated)
    const timer = window.setInterval(() => void refresh(), 8000)
    return () => { window.removeEventListener('student-updated', onStudentUpdated); window.clearInterval(timer) }
  }, [hash, id])
  if (loading) return <Loading />; if (error || !task) return <PageError message={error || 'Задача не найдена.'} />
  const owner = isOwner(task.id)
  const published = task.status === 'published'
  const myProposal = team ? task.proposals?.find(item => item.team.id === team.team.id) : undefined
  const selected = myProposal?.status === 'selected' ? myProposal : undefined
  const hashProposalMatch = hash.match(/^#proposal-(\d+)$/)
  const initialProposalId = hashProposalMatch ? Number(hashProposalMatch[1]) : undefined
  const count = task.proposals?.length || 0
  async function send(token: string, quick = false) {
    if (quick && myProposal) { setSubmitError('Быстрый отклик уже отправлен. Можно добавить детали ниже.'); return }
    setBusy(quick ? 'quick' : 'full'); setSubmitError('')
    try {
      await api<Proposal>(`/tasks/${id}/proposals`, withToken(token, json('POST', quick ? { quick: true } : { idea, plan, deadline, link })))
      window.dispatchEvent(new CustomEvent('student-updated'))
      toast.success(quick ? 'Быстрый отклик отправлен. Бизнес увидит ваш профиль.' : 'Предложение отправлено. Заявитель увидит его в списке предложений.')
      if (!quick) { setIdea(''); setPlan(''); setDeadline(''); setLink('') }
      await refresh()
    } catch (err) { setSubmitError(explain(err, 'team', fresh => void send(fresh, quick))) } finally { setBusy('') }
  }
  function startQuick() {
    if (myProposal) { document.getElementById(`proposal-${myProposal.id}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }); return }
    if (!team) { requestLogin('team', token => void send(token, true)); return }
    void send(team.token, true)
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    if (!idea.trim() || !plan.trim() || !deadline.trim() || !link.trim()) { setSubmitError('Заполните все четыре поля: идею, план, срок и ссылку.'); return }
    if (!team) { requestLogin('team', token => void send(token)); return }
    void send(team.token)
  }
  async function saveDetails(event: FormEvent) {
    event.preventDefault()
    if (!myProposal || !team) return
    if (!myProposal.quick && (!editIdea.trim() || !editPlan.trim() || !editDeadline.trim() || !editLink.trim())) { setSubmitError('Для подробного отклика заполните идею, план, срок и ссылку.'); return }
    if (editLink.trim() && !/^https?:\/\/\S+$/i.test(editLink.trim())) { setSubmitError('Ссылка должна начинаться с http:// или https://.'); return }
    setBusy('edit'); setSubmitError('')
    try {
      await api<Proposal>(`/proposals/${myProposal.id}`, withToken(team.token, json('PUT', { idea: editIdea.trim(), plan: editPlan.trim(), deadline: editDeadline.trim(), link: editLink.trim() })))
      window.dispatchEvent(new CustomEvent('student-updated'))
      toast.success('Детали отклика сохранены.')
      await refresh()
    } catch (err) { setSubmitError(explain(err, 'team', fresh => void saveDetailsWithToken(fresh))) } finally { setBusy('') }
  }
  async function saveDetailsWithToken(token: string) {
    if (!myProposal) return
    setBusy('edit'); setSubmitError('')
    try {
      await api<Proposal>(`/proposals/${myProposal.id}`, withToken(token, json('PUT', { idea: editIdea.trim(), plan: editPlan.trim(), deadline: editDeadline.trim(), link: editLink.trim() })))
      window.dispatchEvent(new CustomEvent('student-updated'))
      toast.success('Детали отклика сохранены.')
      await refresh()
    } catch (err) { setSubmitError(explain(err, 'team', fresh => void saveDetailsWithToken(fresh))) } finally { setBusy('') }
  }
  return <div className="container task-detail">
    <nav className="task-breadcrumb" aria-label="Навигация по странице"><Link to="/catalog">← Все задачи</Link><span aria-hidden="true">/</span><span>{task.industry || 'Проект для команды'}</span></nav>
    <header className="task-intro">
      <div className="task-intro-meta"><span className="task-category">{task.industry || 'Бизнес-задача'}</span><span className={`task-availability${published ? ' is-published' : ''}`}>{published ? 'Принимает предложения' : 'Ещё не опубликована'}</span>{owner && <span className="task-owned">Ваша задача</span>}</div>
      <h1>{task.fields.title || 'Задача без названия'}</h1>
      <p>{owner ? 'Управляйте описанием и сравнивайте предложения команд. Вы решаете, с кем работать.' : student ? 'Изучите задачу и предложите, как ваша команда её решит. Решение о сотрудничестве принимает бизнес.' : 'Посмотрите, как описаны результат и условия проекта. AI поможет подготовить вашу собственную задачу.'}</p>
    </header>
    {selected && <Alert tone="success" className="task-selected" title="Бизнес выбрал вашу команду">Подтвердите участие или откажитесь в своём отклике. <a href="#proposals">Перейти к откликам →</a></Alert>}
    <nav className="task-section-nav" aria-label="Разделы задачи"><a href="#brief">Что сделать</a><a href="#resources">Данные и условия</a><a href="#proposals">Отклики <span>{count}</span></a>{!owner && student && published && <a className="task-nav-cta" href="#respond">Хочу выполнить <span aria-hidden="true">↗</span></a>}</nav>
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
          <p className="task-panel-note">{owner ? 'Сравните идеи и планы. Вы сами решаете, с кем продолжить работу.' : student ? 'Посмотрите, что предлагают другие. Вы тоже можете откликнуться, даже если бизнес уже выбрал команду.' : 'Идеи и планы команд открыты для просмотра. Выбрать исполнителя может только владелец задачи.'}</p>
          {task.proposals?.length ? <ProposalTable proposals={task.proposals} teams={teams} canDecide={owner && mode === 'business'} initialOpenChatId={initialProposalId} update={() => void refresh()} /> : <EmptyState slim title={student && !owner ? 'Станьте первой командой' : 'Откликов пока нет'}>Здесь появятся идеи и планы тех, кто хочет решить задачу.</EmptyState>}
        </section>
        {!owner && student && <section className="task-panel task-response" id="respond">
          <div className="task-panel-heading"><span className="task-section-number">05</span><h2>Откликнуться на задачу</h2></div>
          <p className="task-panel-note">Можно начать с профиля команды. Подробности добавите после отправки.</p>
          {!published ? <Alert>Отклики станут доступны после публикации задачи.</Alert> : <>
            <div className="task-response-meta">
              {checking ? <p className="task-response-team"><Spinner size="sm" /> Проверяем вход…</p> : team ? <p className="task-response-team">От команды <strong>{team.team.name}</strong></p> : <p className="task-response-team">Заполните сейчас — войдите при отправке.</p>}
              <span>{myProposal ? 'Отклик уже отправлен' : 'Быстрый отклик — без формы'}</span>
            </div>
            {submitError && <Alert tone="error" className="task-form-wide">{submitError}</Alert>}
            {!myProposal ? <>
              <div className="task-quick-response">
                <div><strong>Быстрый отклик</strong><p>Бизнес получит вашу команду, навыки и достижения. Идею можно обсудить позже.</p></div>
                <Button type="button" variant="primary" size="lg" loading={busy === 'quick'} disabled={Boolean(busy) || checking} onClick={startQuick}>Откликнуться за один клик <span aria-hidden="true">→</span></Button>
              </div>
              <details className="task-full-response">
                <summary>Добавить идею, план, срок и ссылку <span>подробный отклик</span></summary>
                <div className="task-full-response-heading"><strong>Или отправьте подробный отклик</strong><span>Все 4 поля обязательны</span></div>
                <form onSubmit={submit} noValidate className="task-response-form">
                  <Field className="task-form-wide" label="Ваша идея" required hint="Что вы создадите и какую пользу это принесёт бизнесу."><Textarea minRows={3} value={idea} onChange={event => setIdea(event.target.value)} placeholder="Предлагаем сделать… В результате бизнес сможет…" /></Field>
                  <Field className="task-form-wide" label="План работы" required hint="Достаточно основных этапов — подробное техническое задание не нужно."><Textarea minRows={3} value={plan} onChange={event => setPlan(event.target.value)} placeholder={'1. Изучим задачу и данные\n2. Соберём и проверим прототип\n3. Передадим результат бизнесу'} /></Field>
                  <Field label="Срок выполнения" required hint="Сколько времени нужно от старта до результата."><Input value={deadline} onChange={event => setDeadline(event.target.value)} placeholder="Например, 3 недели" /></Field>
                  <Field label="Ссылка на материалы" required hint="Макет, репозиторий или документ с идеей. Откройте доступ по ссылке."><Input type="url" inputMode="url" value={link} onChange={event => setLink(event.target.value)} placeholder="https://…" /></Field>
                  <div className="task-response-submit task-form-wide"><p>Отклик будет виден всем.<br />Начало работы — после согласования с бизнесом.</p><Button type="submit" variant="secondary" size="lg" loading={busy === 'full'} disabled={Boolean(busy)}>{busy === 'full' ? 'Отправляем…' : 'Отправить подробный отклик'} <span aria-hidden="true">→</span></Button></div>
                </form>
              </details>
            </> : <div className="task-response-sent">
              <div className="task-response-sent-heading"><strong>{myProposal.quick ? 'Быстрый отклик отправлен' : 'Ваш отклик отправлен'}</strong><span>{myProposal.status === 'new' ? 'На рассмотрении' : myProposal.status === 'selected' ? 'Бизнес выбрал вашу команду' : myProposal.status === 'accepted' ? 'Проект принят' : myProposal.status === 'declined' ? 'Команда отказалась' : myProposal.status === 'rejected' ? 'Отклонён бизнесом' : 'Отложен'}</span></div>
              <p>{myProposal.quick ? 'Добавьте детали, чтобы бизнесу было проще оценить ваш подход. Все поля необязательны.' : 'Измените подробный отклик. Для сохранения заполните все четыре поля.'}</p>
              <details className="task-edit-details">
                <summary>{myProposal.quick ? 'Добавить или изменить детали' : 'Редактировать подробный отклик'}</summary>
                <form onSubmit={saveDetails} noValidate className="task-response-form">
                  <Field className="task-form-wide" label="Идея" required={!myProposal.quick}><Textarea minRows={3} value={editIdea} onChange={event => setEditIdea(event.target.value)} placeholder="Опишите, что вы создадите…" /></Field>
                  <Field className="task-form-wide" label="План работы" required={!myProposal.quick}><Textarea minRows={3} value={editPlan} onChange={event => setEditPlan(event.target.value)} placeholder="Основные этапы работы…" /></Field>
                  <Field label="Срок выполнения" required={!myProposal.quick}><Input value={editDeadline} onChange={event => setEditDeadline(event.target.value)} placeholder="Например, 3 недели" /></Field>
                  <Field label="Ссылка на материалы" required={!myProposal.quick} hint={myProposal.quick ? 'Если добавляете ссылку, используйте http:// или https://.' : 'Откройте доступ по ссылке. Используйте http:// или https://.'}><Input type="url" inputMode="url" value={editLink} onChange={event => setEditLink(event.target.value)} placeholder="https://…" /></Field>
                  <div className="task-response-submit task-form-wide"><p>{myProposal.quick ? 'Поля можно заполнить сейчас или обсудить с бизнесом в чате.' : 'Подробный отклик должен содержать все четыре поля.'}</p><Button type="submit" variant="secondary" size="lg" loading={busy === 'edit'} disabled={Boolean(busy)}>{busy === 'edit' ? 'Сохраняем…' : myProposal.quick ? 'Сохранить детали' : 'Сохранить изменения'} <span aria-hidden="true">→</span></Button></div>
                </form>
              </details>
              <details className="task-new-proposal">
                <summary>Отправить новое предложение</summary>
                <p>Можно предложить бизнесу другой подход. Для нового предложения заполните все четыре поля.</p>
                <form onSubmit={submit} noValidate className="task-response-form">
                  <Field className="task-form-wide" label="Ваша идея" required hint="Что вы создадите и какую пользу это принесёт бизнесу."><Textarea minRows={3} value={idea} onChange={event => setIdea(event.target.value)} placeholder="Предлагаем сделать… В результате бизнес сможет…" /></Field>
                  <Field className="task-form-wide" label="План работы" required hint="Достаточно основных этапов — подробное техническое задание не нужно."><Textarea minRows={3} value={plan} onChange={event => setPlan(event.target.value)} placeholder={'1. Изучим задачу и данные\n2. Соберём и проверим прототип\n3. Передадим результат бизнесу'} /></Field>
                  <Field label="Срок выполнения" required hint="Сколько времени нужно от старта до результата."><Input value={deadline} onChange={event => setDeadline(event.target.value)} placeholder="Например, 3 недели" /></Field>
                  <Field label="Ссылка на материалы" required hint="Макет, репозиторий или документ с идеей. Откройте доступ по ссылке."><Input type="url" inputMode="url" value={link} onChange={event => setLink(event.target.value)} placeholder="https://…" /></Field>
                  <div className="task-response-submit task-form-wide"><p>Новое предложение будет видно бизнесу рядом с предыдущим.</p><Button type="submit" variant="secondary" size="lg" loading={busy === 'full'} disabled={Boolean(busy)}>{busy === 'full' ? 'Отправляем…' : 'Отправить новое предложение'} <span aria-hidden="true">→</span></Button></div>
                </form>
              </details>
            </div>}
          </>}
        </section>}
      </div>
      <aside className="task-sidebar">
        <section className="task-action-panel">
          <p className="task-small-label">{owner ? 'УПРАВЛЕНИЕ ЗАДАЧЕЙ' : student ? 'ОТ ИДЕИ К ПЕРВОМУ ПРОЕКТУ' : 'ДЛЯ ВАШЕГО БИЗНЕСА'}</p>
          <h2>{owner ? 'Найдите свою команду' : student ? 'Хотите взяться?' : 'Есть похожая задача?'}</h2>
          <p>{owner ? 'Полное описание помогает командам предложить подходящее решение.' : student ? 'Откликнитесь одним нажатием — бизнес увидит профиль команды. Детали можно добавить позже.' : 'Расскажите о своей проблеме. AI поможет подготовить описание для студенческих команд.'}</p>
          {owner ? <><ButtonLink variant="primary" block to={`/task/${id}/edit`}>Дополнить задачу</ButtonLink><a className="task-secondary-action" href="#proposals">Посмотреть отклики ({count}) →</a></> : !student ? <><Button variant="primary" block aria-haspopup="dialog" onClick={() => startChat()}>Обсудить свою задачу</Button><Button variant="ghost" block onClick={() => setMode('team')}>Я хочу выполнить этот проект</Button></> : published ? <a className="ui-btn ui-btn-primary ui-btn-block" href="#respond">{myProposal ? 'Открыть мой отклик' : 'Быстрый отклик'} <span aria-hidden="true">↗</span></a> : <Alert>Задача ещё не опубликована</Alert>}
          {student && !owner && <ol className="task-start-steps"><li><span>1</span><div><strong>Предложите решение</strong><small>Опишите идею и план в отклике</small></div></li><li><span>2</span><div><strong>Дождитесь выбора бизнеса</strong><small>Бизнес сравнит предложения команд</small></div></li><li><span>3</span><div><strong>Подтвердите участие</strong><small>После выбора примите проект в своём отклике</small></div></li></ol>}
        </section>
        <div className="task-readiness-summary"><div><span>Полнота описания</span><strong>{task.score}<small> / 100</small></strong></div><p>Баллы показывают, сколько бизнес рассказал о проекте. Откликнуться можно с любым рейтингом.</p>
          <details className="task-rating-details"><summary>Что заполнено и чего не хватает</summary><RatingPanel task={task} /></details>
        </div>
      </aside>
    </div>
  </div>
}

import { createContext, useCallback, useContext, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { api, json, withToken, type Message, type Task, type Team } from '../api'
import { useSession } from '../session'
import { Alert, Button, Field, Input, Spinner, Textarea } from '../ui'
import { studentNotices, type StudentNotice, type StudentProposal } from './student-state'
import './StudentHub.css'

type Me = { team: Team; proposals: StudentProposal[] }
interface StudentState { me: Me | null; loading: boolean; error: string; notices: StudentNotice[]; seen: string[]; markSeen: (key: string) => void; refresh: () => void }
const Context = createContext<StudentState | null>(null)
function useStudent() { const value = useContext(Context); if (!value) throw new Error('StudentProvider missing'); return value }
const savedSeen = (key: string): string[] => { try { const value: unknown = JSON.parse(localStorage.getItem(key) || '[]'); return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [] } catch { return [] } }

export function StudentProvider({ active, children }: { active: boolean; children: ReactNode }) {
  const { team, expire } = useSession()
  const token = team?.token
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(Boolean(token))
  const [error, setError] = useState('')
  const [incoming, setIncoming] = useState<Record<number, number>>({})
  const counts = useRef<Record<number, number>>({})
  const readKey = `student-notices:${team?.team.id ?? 'guest'}`
  const [seen, setSeen] = useState<string[]>(() => savedSeen(readKey))
  const [revision, setRevision] = useState(0)
  const refresh = useCallback(() => setRevision(value => value + 1), [])
  useEffect(() => {
    if (!token || !active) { setLoading(false); return }
    let alive = true, busy = false
    const load = async () => {
      if (busy || document.hidden) return
      busy = true
      try {
        const next = await api<Me>('/me', withToken(token))
        if (!alive) return
        setMe(next); setError(''); setLoading(false)
        await Promise.all(next.proposals.filter(p => (p.messages_count || 0) > 0 && counts.current[p.id] !== p.messages_count).map(async p => {
          try {
            const result = await api<{ messages: Message[] }>(`/proposals/${p.id}/messages`, withToken(token))
            if (!alive) return
            counts.current[p.id] = p.messages_count || 0
            const latest = result.messages.filter(m => m.author === 'business').reduce((max, m) => Math.max(max, m.id), 0)
            setIncoming(current => ({ ...current, [p.id]: latest }))
          } catch { /* Retry when the next /me poll succeeds. */ }
        }))
      } catch (err) {
        if (!alive) return
        setError(err instanceof Error ? err.message : 'Не удалось обновить отклики.')
        setLoading(false)
        if (err && typeof err === 'object' && 'status' in err && err.status === 401) expire('team')
      } finally { busy = false }
    }
    void load()
    const timer = window.setInterval(() => void load(), 8000)
    const onFocus = () => void load()
    window.addEventListener('focus', onFocus); document.addEventListener('visibilitychange', onFocus)
    window.addEventListener('student-updated', onFocus)
    return () => { alive = false; window.clearInterval(timer); window.removeEventListener('focus', onFocus); document.removeEventListener('visibilitychange', onFocus); window.removeEventListener('student-updated', onFocus) }
  }, [token, active, revision, expire])
  function markSeen(key: string) {
    setSeen(current => { const next = [...new Set([...current, key])].slice(-300); try { localStorage.setItem(readKey, JSON.stringify(next)) } catch { /* Reading still works for this visit. */ } return next })
  }
  return <Context.Provider value={{ me, loading, error, notices: studentNotices(me?.proposals || [], incoming), seen, markSeen, refresh }}>{children}</Context.Provider>
}

export function StudentNotifications() {
  const { team } = useSession()
  const { notices, seen, markSeen, error } = useStudent()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (event: MouseEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    const key = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', close); document.addEventListener('keydown', key)
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', key) }
  }, [open])
  if (!team) return null
  const unread = notices.filter(n => !seen.includes(n.key)).length
  return <div className="student-notifications" ref={root}>
    <Button size="sm" variant="ghost" aria-expanded={open} aria-controls="student-notifications" onClick={() => setOpen(value => !value)} aria-label={`Обновления${unread ? `: ${unread} новых` : ''}`}><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4" /></svg>{unread > 0 && <span className="student-unread">{unread}</span>}</Button>
    {open && <section id="student-notifications" className="student-notice-panel" aria-label="Обновления по откликам">
      <h2>Обновления</h2><p className="student-note">Выбор команды и сообщения заказчика появятся здесь.</p>
      {error && <Alert tone="error">{error}</Alert>}
      {notices.length ? notices.map(notice => <Link className={`student-notice${seen.includes(notice.key) ? '' : ' is-unread'}`} key={notice.key} to={`/task/${notice.taskId}#proposal-${notice.proposalId}`} onClick={() => { markSeen(notice.key); setOpen(false) }}><strong>{notice.title}</strong><span>{notice.detail}</span><small>Открыть отклик и переписку →</small></Link>) : <p>Пока обновлений нет. После отклика здесь появится решение бизнеса.</p>}
      <Link to="/catalog#my-proposals" onClick={() => setOpen(false)}>Мои отклики →</Link>
    </section>}
  </div>
}

const statuses: Record<StudentProposal['status'], string> = { new: 'На рассмотрении', selected: 'Вас выбрали — подтвердите участие', accepted: 'Участие подтверждено', declined: 'Вы отказались', rejected: 'Бизнес отклонил', on_hold: 'Решение отложено' }
const split = (text: string) => [...new Set(text.split(/[,;\n]/).map(value => value.trim()).filter(Boolean))].slice(0, 30)

export function StudentHub() {
  const { hash } = useLocation()
  const { team, requestLogin, explain } = useSession()
  const { me, loading, error, refresh, notices, markSeen } = useStudent()
  const [editing, setEditing] = useState(false)
  const [skills, setSkills] = useState(''); const [tech, setTech] = useState(''); const [interests, setInterests] = useState('')
  const [experience, setExperience] = useState(''); const [achievements, setAchievements] = useState('')
  const [saving, setSaving] = useState(false); const [profileError, setProfileError] = useState('')
  const [recommendations, setRecommendations] = useState<Task[]>([]); const [recoError, setRecoError] = useState(''); const [recoLoading, setRecoLoading] = useState(false)
  const profile = me?.team
  const profileKey = JSON.stringify(profile ? [profile.id, profile.skills, profile.tech, profile.interests, profile.experience, profile.achievements] : null)
  useEffect(() => {
    if (!profile) return
    let alive = true; setRecoLoading(true)
    api<Task[]>(`/teams/${profile.id}/recommended`).then(result => { if (alive) { setRecommendations(result); setRecoError('') } }).catch(err => { if (alive) setRecoError(err.message) }).finally(() => { if (alive) setRecoLoading(false) })
    return () => { alive = false }
  }, [profileKey])
  function editProfile() {
    if (!profile) return
    setSkills(profile.skills.join(', ')); setTech(profile.tech.join(', ')); setInterests(profile.interests.join(', '))
    setExperience(profile.experience || ''); setAchievements(profile.achievements || ''); setProfileError(''); setEditing(true)
  }
  async function saveProfile(event: FormEvent) {
    event.preventDefault(); if (!team || saving) return
    setSaving(true); setProfileError('')
    try {
      await api('/me/profile', withToken(team.token, json('PUT', { skills: split(skills), tech: split(tech), interests: split(interests), experience: experience.trim(), achievements: achievements.trim() })))
      setEditing(false); refresh(); window.dispatchEvent(new Event('student-updated'))
    } catch (err) { setProfileError(explain(err, 'team')) } finally { setSaving(false) }
  }
  if (!team) return <section className="student-hub student-guest"><div><h2>Проекты под ваши навыки</h2><p>Войдите командой — учтём навыки, интересы и опыт. Все задачи доступны ниже.</p></div><Button variant="secondary" onClick={() => requestLogin('team')}>Подобрать для меня</Button></section>
  return <section className="student-hub" aria-label="Ваши проекты и профиль">
    <div className="student-hub-heading"><div><h2>Подходит вашей команде</h2><p>По навыкам, интересам и описанному опыту. Вы можете выбрать любую задачу ниже.</p></div><div className="student-hub-actions">{profile && <span className="student-points">{profile.points} баллов за этапы</span>}<Button variant="secondary" size="sm" onClick={editProfile} disabled={!profile}>Мой контекст</Button></div></div>
    {error && <Alert tone="error">{error} <button onClick={refresh}>Повторить</button></Alert>}
    {loading && <p role="status"><Spinner size="sm" /> Загружаем профиль…</p>}
    {editing && <form className="student-profile" onSubmit={saveProfile}>
      <p className="student-note student-full">Укажите только то, что готовы показать заказчику. Все поля необязательны; перечисления — через запятую.</p>
      <Field label="Навыки"><Input value={skills} onChange={e => setSkills(e.target.value)} maxLength={1500} placeholder="Анализ данных, дизайн интерфейсов" /></Field>
      <Field label="Технологии"><Input value={tech} onChange={e => setTech(e.target.value)} maxLength={1500} placeholder="Python, React, Figma" /></Field>
      <Field label="Интересы" className="student-full"><Input value={interests} onChange={e => setInterests(e.target.value)} maxLength={1500} placeholder="Образование, экология" /></Field>
      <Field label="Опыт и проекты"><Textarea value={experience} onChange={e => setExperience(e.target.value)} maxLength={2000} minRows={2} placeholder="Что уже делали и какова была ваша роль" /></Field>
      <Field label="Достижения"><Textarea value={achievements} onChange={e => setAchievements(e.target.value)} maxLength={2000} minRows={2} placeholder="Учебный проект, конкурс или конкретный результат" /></Field>
      {profileError && <Alert tone="error" className="student-full">{profileError}</Alert>}
      <div className="student-hub-actions student-full"><Button variant="primary" type="submit" loading={saving}>Сохранить и подобрать</Button><Button disabled={saving} variant="ghost" onClick={() => setEditing(false)}>Отмена</Button></div>
    </form>}
    {recoError && <Alert tone="error">Рекомендации временно недоступны. Все задачи доступны ниже.</Alert>}
    {recoLoading ? <p role="status"><Spinner size="sm" /> Подбираем проекты…</p> : profile && (recommendations.length ? <div className="student-recommendations">{recommendations.slice(0, 3).map(task => <article key={task.id}><span className="student-note">Готовность описания {task.score}/100</span><h3><Link to={`/task/${task.id}`}>{task.fields.title}</Link></h3><p>{task.fields.expected_result || task.fields.need}</p>{task.match_reasons?.length ? <ul>{task.match_reasons.slice(0, 3).map(reason => <li key={reason}>{reason}</li>)}</ul> : <small>Совпадение по контексту вашей команды</small>}<Link to={`/task/${task.id}#respond`}>Посмотреть и откликнуться →</Link></article>)}</div> : <p>Пока точных совпадений нет. <button className="student-text-button" onClick={editProfile}>Добавьте навыки и опыт</button> или выберите задачу в полном каталоге.</p>)}
    <details id="my-proposals" className="student-proposals" open={hash === '#my-proposals' || Boolean(me?.proposals.some(p => p.status === 'selected'))}>
      <summary>Мои отклики <span>{me?.proposals.length || 0}</span></summary>
      {me?.proposals.length ? me.proposals.map(p => <div className="student-proposal" key={p.id}><div><strong>{p.task?.title || `Задача №${p.task_id}`}</strong><p>{statuses[p.status]}{p.stage_confirmed ? ' · Этап подтверждён, +10 баллов' : ''}</p></div><Link to={`/task/${p.task_id}#proposal-${p.id}`} onClick={() => notices.filter(n => n.proposalId === p.id).forEach(n => markSeen(n.key))}>{p.status === 'selected' ? 'Ответить заказчику →' : 'Отклик и переписка →'}</Link></div>) : <p>Откликнитесь на задачу — здесь появятся её статус и переписка.</p>}
    </details>
  </section>
}

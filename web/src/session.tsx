import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { ApiError, api, json, withToken, type Task, type Team } from './api'
import { LoginModal, type Role } from './components/LoginModal'
import type { TeamSession } from './fields'

export type { Role }
export interface BusinessSession { token: string; contact: string; taskIds: number[] }
type OnToken = (token: string, contact: string) => void

interface SessionApi {
  team: TeamSession | null
  business: BusinessSession | null
  checking: boolean
  /** Открыть вход; onSuccess получит токен выбранной роли (например, чтобы повторить действие). */
  requestLogin: (role: Role, onSuccess?: OnToken) => void
  logout: (role: Role) => Promise<void>
  /** Токен больше не действует: забыть его локально. */
  expire: (role: Role) => void
  refreshBusiness: (token?: string) => Promise<void>
  isOwner: (taskId: number) => boolean
  /** Текст ошибки для показа; на 401 заодно открывает вход и повторяет действие после него. */
  explain: (err: unknown, role: Role, retry?: OnToken) => string
}

const KEY: Record<Role, string> = { team: 'team_token', business: 'business_token' }
const SessionContext = createContext<SessionApi | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [team, setTeam] = useState<TeamSession | null>(null)
  const [business, setBusiness] = useState<BusinessSession | null>(null)
  const [pendingChecks, setPendingChecks] = useState(() => [KEY.team, KEY.business].filter(key => localStorage.getItem(key)).length)
  const [login, setLogin] = useState<{ role: Role } | null>(null)
  const pending = useRef<{ role: Role; run: OnToken } | undefined>(undefined)

  const loadBusiness = useCallback(async (token: string) => {
    const me = await api<{ contact: string; tasks: Task[] }>('/business/me', withToken(token))
    setBusiness({ token, contact: me.contact, taskIds: me.tasks.map(task => task.id) })
  }, [])

  useEffect(() => {
    let alive = true
    const done = () => { if (alive) setPendingChecks(count => Math.max(0, count - 1)) }
    const teamToken = localStorage.getItem(KEY.team)
    if (teamToken) api<{ team: Team }>('/me', withToken(teamToken)).then(({ team }) => { if (alive) setTeam({ token: teamToken, team }) }).catch(() => { if (alive) localStorage.removeItem(KEY.team) }).finally(done)
    const businessToken = localStorage.getItem(KEY.business)
    if (businessToken) loadBusiness(businessToken).catch(() => { if (alive) localStorage.removeItem(KEY.business) }).finally(done)
    return () => { alive = false }
  }, [loadBusiness])

  const requestLogin = useCallback((role: Role, onSuccess?: OnToken) => { pending.current = onSuccess ? { role, run: onSuccess } : undefined; setLogin({ role }) }, [])
  const expire = useCallback((role: Role) => { localStorage.removeItem(KEY[role]); if (role === 'team') setTeam(null); else setBusiness(null) }, [])
  const logout = useCallback(async (role: Role) => {
    const token = role === 'team' ? team?.token : business?.token
    if (token) { try { await api(role === 'team' ? '/auth/logout' : '/auth/business/logout', withToken(token, json('POST'))) } catch { /* local logout still applies */ } }
    expire(role)
  }, [team, business, expire])
  const refreshBusiness = useCallback(async (token?: string) => { const current = token || business?.token; if (current) { try { await loadBusiness(current) } catch { /* keep previous list */ } } }, [business, loadBusiness])
  const isOwner = useCallback((taskId: number) => Boolean(business?.taskIds.includes(taskId)), [business])
  const explain = useCallback((err: unknown, role: Role, retry?: OnToken) => {
    if (err instanceof ApiError && err.status === 401) { expire(role); requestLogin(role, retry); return role === 'business' ? 'Войдите как заявитель, чтобы продолжить.' : 'Войдите как команда, чтобы продолжить.' }
    if (err instanceof ApiError && err.status === 403 && role === 'business') return 'Эту задачу может изменять только заявитель.'
    return err instanceof Error ? err.message : 'Что-то пошло не так. Попробуйте ещё раз.'
  }, [expire, requestLogin])

  const value = useMemo(() => ({ team, business, checking: pendingChecks > 0, requestLogin, logout, expire, refreshBusiness, isOwner, explain }), [team, business, pendingChecks, requestLogin, logout, expire, refreshBusiness, isOwner, explain])

  function finish(role: Role, token: string, contact: string) {
    localStorage.setItem(KEY[role], token)
    setLogin(null)
    const next = pending.current; pending.current = undefined
    if (next && next.role === role) next.run(token, contact)
  }

  return <SessionContext.Provider value={value}>
    {children}
    <LoginModal open={Boolean(login)} role={login?.role ?? 'team'} onRoleChange={role => setLogin({ role })}
      onClose={() => { pending.current = undefined; setLogin(null) }}
      onTeam={session => { setTeam(session); finish('team', session.token, session.team.contact || '') }}
      onBusiness={async ({ token, contact }) => {
        setBusiness({ token, contact, taskIds: [] })
        try { await loadBusiness(token) } catch { /* list will load later */ }
        finish('business', token, contact)
      }} />
  </SessionContext.Provider>
}

export function useSession(): SessionApi {
  const value = useContext(SessionContext)
  if (!value) throw new Error('useSession нужен внутри SessionProvider')
  return value
}

/** Контакт в шапке без лишних подробностей: a•••@mail.kz, +7•••12. */
export function maskContact(contact: string) {
  const at = contact.indexOf('@')
  if (at > 0) return `${contact[0]}•••${contact.slice(at)}`
  return contact.length > 5 ? `${contact.slice(0, 2)}•••${contact.slice(-2)}` : contact
}

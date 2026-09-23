import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { api, json, withToken, type Team } from './api'
import { TeamLoginModal } from './components/TeamLoginModal'
import type { TeamSession } from './fields'

interface SessionApi {
  session: TeamSession | null
  checking: boolean
  /** Открыть вход команды; onSuccess вызовется после входа (например, чтобы дослать отклик). */
  requestLogin: (onSuccess?: (session: TeamSession) => void) => void
  logout: () => Promise<void>
  /** Токен больше не действует: забыть его локально. */
  expire: () => void
}

const SessionContext = createContext<SessionApi | null>(null)

export function TeamSessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<TeamSession | null>(null)
  const [checking, setChecking] = useState(() => Boolean(localStorage.getItem('team_token')))
  const [loginOpen, setLoginOpen] = useState(false)
  const pending = useRef<((session: TeamSession) => void) | undefined>(undefined)

  useEffect(() => { const token = localStorage.getItem('team_token'); if (!token) return; let alive = true; api<{ team: Team }>('/me', withToken(token)).then(({ team }) => { if (alive) setSession({ token, team }) }).catch(() => { if (alive) localStorage.removeItem('team_token') }).finally(() => { if (alive) setChecking(false) }); return () => { alive = false } }, [])

  const requestLogin = useCallback((onSuccess?: (session: TeamSession) => void) => { pending.current = onSuccess; setLoginOpen(true) }, [])
  const expire = useCallback(() => { localStorage.removeItem('team_token'); setSession(null) }, [])
  const logout = useCallback(async () => { if (!session) return; try { await api('/auth/logout', withToken(session.token, json('POST'))) } catch { /* local logout still applies */ } expire() }, [session, expire])
  const value = useMemo(() => ({ session, checking, requestLogin, logout, expire }), [session, checking, requestLogin, logout, expire])

  return <SessionContext.Provider value={value}>
    {children}
    <TeamLoginModal open={loginOpen} onClose={() => { pending.current = undefined; setLoginOpen(false) }} onLogin={signedIn => {
      setSession(signedIn); setLoginOpen(false)
      const next = pending.current; pending.current = undefined; next?.(signedIn)
    }} />
  </SessionContext.Provider>
}

export function useTeamSession(): SessionApi {
  const value = useContext(SessionContext)
  if (!value) throw new Error('useTeamSession нужен внутри TeamSessionProvider')
  return value
}

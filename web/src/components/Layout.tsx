import { useEffect, type ReactNode } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import type { Mode } from '../fields'
import { useTeamSession } from '../session'
import { Logo, Segmented } from '../ui'
import { SessionMenu } from './SessionMenu'

export function ScrollToTop() { const { pathname } = useLocation(); useEffect(() => { window.scrollTo(0, 0) }, [pathname]); return null }

const modes: { value: Mode; label: string }[] = [{ value: 'business', label: 'Я бизнес' }, { value: 'team', label: 'Я команда' }]

export function Layout({ mode, setMode, children }: { mode: Mode; setMode: (mode: Mode) => void; children: ReactNode }) {
  const { session, checking, requestLogin, logout } = useTeamSession()
  function changeMode(next: Mode) {
    setMode(next)
    if (next === 'team' && !session && !checking) requestLogin()
  }
  return <>
    <a className="skip-link" href="#main">Перейти к содержимому</a>
    <header className="site-header"><div className="header-inner">
      <Link className="brand" to="/" aria-label="Archibalt — каталог задач"><Logo /></Link>
      <nav className="main-nav" aria-label="Основная навигация"><NavLink to="/" end>Каталог задач</NavLink><NavLink to="/task/new">Создать задачу</NavLink><NavLink to="/teams">Команды</NavLink><NavLink to="/ai">Как работает AI</NavLink></nav>
      <div className="header-role">
        {mode === 'team' && session && <SessionMenu team={session.team} onLogout={logout} />}
        <Segmented label="Режим просмотра" value={mode} options={modes} onChange={changeMode} />
      </div>
    </div></header>
    <main id="main">{children}</main>
    <footer className="site-footer"><div className="container"><span>Archibalt · AI Sana</span><span>Бизнес описывает задачу. Команда выбирает сама.</span></div></footer>
  </>
}

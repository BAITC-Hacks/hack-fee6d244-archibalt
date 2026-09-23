import { useEffect, type ReactNode } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import type { Mode } from '../fields'
import { maskContact, useSession } from '../session'
import { Logo, Segmented, useToast } from '../ui'
import { SessionMenu } from './SessionMenu'

export function ScrollToTop() { const { pathname } = useLocation(); useEffect(() => { window.scrollTo(0, 0) }, [pathname]); return null }

const modes: { value: Mode; label: string }[] = [{ value: 'business', label: 'Я бизнес' }, { value: 'team', label: 'Я команда' }]

export function Layout({ mode, setMode, children }: { mode: Mode; setMode: (mode: Mode) => void; children: ReactNode }) {
  const { team, business, checking, requestLogin, logout } = useSession()
  const navigate = useNavigate(); const toast = useToast()
  function changeMode(next: Mode) {
    setMode(next)
    if (next === 'team' && !team && !checking) requestLogin('team')
  }
  return <>
    <a className="skip-link" href="#main">Перейти к содержимому</a>
    <header className="site-header"><div className="header-inner">
      <Link className="brand" to="/" aria-label="Archibalt — каталог задач"><Logo /></Link>
      <nav className="main-nav" aria-label="Основная навигация">
        <NavLink to="/" end>Каталог задач</NavLink><NavLink to="/task/new">Создать задачу</NavLink>
        {business && <NavLink to="/business">Мои задачи</NavLink>}
        <NavLink to="/teams">Команды</NavLink><NavLink to="/ai">Как работает AI</NavLink>
      </nav>
      <div className="header-role">
        {business && <SessionMenu kind="business" title="Вы вошли как заявитель" label={maskContact(business.contact)} items={[
          { label: 'Мои задачи', onSelect: () => navigate('/business') },
          { label: 'Выйти', onSelect: () => void logout('business').then(() => toast.show('Вы вышли из режима заявителя')) },
        ]} />}
        {team && <SessionMenu kind="team" title="Вы вошли как команда" label={team.team.name} items={[
          { label: 'Выйти', onSelect: () => void logout('team').then(() => toast.show('Вы вышли из команды')) },
        ]} />}
        <Segmented label="Режим просмотра" value={mode} options={modes} onChange={changeMode} />
      </div>
    </div></header>
    <main id="main">{children}</main>
    <footer className="site-footer"><div className="container"><span>Archibalt · AI Sana</span><span>Бизнес описывает задачу. Команда выбирает сама.</span></div></footer>
  </>
}

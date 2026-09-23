import { useEffect, useState, type ReactNode } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import type { Mode } from '../fields'
import { maskContact, useSession } from '../session'
import { Logo, Segmented, useToast } from '../ui'
import { SessionMenu } from './SessionMenu'

export function ScrollToTop() { const { pathname, hash } = useLocation(); useEffect(() => { if (!hash) window.scrollTo(0, 0) }, [pathname, hash]); return null }

const modes: { value: Mode; label: string }[] = [{ value: 'business', label: 'Я бизнес' }, { value: 'team', label: 'Я команда' }]
const ANCHORS = [{ id: 'how', label: 'Как это работает' }, { id: 'compare', label: 'Сравнение' }, { id: 'faq', label: 'Вопросы' }]

/** Шапка прозрачна над hero лендинга и становится стеклянной при прокрутке; на внутренних страницах стеклянная сразу. */
function useGlass(landing: boolean) {
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    let frame = 0
    const check = () => { frame = 0; setScrolled(window.scrollY > 8) }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(check) }
    check(); window.addEventListener('scroll', schedule, { passive: true })
    return () => { window.removeEventListener('scroll', schedule); cancelAnimationFrame(frame) }
  }, [])
  return !landing || scrolled
}

/** Какой якорь лендинга сейчас на экране. */
function useActiveAnchor(enabled: boolean) {
  const [active, setActive] = useState('')
  useEffect(() => {
    if (!enabled || typeof IntersectionObserver === 'undefined') { setActive(''); return }
    const seen = new Map<string, boolean>()
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) seen.set(entry.target.id, entry.isIntersecting)
      setActive(ANCHORS.find(item => seen.get(item.id))?.id ?? '')
    }, { rootMargin: '-45% 0px -50% 0px' })
    const timer = window.setTimeout(() => ANCHORS.forEach(item => { const node = document.getElementById(item.id); if (node) observer.observe(node) }), 100)
    return () => { window.clearTimeout(timer); observer.disconnect() }
  }, [enabled])
  return active
}

export function Layout({ mode, setMode, children }: { mode: Mode; setMode: (mode: Mode) => void; children: ReactNode }) {
  const { team, business, checking, requestLogin, logout } = useSession()
  const navigate = useNavigate(); const toast = useToast(); const { pathname } = useLocation()
  const landing = pathname === '/'
  const glass = useGlass(landing)
  const active = useActiveAnchor(landing)
  const [menu, setMenu] = useState(false)
  useEffect(() => { setMenu(false) }, [pathname])
  function changeMode(next: Mode) {
    setMode(next)
    if (next === 'team' && !team && !checking) requestLogin('team')
  }
  const links = landing
    ? <>{ANCHORS.map(item => <a key={item.id} href={`#${item.id}`} className={active === item.id ? 'is-active' : undefined} onClick={() => setMenu(false)}>{item.label}</a>)}<NavLink to="/catalog">Каталог задач</NavLink></>
    : <><NavLink to="/catalog">Каталог задач</NavLink>{business && <NavLink to="/business">Мои задачи</NavLink>}<NavLink to="/teams">Команды</NavLink><NavLink to="/ai">Как работает AI</NavLink></>
  return <>
    <a className="skip-link" href="#main">Перейти к содержимому</a>
    <header className={`topbar${landing ? ' is-landing' : ''}`}>
      <div className={`capsule${glass || menu ? ' is-glass' : ''}`}>
        <Link className="brand" to="/" aria-label="Archibalt — на главную"><Logo /></Link>
        <nav className="capsule-nav" aria-label="Основная навигация">{links}</nav>
        <div className="capsule-right">
          {business && <SessionMenu kind="business" title="Вы вошли как заявитель" label={maskContact(business.contact)} items={[
            { label: 'Мои задачи', onSelect: () => navigate('/business') },
            { label: 'Выйти', onSelect: () => void logout('business').then(() => toast.show('Вы вышли из режима заявителя')) },
          ]} />}
          {team && <SessionMenu kind="team" title="Вы вошли как команда" label={team.team.name} items={[
            { label: 'Выйти', onSelect: () => void logout('team').then(() => toast.show('Вы вышли из команды')) },
          ]} />}
          <div className="capsule-mode"><Segmented label="Режим просмотра" value={mode} options={modes} onChange={changeMode} /></div>
          <Link className="cta-pill" to="/task/new" onClick={() => setMode('business')}>Описать задачу <span aria-hidden="true">→</span></Link>
          <button type="button" className="capsule-burger" aria-expanded={menu} aria-controls="capsule-panel" aria-label={menu ? 'Закрыть меню' : 'Открыть меню'} onClick={() => setMenu(value => !value)}><span /><span /></button>
        </div>
      </div>
      <div id="capsule-panel" className={`capsule-panel${menu ? ' is-open' : ''}`} hidden={!menu}>
        <nav aria-label="Меню">{links}</nav>
        <Segmented label="Режим просмотра" value={mode} options={modes} onChange={changeMode} />
      </div>
    </header>
    <main id="main" className={landing ? 'main-landing' : 'main-page'}>{children}</main>
    <footer className="site-footer"><div className="container"><span>Archibalt · AI Sana</span><span>Бизнес описывает задачу. Команда выбирает сама.</span></div></footer>
  </>
}

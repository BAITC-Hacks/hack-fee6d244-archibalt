import { useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AgentChatProvider } from './components/AgentChat'
import { Layout, ScrollToTop } from './components/Layout'
import { PageError } from './components/PageState'
import type { Mode } from './fields'
import { AiPage } from './pages/AiPage'
import { Catalog } from './pages/Catalog'
import { Landing } from './pages/Landing'
import { Clarify } from './pages/Clarify'
import { MyTasks } from './pages/MyTasks'
import { TaskEdit } from './pages/TaskEdit'
import { TaskNew } from './pages/TaskNew'
import { TaskShow } from './pages/TaskShow'
import { Teams } from './pages/Teams'
import { SessionProvider } from './session'

export default function App() {
  const [mode, setModeState] = useState<Mode>(() => localStorage.getItem('mode') === 'team' ? 'team' : 'business')
  function setMode(next: Mode) { localStorage.setItem('mode', next); setModeState(next) }
  return <SessionProvider><AgentChatProvider>
    <ScrollToTop />
    <Layout mode={mode} setMode={setMode}>
      <Routes>
        <Route path="/" element={<Landing mode={mode} setMode={setMode} />} />
        <Route path="/catalog" element={<Catalog mode={mode} setMode={setMode} />} />
        <Route path="/task/new" element={<TaskNew />} />
        <Route path="/task/:id/clarify" element={<Clarify />} />
        <Route path="/task/:id/edit" element={<TaskEdit />} />
        <Route path="/task/:id" element={<TaskShow mode={mode} setMode={setMode} />} />
        <Route path="/business" element={<MyTasks />} />
        <Route path="/ai" element={<AiPage />} />
        <Route path="/teams" element={<Teams mode={mode} setMode={setMode} />} />
        <Route path="*" element={<PageError message="Такой страницы нет. Проверьте адрес или откройте каталог." />} />
      </Routes>
    </Layout>
  </AgentChatProvider></SessionProvider>
}

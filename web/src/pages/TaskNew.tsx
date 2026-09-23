import { useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAgentChat } from '../components/AgentChat'
import { readDraft, saveDraft } from '../fields'
/** Сразу открыть диалог с агентом, вход не нужен: он понадобится только при сборке карточки. Пустой текст — агент сам спросит суть в чате. */
export function useStartChat() {
  const { openAgentChat } = useAgentChat()
  return (draft = '', industry = readDraft().industry) => {
    const text = draft.trim()
    if (text) saveDraft(text, industry)
    openAgentChat({ draftText: text, industry: industry.trim() || 'Другое' })
  }
}

/** Совместимость со старыми ссылками: отдельной страницы создания больше нет. */
export function TaskNew() {
  const location = useLocation(); const navigate = useNavigate(); const startChat = useStartChat()
  const opened = useRef(false)
  useEffect(() => {
    if (opened.current) return
    opened.current = true
    const passed = (location.state as { draft?: string } | null)?.draft
    navigate('/catalog', { replace: true })
    startChat(passed)
  }, [location.state, navigate, startChat])
  return null
}

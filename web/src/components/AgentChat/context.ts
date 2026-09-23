import { createContext, useContext } from 'react'
import type { AgentChatStart } from './types'

export interface AgentChatApi {
  /** Открыть диалог: { draftText, industry } создаёт задачу, { taskId } продолжает начатую. Нужен вход заявителя (кроме mock). */
  openAgentChat: (start: AgentChatStart) => void
}

/** Контекст в отдельном модуле без зависимостей: HMR соседних файлов не пересоздаёт его. */
export const AgentChatContext = createContext<AgentChatApi | null>(null)

export function useAgentChat(): AgentChatApi {
  const value = useContext(AgentChatContext)
  if (!value) throw new Error('useAgentChat нужен внутри AgentChatProvider')
  return value
}

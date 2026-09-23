import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import { AgentChat } from './AgentChat'
import { pickTransport } from './transport'
import type { AgentChatStart, Transport } from './types'

export type { AgentChatStart, AgentInput } from './types'

interface AgentChatApi {
  /** Открыть диалог: { draftText, industry } создаёт задачу, { taskId } продолжает начатую. Нужен вход заявителя (кроме mock). */
  openAgentChat: (start: AgentChatStart) => void
}

const AgentChatContext = createContext<AgentChatApi | null>(null)

/** Одна модалка диалога с агентом на всё приложение. Нужны Router, ToastProvider и SessionProvider выше. */
export function AgentChatProvider({ children }: { children: ReactNode }) {
  const [current, setCurrent] = useState<{ key: number; start: AgentChatStart; transport: Transport } | null>(null)
  const openAgentChat = useCallback((start: AgentChatStart) => setCurrent({ key: Date.now(), start, transport: pickTransport() }), [])
  const value = useMemo(() => ({ openAgentChat }), [openAgentChat])
  return <AgentChatContext.Provider value={value}>
    {children}
    {current && <AgentChat key={current.key} start={current.start} transport={current.transport} onClose={() => setCurrent(null)} />}
  </AgentChatContext.Provider>
}

export function useAgentChat(): AgentChatApi {
  const value = useContext(AgentChatContext)
  if (!value) throw new Error('useAgentChat нужен внутри AgentChatProvider')
  return value
}

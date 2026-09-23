import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { api, json, withAuth, type Message } from '../api'
import { errorText } from '../fields'
import { Alert, Button, Spinner, Textarea } from '../ui'

const who = { business: 'Бизнес', team: 'Команда' } as const
const time = (iso: string) => new Date(iso).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

/** Переписка по отклику: история по REST, обновление каждые 5 секунд, пока чат открыт. */
export function ProposalChat({ proposalId, token, me }: { proposalId: number; token?: string; me: 'business' | 'team' }) {
  const [messages, setMessages] = useState<Message[] | null>(null)
  const [text, setText] = useState(''); const [sending, setSending] = useState(false); const [error, setError] = useState('')
  const list = useRef<HTMLOListElement>(null)
  const load = useCallback(async () => {
    try { const { messages } = await api<{ messages: Message[] }>(`/proposals/${proposalId}/messages`, withAuth(token)); setMessages(messages); setError('') }
    catch (err) { setError(errorText(err)); setMessages(current => current ?? []) }
  }, [proposalId, token])
  useEffect(() => { void load(); const timer = window.setInterval(() => void load(), 5000); return () => window.clearInterval(timer) }, [load])
  useEffect(() => { const node = list.current; if (node) node.scrollTop = node.scrollHeight }, [messages?.length])
  async function send() {
    const body = text.trim(); if (!body || sending) return
    setSending(true); setError('')
    try { const message = await api<Message>(`/proposals/${proposalId}/messages`, withAuth(token, json('POST', { text: body }))); setText(''); setMessages(current => current?.some(item => item.id === message.id) ? current : [...(current ?? []), message]) }
    catch (err) { setError(errorText(err)) } finally { setSending(false) }
  }
  function onKey(event: KeyboardEvent<HTMLTextAreaElement>) { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send() } }
  return <div className="chat">
    {messages === null ? <p className="chat-empty"><Spinner size="sm" /> Загружаем переписку…</p>
      : messages.length ? <ol className="chat-list" ref={list} aria-live="polite">{messages.map(item => <li key={item.id} className={`chat-msg${item.author === me ? ' is-mine' : ''}`}>
        <span className="chat-meta">{who[item.author]} · {time(item.created_at)}</span><p>{item.text}</p>
      </li>)}</ol>
      : <p className="chat-empty">{me === 'business' ? 'Напишите команде, чтобы договориться о старте.' : 'Напишите заявителю, чтобы уточнить детали.'}</p>}
    {error && <Alert tone="error">{error}</Alert>}
    <div className="chat-compose">
      <Textarea minRows={1} value={text} onChange={event => setText(event.target.value)} onKeyDown={onKey} placeholder="Сообщение. Enter — отправить, Shift+Enter — новая строка" aria-label="Сообщение" maxLength={2000} />
      <Button variant="primary" loading={sending} disabled={!text.trim()} onClick={() => void send()}>Отправить</Button>
    </div>
  </div>
}

import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { api, json, withAuth, type Message } from '../api'
import { errorText } from '../fields'
import { Alert, Button, Spinner, Textarea } from '../ui'

const who = { business: 'Бизнес', team: 'Команда' } as const
const time = (iso: string) => new Date(iso).toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
type Link = 'connecting' | 'online' | 'offline'
type Frame = { type: 'history'; messages: Message[] } | { type: 'message'; message: Message } | { type: 'error'; error: string } | { type: 'ping' | 'pong' }

const merge = (current: Message[] | null, incoming: Message[]) => {
  const byId = new Map((current ?? []).map(item => [item.id, item]))
  for (const item of incoming) byId.set(item.id, item)
  return [...byId.values()].sort((a, b) => a.id - b.id)
}

/** Переписка по отклику: WebSocket по контракту; при обрыве — история и отправка по REST, опрос раз в 5 с и переподключение. */
export function ProposalChat({ proposalId, token, me }: { proposalId: number; token?: string; me: 'business' | 'team' }) {
  const [messages, setMessages] = useState<Message[] | null>(null)
  const [text, setText] = useState(''); const [sending, setSending] = useState(false); const [error, setError] = useState('')
  const [link, setLink] = useState<Link>('connecting')
  const socket = useRef<WebSocket | null>(null)
  const list = useRef<HTMLOListElement>(null)

  const load = useCallback(async () => {
    try { const { messages } = await api<{ messages: Message[] }>(`/proposals/${proposalId}/messages`, withAuth(token)); setMessages(current => merge(current, messages)); setError('') }
    catch (err) { setError(errorText(err)); setMessages(current => current ?? []) }
  }, [proposalId, token])

  useEffect(() => {
    if (!token || typeof WebSocket === 'undefined') { setLink('offline'); return }
    let alive = true; let retry = 0; let timer = 0
    const connect = () => {
      if (!alive) return
      setLink(current => (current === 'online' ? current : 'connecting'))
      const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/proposals/${proposalId}/ws?token=${encodeURIComponent(token)}`)
      socket.current = ws
      ws.onopen = () => { retry = 0; setLink('online'); setError('') }
      ws.onmessage = event => {
        let frame: Frame
        try { frame = JSON.parse(String(event.data)) as Frame } catch { return }
        if (frame.type === 'history') setMessages(current => merge(current, frame.messages))
        else if (frame.type === 'message') setMessages(current => merge(current, [frame.message]))
        else if (frame.type === 'error') setError(frame.error)
        else if (frame.type === 'ping') ws.send(JSON.stringify({ type: 'pong' }))
      }
      ws.onclose = () => {
        if (socket.current === ws) socket.current = null
        if (!alive) return
        setLink('offline'); void load()
        timer = window.setTimeout(connect, Math.min(15000, 2000 * 2 ** retry++))
      }
    }
    connect()
    return () => { alive = false; window.clearTimeout(timer); socket.current?.close(); socket.current = null }
  }, [proposalId, token, load])

  // Запасной путь: пока сокета нет, история приходит по REST раз в 5 секунд.
  useEffect(() => {
    if (link === 'online') return
    void load(); const timer = window.setInterval(() => void load(), 5000)
    return () => window.clearInterval(timer)
  }, [link, load])

  useEffect(() => { const node = list.current; if (node) node.scrollTop = node.scrollHeight }, [messages?.length])

  async function send() {
    const body = text.trim(); if (!body || sending) return
    const ws = socket.current
    if (ws && ws.readyState === WebSocket.OPEN) { ws.send(JSON.stringify({ type: 'message', text: body })); setText(''); return }
    setSending(true); setError('')
    try { const message = await api<Message>(`/proposals/${proposalId}/messages`, withAuth(token, json('POST', { text: body }))); setText(''); setMessages(current => merge(current, [message])) }
    catch (err) { setError(errorText(err)) } finally { setSending(false) }
  }
  function onKey(event: KeyboardEvent<HTMLTextAreaElement>) { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send() } }
  return <div className="chat">
    <p className={`chat-link chat-link-${link}`} role="status">{link === 'online' ? 'На связи: сообщения приходят сразу' : link === 'connecting' ? 'Подключаемся…' : 'Соединение прервано, переподключаемся. Сообщения можно отправлять.'}</p>
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

import { ApiError, api, json, withToken, type Fields, type Task } from '../../api'
import { fieldSpecs, ratingKey } from '../../fields'
import { mockTransport } from './mock'
import type { AgentQuestion, ChatSession, NextResponse, ResultOption, Step, Transport } from './types'

export const emptyCard = (): Fields => Object.fromEntries(fieldSpecs.map(spec => [spec.key, ''])) as Fields
const toStep = (r: NextResponse): Step => r.done
  ? { done: true, reason: r.reason, asked: r.asked, card: r.card_preview, score: r.score_preview }
  : { done: false, question: r.question, asked: r.asked, card: r.card_preview, score: r.score_preview }

/** Живая карточка для локального режима: черновик в контексте, ответ — в своё поле; баллы — по gain из missing. */
export function localStep(session: ChatSession, reason?: string): Step {
  const local = session.local!
  const card = { ...local.card }
  const earned = new Set<string>()
  for (const q of local.asked) {
    const answer = (local.answers[String(q.id)] || '').trim()
    if (!answer) continue
    card[q.field_key] = card[q.field_key] && q.field_key !== 'context' ? `${card[q.field_key]} ${answer}` : answer
    earned.add(ratingKey(q.field_key))
  }
  const gain = session.missing.filter(item => earned.has(item.key)).reduce((sum, item) => sum + item.gain, 0)
  const question = local.queue[0]
  return { done: !question, question, reason: question ? undefined : reason, asked: local.asked.length + (question ? 1 : 0), card, score: Math.min(100, local.base + gain) }
}

function advance(session: ChatSession, answer: string | undefined) {
  const local = session.local!
  const current = local.queue[0]
  if (current && answer !== undefined) { local.answers[String(current.id)] = answer.trim(); local.asked.push(current); local.queue.shift() }
}

const liveTransport: Transport = {
  async start(input, token) {
    const task = await api<Task>('/tasks', withToken(token, json('POST', { draft_text: input.draftText, industry: input.industry, mode: 'dynamic' })))
    return { ...sessionFromTask(task), anonymous: !token }
  },
  async next(session, answer, token) {
    if (session.mode === 'legacy') { advance(session, answer); return localStep(session, 'Вопросы закончились. Соберу карточку из ваших ответов, её можно будет поправить.') }
    const body = answer === undefined ? {} : { answer }
    return toStep(await api<NextResponse>(`/tasks/${session.taskId}/next-question`, withToken(token, json('POST', body))))
  },
  async resume(taskId, token) {
    return sessionFromTask(await api<Task>(`/tasks/${taskId}`, withToken(token)))
  },
  async resultOptions(session, token) {
    try { return (await api<{ options?: ResultOption[] }>(`/tasks/${session.taskId}/result-options`, withToken(token, json('POST', {})))).options ?? [] }
    catch (err) { if (err instanceof ApiError && err.status === 401) throw err; return [] }
  },
  async applyResult(session, index, token) {
    await claimIfNeeded(session, token)
    await api<Task>(`/tasks/${session.taskId}/apply-result`, withToken(token, json('POST', { index })))
  },
  async finish(session, token) {
    await claimIfNeeded(session, token)
    const answers = session.mode === 'legacy' ? session.local!.answers : {}
    const task = await api<Task>(`/tasks/${session.taskId}/answers`, withToken(token, json('POST', { answers })))
    // Сборка карточки пишет поля заново — выбранный вариант результата кладём поверх.
    if (session.chosen === undefined) return task
    try { return await api<Task>(`/tasks/${session.taskId}/apply-result`, withToken(token, json('POST', { index: session.chosen }))) } catch { return task }
  },
  async claim(session, token) {
    await api<Task>(`/tasks/${session.taskId}/owner`, withToken(token, json('POST', {})))
    session.anonymous = false
  },
}

/** Анонимную задачу сначала присваиваем вошедшему; без токена сервер ответит 401 — AgentChat откроет вход и повторит действие. */
async function claimIfNeeded(session: ChatSession, token: string) {
  if (session.anonymous && token) await liveTransport.claim!(session, token)
}

/** Сессия по задаче: новой (после POST /tasks) или ранее начатой (GET /tasks/{id}). */
export function sessionFromTask(task: Task): ChatSession {
  const questions = task.questions as AgentQuestion[]
  const card = { ...emptyCard(), ...task.fields }
  if (!card.context) card.context = task.draft_text
  // input_type может не прийти (omitempty) — пошаговый режим узнаём и по одному вопросу: батч всегда присылает ≥ 3.
  if (questions.length <= 1 || questions.some(q => q.input_type)) {
    // Открыт последний вопрос без ответа; пропущенные раньше тоже пустые, но за ними уже есть следующий.
    const last = questions[questions.length - 1]
    const open = last && !last.answer ? last : undefined
    const first: Step = { done: !open, question: open, asked: questions.length, card, score: task.score }
    if (!open) first.reason = 'Вопросы уже заданы. Можно собирать карточку.'
    return { taskId: task.id, mode: 'dynamic', missing: task.missing, first, history: open ? questions.slice(0, -1) : questions }
  }
  // Сервер без пошагового режима прислал все вопросы сразу — задаём их по одному сами.
  const session: ChatSession = { taskId: task.id, mode: 'legacy', missing: task.missing, first: undefined as unknown as Step, history: questions.filter(q => q.answer), local: { queue: questions.filter(q => !q.answer).map(q => ({ ...q, input_type: 'text', suggestions: [] })), asked: questions.filter(q => q.answer), answers: Object.fromEntries(questions.map(q => [String(q.id), q.answer || ''])), base: task.score, card } }
  session.first = localStep(session)
  return session
}

/** Mock включается параметром ?agent-mock в адресе или localStorage agent_mock=1. */
export function pickTransport(): Transport {
  let flag = new URLSearchParams(window.location.search).has('agent-mock')
  try { flag ||= localStorage.getItem('agent_mock') === '1' } catch { /* storage unavailable */ }
  return flag ? mockTransport : liveTransport
}

import type { Fields, Missing } from '../../api'
import type { AgentQuestion, ChatSession, NextResponse, Step, Transport } from './types'

/** Демо-вопросы того же формата, что отдаёт POST /tasks/{id}/next-question: по одному на каждый input_type. */
export const MOCK_QUESTIONS: AgentQuestion[] = [
  { id: 1, field_key: 'users', answer: '', input_type: 'text', text: 'Кто будет пользоваться решением каждый день?', suggestions: ['Администраторы', 'Руководитель', 'Клиенты'] },
  { id: 2, field_key: 'data', answer: '', input_type: 'choice', text: 'Какие данные вы можете передать команде?', suggestions: ['Выгрузка из Excel', 'Доступ к CRM', 'Только примеры', 'Пока ничего'] },
  { id: 3, field_key: 'expected_result', answer: '', input_type: 'multi', text: 'Что команда должна передать в конце? Можно выбрать несколько.', suggestions: ['Рабочий прототип', 'Исходный код', 'Инструкция', 'Презентация'] },
  { id: 4, field_key: 'interaction_format', answer: '', input_type: 'yes_no', text: 'Готовы созваниваться с командой раз в неделю?', suggestions: ['Да', 'Нет'] },
  { id: 5, field_key: 'success_criteria', answer: '', input_type: 'text', text: 'Как поймёте, что решение работает?', suggestions: ['Ни одна заявка не теряется', 'Отчёт собирается за минуту'] },
]

export const MOCK_MISSING: Missing[] = [
  { key: 'users', label: 'Пользователи', gain: 10, hint: '' },
  { key: 'data', label: 'Данные', gain: 15, hint: '' },
  { key: 'expected_result', label: 'Результат', gain: 15, hint: '' },
  { key: 'business_link', label: 'Связь с бизнесом', gain: 10, hint: '' },
  { key: 'success_criteria', label: 'Критерии успеха', gain: 15, hint: '' },
]

/** Первый ответ next-question — образец формата. */
export const MOCK_NEXT: NextResponse = {
  done: false, question: MOCK_QUESTIONS[1], asked: 2, score_preview: 32,
  card_preview: { title: '', context: 'Языковой центр вырос из Excel', need: '', users: 'Администраторы', data: '', constraints: '', expected_result: '', success_criteria: '', contact: '', interaction_format: '' } as Fields,
}

const wait = (ms: number) => new Promise(resolve => window.setTimeout(resolve, ms))
let calls = 0
/** ?agent-mock=flaky — каждый третий запрос падает, чтобы проверить повтор. */
async function latency() {
  calls += 1
  await wait(600 + Math.random() * 500)
  if (new URLSearchParams(window.location.search).get('agent-mock') === 'flaky' && calls % 3 === 0) throw new Error('Не удалось связаться с сервером.')
}

function step(session: ChatSession): Step {
  const local = session.local!
  const card = { ...local.card }
  let score = local.base
  for (const q of local.asked) {
    const answer = local.answers[String(q.id)]
    if (!answer) continue
    card[q.field_key] = answer
    score += MOCK_MISSING.find(item => item.key === (q.field_key === 'interaction_format' ? 'business_link' : q.field_key))?.gain ?? 0
  }
  const question = local.queue[0]
  const stop = !question || (local.asked.length >= 4 && !local.answers.more)
  if (stop) return { done: true, reason: 'Этого хватит, чтобы команда поняла задачу: есть пользователи, данные и ожидаемый результат.', asked: local.asked.length, card, score: Math.min(100, score) }
  return { done: false, question, asked: local.asked.length + 1, card, score: Math.min(100, score) }
}

export const mockTransport: Transport = {
  async start(input) {
    await latency()
    const card = { title: '', context: input.draftText, need: '', users: '', data: '', constraints: '', expected_result: '', success_criteria: '', contact: '', interaction_format: '' } as Fields
    const session: ChatSession = { taskId: 0, mode: 'mock', missing: MOCK_MISSING, first: undefined as unknown as Step, history: [], local: { queue: MOCK_QUESTIONS.map(q => ({ ...q })), asked: [], answers: {}, base: 22, card } }
    session.first = step(session)
    return session
  },
  async next(session, answer) {
    await latency()
    const local = session.local!
    if (answer === undefined) local.answers.more = '1'
    else { const q = local.queue.shift(); if (q) { local.answers[String(q.id)] = answer.trim(); local.asked.push(q) } }
    return step(session)
  },
  async resume() {
    throw new Error('В демо-режиме продолжить задачу нельзя.')
  },
  async finish(session) {
    await latency()
    return { id: 0, score: step(session).score }
  },
}

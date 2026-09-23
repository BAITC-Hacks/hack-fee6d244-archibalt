import type { Fields, Missing, Question, Task } from '../../api'

export type InputType = 'text' | 'choice' | 'multi' | 'yes_no'
/** Вопрос пошагового режима: к обычному добавлены тип ввода и подсказки. */
export interface AgentQuestion extends Question { input_type?: InputType; suggestions?: string[] }

/** Ответ POST /tasks/{id}/next-question. */
export type NextResponse =
  | { done: false; question: AgentQuestion; asked: number; card_preview: Fields; score_preview: number }
  | { done: true; reason: string; asked: number; card_preview: Fields; score_preview: number }

/** Вариант первого результата (POST /tasks/{id}/result-options). */
export interface ResultOption { title: string; result: string; check: string; needs: string; weeks: number }

/** Визуальный концепт выбранного результата: src — адрес для <img>, mock — заглушка без ключа OpenAI. */
export interface Visual { src: string; mock: boolean }

export interface AgentInput { draftText: string; industry: string }
/** Что открыть: новый диалог по черновику или продолжение задачи. */
export type AgentChatStart = AgentInput | { taskId: number }

/** Шаг диалога после любого запроса: следующий вопрос или финал, плюс живая карточка. */
export interface Step { question?: AgentQuestion; done: boolean; reason?: string; asked: number; card: Fields; score: number }

/** Состояние диалога с задачей. mode: dynamic — живой API; legacy — старый батч-API, вопросы по одному на клиенте; mock — без сервера. */
export interface ChatSession {
  taskId: number
  mode: 'dynamic' | 'legacy' | 'mock'
  missing: Missing[]
  first: Step
  /** Уже заданные вопросы (при продолжении), чтобы восстановить ленту. */
  history: AgentQuestion[]
  /** Индекс выбранного варианта результата: после сборки карточки применяется ещё раз, чтобы сборка его не затёрла. */
  chosen?: number
  /** Только legacy/mock: очередь вопросов и ответы, которые копятся на клиенте. */
  local?: { queue: AgentQuestion[]; asked: AgentQuestion[]; answers: Record<string, string>; base: number; card: Fields }
}

export interface Transport {
  start(input: AgentInput, token: string): Promise<ChatSession>
  /** answer: строка — ответ ('' — пропуск), undefined — «спросить ещё» после финала. */
  next(session: ChatSession, answer: string | undefined, token: string): Promise<Step>
  /** Продолжить начатую задачу: GET /tasks/{id}. */
  resume(taskId: number, token: string): Promise<ChatSession>
  /** Варианты первого результата; [] — шаг пропускается (эндпоинта нет или AI не ответил). */
  resultOptions(session: ChatSession, token: string): Promise<ResultOption[]>
  /** Перенести вариант в «Ожидаемый результат» и «Критерии успеха». */
  applyResult(session: ChatSession, index: number, token: string): Promise<void>
  finish(session: ChatSession, token: string): Promise<Pick<Task, 'id' | 'score'>>
  /** Необязательно: нарисовать концепт выбранного результата (POST /tasks/{id}/visual). Нет метода — кнопки нет. */
  visual?(session: ChatSession, token: string): Promise<Visual>
}

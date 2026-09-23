export type Level = 'draft' | 'working' | 'ready' | 'priority'
export type FieldKey = 'title' | 'context' | 'need' | 'users' | 'data' | 'constraints' | 'expected_result' | 'success_criteria' | 'contact' | 'interaction_format'
export type Fields = Record<FieldKey, string>
export interface Breakdown { key: string; label: string; weight: number; earned: number; reason: string }
export interface Missing { key: string; label: string; gain: number; hint: string }
export interface Question { id: number; text: string; field_key: FieldKey; answer: string }
export interface Team { id: number; name: string; contact?: string; skills: string[]; interests: string[]; tech: string[]; points: number }
export interface Proposal { id: number; task_id: number; team: { id: number; name: string }; idea: string; plan: string; deadline: string; link: string; status: 'new' | 'selected' | 'accepted' | 'declined' | 'rejected' | 'on_hold'; stage_confirmed: boolean; created_at: string; accepted_at?: string | null; messages_count?: number }
export interface Task { id: number; industry: string; status: 'draft' | 'clarifying' | 'editing' | 'published'; draft_text: string; fields: Fields; confirmed: boolean; score: number; level: Level; level_label: string; breakdown: Breakdown[]; missing: Missing[]; questions: Question[]; ai_mode: 'openai' | 'mock'; created_at: string; published_at: string | null; proposals?: Proposal[]; owner_contact?: string; rank?: number; rank_if_confirmed?: number; catalog_size?: number; previous_score?: number | null; next_level_gain?: number; proposals_count?: number; has_visual?: boolean }
export interface CatalogResponse { tasks: Task[]; industries: string[]; levels: { key: string; label: string }[]; stats?: { tasks: number; proposals: number; teams?: number } }
export interface LastCall { at: string; industry: string; draft: string; questions: Question[]; raw_fields: Partial<Fields>; fields: Partial<Fields>; removed_by_guard: FieldKey[] }
export interface AiInfo { mode: string; prompt_questions: string; prompt_card: string; schema_example: string; last_error: string | null; last_call?: LastCall | null }

export interface Message { id: number; author: 'team' | 'business'; text: string; created_at: string }

/** Ошибка API с HTTP-статусом: 401 — нужен вход, 403 — нет прав. */
export class ApiError extends Error { constructor(message: string, readonly status: number) { super(message) } }

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  let response: Response
  try { response = await fetch(`/api${path}`, { ...options, headers: { 'Content-Type': 'application/json', ...options?.headers } }) }
  catch { throw new Error('Не удалось связаться с сервером. Проверьте подключение и попробуйте ещё раз.') }
  if (!response.ok) {
    let message = `Ошибка ${response.status}. Попробуйте ещё раз.`
    try { const body = await response.json() as { error?: string }; if (body.error) message = body.error } catch { /* use status message */ }
    throw new ApiError(message, response.status)
  }
  return response.json() as Promise<T>
}
export const json = (method: 'POST' | 'PUT', body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })
/** Пустой токен — без заголовка Authorization (анонимный диалог). */
export const withToken = (token: string, options: RequestInit = {}): RequestInit => (token ? { ...options, headers: { ...options.headers, Authorization: `Bearer ${token}` } } : options)
export const withAuth = (token: string | undefined, options: RequestInit = {}): RequestInit => (token ? withToken(token, options) : options)

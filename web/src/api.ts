export type Level = 'draft' | 'working' | 'ready' | 'priority'
export type FieldKey = 'title' | 'context' | 'need' | 'users' | 'data' | 'constraints' | 'expected_result' | 'success_criteria' | 'contact' | 'interaction_format'
export type Fields = Record<FieldKey, string>
export interface Breakdown { key: string; label: string; weight: number; earned: number; reason: string }
export interface Missing { key: string; label: string; gain: number; hint: string }
export interface Question { id: number; text: string; field_key: FieldKey; answer: string }
export interface Team { id: number; name: string; skills: string[]; interests: string[]; tech: string[]; points: number }
export interface Proposal { id: number; task_id: number; team: { id: number; name: string }; idea: string; plan: string; deadline: string; link: string; status: 'new' | 'selected' | 'rejected'; stage_confirmed: boolean; created_at: string }
export interface Task { id: number; industry: string; status: 'draft' | 'clarifying' | 'editing' | 'published'; draft_text: string; fields: Fields; confirmed: boolean; score: number; level: Level; level_label: string; breakdown: Breakdown[]; missing: Missing[]; questions: Question[]; ai_mode: 'openai' | 'mock'; created_at: string; published_at: string | null; proposals?: Proposal[] }
export interface CatalogResponse { tasks: Task[]; industries: string[]; levels: { key: string; label: string }[] }
export interface AiInfo { mode: string; prompt_questions: string; prompt_card: string; schema_example: string; last_error: string | null }

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  let response: Response
  try { response = await fetch(`/api${path}`, { ...options, headers: { 'Content-Type': 'application/json', ...options?.headers } }) }
  catch { throw new Error('Не удалось связаться с сервером. Проверьте подключение и попробуйте ещё раз.') }
  if (!response.ok) {
    let message = `Ошибка ${response.status}. Попробуйте ещё раз.`
    try { const body = await response.json() as { error?: string }; if (body.error) message = body.error } catch { /* use status message */ }
    throw new Error(message)
  }
  return response.json() as Promise<T>
}
export const json = (method: 'POST' | 'PUT', body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })

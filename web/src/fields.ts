import type { FieldKey, Team } from './api'

export type Mode = 'business' | 'team'
export type TeamSession = { token: string; team: Team }

export const fieldSpecs: { key: FieldKey; label: string; hint: string }[] = [
  { key: 'title', label: 'Название', hint: 'Коротко назовите задачу' },
  { key: 'context', label: 'Контекст', hint: 'Что происходит сейчас?' },
  { key: 'need', label: 'Потребность', hint: 'Что нужно изменить или решить?' },
  { key: 'users', label: 'Пользователи', hint: 'Для кого создаётся решение?' },
  { key: 'data', label: 'Данные и материалы', hint: 'Какие данные, примеры или источники доступны?' },
  { key: 'constraints', label: 'Ограничения', hint: 'Сроки, технологии, доступы и другие границы' },
  { key: 'expected_result', label: 'Ожидаемый результат', hint: 'Что команда должна передать в конце?' },
  { key: 'success_criteria', label: 'Критерии успеха', hint: 'Как поймёте, что решение работает?' },
  { key: 'contact', label: 'Контакт', hint: 'Как с вами связаться?' },
  { key: 'interaction_format', label: 'Формат взаимодействия', hint: 'Как будете консультировать команду и давать обратную связь?' },
]

export const capitalize = (text: string) => (text ? text[0].toUpperCase() + text.slice(1) : text)
export const errorText = (err: unknown) => (err instanceof Error ? err.message : 'Что-то пошло не так. Попробуйте ещё раз.')

export function plural(n: number, one: string, few: string, many: string) {
  const mod10 = n % 10, mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return one
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few
  return many
}

/** Ключ автосохранения черновика задачи в localStorage. */
export const DRAFT_KEY = 'task_draft'
export const readDraft = (): { text: string; industry: string } => {
  try { const saved = JSON.parse(localStorage.getItem(DRAFT_KEY) || '{}') as { text?: string; industry?: string }; return { text: saved.text || '', industry: saved.industry || '' } } catch { return { text: '', industry: '' } }
}
export const saveDraft = (text: string, industry: string) => { try { if (text || industry) localStorage.setItem(DRAFT_KEY, JSON.stringify({ text, industry })); else localStorage.removeItem(DRAFT_KEY) } catch { /* storage unavailable */ } }

/** Поле карточки → показатель рейтинга (missing/breakdown). */
export const ratingKey = (field: string) => ({ context: 'context_need', need: 'context_need', contact: 'business_link', interaction_format: 'business_link', title: '' } as Record<string, string>)[field] ?? field

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

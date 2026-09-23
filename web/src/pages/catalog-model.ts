import type { Task } from '../api'

/** API отдаёт весь каталог; фильтруем локально, сохраняя глобальные ранги. */
export function selectTasks(tasks: Task[], filters: { industry: string; level: string; search: string; sort: string }) {
  const normalize = (text: string) => text.toLocaleLowerCase('ru').replaceAll('ё', 'е')
  const words = normalize(filters.search).trim().split(/\s+/).filter(Boolean)
  const result = tasks.filter(task => {
    if (filters.industry && task.industry !== filters.industry) return false
    if (filters.level && task.level !== filters.level) return false
    const text = normalize([task.industry, task.fields.title, task.fields.need, task.fields.context, task.fields.expected_result, task.fields.data, task.fields.users].join(' '))
    return words.every(word => text.includes(word))
  })
  if (filters.sort === 'newest') result.sort((a, b) => (Date.parse(b.published_at || b.created_at) || 0) - (Date.parse(a.published_at || a.created_at) || 0))
  else if (filters.sort === 'proposals') result.sort((a, b) => (a.proposals_count ?? 0) - (b.proposals_count ?? 0))
  return result
}

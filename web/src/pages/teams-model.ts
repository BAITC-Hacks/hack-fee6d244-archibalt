import type { Team } from '../api'

/** Поиск и фильтры работают по уже загруженному открытому списку команд. */
export function selectTeams(teams: Team[], filters: { search: string; interest: string; sort: string }) {
  const normalize = (value: string) => value.toLocaleLowerCase('ru').replaceAll('ё', 'е')
  const words = normalize(filters.search).trim().split(/\s+/).filter(Boolean)
  return teams.filter(team => {
    if (filters.interest && !team.interests.includes(filters.interest)) return false
    const text = normalize([team.name, ...team.skills, ...team.interests, ...team.tech].join(' '))
    return words.every(word => text.includes(word))
  }).sort((a, b) => filters.sort === 'name' ? a.name.localeCompare(b.name, 'ru') || a.id - b.id : b.points - a.points || a.id - b.id)
}

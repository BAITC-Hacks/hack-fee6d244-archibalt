// node --experimental-strip-types web/teams-check.mjs
import assert from 'node:assert/strict'
import { selectTeams } from './src/pages/teams-model.ts'
const teams = [
  { id: 3, name: 'Ясно', skills: ['учёт заявок'], interests: ['сервис'], tech: ['React'], points: 10 },
  { id: 1, name: 'Альфа', skills: ['анализ данных'], interests: ['образование'], tech: ['Python', 'SQL'], points: 0 },
  { id: 2, name: 'Новая команда', skills: [], interests: [], tech: [], points: 10 },
]
const pick = (filters = {}) => selectTeams(teams, { search: '', interest: '', sort: '', ...filters }).map(team => team.id)
assert.deepEqual(pick({ search: '  УЧЕТ   react ' }), [3])
assert.deepEqual(pick({ search: 'АЛЬФА Python' }), [1])
assert.deepEqual(pick({ search: 'образование SQL', interest: 'образование' }), [1])
assert.deepEqual(pick({ search: 'Python', interest: 'сервис' }), [])
assert.deepEqual(pick({ search: 'несуществующее' }), [])
assert.deepEqual(pick({ search: 'новая' }), [2])
assert.deepEqual(pick(), [2, 3, 1])
assert.deepEqual(pick({ sort: 'name' }), [1, 2, 3])
assert.deepEqual(pick({ sort: 'unknown' }), [2, 3, 1])
assert.deepEqual(teams.map(team => team.id), [3, 1, 2])
assert.deepEqual(selectTeams([], { search: '', interest: '', sort: '' }), [])
console.log('Teams search, combined filters, sorting and empty profiles: PASS')

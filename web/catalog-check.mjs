// node --experimental-strip-types catalog-check.mjs
import assert from 'node:assert/strict'
import { selectTasks } from './src/pages/catalog-model.ts'
const tasks = [
  { id: 1, rank: 1, industry: 'Сервис', level: 'priority', fields: { title: 'Учёт заявок', need: 'Очередь ремонта' }, proposals_count: 3, created_at: '2026-09-21' },
  { id: 2, rank: 2, industry: 'Экология', level: 'draft', fields: { title: 'Сбор сырья', need: 'Доставка' }, proposals_count: 0, created_at: '2026-09-23' },
]
const pick = (overrides = {}) => selectTasks(tasks, { industry: '', level: '', search: '', sort: '', ...overrides })
assert.deepEqual(pick({ search: '  УЧЕТ ремонт ' }).map(task => task.id), [1])
assert.deepEqual(pick({ industry: 'Экология', level: 'draft', search: 'доставка' }).map(task => task.rank), [2])
assert.equal(pick({ industry: 'Экология', level: 'ready' }).length, 0)
assert.equal(pick({ sort: 'newest' })[0].id, 2)
assert.equal(pick({ sort: 'proposals' })[0].id, 2)
assert.deepEqual(pick().map(task => task.id), [1, 2])
assert.equal(tasks[0].id, 1)
console.log('Catalog search, filters, sorting and global ranks: PASS')

// Run Lightpanda on :9222, then node web/task-edit-check.mjs [base URL] [task ID].
// All PUT/POST requests are intercepted; the real task is never changed.
import assert from 'node:assert/strict'
const base = process.argv[2] || 'http://127.0.0.1:8080'
const taskId = Number(process.argv[3] || 48)
const socket = new WebSocket('ws://127.0.0.1:9222')
await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject })
let id = 0
const pending = new Map()
socket.onmessage = event => {
  const data = JSON.parse(event.data)
  if (!pending.has(data.id)) return
  const { resolve, reject, timer } = pending.get(data.id)
  clearTimeout(timer); pending.delete(data.id)
  if (data.error) reject(new Error(data.error.message)); else resolve(data.result)
}
function send(method, params = {}, sessionId) {
  return new Promise((resolve, reject) => {
    const next = ++id
    const timer = setTimeout(() => { pending.delete(next); reject(new Error(`CDP timeout: ${method}`)) }, 15000)
    pending.set(next, { resolve, reject, timer })
    socket.send(JSON.stringify({ id: next, method, params, ...(sessionId ? { sessionId } : {}) }))
  })
}
let targetId
try {
  targetId = (await send('Target.createTarget', { url: 'about:blank' })).targetId
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
  const call = (method, params) => send(method, params, sessionId)
  await call('Page.enable')
  const evaluate = async expression => {
    const result = await call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails))
    return result.result.value
  }
  const wait = async expression => {
    for (let i = 0; i < 100; i++) {
      if (await evaluate(expression)) return
      await new Promise(resolve => setTimeout(resolve, 100))
    }
    throw new Error(`UI timeout: ${expression}`)
  }
  await call('Page.navigate', { url: `${base}/task/${taskId}/edit` })
  await wait(`Boolean(document.querySelector('.task-editor #title'))`)
  const initial = await evaluate(`Object.fromEntries(Array.from(document.querySelectorAll('.task-editor-form input,.task-editor-form textarea')).map(el => [el.id,el.value]))`)
  assert.equal(Object.keys(initial).length, 10)
  assert.equal(await evaluate(`document.querySelectorAll('.task-editor-section').length`), 3)
  await evaluate(`(async () => { window.fixture = await (await fetch('/api/tasks/${taskId}')).json(); window.calls = []; window.failPublish = true; const realFetch = window.fetch; window.fetch = (url, opts = {}) => { if (['PUT','POST'].includes(opts.method)) { window.calls.push({url: String(url), body: opts.body ? JSON.parse(opts.body) : null}); const isSave = String(url).endsWith('/fields'); if (isSave) window.fixture = {...window.fixture,fields: JSON.parse(opts.body).fields,score:50,confirmed:false,status:'editing'}; return new Promise(resolve => setTimeout(() => resolve(new Response(JSON.stringify(!isSave && window.failPublish ? {error:'Проверочный сбой публикации'} : {...window.fixture,confirmed:!isSave,rank:3}),{status:!isSave && window.failPublish ? 500 : 200,headers:{'Content-Type':'application/json'}})), 200)); } return realFetch(url, opts); }; })()` )
  await evaluate(`const field = document.getElementById('title'); Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(field,'Проверка редактора'); field.dispatchEvent(new Event('input',{bubbles:true}))`)
  await wait(`document.querySelector('.task-editor-save-state').textContent.includes('несохранённые')`)
  await evaluate(`document.querySelector('.task-editor-buttons button[type="submit"]').click()`)
  await wait(`document.querySelector('fieldset').disabled`)
  await wait(`!document.querySelector('fieldset').disabled && window.calls.length === 1`)
  const saved = await evaluate(`window.calls[0].body.fields`)
  assert.deepEqual(saved, {...initial, title:'Проверка редактора'})
  assert.equal(await evaluate(`document.querySelector('.task-editor-save-state').textContent.includes('Все изменения сохранены')`), true)
  assert.equal(await evaluate(`document.querySelector('[role="progressbar"]').getAttribute('aria-valuenow')`), '50')
  await evaluate(`document.querySelector('.task-editor-buttons .ui-btn-primary').click()`)
  await wait(`Boolean(document.querySelector('.task-editor-dock .ui-alert-error')) && !document.querySelector('fieldset').disabled`)
  assert.deepEqual(await evaluate(`window.calls.map(c => c.url)`), [`/api/tasks/${taskId}/fields`,`/api/tasks/${taskId}/fields`,`/api/tasks/${taskId}/confirm`])
  assert.equal(await evaluate(`location.pathname`), `/task/${taskId}/edit`)
  assert.equal(await evaluate(`document.getElementById('title').value`), 'Проверка редактора')
  await evaluate(`window.failPublish = false; document.querySelector('.task-editor-buttons .ui-btn-primary').click()`)
  await wait(`location.pathname === '/task/${taskId}'`)
  assert.equal(await evaluate(`window.calls.length`), 5)
  console.log('Editor: ten fields, grouped UI, save, busy guard, rating update, publish failure/retry and request order: PASS')
} finally {
  if (targetId) await send('Target.closeTarget', { targetId })
  socket.close()
}

// Start Lightpanda on :9222, then: node web/chat-entry-check.mjs http://127.0.0.1:8080
import assert from 'node:assert/strict'
const base = process.argv[2] || 'http://127.0.0.1:8080'
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
  await call('Page.navigate', { url: `${base}/teams` })
  await wait(`Boolean(document.querySelector('button.cta-pill'))`)
  // Isolated browser target: no real task is created and no login code is requested.
  await evaluate(`localStorage.removeItem('business_token'); localStorage.removeItem('task_draft'); window.taskPosts = 0; const originalFetch = window.fetch; window.fetch = (url, options = {}) => { if (String(url) === '/api/tasks' && options.method === 'POST') { window.taskPosts++; return Promise.resolve(new Response(JSON.stringify({error: 'Войдите как заявитель'}), {status: 401, headers: {'Content-Type': 'application/json'}})); } return originalFetch(url, options); }`)
  await evaluate(`document.querySelector('button.cta-pill').click()`)
  await wait(`Boolean(document.querySelector('.agent-dialog textarea'))`)
  assert.equal(await evaluate(`location.pathname`), '/teams')
  assert.equal(await evaluate(`document.querySelectorAll('[role="dialog"]').length`), 1)
  assert.equal(await evaluate(`window.taskPosts`), 0)
  assert.equal(await evaluate(`document.querySelector('.agent-card').hidden`), true)
  assert.equal(await evaluate(`document.querySelector('.agent-send').getAttribute('aria-label')`), 'Отправить сообщение')
  await evaluate(`document.querySelector('.agent-card-toggle').click()`)
  await wait(`!document.querySelector('.agent-card').hidden && document.querySelector('.agent-card-toggle').getAttribute('aria-expanded') === 'true'`)
  await evaluate(`document.querySelector('.agent-card-toggle').click()`)
  await wait(`document.querySelector('.agent-card').hidden && document.querySelector('.agent-card-toggle').getAttribute('aria-expanded') === 'false'`)
  await evaluate(`const input = document.querySelector('.agent-dialog textarea'); Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set.call(input, 'Проверочный черновик'); input.dispatchEvent(new Event('input', {bubbles: true}))`)
  await wait(`JSON.parse(localStorage.getItem('task_draft') || '{}').text === 'Проверочный черновик'`)
  await evaluate(`document.querySelector('.agent-head .ui-modal-close').click()`)
  await wait(`!document.querySelector('.agent-dialog')`)
  assert.equal(await evaluate(`location.pathname`), '/teams')
  await evaluate(`document.querySelector('button.cta-pill').click()`)
  await wait(`document.querySelector('.agent-dialog textarea')?.value === 'Проверочный черновик'`)
  await evaluate(`document.querySelector('.agent-composer button[type="submit"]').click()`)
  await wait(`Boolean(document.querySelector('.ui-modal'))`)
  assert.equal(await evaluate(`window.taskPosts`), 1)
  await evaluate(`document.querySelector('.ui-modal .ui-modal-close').click()`)
  await wait(`!document.querySelector('.ui-modal')`)
  assert.equal(await evaluate(`document.body.classList.contains('ui-scroll-lock')`), true)
  assert.equal(await evaluate(`document.querySelector('.agent-bubble-error button').disabled`), false)
  await evaluate(`document.querySelector('.agent-bubble-error button').click()`)
  await wait(`Boolean(document.querySelector('.ui-modal'))`)
  assert.equal(await evaluate(`window.taskPosts`), 2)
  await evaluate(`document.querySelector('.ui-modal .ui-modal-close').click()`)
  await wait(`!document.querySelector('.ui-modal')`)
  await evaluate(`document.querySelector('.agent-head .ui-modal-close').click()`)
  await wait(`!document.querySelector('.agent-dialog') && !document.body.classList.contains('ui-scroll-lock')`)
  await call('Page.navigate', { url: `${base}/task/new` })
  await wait(`location.pathname === '/catalog' && Boolean(document.querySelector('.agent-dialog textarea'))`)
  assert.equal(await evaluate(`document.querySelectorAll('.agent-dialog').length`), 1)
  assert.equal(await evaluate(`Boolean(document.querySelector('.paper-form'))`), false)
  console.log('Direct chat, unchanged page, draft restore, login overlay and old URL: PASS')
} finally {
  if (targetId) await send('Target.closeTarget', { targetId })
  socket.close()
}

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
    const result = await call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }).catch(error => { throw new Error(`${error.message}: ${expression}`) })
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
  await call('Page.navigate', { url: base })
  await wait(`Boolean(document.querySelector('.capsule-mode button'))`)
  const chooseMode = async label => {
    await evaluate(`Array.from(document.querySelectorAll('.capsule-mode button')).find(button => button.textContent === ${JSON.stringify(label)}).click()`)
    await wait(`document.querySelector('.capsule-mode [aria-pressed="true"]').textContent === ${JSON.stringify(label)}`)
  }
  await chooseMode('Я студент')
  await wait(`document.querySelector('h1').textContent.includes('Ваши навыки')`)
  await evaluate(`document.querySelector('.capsule-burger').click()`)
  await wait(`!document.querySelector('#capsule-panel').hidden`)
  assert.equal(await evaluate(`Boolean(document.querySelector('#capsule-panel [aria-label="Войти как команда"]'))`), true)
  await evaluate(`Array.from(document.querySelectorAll('#capsule-panel .ui-segmented button')).find(button => button.textContent === 'Я бизнес').click()`)
  await wait(`document.querySelector('#capsule-panel').hidden && document.querySelector('h1').textContent.includes('От проблемы бизнеса')`)
  await chooseMode('Я студент')
  assert.equal(await evaluate(`Boolean(document.querySelector('[role="dialog"]'))`), false)
  await evaluate(`document.querySelector('[aria-label="Войти как команда"]').click()`)
  await wait(`document.querySelector('.ui-modal')?.textContent.includes('Войти как команда')`)
  await evaluate(`document.querySelector('.ui-modal-close').click()`)
  await wait(`!document.querySelector('[role="dialog"]')`)
  assert.equal(await evaluate(`document.querySelector('.hero-secondary .ui-btn-primary').getAttribute('href')`), '/catalog')
  assert.equal(await evaluate(`document.querySelector('.landing-steps').textContent.includes('Предложите решение')`), true)
  assert.equal(await evaluate(`document.querySelector('.faq').textContent.includes('нулём баллов')`), true)
  assert.equal(await evaluate(`localStorage.getItem('mode')`), 'team')
  await call('Page.navigate', { url: base })
  await wait(`document.querySelector('h1')?.textContent.includes('Ваши навыки')`)
  await evaluate(`document.querySelector('.hero-secondary .ui-btn-primary').click()`)
  await wait(`location.pathname === '/catalog' && Boolean(document.querySelector('.project-card'))`)
  assert.equal(await evaluate(`document.querySelector('.project-card-footer a').getAttribute('href').endsWith('#respond')`), true)
  await evaluate(`document.querySelector('.project-card-footer a').click()`)
  await wait(`Boolean(document.querySelector('#respond form'))`)
  await chooseMode('Я бизнес')
  await wait(`!document.querySelector('#respond') && document.querySelector('.task-action-panel').textContent.includes('Есть похожая задача?')`)
  assert.equal(await evaluate(`Boolean(document.querySelector('[role="dialog"]'))`), false)
  await chooseMode('Я студент')
  await wait(`Boolean(document.querySelector('#respond form'))`)
  await call('Page.navigate', { url: `${base}/catalog?q=несуществующийпроект&sort=newest` })
  await wait(`Boolean(document.querySelector('.catalog-intro'))`)
  await chooseMode('Я бизнес')
  assert.equal(await evaluate(`decodeURIComponent(location.search)`), '?q=несуществующийпроект&sort=newest')
  assert.equal(await evaluate(`document.querySelector('h1').textContent.includes('Задачи бизнеса')`), true)
  await chooseMode('Я студент')
  assert.equal(await evaluate(`document.querySelector('input[type="search"]').value`), 'несуществующийпроект')
  await call('Page.navigate', { url: `${base}/teams` })
  await wait(`document.querySelector('h1')?.textContent.includes('Сообщество')`)
  assert.equal(await evaluate(`document.querySelector('.teams-start a').getAttribute('href')`), '/catalog')
  // Empty catalog should keep the student journey; no API writes.
  await evaluate(`const original = window.fetch; window.fetch = (url, options) => String(url) === '/api/tasks' ? Promise.resolve(new Response(JSON.stringify({tasks: [], industries: [], levels: []}), {headers: {'Content-Type': 'application/json'}})) : original(url, options); document.querySelector('.teams-start a').click()`)
  await wait(`Boolean(document.querySelector('.catalog-projects .ui-empty'))`)
  assert.equal(await evaluate(`document.querySelector('.catalog-projects').textContent.includes('Посмотреть команды')`), true)
  assert.equal(await evaluate(`document.querySelector('.catalog-projects').textContent.includes('Обсудить задачу')`), false)
  await chooseMode('Я бизнес')
  await call('Page.navigate', { url: base })
  await wait(`document.querySelector('h1')?.textContent.includes('От проблемы бизнеса')`)
  await evaluate(`document.querySelector('[aria-label="Войти как бизнес"]').click()`)
  await wait(`document.querySelector('.ui-modal')?.textContent.includes('Войти как заявитель')`)
  await evaluate(`document.querySelector('.ui-modal-close').click()`)
  await wait(`!document.querySelector('[role="dialog"]')`)
  assert.equal(await evaluate(`document.querySelector('.hero-secondary .ui-btn-primary').textContent`), 'Обсудить задачу')
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
  console.log('Role personalization, persistence, catalog filters, task response, empty catalog, direct chat, draft restore, login overlay and old URL: PASS')
} finally {
  if (targetId) await send('Target.closeTarget', { targetId })
  socket.close()
}

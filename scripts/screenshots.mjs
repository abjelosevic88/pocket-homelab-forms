// README screenshots and a browser smoke test, against a demo instance (DEMO_DATA=true seeds months of forms and answers):
//
//   DATA_DIR=/tmp/forms-demo DEMO_DATA=true ADMIN_EMAIL=demo@example.com ADMIN_PASSWORD=demodemo1 ADMIN_NAME=Lejla PORT=18248 ./pocket-forms
//   CDP=http://127.0.0.1:9338 BASE=http://127.0.0.1:18248 node scripts/screenshots.mjs   # any Chrome with remote debugging
//   SHOTS=0 … node scripts/screenshots.mjs                                                # checks only
// playwright-core comes from web/node_modules, or set PW_FROM to a package.json next to an install of it.
import { createRequire } from 'node:module'
import assert from 'node:assert/strict'

const require = createRequire(process.env.PW_FROM || new URL('../web/package.json', import.meta.url))
let pw
try { pw = require('playwright') } catch { pw = require('playwright-core') }
const { chromium } = pw
const base = process.env.BASE || 'http://127.0.0.1:18248'
const shots = process.env.SHOTS !== '0'
const only = process.env.ONLY || ''
const dir = new URL('../docs/screenshots/', import.meta.url).pathname
const browser = process.env.CDP ? await chromium.connectOverCDP(process.env.CDP) : await chromium.launch()
const errors = []

async function session({ phone = false, dark = false } = {}) {
  const ctx = await browser.newContext({
    viewport: phone ? { width: 390, height: 844 } : { width: 1440, height: 900 },
    deviceScaleFactor: phone ? 3 : 2, isMobile: phone, hasTouch: phone, colorScheme: dark ? 'dark' : 'light',
    userAgent: phone ? 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1' : undefined,
  })
  const page = await ctx.newPage()
  page.on('pageerror', e => errors.push(`pageerror: ${e}`))
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource|Content Security Policy/.test(m.text())) errors.push(m.text()) })
  await page.goto(base + '/login')
  const res = await page.evaluate(() => fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: 'demo@example.com', password: 'demodemo1' }) }).then(r => r.status))
  assert.equal(res, 200, 'demo sign-in')
  return { ctx, page }
}
async function go(page, path, wait = 1500, frozen = true) {
  await page.goto(base + path, { waitUntil: 'networkidle' })
  // headless Chrome freezes CSS fade-ins: show everything at once
  if (frozen) await page.addStyleTag({ content: '*, *::before, *::after { transition: none !important; animation: none !important; }' }).catch(() => {})
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(wait)
}
async function shot(page, name) { if (shots && (!only || only.split(',').includes(name))) await page.screenshot({ path: dir + name + '.png' }) }
const api = async (page, path) => page.evaluate(p => fetch('/api/v1' + p).then(r => r.json()), path)
const formID = async (page, title) => (await api(page, '/forms')).find(f => f.title === title).id
const answerID = async (page, q) => (await api(page, '/responses?q=' + encodeURIComponent(q))).items[0].id

// ---- desktop ----
{
  const { ctx, page } = await session()
  const guest = await answerID(page, 'guest house')
  await go(page, `/answers/${guest}`, 2000)
  assert.ok((await page.content()).includes('A booking site for our guest house'), 'the inbox shows the brief')
  await shot(page, 'desktop-answers')
  await page.getByRole('button', { name: /Send on/ }).click()
  await page.waitForTimeout(600)
  await shot(page, 'desktop-send-on')
  await page.keyboard.press('Escape')
  await go(page, '/forms', 1500)
  assert.ok((await page.content()).includes('Workshop: build your first website'), 'the forms are listed')
  await shot(page, 'desktop-forms')
  const brief = await formID(page, 'Start a project with me')
  await go(page, `/forms/${brief}`, 1500)
  await page.getByText('Tell me more').first().click()
  await page.waitForTimeout(400)
  await shot(page, 'desktop-editor')
  await go(page, `/forms/${brief}/after`, 1500)
  await shot(page, 'desktop-after')
  const fb = await formID(page, 'How did we do?')
  await go(page, `/forms/${fb}/results`, 1800)
  await shot(page, 'desktop-results')
  await go(page, '/forms/new', 1500)
  await shot(page, 'desktop-new')
  const handle = (await api(page, `/forms/${brief}`)).handle
  await go(page, `/f/${handle}`, 1200, false)
  assert.ok((await page.content()).includes('In one line: what is it?'), 'the public form shows its questions')
  await shot(page, 'desktop-public')
  await ctx.close()
}

// ---- phone ----
{
  const { ctx, page } = await session({ phone: true })
  await go(page, '/', 2000)
  await shot(page, 'phone-answers')
  const guest = await answerID(page, 'guest house')
  await go(page, `/answers/${guest}`, 2000)
  await shot(page, 'phone-answer')
  const brief = await formID(page, 'Start a project with me')
  await go(page, `/forms/${brief}`, 1500)
  await shot(page, 'phone-editor')
  const ws = (await api(page, '/forms')).find(f => f.title.startsWith('Workshop'))
  await go(page, `/f/${ws.handle}`, 1200, false)
  await shot(page, 'phone-public')
  await ctx.close()
}

// ---- dark ----
{
  const { ctx, page } = await session({ dark: true })
  const guest = await answerID(page, 'guest house')
  await go(page, `/answers/${guest}`, 2000)
  await shot(page, 'desktop-answers-dark')
  const fb = (await api(page, '/forms')).find(f => f.title === 'How did we do?')
  await go(page, `/f/${fb.handle}`, 1200, false)
  await shot(page, 'desktop-public-dark')
  await ctx.close()
}

await browser.close()
if (errors.length) { console.error(errors.join('\n')); process.exit(1) }
console.log('ok')

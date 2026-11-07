// One form: its questions, what happens after sending, how to share it and
// what the answers say. Every change saves by itself a moment later.
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import { ACCENTS, APP_INFO, FIELD_TYPES, LANGUAGES, ROLES, formState, hasOptions, newFieldID, typeInfo } from '../lib/forms'
import { fmtDate } from '../lib/format'
import type { Auto, FieldType, Form, FormField, FormStats } from '../lib/types'
import { useCrumb } from '../components/Layout'
import { useApps } from '../components/AnswerActions'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Confirm, CopyField, CoreHint, Field, Loading, type MenuItem, Menu, MenuButton, SettingRow, Spinner, Switch, Tabs, errMsg, rectOf, type Anchor, useAsync, useToast } from '../components/ui'

type Tab = 'questions' | 'after' | 'share' | 'results'
const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: 'questions', label: 'Questions', icon: 'list' }, { id: 'after', label: 'After sending', icon: 'settings' },
  { id: 'share', label: 'Share', icon: 'share' }, { id: 'results', label: 'Results', icon: 'chart' },
]

const SAVED_KEYS = ['title', 'description', 'emoji', 'fields', 'accent', 'language', 'submit_label', 'thanks', 'redirect_url', 'close_at', 'max_answers', 'one_per_email', 'branding', 'notify', 'send_copy', 'auto'] as const

export default function FormEditor() {
  const { id, tab: rawTab } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const tab: Tab = TABS.find(t => t.id === rawTab)?.id || 'questions'
  const [f, setF] = useState<Form | null>(null)
  const [err, setErr] = useState('')
  const [save, setSave] = useState<'' | 'saving' | 'saved' | 'error'>('')
  const [del, setDel] = useState(false)
  const dirty = useRef(false)
  const latest = useRef<Form | null>(null)
  latest.current = f
  useCrumb(f?.title)

  useEffect(() => { api.get<Form>(`${V1}/forms/${id}`).then(setF).catch(e => setErr(errMsg(e))) }, [id])

  const flush = useCallback(async () => {
    const cur = latest.current
    if (!cur || !dirty.current) return
    dirty.current = false
    setSave('saving')
    const body: Record<string, unknown> = {}
    for (const k of SAVED_KEYS) body[k] = cur[k]
    try {
      const n = await api.put<Form>(`${V1}/forms/${cur.id}`, body)
      setF(x => x && { ...x, fields: dirty.current ? x.fields : n.fields, taking: n.taking, why: n.why, questions: n.questions, updated_at: n.updated_at })
      setSave('saved')
      announceChange()
    } catch (e) { setSave('error'); toast(errMsg(e), 'err') }
  }, [toast])
  useEffect(() => {
    if (!dirty.current) return
    const t = window.setTimeout(flush, 700)
    return () => window.clearTimeout(t)
  }, [f, flush])
  useEffect(() => () => { flush() }, [flush])
  const edit = (patch: Partial<Form>) => { dirty.current = true; setF(x => x && { ...x, ...patch }) }

  const act = async (what: 'open' | 'close' | 'duplicate') => {
    if (!f) return
    await flush()
    try {
      const n = await api.post<Form>(`${V1}/forms/${f.id}/${what}`)
      announceChange()
      if (what === 'duplicate') { toast('Copied — this is the copy'); navigate(`/forms/${n.id}`); return }
      setF(n)
      toast(what === 'open' ? 'The form takes answers now' : 'Closed: it takes no more answers')
    } catch (e) { toast(errMsg(e), 'err') }
  }

  if (err) return <div className="page"><div className="callout danger"><Icon name="alert" size={15} /><span>{err}</span></div></div>
  if (!f) return <Loading />
  const st = formState(f)
  const more = (): MenuItem[] => [
    { label: 'See the answers', icon: 'inbox', onSelect: () => navigate(`/?form=${f.id}`) },
    { label: 'Download the answers (CSV)', icon: 'download', onSelect: () => { window.location.href = `${V1}/forms/${f.id}/export.csv` } },
    { label: 'Make a copy', icon: 'copy', onSelect: () => act('duplicate') },
    'sep',
    { label: 'Delete the form', icon: 'trash', danger: true, onSelect: () => setDel(true) },
  ]
  return (
    <div className="page editor stack loose">
      <div className="editor-head">
        <div className="grow editor-title">
          <h1 className="page-title ellipsis">{f.emoji && <span className="title-emoji">{f.emoji}</span>}{f.title || 'Untitled form'}</h1>
          <div className="row wrap small">
            <span className={`pill ${st.cls}`}><Icon name={st.icon} size={11} />{st.label}</span>
            <Link to={`/?form=${f.id}`} className="muted">{f.responses} {f.responses === 1 ? 'answer' : 'answers'}{f.new ? ` · ${f.new} new` : ''}</Link>
            <span className={`save-state ${save === 'error' ? 'error' : ''}`} aria-live="polite">{save === 'saving' ? 'Saving…' : save === 'saved' ? 'Saved' : save === 'error' ? 'Not saved' : ''}</span>
          </div>
        </div>
        <div className="row editor-tools">
          <a className="btn" href={f.url} target="_blank" rel="noreferrer" title={f.status === 'draft' ? 'See it as people will (only you can, until it is open)' : 'Open the form'}><Icon name="eye" size={15} /><span className="hide-sm">{f.status === 'draft' ? 'Preview' : 'Open'}</span></a>
          {f.status === 'open'
            ? <button type="button" className="btn" onClick={() => act('close')}><Icon name="lock" size={15} /><span className="hide-sm">Close</span></button>
            : <button type="button" className="btn primary" onClick={() => act('open')} disabled={f.questions === 0} title={f.questions === 0 ? 'Add a question first' : 'Start taking answers'}><Icon name="send" size={15} />{f.status === 'draft' ? 'Open the form' : 'Open again'}</button>}
          <MenuButton label="More" items={more} align="right" width={250}><Icon name="more" size={17} /></MenuButton>
        </div>
      </div>
      <Tabs tabs={TABS} value={tab} onChange={t => navigate(`/forms/${f.id}/${t === 'questions' ? '' : t}`, { replace: true })} />
      {tab === 'questions' && <Questions f={f} edit={edit} />}
      {tab === 'after' && <After f={f} edit={edit} />}
      {tab === 'share' && <Share f={f} onChange={n => setF(n)} onOpen={() => act('open')} />}
      {tab === 'results' && <Results f={f} />}
      {del && <Confirm title={`Delete “${f.title}”?`} message={`The form and its ${f.responses} ${f.responses === 1 ? 'answer' : 'answers'} are deleted, and its link stops working. Download the answers first if you want to keep them.`} confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={async () => { setDel(false); dirty.current = false; try { await api.del(`${V1}/forms/${f.id}`); announceChange(); toast('Form deleted'); navigate('/forms') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}

// ---- questions ----

function Questions({ f, edit }: { f: Form; edit: (p: Partial<Form>) => void }) {
  const [open, setOpen] = useState<string>(f.fields[0]?.id || '')
  const [adding, setAdding] = useState<{ anchor: Anchor; after: number } | null>(null)
  const setField = (i: number, p: Partial<FormField>) => edit({ fields: f.fields.map((x, j) => j === i ? { ...x, ...p } : x) })
  const move = (i: number, d: number) => { const n = [...f.fields]; const [x] = n.splice(i, 1); n.splice(i + d, 0, x); edit({ fields: n }) }
  const remove = (i: number) => edit({ fields: f.fields.filter((_, j) => j !== i) })
  const copy = (i: number) => { const n = [...f.fields]; const c = { ...n[i], id: newFieldID(n), role: '' as const }; n.splice(i + 1, 0, c); edit({ fields: n }); setOpen(c.id) }
  const add = (type: FieldType, after: number) => {
    const n = [...f.fields]
    const fl: FormField = { id: newFieldID(n), type, label: type === 'section' ? 'A new part' : '', ...(hasOptions(type) ? { options: ['', ''] } : {}) }
    n.splice(after + 1, 0, fl)
    edit({ fields: n })
    setOpen(fl.id)
    window.setTimeout(() => document.getElementById(`lbl-${fl.id}`)?.focus(), 30)
  }
  const typeItems = (after: number): MenuItem[] => FIELD_TYPES.map(t => ({ label: <span className="type-item"><b>{t.label}</b><span className="muted small">{t.help}</span></span>, icon: t.icon, onSelect: () => add(t.type, after) }))
  const roleTaken = (role: string, id: string) => !!role && f.fields.some(x => x.role === role && x.id !== id)
  return (
    <div className="q-editor stack">
      <section className="card q-head">
        <div className="q-title-row">
          <input className="emoji-in" value={f.emoji} onChange={e => edit({ emoji: e.target.value })} aria-label="Emoji" maxLength={8} placeholder="📝" />
          <input className="title-in" value={f.title} onChange={e => edit({ title: e.target.value })} placeholder="The form’s title" aria-label="Title" />
        </div>
        <textarea data-gramm="false" className="desc-in" rows={2} value={f.description} onChange={e => edit({ description: e.target.value })} placeholder="A few words on top: what it is for, how long it takes, what happens next" aria-label="Description" />
      </section>
      {f.fields.map((fl, i) => {
        const ti = typeInfo(fl.type)
        const isOpen = open === fl.id
        return (
          <section key={fl.id} className={`card q-card ${isOpen ? 'open' : ''} ${fl.type === 'section' ? 'is-section' : ''}`} onClick={() => !isOpen && setOpen(fl.id)}>
            {!isOpen ? <div className="q-closed">
              <span className="q-ico"><Icon name={ti.icon} size={15} /></span>
              <span className="grow ellipsis">{fl.label || <span className="faint">No question yet</span>}{fl.required && <span className="req"> *</span>}</span>
              {fl.role && <span className="badge">{ROLES.find(r => r.role === fl.role)?.label}</span>}
              <span className="muted small hide-sm">{ti.label}</span>
            </div> : <div className="q-open stack" onClick={e => e.stopPropagation()}>
              <div className="q-row">
                <div className="q-type"><Select ariaLabel="Kind of question" value={fl.type} onChange={v => {
                  const t = v as FieldType
                  setField(i, { type: t, options: hasOptions(t) ? (fl.options?.length ? fl.options : ['', '']) : undefined, other: hasOptions(t) && t !== 'dropdown' ? fl.other : false, role: t === 'section' ? '' : fl.role })
                }} options={FIELD_TYPES.map(t => ({ value: t.type, label: t.label, hint: t.help }))} /></div>
                <span className="spacer" />
                <button type="button" className="btn ghost icon-only sm" aria-label="Move up" disabled={i === 0} onClick={() => move(i, -1)}><Icon name="arrow-up" size={15} /></button>
                <button type="button" className="btn ghost icon-only sm" aria-label="Move down" disabled={i === f.fields.length - 1} onClick={() => move(i, 1)}><Icon name="arrow-down" size={15} /></button>
                <button type="button" className="btn ghost icon-only sm" aria-label="Copy the question" title="Copy" onClick={() => copy(i)}><Icon name="copy" size={15} /></button>
                <button type="button" className="btn ghost icon-only sm danger-text" aria-label="Delete the question" title="Delete" onClick={() => remove(i)}><Icon name="trash" size={15} /></button>
              </div>
              <Field label={fl.type === 'section' ? 'Heading' : fl.type === 'consent' ? 'What they agree to' : 'Question'}>
                <input id={`lbl-${fl.id}`} value={fl.label} onChange={e => setField(i, { label: e.target.value })} placeholder={fl.type === 'section' ? 'About your project' : 'What would you like to ask?'} />
              </Field>
              <Field label={fl.type === 'section' ? 'Words under it (optional)' : 'A hint under the question (optional)'}><input value={fl.help || ''} onChange={e => setField(i, { help: e.target.value })} /></Field>
              {hasOptions(fl.type) && <div className="opts-edit">
                <span className="label">Choices</span>
                <div id={`opts-${fl.id}`} className="stack tight">
                {(fl.options || []).map((o, k) => (
                  <div key={k} className="opt-row">
                    <Icon name={fl.type === 'checkboxes' ? 'check-square' : fl.type === 'dropdown' ? 'chevron-down' : 'circle'} size={14} />
                    <input value={o} onChange={e => setField(i, { options: (fl.options || []).map((x, j) => j === k ? e.target.value : x) })} placeholder={`Choice ${k + 1}`}
                      onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); const n = [...(fl.options || [])]; n.splice(k + 1, 0, ''); setField(i, { options: n }); window.setTimeout(() => (document.querySelectorAll(`#opts-${fl.id} input`)[k + 1] as HTMLInputElement | undefined)?.focus(), 20) } }} />
                    <button type="button" className="btn ghost icon-only sm" aria-label="Remove the choice" onClick={() => setField(i, { options: (fl.options || []).filter((_, j) => j !== k) })}><Icon name="x" size={14} /></button>
                  </div>
                ))}
                </div>
                <div className="row wrap">
                  <button type="button" className="btn sm" onClick={() => setField(i, { options: [...(fl.options || []), ''] })}><Icon name="plus" size={14} />Add a choice</button>
                  {fl.type !== 'dropdown' && <label className="check-row"><Switch size="sm" checked={!!fl.other} onChange={v => setField(i, { other: v })} label="Other" /><span>“Other” with their own words</span></label>}
                </div>
              </div>}
              {['text', 'long', 'email', 'phone', 'number', 'url'].includes(fl.type) && <Field label="Example inside the box (optional)"><input value={fl.placeholder || ''} onChange={e => setField(i, { placeholder: e.target.value })} /></Field>}
              {fl.type !== 'section' && <div className="q-row wrap q-foot">
                <label className="check-row"><Switch size="sm" checked={!!fl.required} onChange={v => setField(i, { required: v })} label="Required" /><span>Must be answered</span></label>
                <span className="spacer" />
                <div className="q-role">
                  <span className="muted small">This answer is</span>
                  <Select ariaLabel="What the answer is" value={fl.role || ''} onChange={v => setField(i, { role: v as FormField['role'] })}
                    options={ROLES.map(r => ({ value: r.role, label: r.label, hint: roleTaken(r.role, fl.id) ? 'used by another question' : r.help, disabled: roleTaken(r.role, fl.id) }))} />
                </div>
              </div>}
            </div>}
          </section>
        )
      })}
      <div className="row wrap">
        <button type="button" className="btn" onClick={e => setAdding({ anchor: rectOf(e.currentTarget), after: f.fields.findIndex(x => x.id === open) >= 0 ? f.fields.findIndex(x => x.id === open) : f.fields.length - 1 })}><Icon name="plus" size={15} />Add a question</button>
        <span className="muted small">“This answer is” tells your other apps who sent it: the name and email go into Contacts, “what it is about” names the project or task.</span>
      </div>
      {adding && <Menu anchor={adding.anchor} items={typeItems(adding.after)} onClose={() => setAdding(null)} width={280} />}
    </div>
  )
}

// ---- after sending, limits, look, what happens by itself ----

function After({ f, edit }: { f: Form; edit: (p: Partial<Form>) => void }) {
  const { on, why } = useApps()
  const { settings } = useApp()
  const coreOn = !!settings?.core_configured && settings?.core_registered !== false
  const hasEmail = f.fields.some(x => x.role === 'email')
  const hasDay = f.fields.some(x => x.role === 'date' || x.role === 'deadline')
  const auto = (p: Partial<Auto>) => edit({ auto: { ...f.auto, ...p } })
  const appRow = (slug: string, title: string, help: string, checked: boolean, set: (v: boolean) => void, extra?: string) => {
    const ok = on(slug) && !extra
    return (
      <SettingRow title={<span className="row" style={{ gap: 8 }}><Icon name={APP_INFO[slug as keyof typeof APP_INFO].icon} size={15} />{title}</span>}
        help={ok ? help : extra || `${APP_INFO[slug as keyof typeof APP_INFO].name} is not connected${why(slug) ? `: ${why(slug)}` : ''} (Settings → Connected apps).`}>
        <Switch checked={checked && ok} onChange={set} label={title} disabled={!ok} />
      </SettingRow>
    )
  }
  return (
    <div className="stack loose">
      <section className="card pad-card">
        <h2 className="card-h">When someone sends it</h2>
        <div className="stack">
          <Field label="Thank-you message" help="Shown after sending. Empty: “Your answers were sent.”"><textarea data-gramm="false" rows={3} value={f.thanks} onChange={e => edit({ thanks: e.target.value })} /></Field>
          <div className="two-col tight">
            <Field label="The send button says" help="Empty: “Send”."><input value={f.submit_label} onChange={e => edit({ submit_label: e.target.value })} placeholder="Send" /></Field>
            <Field label="Or go to a page of yours instead" help="A web address; empty shows the thank-you message."><input value={f.redirect_url} onChange={e => edit({ redirect_url: e.target.value })} placeholder="https://" /></Field>
          </div>
        </div>
      </section>
      <section className="card pad-card">
        <h2 className="card-h">What happens by itself when an answer arrives</h2>
        {!coreOn && <CoreHint />}
        <SettingRow title={<span className="row" style={{ gap: 8 }}><Icon name="bell" size={15} />Tell me on my phone</span>} help={settings?.notify_each === false ? 'Pushes for new answers are off for all forms in Settings → Notifications.' : 'A push with who answered and what about.'}>
          <Switch checked={f.notify} onChange={v => edit({ notify: v })} label="Tell me" />
        </SettingRow>
        {appRow('contacts', 'Add them to my contacts', 'Unless their email is there already. Needs a question used as their name or email.', f.auto.contact, v => auto({ contact: v }))}
        {appRow('tasks', 'Make a follow-up task', f.auto.task_days ? `Due ${f.auto.task_days} ${f.auto.task_days === 1 ? 'day' : 'days'} after the answer arrives.` : 'With no due day.', f.auto.task, v => auto({ task: v }))}
        {f.auto.task && on('tasks') && <SettingRow title="Due after" help="0: no due day."><input type="number" className="num-sm" min={0} max={365} value={f.auto.task_days} onChange={e => auto({ task_days: Math.max(0, Number(e.target.value) || 0) })} /></SettingRow>}
        {appRow('calendar', 'Put the day they picked in my calendar', 'For appointments and sign-ups for a day.', f.auto.event, v => auto({ event: v }), hasDay ? '' : 'Needs a question used as “a day for the calendar” (Questions → This answer is).')}
        {appRow('notes', 'Save each answer as a note', 'In Pocket Notes, with a link back.', f.auto.note, v => auto({ note: v }))}
        {appRow('crm', 'Add each answer to Pocket CRM as a lead', 'A new deal with what they asked for, their budget and their deadline.', !!f.auto.deal, v => auto({ deal: v }))}
        {appRow('mail', 'Email them a copy of their answers', 'From your own mailbox, in the form’s language.', f.send_copy, v => edit({ send_copy: v }), hasEmail ? '' : 'Needs a question used as their email (Questions → This answer is).')}
        <p className="muted small">Every answer is also an event for Pocket Flows (“an answer arrived”), for anything else you want to happen.</p>
      </section>
      <section className="card pad-card">
        <h2 className="card-h">Taking answers</h2>
        <SettingRow title="Last day" help={f.close_at ? `Takes answers until the end of ${fmtDate(f.close_at)}.` : 'Empty: until you close it.'}><input type="date" value={f.close_at} onChange={e => edit({ close_at: e.target.value })} /></SettingRow>
        <SettingRow title="At most this many answers" help="For limited places. 0: no limit."><input type="number" className="num-sm" min={0} value={f.max_answers} onChange={e => edit({ max_answers: Math.max(0, Number(e.target.value) || 0) })} /></SettingRow>
        <SettingRow title="One answer per email address" help={hasEmail ? 'A second answer from the same address is thanked, not kept.' : 'Needs a question used as their email.'}><Switch checked={f.one_per_email && hasEmail} onChange={v => edit({ one_per_email: v })} label="One per email" disabled={!hasEmail} /></SettingRow>
      </section>
      <section className="card pad-card">
        <h2 className="card-h">Look</h2>
        <SettingRow title="Colour"><div className="swatches">{ACCENTS.map(a => <button key={a} type="button" className={`swatch ${f.accent === a ? 'on' : ''}`} style={{ background: a }} aria-label={a} onClick={() => edit({ accent: a })} />)}
          <label className={`swatch custom ${f.accent && !ACCENTS.includes(f.accent) ? 'on' : ''}`} title="Your own colour"><input type="color" value={f.accent || '#5f3dc4'} onChange={e => edit({ accent: e.target.value })} aria-label="Your own colour" /></label></div></SettingRow>
        <SettingRow title="Language of the page" help="The words the page says itself: “Send”, “Required”, the thank-you title, the errors. Your questions stay as you wrote them."><Select ariaLabel="Language" value={f.language} onChange={v => edit({ language: v })} options={LANGUAGES} /></SettingRow>
        <SettingRow title="“Made with Pocket Forms” under the form"><Switch checked={f.branding} onChange={v => edit({ branding: v })} label="Made with" /></SettingRow>
      </section>
    </div>
  )
}

// ---- share ----

function Share({ f, onChange, onOpen }: { f: Form; onChange: (f: Form) => void; onOpen: () => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [handle, setHandle] = useState(f.handle)
  const [check, setCheck] = useState<{ ok: boolean; why?: string } | null>(null)
  useEffect(() => {
    if (handle === f.handle) { setCheck(null); return }
    const t = window.setTimeout(() => api.get<{ ok: boolean; why?: string }>(`${V1}/forms/check-handle?handle=${encodeURIComponent(handle)}&form=${f.id}`).then(setCheck).catch(() => {}), 300)
    return () => window.clearTimeout(t)
  }, [handle, f.handle, f.id])
  const saveHandle = async () => { try { const n = await api.put<Form>(`${V1}/forms/${f.id}`, { handle }); onChange(n); toast('The address changed; the old link no longer works') } catch (e) { toast(errMsg(e), 'err') } }
  const base = f.url.slice(0, f.url.length - f.handle.length)
  const embed = `<iframe src="${f.url}?embed=1" style="width:100%;min-height:720px;border:0" title="${f.title.replace(/"/g, '&quot;')}"></iframe>`
  const roleIDs = f.fields.filter(x => x.role === 'name' || x.role === 'email')
  return (
    <div className="stack loose">
      {f.status === 'draft' && <div className="callout info"><Icon name="info" size={15} /><div>This form is a draft: the link shows it only to you. <button type="button" className="linkish" onClick={onOpen}>Open the form</button> to take answers.</div></div>}
      {f.status === 'closed' && <div className="callout warn"><Icon name="lock" size={15} /><div>Closed: the link says it takes no more answers. <button type="button" className="linkish" onClick={onOpen}>Open it again</button>.</div></div>}
      <section className="card pad-card">
        <h2 className="card-h">The link</h2>
        <CopyField value={f.url} />
        <div className="row wrap mt">
          <a className="btn" href={f.url} target="_blank" rel="noreferrer"><Icon name="external" size={15} />Open it</a>
          <a className="btn" href={`mailto:?subject=${encodeURIComponent(f.title)}&body=${encodeURIComponent(f.url)}`}><Icon name="mail" size={15} />Send by email</a>
          {'share' in navigator && <button type="button" className="btn" onClick={() => navigator.share({ title: f.title, url: f.url }).catch(() => {})}><Icon name="share" size={15} />Share…</button>}
        </div>
        {!settings?.public_url && <p className="muted small mt">People open it at this box’s address. If they are not on your home network, the box needs an address on the internet for forms (<code>PUBLIC_URL</code>, which serves only the form pages).</p>}
      </section>
      <section className="card pad-card">
        <h2 className="card-h">Its address</h2>
        <div className="handle-row"><span className="muted small ellipsis">{base}</span><input value={handle} onChange={e => setHandle(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))} aria-label="Address" /></div>
        <div className="row wrap mt">
          <button type="button" className="btn sm" onClick={saveHandle} disabled={handle === f.handle || !check?.ok}>Change the address</button>
          {check && <span className={`help ${check.ok ? 'ok' : 'bad'}`}>{check.ok ? 'Free' : check.why}</span>}
          {handle === f.handle && <span className="muted small">The last part ends in four random letters, so nobody finds the form by guessing.</span>}
        </div>
      </section>
      <section className="card pad-card">
        <h2 className="card-h">On your own website</h2>
        <p className="muted small">Paste this where the form should appear; it shows without the page around it.</p>
        <CopyField value={embed} />
      </section>
      {roleIDs.length > 0 && <section className="card pad-card">
        <h2 className="card-h">Filled in for them</h2>
        <p className="muted small">Add answers to the link and they are already there — handy in an email to someone you know: <code>{f.url}?{roleIDs.map(x => `${x.role}=…`).join('&')}</code></p>
      </section>}
    </div>
  )
}

// ---- results ----

function Results({ f }: { f: Form }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const { data } = useAsync(() => api.get<FormStats>(`${V1}/forms/${f.id}/stats`), [f.id])
  const [sum, setSum] = useState<{ summary: string; points: string[]; quotes: string[]; next: string[] } | null>(null)
  const [busy, setBusy] = useState(false)
  if (!data) return <Loading />
  const rate = data.total_views ? Math.round(data.total_answers / data.total_views * 100) : 0
  const max = Math.max(1, ...data.answers.map(d => d.count))
  const summarize = async () => { setBusy(true); try { setSum(await api.post(`${V1}/forms/${f.id}/ai/summary`)) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) } }
  return (
    <div className="stack loose">
      <div className="stats-row">
        <div className="stat"><div className="stat-l">Answers</div><div className="stat-v tnum">{data.total_answers}</div></div>
        <div className="stat"><div className="stat-l">Opened</div><div className="stat-v tnum">{data.total_views}</div><div className="stat-s small">people, once a day each</div></div>
        <div className="stat"><div className="stat-l">Answered</div><div className="stat-v tnum">{data.total_views ? `${Math.min(100, rate)}%` : '—'}</div><div className="stat-s small">of those who opened it</div></div>
      </div>
      <section className="card pad-card">
        <div className="row"><h2 className="card-h grow" style={{ margin: 0 }}>Answers in the last 30 days</h2><a className="btn sm" href={`${V1}/forms/${f.id}/export.csv`}><Icon name="download" size={14} />CSV</a></div>
        <div className="daybars mt" role="img" aria-label="Answers per day">{data.answers.map(d => <span key={d.day} className="daybar" style={{ height: `${Math.max(2, d.count / max * 100)}%`, opacity: d.count ? .85 : .25 }} title={`${fmtDate(d.day)}: ${d.count}`} />)}</div>
        <div className="daybars-axis muted small"><span>{data.answers[0] && fmtDate(data.answers[0].day, false)}</span><span>today</span></div>
      </section>
      {settings?.ai_help !== false && data.total_answers > 0 && <section className="card pad-card">
        <div className="row"><h2 className="card-h grow row" style={{ margin: 0, gap: 6 }}><Icon name="sparkles" size={15} />What the answers say</h2>
          <button type="button" className="btn sm" onClick={summarize} disabled={busy}>{busy ? <Spinner /> : <Icon name="sparkles" size={14} />}{sum ? 'Again' : 'Sum it up with Pocket AI'}</button></div>
        {sum && <div className="ai-sum mt">
          <p>{sum.summary}</p>
          {sum.points?.length > 0 && <ul>{sum.points.map(p => <li key={p}>{p}</li>)}</ul>}
          {sum.quotes?.length > 0 && sum.quotes.map(q => <blockquote key={q}>“{q}”</blockquote>)}
          {sum.next?.length > 0 && <><h3>You could</h3><ul>{sum.next.map(p => <li key={p}>{p}</li>)}</ul></>}
        </div>}
      </section>}
      {data.fields.map(st => (
        <section key={st.id} className="card pad-card">
          <h3 className="q-stat-h">{st.label}</h3>
          <p className="muted small">{st.answered} {st.answered === 1 ? 'answer' : 'answers'}{st.average ? ` · average ${st.average}` : ''}</p>
          {st.options && st.options.length > 0 && <div className="bars">{st.options.map(o => {
            const pct = st.answered ? Math.round(o.count / st.answered * 100) : 0
            return <div key={o.option} className="bar-row"><span className="bar-l ellipsis">{st.type === 'rating' ? '★'.repeat(Number(o.option)) : o.option}</span><span className="bar-track"><span style={{ width: `${pct}%` }} /></span><span className="bar-n tnum small">{o.count} · {pct}%</span></div>
          })}</div>}
          {st.recent && st.recent.length > 0 && <ul className="recent">{st.recent.map((r, k) => <li key={k}>{r}</li>)}</ul>}
        </section>
      ))}
    </div>
  )
}

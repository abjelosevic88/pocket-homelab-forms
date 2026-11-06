// What one answer can become in the other Pocket apps: the sheets behind
// "Accept the brief", "Make…" and "Reply".
import { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { APP_INFO, TARGET_APP } from '../lib/forms'
import { addDays, todayISO } from '../lib/format'
import type { AnswerView, AppState, Made } from '../lib/types'
import { Icon } from './icons'
import { Select } from './Select'
import { Field, Modal, Spinner, Switch, errMsg, useAsync, useToast } from './ui'

let appsCache: Promise<AppState[]> | null = null

/** Which apps answers can go to for this person (asked once per page load). */
export function useApps() {
  const { data } = useAsync(() => { appsCache = appsCache || api.get<AppState[]>(`${V1}/apps`).catch(() => { appsCache = null; return [] as AppState[] }); return appsCache }, [])
  const by = (slug: string) => data?.find(a => a.slug === slug)
  return { apps: data, on: (slug: string) => !!by(slug)?.connected, why: (slug: string) => by(slug)?.detail || '' }
}

const TITLES: Record<string, string> = { contact: 'Add to Pocket Contacts', project: 'Start a project', task: 'Make a follow-up task', event: 'Add to Pocket Calendar', note: 'Save as a note', signing: 'Send a document to sign' }

type Opt = { id: number; name: string; key?: string; roles?: { id: number; role_name: string; name: string; email: string }[] }

function useOptions(what: 'teams' | 'templates', active: boolean) {
  const [list, setList] = useState<Opt[] | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    if (!active) return
    api.get<Opt[]>(`${V1}/apps/options/${what}`).then(setList).catch(e => { setErr(errMsg(e)); setList([]) })
  }, [what, active])
  return { list, err }
}

export function MakeSheet({ view, target, onClose, onDone }: { view: AnswerView; target: string; onClose: () => void; onDone: (m: Made) => void }) {
  const { toast } = useToast()
  const r = view.response
  const who = r.name || r.email || `answer #${r.number}`
  const subject = view.items.find(i => i.role === 'subject')?.text || view.form.title
  const dayAnswer = view.items.find(i => i.role === 'date')?.text || view.items.find(i => i.role === 'deadline')?.text || ''
  const timeAnswer = view.items.find(i => i.type === 'time')?.text || ''
  const [f, setF] = useState({
    title: target === 'task' ? `Answer ${who} (${view.form.title})` : target === 'event' ? `${subject} — ${who}` : target === 'note' ? `${view.form.title} — ${who}` : target === 'project' ? '' : '',
    notes: '', due: addDays(todayISO(), 2), date: dayAnswer, time: timeAnswer, minutes: 60, team_id: 0, template_id: 0, send: false,
  })
  const teams = useOptions('teams', target === 'project')
  const templates = useOptions('templates', target === 'signing')
  const [busy, setBusy] = useState(false)
  const app = TARGET_APP[target]
  const go = async (e?: React.FormEvent) => {
    e?.preventDefault()
    setBusy(true)
    try {
      const m = await api.post<Made>(`${V1}/responses/${r.id}/make`, { target, ...f, due: target === 'task' ? f.due : '' })
      toast(m.title ? `Done: ${m.title}` : 'Done')
      onDone(m)
    } catch (err) { toast(errMsg(err), 'err') } finally { setBusy(false) }
  }
  const tpl = templates.list?.find(t => t.id === f.template_id)
  return (
    <Modal title={TITLES[target]} onClose={onClose} size="sm" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={() => go()} disabled={busy || (target === 'signing' && !f.template_id) || (target === 'event' && !f.date)}>
        {busy ? <Spinner /> : <Icon name={APP_INFO[app].icon} size={15} />}{target === 'signing' ? (f.send ? 'Send it' : 'Make the document') : 'Make it'}
      </button>
    </>}>
      <form className="stack" onSubmit={go}>
        {target === 'contact' && <p className="muted">{who}{r.email && r.name ? ` · ${r.email}` : ''} goes into your contacts, with a note saying where they came from. When their email is there already, you get a link to that contact instead.</p>}
        {target === 'project' && <>
          <Field label="Project name" help={`Empty: “${[view.items.find(i => i.role === 'company')?.text || r.name, subject].filter(Boolean).join(' — ')}”. All the answers go into its description.`}><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} autoFocus /></Field>
          <Field group label="Team" help={teams.err || 'Empty: the team set in Settings → Answers, or Projects’ default.'}>
            <Select ariaLabel="Team" value={String(f.team_id || '')} onChange={v => setF({ ...f, team_id: Number(v) })} placeholder={teams.list ? 'Default team' : 'Loading…'} clearLabel="Default team" options={(teams.list || []).map(t => ({ value: String(t.id), label: t.name, hint: t.key }))} />
          </Field>
        </>}
        {(target === 'task' || target === 'event' || target === 'note') && <Field label="Title"><input value={f.title} onChange={e => setF({ ...f, title: e.target.value })} autoFocus /></Field>}
        {target === 'task' && <Field label="Due" help="Empty: no due day."><input type="date" value={f.due} onChange={e => setF({ ...f, due: e.target.value })} /></Field>}
        {target === 'event' && <div className="two-col tight">
          <Field label="Day"><input type="date" value={f.date} onChange={e => setF({ ...f, date: e.target.value })} required /></Field>
          <Field label="Time" help="Empty: all day."><input type="time" value={f.time} onChange={e => setF({ ...f, time: e.target.value })} /></Field>
        </div>}
        {target === 'signing' && <>
          <Field group label="Template from Pocket Sign" help={templates.err || (r.email ? `${who} (${r.email}) signs it.` : 'This answer has no email address.')}>
            <Select ariaLabel="Template" value={String(f.template_id || '')} onChange={v => setF({ ...f, template_id: Number(v) })} placeholder={templates.list ? (templates.list.length ? 'Choose a template' : 'No templates in Pocket Sign yet') : 'Loading…'} options={(templates.list || []).map(t => ({ value: String(t.id), label: t.name }))} />
          </Field>
          {tpl && tpl.roles && tpl.roles.length > 1 && <p className="muted small">Signers: {tpl.roles.map(x => x.role_name || x.name || x.email).join(', ')}. The person who answered takes the first place that has nobody yet.</p>}
          <Field label="A message with it (optional)"><textarea rows={3} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} /></Field>
          <label className="check-row"><Switch checked={f.send} onChange={v => setF({ ...f, send: v })} label="Send now" /><span><b>Send it now</b><br /><span className="muted small">Off: the document waits in Pocket Sign for you to check and send.</span></span></label>
        </>}
        {(target === 'task' || target === 'event' || target === 'note' || target === 'project') && <Field label="Notes (optional)" help="The answers and a link back are added by themselves."><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} /></Field>}
      </form>
    </Modal>
  )
}

export function AcceptSheet({ view, onClose, onDone }: { view: AnswerView; onClose: () => void; onDone: () => void }) {
  const { toast } = useToast()
  const { on, why } = useApps()
  const r = view.response
  const [f, setF] = useState({ contact: true, project: true, task: true, team_id: 0, name: '', due: addDays(todayISO(), 2) })
  const teams = useOptions('teams', true)
  const [busy, setBusy] = useState(false)
  const [steps, setSteps] = useState<{ target: string; made?: Made; error?: string }[] | null>(null)
  const go = async () => {
    setBusy(true)
    try {
      const res = await api.post<{ steps: { target: string; made?: Made; error?: string }[] }>(`${V1}/responses/${r.id}/accept`, f)
      setSteps(res.steps)
      if (res.steps.every(s => !s.error)) toast('Brief accepted')
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const row = (k: 'contact' | 'project' | 'task', slug: string, label: string, help: string) => (
    <label className={`check-row ${on(slug) ? '' : 'disabled'}`} title={on(slug) ? undefined : why(slug)}>
      <Switch checked={f[k] && on(slug)} onChange={v => setF({ ...f, [k]: v })} label={label} disabled={!on(slug)} />
      <span><b>{label}</b><br /><span className="muted small">{on(slug) ? help : `Not connected: ${why(slug) || 'switch it on in Settings → Connected apps'}`}</span></span>
    </label>
  )
  return (
    <Modal title="Accept the brief" onClose={steps ? onDone : onClose} size="sm" footer={steps ? <button type="button" className="btn primary" onClick={onDone}>Close</button> : <>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={go} disabled={busy || !((f.contact && on('contacts')) || (f.project && on('projects')) || (f.task && on('tasks')))}>{busy ? <Spinner /> : <Icon name="check" size={15} />}Accept</button>
    </>}>
      {steps ? <div className="stack">
        {steps.map(s => <div key={s.target} className={`made-step ${s.error ? 'bad' : 'ok'}`}>
          <Icon name={s.error ? 'x-circle' : 'check-circle'} size={16} />
          <div className="grow"><b>{APP_INFO[TARGET_APP[s.target]].name}</b><div className="small muted">{s.error || s.made?.title}</div></div>
          {s.made?.url && <a className="btn sm" href={s.made.url} target="_blank" rel="noreferrer">Open<Icon name="external" size={13} /></a>}
        </div>)}
        <p className="muted small">The answer is marked done. Anything that did not work can be made again from the answer.</p>
      </div> : <div className="stack">
        <p className="muted">One step for a new client: {r.name || r.email} goes into your contacts, the brief becomes a project, and you get a task to get back to them.</p>
        {row('contact', 'contacts', 'Add to Pocket Contacts', 'Unless their email is there already.')}
        {row('project', 'projects', 'Start a project in Pocket Projects', 'With all the answers as its description and the deadline as its target date.')}
        {f.project && on('projects') && <div className="two-col tight indent">
          <Field label="Project name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} placeholder="From the answers" /></Field>
          <Field group label="Team"><Select ariaLabel="Team" value={String(f.team_id || '')} onChange={v => setF({ ...f, team_id: Number(v) })} placeholder={teams.list ? 'Default team' : 'Loading…'} clearLabel="Default team" options={(teams.list || []).map(t => ({ value: String(t.id), label: t.name, hint: t.key }))} /></Field>
        </div>}
        {row('task', 'tasks', 'A task to get back to them', 'With a link to the project.')}
        {f.task && on('tasks') && <div className="indent"><Field label="Due"><input type="date" value={f.due} onChange={e => setF({ ...f, due: e.target.value })} /></Field></div>}
      </div>}
    </Modal>
  )
}

export function ReplySheet({ view, onClose, onDone, aiOn }: { view: AnswerView; onClose: () => void; onDone: (m: Made) => void; aiOn: boolean }) {
  const { toast } = useToast()
  const r = view.response
  const [f, setF] = useState({ subject: `Re: ${view.form.title}`, body: '' })
  const [hint, setHint] = useState('')
  const [busy, setBusy] = useState<'' | 'ai' | 'send'>('')
  const draft = async () => {
    setBusy('ai')
    try { const d = await api.post<{ subject: string; body: string }>(`${V1}/responses/${r.id}/ai/reply`, { instruction: hint }); setF({ subject: d.subject || f.subject, body: d.body }) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const send = async () => {
    setBusy('send')
    try { const m = await api.post<Made>(`${V1}/responses/${r.id}/make`, { target: 'email', subject: f.subject, body: f.body }); toast(`Sent to ${r.email}`); onDone(m) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  return (
    <Modal title={`Reply to ${r.name || r.email}`} onClose={onClose} footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={send} disabled={!!busy || !f.body.trim()}>{busy === 'send' ? <Spinner /> : <Icon name="send" size={15} />}Send</button>
    </>}>
      <div className="stack">
        <p className="muted small">From your own mailbox through Pocket Mail, to <b>{r.email}</b>. Their answers are quoted below your words; it lands in your Sent folder.</p>
        <Field label="Subject"><input value={f.subject} onChange={e => setF({ ...f, subject: e.target.value })} /></Field>
        <Field label="Your reply"><textarea data-gramm="false" rows={9} value={f.body} onChange={e => setF({ ...f, body: e.target.value })} autoFocus /></Field>
        {aiOn && <div className="ai-row">
          <input value={hint} onChange={e => setHint(e.target.value)} placeholder="What to say, e.g. “yes, I can start in March; ask for their logo”" aria-label="What to say" />
          <button type="button" className="btn" onClick={draft} disabled={!!busy}>{busy === 'ai' ? <Spinner /> : <Icon name="sparkles" size={15} />}{f.body ? 'Write again' : 'Write it with Pocket AI'}</button>
        </div>}
      </div>
    </Modal>
  )
}

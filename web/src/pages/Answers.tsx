// The answers inbox: every answer to every form, newest first, with the one
// you opened beside the list (on a phone, on its own page).
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData, useOnChange } from '../lib/data'
import { APP_INFO, STATUS_LABEL, TARGET_APP, TARGET_LABEL } from '../lib/forms'
import { ago, fmtDate, fmtDateTime } from '../lib/format'
import type { Answer, AnswerView, Made } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { AcceptSheet, MakeSheet, ReplySheet, useApps } from '../components/AnswerActions'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Avatar, Confirm, Empty, Loading, type MenuItem, MenuButton, Segmented, errMsg, useAsync, useDebounce, useMediaQuery, useToast } from '../components/ui'

type List = { items: Answer[]; total: number; new: number }

const FILTERS = [{ value: '', label: 'All' }, { value: 'new', label: 'New' }, { value: 'done', label: 'Done' }, { value: 'starred', label: 'Starred' }, { value: 'spam', label: 'Spam' }]

export default function Answers() {
  const { id } = useParams()
  const openID = Number(id || 0)
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const { forms } = useData()
  const wide = useMediaQuery('(min-width: 1100px)')
  const form = Number(sp.get('form') || 0)
  const status = sp.get('status') || ''
  const [q, setQ] = useState(sp.get('q') || '')
  const dq = useDebounce(q, 250)
  const [limit, setLimit] = useState(100)
  const qs = `form=${form || ''}&status=${status === 'starred' ? '' : status}&starred=${status === 'starred' ? 1 : ''}&q=${encodeURIComponent(dq)}&limit=${limit}`
  const { data, reload } = useAsync(() => api.get<List>(`${V1}/responses?${qs}`), [qs])
  useOnChange(reload)
  useEffect(() => { const t = window.setInterval(() => { if (document.visibilityState === 'visible') reload() }, 60000); return () => window.clearInterval(t) }, [reload])
  const set = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n, { replace: true }) }
  const keep = sp.toString() ? `?${sp.toString()}` : ''
  const open = (aid: number) => navigate(`/answers/${aid}${keep}`)
  const cur = forms?.find(f => f.id === form)
  useCrumb(openID && !wide ? 'Answer' : cur?.title)

  const list = (
    <div className="inbox-list">
      <div className="inbox-filters">
        <div className="row wrap filters">
          <input className="search-in" value={q} onChange={e => setQ(e.target.value)} placeholder="Search answers…" aria-label="Search answers" />
          <Select ariaLabel="Form" value={String(form || '')} onChange={v => set('form', v)} placeholder="Every form" clearLabel="Every form"
            options={(forms || []).map(f => ({ value: String(f.id), label: `${f.emoji ? f.emoji + ' ' : ''}${f.title}`, hint: f.new ? `${f.new} new` : String(f.responses) }))} />
        </div>
        <div className="scroll-x"><Segmented size="sm" value={status} onChange={v => set('status', v)} options={FILTERS} label="Show" /></div>
      </div>
      {!data ? <Loading /> : data.items.length === 0 ? <Empty icon="inbox" title={q || status || form ? 'Nothing here' : 'No answers yet'}>
        {!(q || status || form) && <><p>Answers people send to your forms land here.</p>{forms && forms.length === 0 ? <button type="button" className="btn primary mt" onClick={() => openCreate('form')}>Make your first form</button> : <Link to="/forms" className="btn mt">Your forms</Link>}</>}
      </Empty> : <section className="card flush"><ul className="list">{data.items.map(a => <AnswerRow key={a.id} a={a} active={a.id === openID} onOpen={() => open(a.id)} showForm={!form} />)}</ul></section>}
      {data && data.total > data.items.length && <button type="button" className="btn ghost sm" onClick={() => setLimit(l => l + 200)}>Show more ({data.items.length} of {data.total})</button>}
    </div>
  )
  if (!wide) {
    if (openID) return <AnswerPanel key={openID} id={openID} back={() => navigate(`/${keep}`)} onGo={open} formFilter={form} />
    return <div className="page stack loose"><InboxHead count={data?.total} form={cur?.title} />{list}</div>
  }
  return (
    <div className="inbox">
      <div className="inbox-left"><InboxHead count={data?.total} form={cur?.title} />{list}</div>
      <div className="inbox-right">
        {openID ? <AnswerPanel key={openID} id={openID} onGo={open} formFilter={form} /> : <div className="inbox-empty muted"><Icon name="inbox" size={28} /><p>Open an answer to read it, reply, or send it on to your other apps.</p></div>}
      </div>
    </div>
  )
}

function InboxHead({ count, form }: { count?: number; form?: string }) {
  return <div className="page-head"><div className="grow"><h1 className="page-title">{form || 'Answers'}</h1><p className="muted">{count === undefined ? ' ' : count === 0 ? 'Nothing to show.' : `${count.toLocaleString()} ${count === 1 ? 'answer' : 'answers'}`}</p></div></div>
}

function AnswerRow({ a, active, onOpen, showForm }: { a: Answer; active: boolean; onOpen: () => void; showForm: boolean }) {
  const who = a.name || a.email || `Answer #${a.number}`
  const made = [...new Set((a.made || []).filter(m => !m.error).map(m => m.target))]
  return (
    <li>
      <button type="button" className={`list-row answer-row ${active ? 'active' : ''} ${a.status === 'new' ? 'is-new' : ''}`} onClick={onOpen}>
        <Avatar name={a.name || undefined} email={a.email || String(a.id)} size={34} />
        <span className="grow row-main">
          <span className="row-title"><span className="ellipsis">{who}</span>{a.status === 'new' && <span className="new-dot" title="New" />}{a.starred && <Icon name="star" size={13} className="star-on" />}</span>
          {showForm && <span className="muted small ellipsis">{a.form_emoji} {a.form_title} · #{a.number}</span>}
          {a.preview && <span className="small ellipsis row-preview">{a.preview}</span>}
        </span>
        <span className="row-side">
          <span className="muted small" title={fmtDateTime(a.created_at)}>{ago(a.created_at)}</span>
          <span className="made-icons">{a.status === 'done' && <Icon name="check" size={13} className="ok" />}{made.map(t => <Icon key={t} name={APP_INFO[TARGET_APP[t]]?.icon || 'dot'} size={13} />)}</span>
        </span>
      </button>
    </li>
  )
}

function AnswerPanel({ id, back, onGo, formFilter }: { id: number; back?: () => void; onGo: (id: number) => void; formFilter: number }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const { on, why } = useApps()
  const [v, setV] = useState<AnswerView | null>(null)
  const [err, setErr] = useState('')
  const [sheet, setSheet] = useState<'' | 'accept' | 'reply' | 'contact' | 'project' | 'task' | 'event' | 'note' | 'signing' | 'deal'>('')
  const [del, setDel] = useState(false)
  const [note, setNote] = useState('')
  const load = useCallback(async () => {
    try { const x = await api.get<AnswerView>(`${V1}/responses/${id}?form=${formFilter || ''}`); setV(x); setNote(x.response.note) } catch (e) { setErr(errMsg(e)) }
  }, [id, formFilter])
  useEffect(() => { load().then(() => announceChange()) }, [load])
  const r = v?.response
  const patch = async (p: Partial<{ status: string; starred: boolean; note: string }>, msg?: string) => {
    if (!r) return
    try { const n = await api.put<typeof r>(`${V1}/responses/${r.id}`, p); setV(x => x && { ...x, response: { ...x.response, ...n } }); announceChange(); if (msg) toast(msg) } catch (e) { toast(errMsg(e), 'err') }
  }
  const madeOne = (m: Made) => { setSheet(''); setV(x => x && { ...x, response: { ...x.response, made: [...(x.response.made || []).filter(o => !(o.target === m.target && o.error)), m] } }); announceChange() }
  const keyRef = useRef<(e: KeyboardEvent) => void>(() => {})
  keyRef.current = (e: KeyboardEvent) => {
    if (!v || !r || sheet || del) return
    const t = e.target as HTMLElement
    if (e.metaKey || e.ctrlKey || e.altKey || t.closest('input, textarea, [contenteditable="true"]') || document.querySelector('.modal-backdrop, .menu, .sel-pop')) return
    if (e.key === 'j' && v.next) onGo(v.next)
    else if (e.key === 'k' && v.prev) onGo(v.prev)
    else if (e.key === 'e') patch({ status: r.status === 'done' ? 'seen' : 'done' })
    else if (e.key === 'x') patch({ starred: !r.starred })
    else if (e.key === 'r' && r.email && on('mail')) setSheet('reply')
    else return
    e.preventDefault()
  }
  useEffect(() => { const h = (e: KeyboardEvent) => keyRef.current(e); window.addEventListener('keydown', h); return () => window.removeEventListener('keydown', h) }, [])
  if (err) return <div className="answer-panel"><div className="callout danger"><Icon name="alert" size={15} /><span>{err}</span></div></div>
  if (!v || !r) return <div className="answer-panel"><Loading /></div>
  const who = r.name || r.email || `Answer #${r.number}`
  const isBrief = !!r.email && (v.form.template === 'project' || v.items.some(i => i.role === 'subject' || i.role === 'budget' || i.role === 'deadline'))
  const aiOn = !!settings?.ai_help
  const gate = (slug: string, label: string, icon: string, target: typeof sheet, extraOff?: string): MenuItem => ({
    label: on(slug) && !extraOff ? label : `${label} — ${extraOff || (why(slug) ? 'not connected' : 'not connected')}`, icon, disabled: !on(slug) || !!extraOff, onSelect: () => setSheet(target),
  })
  const makeItems = (): MenuItem[] => [
    gate('contacts', 'Add to Pocket Contacts', 'users', 'contact', !r.name && !r.email ? 'no name or email' : ''),
    gate('projects', 'Start a project', 'layers', 'project'),
    gate('tasks', 'Make a task', 'check-square', 'task'),
    gate('calendar', 'Add to Pocket Calendar', 'calendar', 'event'),
    gate('notes', 'Save as a note', 'note', 'note'),
    gate('sign', 'Send a document to sign', 'pen', 'signing', !r.email ? 'no email' : ''),
    gate('crm', 'Add to Pocket CRM as a deal', 'target', 'deal'),
  ]
  const moreItems = (): MenuItem[] => [
    { label: r.status === 'new' ? 'Mark as read' : 'Mark as new', icon: 'eye', onSelect: () => patch({ status: r.status === 'new' ? 'seen' : 'new' }) },
    { label: r.status === 'spam' ? 'Not spam' : 'Spam', icon: 'flag', onSelect: () => patch({ status: r.status === 'spam' ? 'seen' : 'spam' }, r.status === 'spam' ? 'Back in the answers' : 'Moved to spam') },
    { label: 'Open the form', icon: 'external', onSelect: () => window.open(v.form.url, '_blank') },
    'sep',
    { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDel(true) },
  ]
  return (
    <div className="answer-panel">
      <div className="answer-bar">
        {back && <button type="button" className="btn ghost icon-only" aria-label="Back to the answers" onClick={back}><Icon name="chevron-left" size={18} /></button>}
        <button type="button" className="btn ghost icon-only" aria-label="Newer answer" title="Newer (k)" disabled={!v.prev} onClick={() => onGo(v.prev)}><Icon name="chevron-up" size={17} /></button>
        <button type="button" className="btn ghost icon-only" aria-label="Older answer" title="Older (j)" disabled={!v.next} onClick={() => onGo(v.next)}><Icon name="chevron-down" size={17} /></button>
        <span className="spacer" />
        <button type="button" className={`btn ghost icon-only ${r.starred ? 'star-on' : ''}`} aria-label={r.starred ? 'Remove the star' : 'Star'} title="Star (x)" onClick={() => patch({ starred: !r.starred })}><Icon name="star" size={17} /></button>
        <button type="button" className={`btn sm ${r.status === 'done' ? 'active' : ''}`} title="Done (e)" onClick={() => patch({ status: r.status === 'done' ? 'seen' : 'done' }, r.status === 'done' ? undefined : 'Marked done')}><Icon name="check" size={15} />{r.status === 'done' ? 'Done' : 'Mark done'}</button>
        <MenuButton label="More" items={moreItems} align="right"><Icon name="more" size={17} /></MenuButton>
      </div>
      <header className="answer-head">
        <Avatar name={r.name || undefined} email={r.email || String(r.id)} size={44} />
        <div className="grow">
          <h2>{who}</h2>
          <p className="muted small">{r.email && r.name ? <>{r.email} · </> : null}<Link to={`/forms/${v.form.id}`}>{v.form.emoji} {v.form.title}</Link> · #{r.number} · {fmtDateTime(r.created_at)}{r.origin === 'import' ? ' · imported' : ''}</p>
        </div>
        <span className={`pill st-${r.status}`}>{STATUS_LABEL[r.status]}</span>
      </header>
      <div className="answer-actions row wrap">
        {isBrief && <button type="button" className="btn primary" onClick={() => setSheet('accept')} disabled={!on('contacts') && !on('projects') && !on('tasks') && !on('crm')} title="Contact, project and a follow-up task in one step"><Icon name="check-circle" size={15} />Accept the brief</button>}
        {r.email && <button type="button" className={`btn ${isBrief ? '' : 'primary'}`} onClick={() => setSheet('reply')} disabled={!on('mail')} title={on('mail') ? 'Reply (r)' : `Pocket Mail: ${why('mail') || 'not connected'}`}><Icon name="mail" size={15} />Reply</button>}
        <MenuButton label="Send on to an app" className="btn" items={makeItems} width={300}><Icon name="layers" size={15} />Send on…<Icon name="chevron-down" size={14} /></MenuButton>
      </div>
      {(r.made || []).length > 0 && <div className="made-list">
        {(r.made || []).map(m => <MadeChip key={m.id} m={m} />)}
      </div>}
      <dl className="answer-items">
        {v.items.map(i => i.type === 'section'
          ? <div key={i.id} className="answer-sec">{i.label}</div>
          : <div key={i.id} className={`answer-item ${i.text ? '' : 'is-empty'}`}>
            <dt>{i.label}</dt>
            <dd>{!i.text ? <span className="faint">No answer</span> : i.type === 'rating' ? <span className="stars">{[1, 2, 3, 4, 5].map(n => <Icon key={n} name="star" size={15} className={n <= Number(i.text) ? 'on' : ''} />)}<span className="muted small">{i.text} of 5</span></span>
              : Array.isArray(i.value) ? <span className="chips">{i.value.map(x => <span key={x} className="chip">{x}</span>)}</span>
              : i.type === 'email' ? <a href={`mailto:${i.text}`}>{i.text}</a>
              : i.type === 'url' ? <a href={i.text} target="_blank" rel="noreferrer noopener">{i.text.replace(/^https?:\/\//, '')}</a>
              : i.type === 'phone' ? <a href={`tel:${i.text.replace(/[^\d+]/g, '')}`}>{i.text}</a>
              : i.type === 'date' ? fmtDate(i.text)
              : <span className="pre">{i.text}</span>}</dd>
          </div>)}
      </dl>
      <div className="answer-note">
        <label className="label" htmlFor="note">Your note <span className="muted small">(only you see it)</span></label>
        <textarea id="note" data-gramm="false" rows={2} value={note} onChange={e => setNote(e.target.value)} onBlur={() => { if (note !== r.note) patch({ note }, 'Note saved') }} placeholder="Call on Monday, sent a quote…" />
      </div>
      {sheet === 'accept' && <AcceptSheet view={v} onClose={() => setSheet('')} onDone={() => { setSheet(''); load(); announceChange() }} />}
      {sheet === 'reply' && <ReplySheet view={v} aiOn={aiOn} onClose={() => setSheet('')} onDone={madeOne} />}
      {['contact', 'project', 'task', 'event', 'note', 'signing', 'deal'].includes(sheet) && <MakeSheet view={v} target={sheet} onClose={() => setSheet('')} onDone={madeOne} />}
      {del && <Confirm title="Delete this answer?" message={`${who}’s answer to “${v.form.title}” is deleted. What was made from it in your other apps stays there.`} confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={async () => { setDel(false); try { await api.del(`${V1}/responses/${r.id}`); announceChange(); toast('Deleted'); if (v.next) onGo(v.next); else if (v.prev) onGo(v.prev); else back?.() } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}

function MadeChip({ m }: { m: Made }) {
  const app = APP_INFO[TARGET_APP[m.target]]
  const body = <><Icon name={m.error ? 'alert' : app?.icon || 'dot'} size={13} /><span className="ellipsis">{TARGET_LABEL[m.target] || m.target}{m.title ? `: ${m.title}` : ''}</span>{m.auto && <span className="muted small">by itself</span>}{m.url && <Icon name="external" size={12} />}</>
  if (m.error) return <span className="made-chip bad" title={m.error}>{body}<span className="small">— {m.error}</span></span>
  return m.url ? <a className="made-chip" href={m.url} target="_blank" rel="noreferrer" title={`Open in ${app?.name}`}>{body}</a> : <span className="made-chip">{body}</span>
}

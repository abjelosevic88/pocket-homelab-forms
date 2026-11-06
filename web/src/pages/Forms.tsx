// Your forms (taking answers first), and a new form: from a ready-made one,
// from a sentence for Pocket AI, or blank.
import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useData } from '../lib/data'
import { APP_INFO, LANGUAGES, formState } from '../lib/forms'
import { ago } from '../lib/format'
import type { Form, FormField, Template } from '../lib/types'
import { openCreate, useCrumb } from '../components/Layout'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Empty, Loading, Spinner, errMsg, useAsync, useToast } from '../components/ui'

export default function Forms() {
  const { forms } = useData()
  const navigate = useNavigate()
  const { toast } = useToast()
  const copy = (f: Form) => { navigator.clipboard?.writeText(f.url).then(() => toast('Link copied'), () => toast(f.url)) }
  return (
    <div className="page stack loose">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Forms</h1><p className="muted">A form takes answers once you open it. Share its link, put it on your website, or send it in a message.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('form')}><Icon name="plus" size={15} />New form</button>
      </div>
      {!forms ? <Loading /> : forms.length === 0 ? <Empty icon="list" title="No forms yet">
        <p>Start from a ready-made one — a client brief, feedback, a sign-up — or describe what you need to Pocket AI.</p>
        <button type="button" className="btn primary mt" onClick={() => openCreate('form')}>Make your first form</button>
      </Empty> : <div className="form-grid-cards">
        {forms.map(f => {
          const st = formState(f)
          return (
            <article key={f.id} className="form-card" onClick={() => navigate(`/forms/${f.id}`)}>
              <div className="fc-head">
                <span className="fc-emoji" aria-hidden="true">{f.emoji || '📝'}</span>
                <span className={`pill ${st.cls}`}><Icon name={st.icon} size={11} />{st.label}</span>
              </div>
              <h3 className="fc-title">{f.title}</h3>
              <div className="fc-nums">
                <Link to={`/?form=${f.id}`} onClick={e => e.stopPropagation()} className="fc-num"><b className="tnum">{f.responses}</b> {f.responses === 1 ? 'answer' : 'answers'}{f.new > 0 && <span className="badge accent">{f.new} new</span>}</Link>
                <span className="fc-num muted"><b className="tnum">{f.views}</b> opened</span>
              </div>
              <div className="fc-foot muted small">
                <span className="grow ellipsis">{f.last_answer_at ? `Last answer ${ago(f.last_answer_at)}` : f.status === 'draft' ? `Changed ${ago(f.updated_at)}` : 'No answers yet'}</span>
                {f.status !== 'draft' && <button type="button" className="btn ghost sm icon-only" aria-label="Copy the link" title="Copy the link" onClick={e => { e.stopPropagation(); copy(f) }}><Icon name="link" size={14} /></button>}
                <a className="btn ghost sm icon-only" href={f.url} target="_blank" rel="noreferrer" aria-label="Open the form" title={f.status === 'draft' ? 'Preview' : 'Open the form'} onClick={e => e.stopPropagation()}><Icon name="external" size={14} /></a>
              </div>
            </article>
          )
        })}
      </div>}
    </div>
  )
}

export function NewForm() {
  const navigate = useNavigate()
  const [sp] = useSearchParams()
  const { settings } = useApp()
  const { toast } = useToast()
  const { data: tpls } = useAsync(() => api.get<Template[]>(`${V1}/templates`), [])
  const { data: ai } = useAsync(() => api.get<{ ready: boolean; detail: string }>(`${V1}/ai/status`), [])
  const [busy, setBusy] = useState('')
  const [desc, setDesc] = useState('')
  const [lang, setLang] = useState(settings?.language || 'en')
  useCrumb('New form')
  useEffect(() => { if (sp.get('ai')) document.getElementById('ai-desc')?.focus() }, [sp])
  const make = async (body: Record<string, unknown>, key: string) => {
    setBusy(key)
    try { const f = await api.post<Form>(`${V1}/forms`, body); announceChange(); navigate(`/forms/${f.id}`) } catch (e) { toast(errMsg(e), 'err'); setBusy('') }
  }
  const draft = async () => {
    setBusy('ai')
    try {
      const d = await api.post<{ title: string; description: string; emoji: string; thanks: string; submit_label: string; fields: FormField[] }>(`${V1}/ai/draft`, { description: desc, language: lang })
      await make({ ...d, language: lang }, 'ai')
    } catch (e) { toast(errMsg(e), 'err'); setBusy('') }
  }
  const aiOK = !!ai?.ready && settings?.ai_help !== false
  return (
    <div className="page stack loose">
      <div className="page-head"><div className="grow"><h1 className="page-title">A new form</h1><p className="muted">Pick one to start from; every question, word and setting can be changed afterwards. Nobody sees it until you open it.</p></div></div>
      <section className="card ai-start">
        <div className="row" style={{ gap: 10 }}><Icon name="sparkles" size={18} /><h2 className="card-h" style={{ margin: 0 }}>Describe it, and Pocket AI makes the questions</h2></div>
        <textarea id="ai-desc" data-gramm="false" rows={2} value={desc} onChange={e => setDesc(e.target.value)} disabled={!aiOK}
          placeholder="e.g. Sign-ups for a Saturday photography walk in Mostar: name, phone, camera or phone, and whether they need a lift from Sarajevo" />
        <div className="row wrap">
          <Select ariaLabel="Language" value={lang} onChange={setLang} options={LANGUAGES} />
          <button type="button" className="btn primary" onClick={draft} disabled={!aiOK || !desc.trim() || !!busy}>{busy === 'ai' ? <Spinner /> : <Icon name="sparkles" size={15} />}Make the form</button>
          {ai && !aiOK && <span className="muted small">{settings?.ai_help === false ? 'AI help is switched off in Settings → AI.' : `No AI connected${ai.detail ? ` (${ai.detail})` : ''}.`}</span>}
        </div>
      </section>
      {!tpls ? <Loading /> : <div className="tpl-grid">
        {tpls.map(t => (
          <button key={t.key} type="button" className="tpl-card" onClick={() => make({ template: t.key }, t.key)} disabled={!!busy}>
            <span className="tpl-emoji" aria-hidden="true">{busy === t.key ? <Spinner size={20} /> : t.emoji}</span>
            <span className="tpl-text">
              <b>{t.name}</b>
              <span className="muted small">{t.blurb}</span>
              {(t.links || []).length > 0 && <span className="tpl-links">{(t.links || []).map(l => <span key={l} className="tpl-app" title={APP_INFO[l].name}><Icon name={APP_INFO[l].icon} size={12} />{APP_INFO[l].name.replace('Pocket ', '')}</span>)}</span>}
            </span>
          </button>
        ))}
      </div>}
      <p className="muted small">Have answers somewhere else already? <button type="button" className="linkish" onClick={() => openCreate('import')}>Bring them from Google Forms, Typeform, Tally or Microsoft Forms</button>.</p>
    </div>
  )
}

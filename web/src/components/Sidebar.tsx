// Left column: a new form (and the new-form menu), the answers inbox and
// the forms, then the forms taking answers with their new answers. The
// Pocket Calendar sidebar.
import { useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const { forms, newCount } = useData()
  const navigate = useNavigate()
  const loc = useLocation()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const title = settings?.title || status?.title || 'Pocket Forms'
  const sp = new URLSearchParams(loc.search)
  const onInbox = loc.pathname === '/' || loc.pathname.startsWith('/answers')
  const curForm = onInbox ? Number(sp.get('form') || 0) : 0
  const curStatus = onInbox ? sp.get('status') || '' : ''
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string }[] = [
    { to: '/', label: 'Answers', icon: 'inbox', active: onInbox && !curForm && curStatus !== 'starred', count: newCount, hint: 'New answers' },
    { to: '/?status=starred', label: 'Starred', icon: 'star', active: onInbox && !curForm && curStatus === 'starred' },
    { to: '/forms', label: 'Forms', icon: 'list', active: loc.pathname.startsWith('/forms') },
  ]
  const live = (forms || []).filter(f => f.status !== 'draft').slice(0, 12)
  return (
    <aside className="sidebar" aria-label="Pocket Forms">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Forms' ? title : <>Pocket <b>Forms</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <div className="create-split">
        <button type="button" className="btn primary create-btn" onClick={() => onCreate('form')} title="New form (N)">
          <Icon name="plus" size={15} strokeWidth={2.2} />New form
        </button>
        <button type="button" className="btn primary create-more" aria-label="More ways to start" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
          <Icon name="chevron-down" size={15} />
        </button>
      </div>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={290} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className="sb-count" title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {live.length > 0 && <section className="sb-sec">
        <h4 className="sb-h">Answers by form</h4>
        {live.map(f => (
          <button key={f.id} type="button" className={`sb-row sb-list ${curForm === f.id ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/?form=${f.id}`) }} title={f.taking ? 'Taking answers' : 'Closed'}>
            <span className="sb-list-ico sb-emoji" aria-hidden="true">{f.emoji || '📝'}</span>
            <span className="grow ellipsis">{f.title}</span>
            {f.new > 0 ? <span className="sb-dot">{f.new}</span> : <span className="muted small tnum">{f.responses}</span>}
          </button>
        ))}
      </section>}
      {forms && forms.length === 0 && <section className="sb-sec">
        <Link to="/forms/new" className="sb-row sb-list" onClick={onNavigate}><span className="sb-list-ico" aria-hidden="true"><Icon name="plus" size={14} /></span><span className="grow">Make your first form</span></Link>
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pfr:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}

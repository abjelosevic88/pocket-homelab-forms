// Bring answers in: a Pocket Forms export, or the answers of a form from
// Google Forms, Typeform, Tally, Microsoft Forms or Jotform as CSV.
import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { upload, V1 } from '../lib/api'
import { announceChange } from '../lib/data'
import { Icon } from './icons'
import { Modal, Spinner, errMsg, useToast } from './ui'

type Result = { form_id?: number; form?: string; created?: boolean; answers: number; already?: number; forms?: number; skipped?: number; questions?: number }

export function ImportModal({ onClose }: { onClose: () => void }) {
  const { toast } = useToast()
  const navigate = useNavigate()
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<Result | null>(null)
  const pick = async (file: File) => {
    setBusy(true)
    try { setRes(await upload<Result>(`${V1}/import`, 'file', file)); announceChange() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Bring answers here" onClose={onClose} footer={res ? <>
      <button type="button" className="btn" onClick={onClose}>Close</button>
      {res.form_id && <button type="button" className="btn primary" onClick={() => { onClose(); navigate(`/?form=${res.form_id}`) }}>See the answers</button>}
    </> : undefined}>
      {res ? <div className="stack">
        <div className="callout ok"><Icon name="check-circle" size={15} /><div>
          {res.form ? <>{res.created ? <>Made the form <b>{res.form}</b> with {res.questions} questions</> : <>Added to <b>{res.form}</b></>} and brought {res.answers} {res.answers === 1 ? 'answer' : 'answers'}{res.already ? ` (${res.already} were here already)` : ''}.</>
            : <>Brought {res.forms} {res.forms === 1 ? 'form' : 'forms'} and {res.answers} {res.answers === 1 ? 'answer' : 'answers'}{res.skipped ? `; ${res.skipped} could not be read` : ''}.</>}
        </div></div>
        {res.form && <p className="muted small">The questions were guessed from the answers: emails became Email questions, a few repeating answers became choices. Check them under Forms; the form is closed until you open it.</p>}
      </div> : <div className="stack">
        <p className="muted">Bringing the same file again adds nothing twice, so you can keep importing as new answers come in somewhere else.</p>
        <div className="import-ways">
          <div className="radio-row"><Icon name="file" size={18} /><div><b>Answers as a spreadsheet (CSV)</b><p className="muted small">Google Forms (Responses → Download .csv), Typeform, Tally, Microsoft Forms, Jotform. A new form is made from the first row.</p></div></div>
          <div className="radio-row"><Icon name="archive" size={18} /><div><b>A Pocket Forms export</b><p className="muted small">The JSON file from Settings → Backup & data, from this box or another one.</p></div></div>
        </div>
        <input ref={input} type="file" accept=".csv,.json,text/csv,application/json" hidden onChange={e => { const f = e.target.files?.[0]; if (f) pick(f); e.target.value = '' }} />
        <button type="button" className="btn primary" onClick={() => input.current?.click()} disabled={busy}>{busy ? <Spinner /> : <Icon name="upload" size={15} />}Choose a file…</button>
      </div>}
    </Modal>
  )
}

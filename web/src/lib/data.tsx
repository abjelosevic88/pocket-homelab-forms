// The person's forms and how many answers wait, shared by every page.
import React, { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api, V1 } from './api'
import type { Form } from './types'

const EVENT = 'pfr:changed'

interface DataState {
  forms: Form[] | null
  newCount: number
  reload: () => Promise<void>
}
const Ctx = createContext<DataState>({ forms: null, newCount: 0, reload: async () => {} })
export function useData() { return useContext(Ctx) }

/** Something changed (an answer marked, a form saved): lists and counts reload. */
export function announceChange() { window.dispatchEvent(new CustomEvent(EVENT)) }

export function useOnChange(fn: () => void) {
  useEffect(() => {
    window.addEventListener(EVENT, fn)
    return () => window.removeEventListener(EVENT, fn)
  }, [fn])
}

export function DataProvider({ children }: { children: React.ReactNode }) {
  const [forms, setForms] = useState<Form[] | null>(null)
  const reload = useCallback(async () => { try { setForms(await api.get<Form[]>(`${V1}/forms`)) } catch { setForms(f => f || []) } }, [])
  useEffect(() => { reload() }, [reload])
  useOnChange(reload)
  // new answers arrive while the page is open: look once a minute, quietly
  useEffect(() => {
    const t = window.setInterval(() => { if (document.visibilityState === 'visible') reload() }, 60000)
    return () => window.clearInterval(t)
  }, [reload])
  const newCount = (forms || []).reduce((n, f) => n + f.new, 0)
  return <Ctx.Provider value={{ forms, newCount, reload }}>{children}</Ctx.Provider>
}

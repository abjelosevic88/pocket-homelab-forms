import type { AnswerStatus, AppSlug, FieldType, Form, Role } from './types'

export const FIELD_TYPES: { type: FieldType; label: string; icon: string; help: string }[] = [
  { type: 'text', label: 'Short answer', icon: 'text', help: 'One line: a name, a city' },
  { type: 'long', label: 'Paragraph', icon: 'list', help: 'A longer text' },
  { type: 'email', label: 'Email', icon: 'mail', help: 'Checked to be an address' },
  { type: 'phone', label: 'Phone', icon: 'phone', help: 'A phone number' },
  { type: 'number', label: 'Number', icon: 'percent', help: 'How many, how much' },
  { type: 'url', label: 'Web address', icon: 'link', help: 'A website' },
  { type: 'date', label: 'Date', icon: 'calendar', help: 'Pick a day' },
  { type: 'time', label: 'Time', icon: 'clock', help: 'Pick a time' },
  { type: 'choice', label: 'One of a list', icon: 'circle', help: 'Pick one, shown as buttons' },
  { type: 'checkboxes', label: 'Several of a list', icon: 'check-square', help: 'Pick as many as you like' },
  { type: 'dropdown', label: 'Dropdown', icon: 'chevron-down', help: 'Pick one from a long list' },
  { type: 'yesno', label: 'Yes or no', icon: 'check-circle', help: 'Two buttons' },
  { type: 'rating', label: 'Stars', icon: 'star', help: '1 to 5 stars' },
  { type: 'consent', label: 'I agree', icon: 'shield', help: 'A box to tick' },
  { type: 'section', label: 'Heading', icon: 'tag', help: 'Words between questions' },
]

export const typeInfo = (t: FieldType) => FIELD_TYPES.find(x => x.type === t) || FIELD_TYPES[0]
export const hasOptions = (t: FieldType) => t === 'choice' || t === 'checkboxes' || t === 'dropdown'

export const ROLES: { role: Role; label: string; help: string }[] = [
  { role: '', label: 'Nothing special', help: '' },
  { role: 'name', label: 'Their name', help: 'Shown in the inbox; the name of a new contact' },
  { role: 'email', label: 'Their email', help: 'To reply and send a copy; the contact’s email' },
  { role: 'phone', label: 'Their phone', help: 'The contact’s phone' },
  { role: 'company', label: 'Their company', help: 'The contact’s company; the project’s name' },
  { role: 'website', label: 'Their website', help: 'The contact’s website' },
  { role: 'subject', label: 'What it is about', help: 'The line in the inbox; the name of a project or task' },
  { role: 'message', label: 'The main text', help: 'The summary of a project' },
  { role: 'budget', label: 'Budget', help: '' },
  { role: 'deadline', label: 'Deadline', help: 'The project’s target date' },
  { role: 'date', label: 'A day for the calendar', help: 'Put in Pocket Calendar' },
]

export const LANGUAGES = [{ value: 'en', label: 'English' }, { value: 'bs', label: 'Bosanski' }, { value: 'hr', label: 'Hrvatski' }, { value: 'sr', label: 'Srpski' }, { value: 'de', label: 'Deutsch' }]

export const ACCENTS = ['#5f3dc4', '#3157d5', '#0f766e', '#15803d', '#b4532a', '#be123c', '#a16207', '#1f2937']

export const APP_INFO: Record<AppSlug, { name: string; icon: string; verb: string }> = {
  contacts: { name: 'Pocket Contacts', icon: 'users', verb: 'Add to contacts' },
  projects: { name: 'Pocket Projects', icon: 'layers', verb: 'Start a project' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square', verb: 'Make a task' },
  calendar: { name: 'Pocket Calendar', icon: 'calendar', verb: 'Add to calendar' },
  notes: { name: 'Pocket Notes', icon: 'note', verb: 'Save as a note' },
  mail: { name: 'Pocket Mail', icon: 'mail', verb: 'Reply' },
  sign: { name: 'Pocket Sign', icon: 'pen', verb: 'Send a document to sign' },
  crm: { name: 'Pocket CRM', icon: 'target', verb: 'Add as a deal' },
}

export const TARGET_APP: Record<string, AppSlug> = { contact: 'contacts', project: 'projects', task: 'tasks', event: 'calendar', note: 'notes', email: 'mail', signing: 'sign', deal: 'crm' }
export const TARGET_LABEL: Record<string, string> = { contact: 'Contact', project: 'Project', task: 'Task', event: 'Event', note: 'Note', email: 'Email', signing: 'To sign', deal: 'Deal' }

export const STATUS_LABEL: Record<AnswerStatus, string> = { new: 'New', seen: 'Read', done: 'Done', spam: 'Spam' }

export function formState(f: Form): { label: string; cls: string; icon: string } {
  if (f.status === 'draft') return { label: 'Draft', cls: 'st-draft', icon: 'edit' }
  if (f.status === 'closed') return { label: 'Closed', cls: 'st-closed', icon: 'lock' }
  if (f.why === 'date') return { label: 'Closing day passed', cls: 'st-closed', icon: 'clock' }
  if (f.why === 'full') return { label: 'Full', cls: 'st-closed', icon: 'check-circle' }
  return { label: 'Taking answers', cls: 'st-open', icon: 'dot' }
}

export function newFieldID(fields: { id: string }[]): string {
  let n = fields.length + 1
  while (fields.some(f => f.id === `q${n}`)) n++
  return `q${n}`
}

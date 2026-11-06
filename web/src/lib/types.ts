export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'contacts' | 'projects' | 'tasks' | 'calendar' | 'notes' | 'mail' | 'sign'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  language: string; accent: string; thanks: string; mark_seen: boolean; project_team: number; task_days: number; mail_from: string
  ai_help: boolean; reply_subject: string
  notify_each: boolean; notify_digest: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; public_url: string; demo: boolean
}

export type FieldType = 'text' | 'long' | 'email' | 'phone' | 'number' | 'url' | 'date' | 'time' | 'choice' | 'checkboxes' | 'dropdown' | 'yesno' | 'rating' | 'consent' | 'section'
export type Role = '' | 'name' | 'email' | 'phone' | 'company' | 'website' | 'subject' | 'message' | 'budget' | 'deadline' | 'date'

export interface FormField {
  id: string; type: FieldType; label: string; help?: string; required?: boolean; options?: string[]; other?: boolean; placeholder?: string; role?: Role
}

export interface Auto { contact: boolean; task: boolean; task_days: number; event: boolean; note: boolean }

export type FormStatus = 'draft' | 'open' | 'closed'

export interface Form {
  id: number; handle: string; title: string; description: string; emoji: string; status: FormStatus; fields: FormField[]
  accent: string; language: string; submit_label: string; thanks: string; redirect_url: string; close_at: string; max_answers: number
  one_per_email: boolean; branding: boolean; notify: boolean; send_copy: boolean; auto: Auto; template: string; source: string
  opened_at: string; created_at: string; updated_at: string
  url: string; responses: number; new: number; views: number; last_answer_at?: string; taking: boolean; why?: '' | 'draft' | 'closed' | 'date' | 'full'; questions: number
}

export type AnswerStatus = 'new' | 'seen' | 'done' | 'spam'

export interface Made { id: number; response_id: number; target: string; app: string; ref: string; title: string; url: string; error?: string; auto: boolean; created_at: string }

export interface Answer {
  id: number; form_id: number; number: number; answers?: Record<string, string | string[]>; name: string; email: string; preview: string
  status: AnswerStatus; starred: boolean; note: string; origin: string; created_at: string; updated_at: string
  form_title?: string; form_emoji?: string; made?: Made[]
}

export interface AnswerItem { id: string; label: string; type: FieldType; role?: Role; value: string | string[] | null; text: string }

export interface AnswerView {
  response: Answer; items: AnswerItem[]; prev: number; next: number; url: string
  form: { id: number; title: string; emoji: string; handle: string; url: string; language: string; has_date: boolean; template: string }
}

export interface Template {
  key: string; name: string; emoji: string; blurb: string; title: string; description: string; thanks: string; submit_label: string
  auto: Auto; fields: FormField[]; links: AppSlug[]
}

export interface AppState { slug: AppSlug; name: string; reason: string; connected: boolean; detail: string }

export interface OptionCount { option: string; count: number }
export interface FieldStats { id: string; label: string; type: FieldType; answered: number; options?: OptionCount[]; average?: number; recent?: string[] }
export interface DayCount { day: string; count: number }
export interface FormStats { views: DayCount[]; answers: DayCount[]; total_views: number; total_answers: number; fields: FieldStats[] }

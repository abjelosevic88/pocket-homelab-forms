package store

import (
	"encoding/json"
	"strconv"
	"strings"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Form statuses.
const (
	FormDraft  = "draft"  // only the owner sees it (a preview)
	FormOpen   = "open"   // takes answers
	FormClosed = "closed" // shows "this form is closed"
)

// FieldTypes are the kinds of question a form can ask. "section" asks
// nothing: it is a heading with some words between questions.
var FieldTypes = []string{"text", "long", "email", "phone", "number", "url", "date", "time",
	"choice", "checkboxes", "dropdown", "yesno", "rating", "consent", "section"}

// FieldRoles say what an answer means for the rest of the box: who the
// person is (name, email, phone, company, website), what they want
// (subject, message, budget, deadline) and a day they picked (date).
var FieldRoles = []string{"name", "email", "phone", "company", "website", "subject", "message", "budget", "deadline", "date"}

// Field is one question.
type Field struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Help        string   `json:"help,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Other       bool     `json:"other,omitempty"` // choice/checkboxes: "Other: …"
	Placeholder string   `json:"placeholder,omitempty"`
	Role        string   `json:"role,omitempty"`
}

// Asks reports whether the field takes an answer.
func (f Field) Asks() bool { return f.Type != "section" }

// Multi reports whether the answer is a list.
func (f Field) Multi() bool { return f.Type == "checkboxes" }

// Auto is what happens by itself when an answer arrives.
type Auto struct {
	Contact  bool `json:"contact"`   // add the person to Pocket Contacts (unless they are there)
	Task     bool `json:"task"`      // a follow-up task in Pocket Tasks
	TaskDays int  `json:"task_days"` // due this many days later (0 = no due date)
	Event    bool `json:"event"`     // the day they picked into Pocket Calendar
	Note     bool `json:"note"`      // the answers as a note in Pocket Notes
}

// Any reports whether anything happens by itself.
func (a Auto) Any() bool { return a.Contact || a.Task || a.Event || a.Note }

// Form is one form.
type Form struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	Handle      string  `json:"handle"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Emoji       string  `json:"emoji"`
	Status      string  `json:"status"`
	Fields      []Field `json:"fields"`
	Accent      string  `json:"accent"`
	Language    string  `json:"language"`
	SubmitLabel string  `json:"submit_label"`
	Thanks      string  `json:"thanks"`
	RedirectURL string  `json:"redirect_url"`
	CloseAt     string  `json:"close_at"`
	MaxAnswers  int     `json:"max_answers"`
	OnePerEmail bool    `json:"one_per_email"`
	Branding    bool    `json:"branding"`
	Notify      bool    `json:"notify"`
	SendCopy    bool    `json:"send_copy"`
	Auto        Auto    `json:"auto"`
	Template    string  `json:"template"`
	Source      string  `json:"source"`
	OpenedAt    string  `json:"opened_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`

	// Filled for lists and pages.
	URL        string `json:"url,omitempty"`
	Responses  int    `json:"responses"`
	New        int    `json:"new"`
	Views      int    `json:"views"`
	LastAnswer string `json:"last_answer_at,omitempty"`
	Taking     bool   `json:"taking"`        // takes answers right now
	Why        string `json:"why,omitempty"` // why not: draft | closed | date | full
	Questions  int    `json:"questions"`     // fields that ask something
}

// Field returns the field with this id.
func (f *Form) Field(id string) (Field, bool) {
	for _, x := range f.Fields {
		if x.ID == id {
			return x, true
		}
	}
	return Field{}, false
}

// RoleField is the first question with this role.
func (f *Form) RoleField(role string) (Field, bool) {
	for _, x := range f.Fields {
		if x.Role == role && x.Asks() {
			return x, true
		}
	}
	return Field{}, false
}

// Response is one set of answers.
type Response struct {
	ID        int64          `json:"id"`
	UserID    int64          `json:"-"`
	FormID    int64          `json:"form_id"`
	Number    int            `json:"number"`
	Answers   map[string]any `json:"answers"`
	Name      string         `json:"name"`
	Email     string         `json:"email"`
	Preview   string         `json:"preview"`
	Status    string         `json:"status"`
	Starred   bool           `json:"starred"`
	Note      string         `json:"note"`
	Origin    string         `json:"origin"`
	IPHash    string         `json:"-"`
	ClientKey string         `json:"-"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`

	// Filled for lists.
	FormTitle string `json:"form_title,omitempty"`
	FormEmoji string `json:"form_emoji,omitempty"`
	Made      []Made `json:"made,omitempty"`
}

// ResponseStatuses are the states of an answer in the inbox.
var ResponseStatuses = []string{"new", "seen", "done", "spam"}

// Text is one answer as words ("" when not answered).
func (r *Response) Text(f Field) string {
	return AnswerText(r.Answers[f.ID])
}

// RoleText is the answer to the first question with this role.
func (r *Response) RoleText(f *Form, role string) string {
	if fl, ok := f.RoleField(role); ok {
		return strings.TrimSpace(r.Text(fl))
	}
	return ""
}

// AnswerText turns a stored answer into words.
func AnswerText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, ", ")
	case []string:
		return strings.Join(t, ", ")
	case bool:
		if t {
			return "Yes"
		}
		return "No"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

// AnswerList is a stored answer as a list (checkboxes; one item otherwise).
func AnswerList(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}
	if s := AnswerText(v); s != "" {
		return []string{s}
	}
	return nil
}

// Made is something made from an answer in another Pocket app.
type Made struct {
	ID         int64  `json:"id"`
	ResponseID int64  `json:"response_id"`
	Target     string `json:"target"`
	App        string `json:"app"`
	Ref        string `json:"ref"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Error      string `json:"error,omitempty"`
	Auto       bool   `json:"auto"`
	CreatedAt  string `json:"created_at"`
}

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// New forms start with these.
	Language string `json:"language"`
	Accent   string `json:"accent"`
	Thanks   string `json:"thanks"`

	// Answers.
	MarkSeen     bool   `json:"mark_seen"`     // opening an answer marks it seen
	ProjectTeam  int64  `json:"project_team"`  // Pocket Projects team for "Start a project" (0 = Projects' default)
	TaskDays     int    `json:"task_days"`     // follow-up tasks are due this many days later
	MailFrom     string `json:"mail_from"`     // which Pocket Mail account replies leave from ("" = its default)
	AIHelp       bool   `json:"ai_help"`       // the Pocket AI buttons
	ReplySubject string `json:"reply_subject"` // "Re: {form}"

	// Notifications.
	NotifyEach   bool   `json:"notify_each"`   // a push for each new answer (a form can switch its own off)
	NotifyDigest bool   `json:"notify_digest"` // morning: answers waiting
	NotifyHour   int    `json:"notify_hour"`
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

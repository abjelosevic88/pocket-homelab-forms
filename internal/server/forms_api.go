package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- forms: the owner's side ----

var (
	reHex     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reFieldID = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

// formURL is the address people open a form at.
func (s *Server) formURL(f *store.Form) string {
	base := firstNonEmpty(s.cfg.PublicURL, s.cfg.BaseURL)
	return strings.TrimRight(base, "/") + "/f/" + f.Handle
}

func (s *Server) formView(f *store.Form) *store.Form {
	f.URL = s.formURL(f)
	return f
}

// cleanFields checks the questions: known types, unique ids, sensible lengths.
func cleanFields(in []store.Field) ([]store.Field, error) {
	out := make([]store.Field, 0, len(in))
	seen := map[string]bool{}
	if len(in) > 200 {
		return nil, fmt.Errorf("a form can have up to 200 questions")
	}
	for i, f := range in {
		f.Type = strings.TrimSpace(f.Type)
		if !oneOf(f.Type, store.FieldTypes) {
			return nil, fmt.Errorf("question %d: unknown type %q", i+1, f.Type)
		}
		f.ID = strings.ToLower(strings.TrimSpace(f.ID))
		if !reFieldID.MatchString(f.ID) || seen[f.ID] {
			f.ID = "q" + store.RandomKey(6)
		}
		seen[f.ID] = true
		f.Label = clip(strings.TrimSpace(f.Label), 300)
		f.Help = clip(strings.TrimSpace(f.Help), 1000)
		f.Placeholder = clip(strings.TrimSpace(f.Placeholder), 120)
		if f.Role != "" && !oneOf(f.Role, store.FieldRoles) {
			f.Role = ""
		}
		switch f.Type {
		case "choice", "checkboxes", "dropdown":
			var o []string
			have := map[string]bool{}
			for _, x := range f.Options {
				x = clip(strings.TrimSpace(x), 200)
				if x != "" && !have[strings.ToLower(x)] && len(o) < 100 {
					have[strings.ToLower(x)] = true
					o = append(o, x)
				}
			}
			f.Options = o
			if f.Type == "dropdown" {
				f.Other = false
			}
		default:
			f.Options, f.Other = nil, false
		}
		if f.Type == "section" {
			f.Required, f.Role = false, ""
		}
		if f.Label == "" && f.Type != "section" {
			return nil, fmt.Errorf("question %d has no question text", i+1)
		}
		out = append(out, f)
	}
	return out, nil
}

type formInput struct {
	Title       *string        `json:"title"`
	Description *string        `json:"description"`
	Emoji       *string        `json:"emoji"`
	Status      *string        `json:"status"`
	Handle      *string        `json:"handle"`
	Fields      *[]store.Field `json:"fields"`
	Accent      *string        `json:"accent"`
	Language    *string        `json:"language"`
	SubmitLabel *string        `json:"submit_label"`
	Thanks      *string        `json:"thanks"`
	RedirectURL *string        `json:"redirect_url"`
	CloseAt     *string        `json:"close_at"`
	MaxAnswers  *int           `json:"max_answers"`
	OnePerEmail *bool          `json:"one_per_email"`
	Branding    *bool          `json:"branding"`
	Notify      *bool          `json:"notify"`
	SendCopy    *bool          `json:"send_copy"`
	Auto        *store.Auto    `json:"auto"`
	Template    string         `json:"template"` // create only
}

// apply copies what was sent onto f; the error is for the person.
func (s *Server) apply(ctx context.Context, f *store.Form, in formInput) error {
	if in.Title != nil {
		f.Title = clip(strings.TrimSpace(*in.Title), 200)
	}
	if in.Description != nil {
		f.Description = clip(strings.TrimSpace(*in.Description), 5000)
	}
	if in.Emoji != nil {
		f.Emoji = clip(strings.TrimSpace(*in.Emoji), 8)
	}
	if in.Status != nil {
		if !oneOf(*in.Status, []string{store.FormDraft, store.FormOpen, store.FormClosed}) {
			return fmt.Errorf("a form is a draft, open or closed")
		}
		f.Status = *in.Status
	}
	if in.Handle != nil {
		h := strings.ToLower(strings.TrimSpace(*in.Handle))
		if h != f.Handle {
			if !store.ValidHandle(h) {
				return fmt.Errorf("the address is 3 to 60 lower-case letters, digits and dashes")
			}
			if !s.store.HandleFree(ctx, h, f.ID) {
				return fmt.Errorf("another form already uses that address")
			}
			f.Handle = h
		}
	}
	if in.Fields != nil {
		fs, err := cleanFields(*in.Fields)
		if err != nil {
			return err
		}
		f.Fields = fs
	}
	if in.Accent != nil {
		if *in.Accent != "" && !reHex.MatchString(*in.Accent) {
			return fmt.Errorf("the colour is like #5f3dc4")
		}
		f.Accent = *in.Accent
	}
	if in.Language != nil {
		if _, ok := siteWords[*in.Language]; !ok {
			return fmt.Errorf("that language is not offered yet")
		}
		f.Language = *in.Language
	}
	if in.SubmitLabel != nil {
		f.SubmitLabel = clip(strings.TrimSpace(*in.SubmitLabel), 60)
	}
	if in.Thanks != nil {
		f.Thanks = clip(strings.TrimSpace(*in.Thanks), 3000)
	}
	if in.RedirectURL != nil {
		u, ok := cleanURL(*in.RedirectURL)
		if !ok {
			return fmt.Errorf("the address after sending must start with https:// or http://")
		}
		f.RedirectURL = u
	}
	if in.CloseAt != nil {
		if !validDate(*in.CloseAt) {
			return fmt.Errorf("the closing day is YYYY-MM-DD")
		}
		f.CloseAt = *in.CloseAt
	}
	if in.MaxAnswers != nil {
		if *in.MaxAnswers < 0 || *in.MaxAnswers > 1000000 {
			return fmt.Errorf("the limit is a number of answers (0 = no limit)")
		}
		f.MaxAnswers = *in.MaxAnswers
	}
	if in.OnePerEmail != nil {
		f.OnePerEmail = *in.OnePerEmail
	}
	if in.Branding != nil {
		f.Branding = *in.Branding
	}
	if in.Notify != nil {
		f.Notify = *in.Notify
	}
	if in.SendCopy != nil {
		f.SendCopy = *in.SendCopy
	}
	if in.Auto != nil {
		a := *in.Auto
		if a.TaskDays < 0 || a.TaskDays > 365 {
			a.TaskDays = 0
		}
		f.Auto = a
	}
	return nil
}

func (s *Server) handleListForms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	fs, err := s.store.ListForms(ctx, actorID(ctx), r.URL.Query().Get("status"))
	if err != nil {
		s.fail(w, err, "forms")
		return
	}
	out := make([]*store.Form, 0, len(fs))
	for _, f := range fs {
		out = append(out, s.formView(f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in formInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	f := &store.Form{UserID: uid, Status: store.FormDraft, Language: firstNonEmpty(st.Language, "en"), Accent: st.Accent,
		Thanks: st.Thanks, Branding: true, Notify: true, Source: linkApp(ctx)}
	if t, ok := templateByKey(in.Template); ok {
		f.Template, f.Title, f.Description, f.Emoji, f.Thanks, f.SubmitLabel, f.Auto = t.Key, t.Title, t.Description, t.Emoji,
			firstNonEmpty(t.Thanks, f.Thanks), t.Submit, t.Auto
		f.Fields = append([]store.Field(nil), t.Fields...)
		if f.Auto.TaskDays == 0 && f.Auto.Task {
			f.Auto.TaskDays = st.TaskDays
		}
	}
	if in.Status != nil && *in.Status != store.FormDraft && linkApp(ctx) != "" {
		in.Status = nil // another app may only start drafts
	}
	if err := s.apply(ctx, f, in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if f.Title == "" {
		f.Title = "Untitled form"
	}
	if f.Handle == "" {
		f.Handle = s.store.NewHandle(ctx, f.Title)
	}
	if f.Status == store.FormOpen {
		f.OpenedAt = store.Now()
	}
	if err := s.store.CreateForm(ctx, f); err != nil {
		s.fail(w, err, "create form")
		return
	}
	if f.Status == store.FormOpen {
		s.publish(ctx, uid, "forms.form_opened", fmt.Sprintf("form:%d", f.ID), s.formPayload(f))
	}
	f, _ = s.store.GetForm(ctx, uid, f.ID)
	writeJSON(w, http.StatusCreated, s.formView(f))
}

// ownForm loads a form of the caller; a linked app may change only the ones it made.
func (s *Server) ownForm(w http.ResponseWriter, r *http.Request, write bool) (*store.Form, bool) {
	ctx := r.Context()
	f, err := s.store.GetForm(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "form")
		return nil, false
	}
	if write {
		if app := linkApp(ctx); app != "" && f.Source != app {
			writeErr(w, http.StatusForbidden, "this form was not made by "+app+"; only its owner can change it")
			return nil, false
		}
	}
	return f, true
}

func (s *Server) handleGetForm(w http.ResponseWriter, r *http.Request) {
	f, ok := s.ownForm(w, r, false)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.formView(f))
}

func (s *Server) handleUpdateForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.ownForm(w, r, true)
	if !ok {
		return
	}
	var in formInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	was := f.Status
	if err := s.apply(ctx, f, in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.saveForm(w, r, f, was)
}

// saveForm stores a changed form and tells the box when it opened or closed.
func (s *Server) saveForm(w http.ResponseWriter, r *http.Request, f *store.Form, was string) {
	ctx := r.Context()
	if f.Status == store.FormOpen && was != store.FormOpen {
		f.OpenedAt = store.Now()
		// opening again after the closing day: that day no longer applies
		if f.CloseAt != "" && f.CloseAt < store.Today() {
			f.CloseAt = ""
		}
	}
	if err := s.store.UpdateForm(ctx, f); err != nil {
		s.fail(w, err, "save form")
		return
	}
	if f.Status != was {
		switch f.Status {
		case store.FormOpen:
			s.publish(ctx, f.UserID, "forms.form_opened", fmt.Sprintf("form:%d", f.ID), s.formPayload(f))
		case store.FormClosed:
			s.publish(ctx, f.UserID, "forms.form_closed", fmt.Sprintf("form:%d", f.ID), s.formPayload(f))
		}
	}
	f, _ = s.store.GetForm(ctx, f.UserID, f.ID)
	writeJSON(w, http.StatusOK, s.formView(f))
}

func (s *Server) handleOpenForm(w http.ResponseWriter, r *http.Request) {
	f, ok := s.ownForm(w, r, true)
	if !ok {
		return
	}
	if f.Questions == 0 {
		writeErr(w, http.StatusBadRequest, "add at least one question first")
		return
	}
	was := f.Status
	f.Status = store.FormOpen
	s.saveForm(w, r, f, was)
}

func (s *Server) handleCloseForm(w http.ResponseWriter, r *http.Request) {
	f, ok := s.ownForm(w, r, true)
	if !ok {
		return
	}
	was := f.Status
	f.Status = store.FormClosed
	s.saveForm(w, r, f, was)
}

func (s *Server) handleDuplicateForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.ownForm(w, r, false)
	if !ok {
		return
	}
	c := *f
	c.ID, c.Status, c.OpenedAt, c.CreatedAt, c.UpdatedAt, c.Source = 0, store.FormDraft, "", "", "", ""
	c.Title = clip(f.Title+" (copy)", 200)
	c.Handle = s.store.NewHandle(ctx, f.Title)
	c.Fields = append([]store.Field(nil), f.Fields...)
	if err := s.store.CreateForm(ctx, &c); err != nil {
		s.fail(w, err, "copy form")
		return
	}
	g, _ := s.store.GetForm(ctx, c.UserID, c.ID)
	writeJSON(w, http.StatusCreated, s.formView(g))
}

func (s *Server) handleDeleteForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteForm(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete form")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckHandle(w http.ResponseWriter, r *http.Request) {
	h := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("handle")))
	but, _ := strconv.ParseInt(r.URL.Query().Get("form"), 10, 64)
	switch {
	case !store.ValidHandle(h):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "why": "3 to 60 lower-case letters, digits and dashes"})
	case !s.store.HandleFree(r.Context(), h, but):
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "why": "another form uses it"})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// closeDue tells the box once when a form's closing day has passed.
func (s *Server) closeDue(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	for _, uid := range ids {
		fs, err := s.store.ListForms(ctx, uid, store.FormOpen)
		if err != nil {
			continue
		}
		for _, f := range fs {
			if f.Why != "date" && f.Why != "full" {
				continue
			}
			if !s.once(ctx, fmt.Sprintf("closed:%d:%s:%s", f.ID, f.Why, f.OpenedAt)) {
				continue
			}
			p := s.formPayload(f)
			p["why"] = map[string]string{"date": "the closing day passed", "full": "the answer limit was reached"}[f.Why]
			s.publish(ctx, uid, "forms.form_closed", fmt.Sprintf("form:%d", f.ID), p)
		}
	}
}

func (s *Server) formPayload(f *store.Form) map[string]any {
	return map[string]any{"form_id": f.ID, "title": f.Title, "handle": f.Handle, "url": s.formURL(f),
		"owner_url": strings.TrimRight(s.cfg.BaseURL, "/") + fmt.Sprintf("/forms/%d", f.ID)}
}

// ---- numbers: who opened, who answered, what they chose ----

type optionCount struct {
	Option string `json:"option"`
	Count  int    `json:"count"`
}

type fieldStats struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Type     string        `json:"type"`
	Answered int           `json:"answered"`
	Options  []optionCount `json:"options,omitempty"`
	Average  float64       `json:"average,omitempty"`
	Recent   []string      `json:"recent,omitempty"` // the latest text answers
}

func (s *Server) handleFormStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.ownForm(w, r, false)
	if !ok {
		return
	}
	from := time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	views, _ := s.store.ViewsByDay(ctx, f.UserID, f.ID, from)
	answers, _ := s.store.AnswersByDay(ctx, f.UserID, f.ID, from)
	rs, _, err := s.store.ListResponses(ctx, f.UserID, store.ResponseFilter{FormID: f.ID, Limit: 5000})
	if err != nil {
		s.fail(w, err, "stats")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"views": views, "answers": answers, "total_views": f.Views, "total_answers": f.Responses,
		"fields": countFields(f, rs)})
}

// countFields tallies the answers per question.
func countFields(f *store.Form, rs []*store.Response) []fieldStats {
	out := []fieldStats{}
	for _, fl := range f.Fields {
		if !fl.Asks() {
			continue
		}
		st := fieldStats{ID: fl.ID, Label: fl.Label, Type: fl.Type}
		counts := map[string]int{}
		sum, rated := 0.0, 0
		for _, r := range rs {
			v, ok := r.Answers[fl.ID]
			if !ok || store.AnswerText(v) == "" {
				continue
			}
			st.Answered++
			switch fl.Type {
			case "choice", "checkboxes", "dropdown", "yesno":
				for _, o := range store.AnswerList(v) {
					counts[o]++
				}
			case "rating", "number":
				if n, err := strconv.ParseFloat(store.AnswerText(v), 64); err == nil {
					sum += n
					rated++
				}
				if fl.Type == "rating" {
					counts[store.AnswerText(v)]++
				}
			case "text", "long":
				if len(st.Recent) < 5 {
					st.Recent = append(st.Recent, clip(store.AnswerText(v), 300))
				}
			}
		}
		if rated > 0 {
			st.Average = float64(int(sum/float64(rated)*10+0.5)) / 10
		}
		order := fl.Options
		switch fl.Type {
		case "yesno":
			order = nil
		case "rating":
			order = []string{"5", "4", "3", "2", "1"}
		}
		done := map[string]bool{}
		for _, o := range order {
			st.Options = append(st.Options, optionCount{o, counts[o]})
			done[o] = true
		}
		var rest []string
		for o := range counts {
			if !done[o] {
				rest = append(rest, o)
			}
		}
		sort.Slice(rest, func(i, j int) bool {
			return counts[rest[i]] > counts[rest[j]] || (counts[rest[i]] == counts[rest[j]] && rest[i] < rest[j])
		})
		for _, o := range rest {
			st.Options = append(st.Options, optionCount{o, counts[o]})
		}
		if len(counts) == 0 && fl.Type != "choice" && fl.Type != "checkboxes" && fl.Type != "dropdown" && fl.Type != "rating" {
			st.Options = nil
		}
		out = append(out, st)
	}
	return out
}

// handleFormCSV downloads every answer of a form as a spreadsheet.
func (s *Server) handleFormCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.ownForm(w, r, false)
	if !ok {
		return
	}
	rs, _, err := s.store.ListResponses(ctx, f.UserID, store.ResponseFilter{FormID: f.ID, Status: r.URL.Query().Get("status"), Oldest: true, Limit: 5000})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	name := firstNonEmpty(store.Slugify(f.Title), "form") + "-answers.csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write([]byte("\ufeff")) // Excel reads it as UTF-8
	cw := csv.NewWriter(w)
	head := []string{"Number", "Sent", "Status", "Starred"}
	var asks []store.Field
	for _, fl := range f.Fields {
		if fl.Asks() {
			asks = append(asks, fl)
			head = append(head, fl.Label)
		}
	}
	head = append(head, "Your note")
	_ = cw.Write(head)
	for _, rr := range rs {
		t, _ := time.Parse(time.RFC3339Nano, rr.CreatedAt)
		row := []string{strconv.Itoa(rr.Number), t.Local().Format("2006-01-02 15:04"), rr.Status, map[bool]string{true: "yes", false: ""}[rr.Starred]}
		for _, fl := range asks {
			row = append(row, csvSafe(rr.Text(fl)))
		}
		row = append(row, csvSafe(rr.Note))
		_ = cw.Write(row)
	}
	cw.Flush()
}

// csvSafe keeps a spreadsheet from running an answer as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "'" + v
		}
	}
	return v
}

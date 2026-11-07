package server

import (
	"context"
	"fmt"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// responseArrived runs after someone sent a form: the event for the rest
// of the box (Pocket Flows starts from it), a push to the owner, what the
// form makes by itself and a copy for the person. It runs in the
// background; nothing here can fail the person who answered.
func (s *Server) responseArrived(ctx context.Context, f *store.Form, r *store.Response) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	s.publish(ctx, f.UserID, "forms.response_received", fmt.Sprintf("response:%d", r.ID), s.responsePayload(f, r))
	st, _ := s.store.GetSettings(ctx, f.UserID)
	if st.NotifyEach && f.Notify {
		title := "New answer: " + f.Title
		body := firstNonEmpty(r.Name, r.Email, fmt.Sprintf("Answer #%d", r.Number))
		if r.Preview != "" {
			body += " — " + r.Preview
		}
		s.push(ctx, f.UserID, st, title, body, s.answerLink(r))
	}
	if !f.Auto.Any() && !(f.SendCopy && r.Email != "") {
		return
	}
	c := s.clients(ctx, f.UserID)
	if f.Auto.Contact {
		_, _ = s.make(ctx, c, f, r, makeIn{Target: "contact"}, true)
	}
	if f.Auto.Task {
		in := makeIn{Target: "task"}
		if f.Auto.TaskDays > 0 {
			in.Due = time.Now().AddDate(0, 0, f.Auto.TaskDays).Format("2006-01-02")
		}
		_, _ = s.make(ctx, c, f, r, in, true)
	}
	if f.Auto.Event && firstNonEmpty(r.RoleText(f, "date"), r.RoleText(f, "deadline")) != "" {
		_, _ = s.make(ctx, c, f, r, makeIn{Target: "event"}, true)
	}
	if f.Auto.Note {
		_, _ = s.make(ctx, c, f, r, makeIn{Target: "note"}, true)
	}
	if f.Auto.Deal {
		_, _ = s.make(ctx, c, f, r, makeIn{Target: "deal"}, true)
	}
	if f.SendCopy && r.Email != "" {
		m := &store.Made{ResponseID: r.ID, Target: "email", App: appMail.slug, Auto: true, Title: "A copy of the answers to " + r.Email}
		if err := s.sendCopy(ctx, c, f, r); err != nil {
			m.Error = plainErr(appMail, err)
		}
		_ = s.store.AddMade(ctx, f.UserID, m)
	}
}

// responsePayload is an answer as an event: who, the answers by question
// text and by id, and links back.
func (s *Server) responsePayload(f *store.Form, r *store.Response) map[string]any {
	byLabel, byID := map[string]any{}, map[string]any{}
	for _, fl := range f.Fields {
		if fl.Asks() {
			byLabel[fl.Label] = r.Text(fl)
			byID[fl.ID] = r.Text(fl)
		}
	}
	p := s.formPayload(f)
	p["form_title"] = f.Title
	delete(p, "title")
	p["response_id"], p["number"], p["name"], p["email"], p["preview"] = r.ID, r.Number, r.Name, r.Email, r.Preview
	p["answers"], p["fields"], p["response_url"] = byLabel, byID, s.answerLink(r)
	for _, role := range store.FieldRoles {
		if v := r.RoleText(f, role); v != "" {
			p[role] = v
		}
	}
	p["sent_at"] = r.CreatedAt
	return p
}

// summaryLine says how many answers wait, for pushes and dashboards.
func summaryLine(n int) string {
	if n == 1 {
		return "1 answer waits for you."
	}
	return fmt.Sprintf("%d answers wait for you.", n)
}

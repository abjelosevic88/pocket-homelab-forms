package server

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- answers into the other Pocket apps ----
//
// From one answer, as the owner (their own Core link or direct token):
// the person into Pocket Contacts (unless their email is there already),
// a project in Pocket Projects, a follow-up task in Pocket Tasks, the day
// they picked in Pocket Calendar, a note in Pocket Notes, a reply from
// Pocket Mail and a contract from a Pocket Sign template. Each thing made
// is remembered with a link to it, and a failed try with why.

// answerLink is the owner's page of an answer.
func (s *Server) answerLink(r *store.Response) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + fmt.Sprintf("/answers/%d", r.ID)
}

// answersMarkdown lists every answer as Markdown (for a project, a note).
func answersMarkdown(f *store.Form, r *store.Response) string {
	var b strings.Builder
	for _, fl := range f.Fields {
		if !fl.Asks() {
			continue
		}
		t := strings.TrimSpace(r.Text(fl))
		if t == "" {
			continue
		}
		if fl.Type == "long" || strings.Contains(t, "\n") {
			fmt.Fprintf(&b, "**%s**\n\n%s\n\n", fl.Label, t)
		} else {
			fmt.Fprintf(&b, "**%s:** %s\n\n", fl.Label, t)
		}
	}
	return strings.TrimSpace(b.String())
}

// answersText lists every answer as plain lines (for a task, an email).
func answersText(f *store.Form, r *store.Response) string {
	var b strings.Builder
	for _, fl := range f.Fields {
		if !fl.Asks() {
			continue
		}
		if t := strings.TrimSpace(r.Text(fl)); t != "" {
			fmt.Fprintf(&b, "%s: %s\n", fl.Label, t)
		}
	}
	return strings.TrimSpace(b.String())
}

// who is how to call the person in titles.
func who(f *store.Form, r *store.Response) string {
	return firstNonEmpty(r.Name, r.RoleText(f, "company"), r.Email, fmt.Sprintf("answer #%d", r.Number))
}

// origin is the line that says where a thing came from.
func (s *Server) origin(f *store.Form, r *store.Response) string {
	return fmt.Sprintf("From Pocket Forms — “%s”, answer #%d: %s", f.Title, r.Number, s.answerLink(r))
}

type makeIn struct {
	Target     string `json:"target"` // contact | project | task | event | note | email | signing
	Title      string `json:"title"`
	Notes      string `json:"notes"`
	TeamID     int64  `json:"team_id"`
	Date       string `json:"date"`
	Time       string `json:"time"`
	Minutes    int    `json:"minutes"`
	Due        string `json:"due"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	TemplateID int64  `json:"template_id"`
	Send       bool   `json:"send"`
}

var makeTargets = map[string]pocketApp{"contact": appContacts, "project": appProjects, "task": appTasks, "event": appCalendar,
	"note": appNotes, "email": appMail, "signing": appSign}

// userErr is a reason the person can act on (400, not 502).
type userErr struct{ msg string }

func (e *userErr) Error() string { return e.msg }

// make makes one thing from an answer and remembers it.
func (s *Server) make(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn, auto bool) (*store.Made, error) {
	a, ok := makeTargets[in.Target]
	if !ok {
		return nil, &userErr{"make a contact, project, task, event, note, email or a document to sign"}
	}
	var (
		res  map[string]any
		path string
		err  error
		m    = &store.Made{ResponseID: r.ID, Target: in.Target, App: a.slug, Auto: auto}
	)
	switch in.Target {
	case "contact":
		made, err := s.makeContact(ctx, c, f, r, m)
		if err != nil && auto {
			m.Error = plainErr(a, err)
			_ = s.store.AddMade(ctx, r.UserID, m)
		}
		return made, err
	case "project":
		res, path, err = s.makeProject(ctx, c, f, r, in)
	case "task":
		res, path, err = s.makeTask(ctx, c, f, r, in)
	case "event":
		res, path, err = s.makeEvent(ctx, c, f, r, in)
	case "note":
		res, path, err = s.makeNote(ctx, c, f, r, in)
	case "email":
		err = s.sendReply(ctx, c, f, r, in)
		m.Title = firstNonEmpty(in.Subject, "Reply")
	case "signing":
		res, path, err = s.makeSigning(ctx, c, f, r, in)
	}
	if err != nil {
		if auto {
			m.Error = plainErr(a, err)
			_ = s.store.AddMade(ctx, r.UserID, m)
		}
		return nil, err
	}
	if res != nil {
		m.Ref = idOf(res)
		m.Title = firstNonEmpty(str(res, "title", "name", "fn", "display_name"), in.Title, m.Title)
		if sib, _ := c.conn(ctx, a, a.scopes[len(a.scopes)-1]); sib != nil && sib.public != "" && m.Ref != "" && path != "" {
			m.URL = strings.TrimRight(sib.public, "/") + path
		}
	}
	s.store.DropMadeErrors(ctx, r.UserID, r.ID, m.Target)
	if err := s.store.AddMade(ctx, r.UserID, m); err != nil {
		return nil, err
	}
	return m, nil
}

// plainErr is a failed try in a sentence.
func plainErr(a pocketApp, err error) string {
	msg := err.Error()
	if isNotConnected(err) {
		return msg
	}
	return a.name + ": " + msg
}

// ---- Pocket Contacts ----

// splitName turns "Ana Marić" into given and family names.
func splitName(n string) (given, family string) {
	parts := strings.Fields(n)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

func (s *Server) makeContact(ctx context.Context, c *clients, f *store.Form, r *store.Response, m *store.Made) (*store.Made, error) {
	if r.Name == "" && r.Email == "" && r.RoleText(f, "phone") == "" {
		return nil, &userErr{"this form does not ask for a name, an email or a phone number (mark one in the question's \"Use as\")"}
	}
	// already there? Then link to that contact instead of making a second one.
	if r.Email != "" {
		if sib, err := c.conn(ctx, appContacts, scopeContactsRead); err == nil {
			if v, err := getJSONFrom(ctx, sib, "/api/v1/lookup", url.Values{"email": {r.Email}}); err == nil {
				if found := rows(v, "contacts"); len(found) > 0 {
					m.Ref, m.Title = idOf(found[0]), firstNonEmpty(str(found[0], "fn", "name", "display_name"), r.Name)
					if sib.public != "" && m.Ref != "" {
						m.URL = strings.TrimRight(sib.public, "/") + "/contacts/" + m.Ref
					}
					m.Title += " (already in your contacts)"
					s.store.DropMadeErrors(ctx, r.UserID, r.ID, "contact")
					return m, s.store.AddMade(ctx, r.UserID, m)
				}
			}
		}
	}
	sib, err := c.conn(ctx, appContacts, scopeContactsWrite)
	if err != nil {
		return nil, err
	}
	given, family := splitName(r.Name)
	card := map[string]any{"kind": "individual", "fn": r.Name, "given": given, "family": family, "org": r.RoleText(f, "company"),
		"note": s.origin(f, r)}
	if r.Name == "" && r.RoleText(f, "company") != "" {
		card["kind"] = "org"
	}
	if r.Email != "" {
		card["emails"] = []map[string]any{{"label": "Other", "value": r.Email, "pref": true}}
	}
	if p := r.RoleText(f, "phone"); p != "" {
		card["phones"] = []map[string]any{{"label": "Mobile", "value": p}}
	}
	if u := r.RoleText(f, "website"); u != "" {
		card["urls"] = []map[string]any{{"label": "Website", "value": u}}
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/contacts", map[string]any{"card": card})
	if err != nil {
		return nil, err
	}
	m.Ref = idOf(res)
	m.Title = firstNonEmpty(str(res, "fn", "name", "display_name"), r.Name, r.Email)
	if sib.public != "" && m.Ref != "" {
		m.URL = strings.TrimRight(sib.public, "/") + "/contacts/" + m.Ref
	}
	s.store.DropMadeErrors(ctx, r.UserID, r.ID, "contact")
	return m, s.store.AddMade(ctx, r.UserID, m)
}

// ---- Pocket Projects ----

func (s *Server) makeProject(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appProjects, scopeProjectsWrite)
	if err != nil {
		return nil, "", err
	}
	name := strings.TrimSpace(in.Title)
	if name == "" {
		name = firstNonEmpty(r.RoleText(f, "subject"), f.Title)
		if w := firstNonEmpty(r.RoleText(f, "company"), r.Name); w != "" {
			name = w + " — " + name
		}
	}
	summary := clip(firstNonEmpty(r.RoleText(f, "subject"), strings.Join(strings.Fields(r.RoleText(f, "message")), " ")), 250)
	content := answersMarkdown(f, r) + "\n\n---\n" + s.origin(f, r)
	if n := strings.TrimSpace(in.Notes); n != "" {
		content = n + "\n\n" + content
	}
	body := map[string]any{"name": clip(name, 200), "summary": summary, "content": content, "status": "planned"}
	team := in.TeamID
	if team == 0 {
		team = c.st.ProjectTeam
	}
	if team > 0 {
		body["team_ids"] = []int64{team}
	}
	if d := r.RoleText(f, "deadline"); validDate(d) && d != "" {
		body["target_date"] = d
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/projects", body)
	if err != nil {
		if strings.Contains(err.Error(), "team") {
			return nil, "", &userErr{"Pocket Projects needs to know the team: pick one (Settings → Answers → Projects team)"}
		}
		return nil, "", err
	}
	return res, "/project/" + idOf(res), nil
}

// ---- Pocket Tasks ----

func (s *Server) makeTask(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appTasks, scopeTasksWrite)
	if err != nil {
		return nil, "", err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = fmt.Sprintf("Answer %s (%s)", who(f, r), f.Title)
	}
	notes := joinNonEmpty("\n\n", in.Notes, answersText(f, r), s.origin(f, r))
	body := map[string]any{"title": clip(title, 200), "notes": clip(notes, 20000), "url": s.answerLink(r)}
	due := in.Due
	if due == "" && in.Date != "" {
		due = in.Date
	}
	if due != "" {
		if !validDate(due) {
			return nil, "", &userErr{"the due day is YYYY-MM-DD"}
		}
		body["due"] = due
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/tasks", body)
	if err != nil {
		return nil, "", err
	}
	return res, "/today?task=" + idOf(res), nil
}

// ---- Pocket Calendar ----

func (s *Server) makeEvent(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) (map[string]any, string, error) {
	date, tm := in.Date, in.Time
	if date == "" {
		date = firstNonEmpty(r.RoleText(f, "date"), r.RoleText(f, "deadline"))
	}
	if tm == "" {
		for _, fl := range f.Fields {
			if fl.Type == "time" {
				if t := strings.TrimSpace(r.Text(fl)); reTime.MatchString(t) {
					tm = t
					break
				}
			}
		}
	}
	if date == "" || !validDate(date) {
		return nil, "", &userErr{"there is no day in this answer: pick one"}
	}
	sib, err := c.conn(ctx, appCalendar, scopeCalendarWrite)
	if err != nil {
		return nil, "", err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = firstNonEmpty(r.RoleText(f, "subject"), f.Title) + " — " + who(f, r)
	}
	body := map[string]any{"title": clip(title, 200), "description": joinNonEmpty("\n\n", in.Notes, answersText(f, r), s.origin(f, r)), "tz": time.Local.String()}
	if tm == "" {
		body["start"], body["all_day"] = date, true
	} else {
		start, err := time.ParseInLocation("2006-01-02 15:04", date+" "+tm, time.Local)
		if err != nil {
			return nil, "", &userErr{"the time is HH:MM"}
		}
		mins := in.Minutes
		if mins <= 0 {
			mins = 60
		}
		body["start"] = start.Format("2006-01-02T15:04")
		body["end"] = start.Add(time.Duration(mins) * time.Minute).Format("2006-01-02T15:04")
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/events", body)
	if err != nil {
		return nil, "", err
	}
	return res, "/event/" + idOf(res), nil
}

// ---- Pocket Notes ----

func (s *Server) makeNote(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appNotes, scopeNotesWrite)
	if err != nil {
		return nil, "", err
	}
	title := firstNonEmpty(strings.TrimSpace(in.Title), fmt.Sprintf("%s — %s", f.Title, who(f, r)))
	content := joinNonEmpty("\n\n", in.Notes, answersMarkdown(f, r), "— "+s.origin(f, r))
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(title, 200), "content": content})
	if err != nil {
		return nil, "", err
	}
	return res, "/note/" + idOf(res), nil
}

// ---- Pocket Mail ----

// mailHTML is a short, plain email: paragraphs, then the answers.
func mailHTML(accent, body string, lines [][2]string) string {
	var h strings.Builder
	esc := html.EscapeString
	h.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>`)
	h.WriteString(`<body style="margin:0;padding:24px 12px;background:#f5f6f8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1f2433">`)
	h.WriteString(`<div style="max-width:560px;margin:0 auto;background:#fff;border:1px solid #e3e6ee;border-top:5px solid ` + esc(accent) + `;border-radius:12px;padding:24px">`)
	for _, p := range paras(body) {
		h.WriteString(`<p style="margin:0 0 12px;line-height:1.55">` + esc(strings.Join(p, "\n")) + `</p>`)
	}
	if len(lines) > 0 {
		h.WriteString(`<table role="presentation" style="width:100%;border-collapse:collapse;margin-top:8px;font-size:14px">`)
		for _, l := range lines {
			h.WriteString(`<tr><td style="padding:7px 0;border-top:1px solid #eef0f4;color:#646b7d;vertical-align:top;width:40%">` + esc(l[0]) +
				`</td><td style="padding:7px 0 7px 10px;border-top:1px solid #eef0f4">` + esc(l[1]) + `</td></tr>`)
		}
		h.WriteString(`</table>`)
	}
	h.WriteString(`</div></body></html>`)
	return strings.ReplaceAll(h.String(), "\n", "<br>")
}

func answerLines(f *store.Form, r *store.Response) [][2]string {
	var out [][2]string
	for _, fl := range f.Fields {
		if fl.Asks() {
			if t := strings.TrimSpace(r.Text(fl)); t != "" {
				out = append(out, [2]string{fl.Label, t})
			}
		}
	}
	return out
}

// sendMail sends an email for the owner through their Pocket Mail.
func (s *Server) sendMail(ctx context.Context, c *clients, to, toName, subject, text, htmlBody string) error {
	if s.cfg.DemoData {
		s.log.Debug("demo: email not sent", "to", to, "subject", subject)
		return nil
	}
	sib, err := c.conn(ctx, appMail, scopeMailSend)
	if err != nil {
		return err
	}
	body := map[string]any{"from": c.st.MailFrom, "to": []map[string]string{{"name": toName, "addr": to}}, "subject": subject,
		"body_text": text, "body_html": htmlBody}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var out map[string]any
	return sib.client.Do(ctx, http.MethodPost, "/api/v1/send", nil, body, &out)
}

func (s *Server) sendReply(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) error {
	if r.Email == "" {
		return &userErr{"this answer has no email address to reply to"}
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return &userErr{"write the reply first"}
	}
	subject := firstNonEmpty(strings.TrimSpace(in.Subject), strings.ReplaceAll(firstNonEmpty(c.st.ReplySubject, "Re: {form}"), "{form}", f.Title))
	quote := fmt.Sprintf("\n\n— %s", fmt.Sprintf(word(f.Language, "sent_by"), f.Title))
	text := body + quote + "\n" + answersText(f, r)
	return s.sendMail(ctx, c, r.Email, r.Name, clip(subject, 200), text, mailHTML(firstNonEmpty(f.Accent, "#5f3dc4"), body+quote, answerLines(f, r)))
}

// sendCopy emails the person what they sent, in the form's language.
func (s *Server) sendCopy(ctx context.Context, c *clients, f *store.Form, r *store.Response) error {
	subject := fmt.Sprintf(word(f.Language, "sent_by"), f.Title)
	intro := word(f.Language, "copy_intro")
	text := intro + "\n\n" + answersText(f, r)
	return s.sendMail(ctx, c, r.Email, r.Name, subject, text, mailHTML(firstNonEmpty(f.Accent, "#5f3dc4"), intro, answerLines(f, r)))
}

// ---- Pocket Sign ----

func (s *Server) makeSigning(ctx context.Context, c *clients, f *store.Form, r *store.Response, in makeIn) (map[string]any, string, error) {
	if r.Email == "" {
		return nil, "", &userErr{"this answer has no email address to send the document to"}
	}
	if in.TemplateID <= 0 {
		return nil, "", &userErr{"pick a template from Pocket Sign"}
	}
	rs, err := c.conn(ctx, appSign, scopeSigningRead)
	if err != nil {
		return nil, "", err
	}
	v, err := getJSONFrom(ctx, rs, "/api/v1/templates", nil)
	if err != nil {
		return nil, "", err
	}
	var roles []map[string]any
	for _, t := range rows(v, "templates") {
		if int64(num(t, "id")) == in.TemplateID {
			roles = rows(map[string]any{"items": t["roles"]})
		}
	}
	if len(roles) == 0 {
		return nil, "", &userErr{"that Pocket Sign template has nobody to sign it"}
	}
	// the person who answered signs in the first role that has nobody yet
	pick := roles[0]
	for _, rl := range roles {
		if str(rl, "email") == "" {
			pick = rl
			break
		}
	}
	people := []map[string]any{{"recipient_id": int64(num(pick, "id")), "name": firstNonEmpty(r.Name, r.Email), "email": r.Email}}
	ws, err := c.conn(ctx, appSign, scopeSigningWrite)
	if err != nil {
		return nil, "", err
	}
	title := firstNonEmpty(strings.TrimSpace(in.Title), "")
	res, err := postJSONTo(ctx, ws, fmt.Sprintf("/api/v1/templates/%d/use", in.TemplateID),
		map[string]any{"title": title, "message": strings.TrimSpace(in.Notes), "people": people, "send": in.Send})
	if err != nil {
		return nil, "", err
	}
	if d, ok := res["document"].(map[string]any); ok {
		res = d
	}
	return res, "/documents/" + idOf(res), nil
}

// ---- the handlers ----

func (s *Server) ownResponse(w http.ResponseWriter, r *http.Request) (*store.Form, *store.Response, bool) {
	ctx := r.Context()
	uid := actorID(ctx)
	resp, err := s.store.GetResponse(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "answer")
		return nil, nil, false
	}
	f, err := s.store.GetForm(ctx, uid, resp.FormID)
	if err != nil {
		s.fail(w, err, "form")
		return nil, nil, false
	}
	return f, resp, true
}

func (s *Server) makeFail(w http.ResponseWriter, a pocketApp, err error) {
	var ue *userErr
	switch {
	case errors.As(err, &ue), isNotConnected(err):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusBadGateway, a.name+": "+err.Error())
	}
}

func (s *Server) handleMake(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, resp, ok := s.ownResponse(w, r)
	if !ok {
		return
	}
	var in makeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	c := s.clients(ctx, resp.UserID)
	m, err := s.make(ctx, c, f, resp, in, false)
	if err != nil {
		s.makeFail(w, makeTargets[in.Target], err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// handleAccept takes a brief on: the person into Contacts, a project, a
// follow-up task — each step on its own, so one app being off does not stop
// the others — and marks the answer done.
func (s *Server) handleAccept(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, resp, ok := s.ownResponse(w, r)
	if !ok {
		return
	}
	var in struct {
		Contact bool   `json:"contact"`
		Project bool   `json:"project"`
		Task    bool   `json:"task"`
		TeamID  int64  `json:"team_id"`
		Name    string `json:"name"`
		Due     string `json:"due"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	c := s.clients(ctx, resp.UserID)
	type step struct {
		Target string      `json:"target"`
		Made   *store.Made `json:"made,omitempty"`
		Error  string      `json:"error,omitempty"`
	}
	var steps []step
	run := func(target string, mi makeIn) {
		m, err := s.make(ctx, c, f, resp, mi, false)
		st := step{Target: target, Made: m}
		if err != nil {
			st.Error = plainErr(makeTargets[target], err)
		}
		steps = append(steps, st)
	}
	if in.Contact {
		run("contact", makeIn{Target: "contact"})
	}
	projectURL := ""
	if in.Project {
		run("project", makeIn{Target: "project", Title: in.Name, TeamID: in.TeamID})
		if last := steps[len(steps)-1]; last.Made != nil {
			projectURL = last.Made.URL
		}
	}
	if in.Task {
		due := in.Due
		if due == "" && c.st.TaskDays > 0 {
			due = time.Now().AddDate(0, 0, c.st.TaskDays).Format("2006-01-02")
		}
		run("task", makeIn{Target: "task", Title: fmt.Sprintf("Get back to %s about “%s”", who(f, resp), firstNonEmpty(resp.RoleText(f, "subject"), f.Title)),
			Notes: joinNonEmpty("\n", projectURL), Due: due})
	}
	worked := 0
	for _, st := range steps {
		if st.Error == "" {
			worked++
		}
	}
	if worked > 0 && resp.Status != "done" {
		resp.Status = "done"
		_ = s.store.UpdateResponse(ctx, resp)
	}
	resp, _ = s.store.GetResponse(ctx, resp.UserID, resp.ID)
	writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "response": resp})
}

// handleApps says per app whether answers can go there (for the buttons).
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	st, _ := s.store.GetSettings(ctx, u.ID)
	out := []map[string]any{}
	for _, a := range pocketApps {
		_, via, detail := s.connStatus(ctx, u, st, a)
		out = append(out, map[string]any{"slug": a.slug, "name": a.name, "reason": a.reason, "connected": via != "", "detail": detail})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAppOptions lists what to pick from in an app: Projects' teams, Sign's templates.
func (s *Server) handleAppOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	var (
		a     pocketApp
		scope string
		path  string
		key   string
	)
	switch chiParam(r, "what") {
	case "teams":
		a, scope, path, key = appProjects, scopeProjectsRead, "/api/v1/teams", "teams"
	case "templates":
		a, scope, path, key = appSign, scopeSigningRead, "/api/v1/templates", "templates"
	default:
		writeErr(w, http.StatusNotFound, "unknown list")
		return
	}
	sib, err := c.conn(ctx, a, scope)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := getJSONFrom(ctx, sib, path, nil)
	if err != nil {
		writeErr(w, http.StatusBadGateway, a.name+": "+err.Error())
		return
	}
	out := []map[string]any{}
	for _, x := range rows(v, key) {
		o := map[string]any{"id": num(x, "id"), "name": str(x, "name", "title"), "key": str(x, "key")}
		if key == "templates" {
			o["roles"] = x["roles"]
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, out)
}

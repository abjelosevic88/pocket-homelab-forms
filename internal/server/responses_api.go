package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- the answers inbox ----

func (s *Server) handleListResponses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	f := store.ResponseFilter{FormID: int64(queryInt(r, "form")), Status: q.Get("status"), Starred: queryBool(r, "starred"), Query: q.Get("q"),
		Email: q.Get("email"), Day: q.Get("day"), Limit: queryInt(r, "limit"), Offset: queryInt(r, "offset")}
	if f.Status != "" && f.Status != "open" && !oneOf(f.Status, store.ResponseStatuses) {
		writeErr(w, http.StatusBadRequest, "status is new, seen, done, spam or open")
		return
	}
	if f.Day != "" && !validDate(f.Day) {
		writeErr(w, http.StatusBadRequest, "day is YYYY-MM-DD")
		return
	}
	rs, total, err := s.store.ListResponses(ctx, actorID(ctx), f)
	if err != nil {
		s.fail(w, err, "answers")
		return
	}
	for _, x := range rs {
		x.Answers = nil // the list shows name, email and a line; one answer has them all
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rs, "total": total, "new": s.store.CountNew(ctx, actorID(ctx))})
}

// answerView is one answer with its questions, in order, and its neighbours.
type answerItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Role  string `json:"role,omitempty"`
	Value any    `json:"value"`
	Text  string `json:"text"`
}

func (s *Server) handleGetResponse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, resp, ok := s.ownResponse(w, r)
	if !ok {
		return
	}
	// opening it is reading it (not for another app reading on your behalf)
	if resp.Status == "new" && linkApp(ctx) == "" && !queryBool(r, "peek") {
		if st, _ := s.store.GetSettings(ctx, resp.UserID); st.MarkSeen {
			resp.Status = "seen"
			_ = s.store.UpdateResponse(ctx, resp)
		}
	}
	items := []answerItem{}
	for _, fl := range f.Fields {
		if fl.Type == "section" {
			items = append(items, answerItem{ID: fl.ID, Label: fl.Label, Type: fl.Type})
			continue
		}
		items = append(items, answerItem{ID: fl.ID, Label: fl.Label, Type: fl.Type, Role: fl.Role, Value: resp.Answers[fl.ID], Text: resp.Text(fl)})
	}
	prev, next := s.store.Neighbours(ctx, resp.UserID, resp, int64(queryInt(r, "form")))
	writeJSON(w, http.StatusOK, map[string]any{"response": resp, "items": items, "prev": prev, "next": next, "url": s.answerLink(resp),
		"form": map[string]any{"id": f.ID, "title": f.Title, "emoji": f.Emoji, "handle": f.Handle, "url": s.formURL(f), "language": f.Language,
			"has_date": firstNonEmpty(resp.RoleText(f, "date"), resp.RoleText(f, "deadline")) != "", "template": f.Template}})
}

func (s *Server) handleUpdateResponse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, err := s.store.GetResponse(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "answer")
		return
	}
	var in struct {
		Status  *string `json:"status"`
		Starred *bool   `json:"starred"`
		Note    *string `json:"note"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Status != nil {
		if !oneOf(*in.Status, store.ResponseStatuses) {
			writeErr(w, http.StatusBadRequest, "status is new, seen, done or spam")
			return
		}
		resp.Status = *in.Status
	}
	if in.Starred != nil {
		resp.Starred = *in.Starred
	}
	if in.Note != nil {
		resp.Note = clip(strings.TrimSpace(*in.Note), 5000)
	}
	if err := s.store.UpdateResponse(ctx, resp); err != nil {
		s.fail(w, err, "save answer")
		return
	}
	resp, _ = s.store.GetResponse(ctx, resp.UserID, resp.ID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteResponse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteResponse(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete answer")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBulk marks or deletes several answers at once.
func (s *Server) handleBulk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
		Delete bool    `json:"delete"`
	}
	if err := decode(r, &in); err != nil || len(in.IDs) == 0 || len(in.IDs) > 1000 {
		writeErr(w, http.StatusBadRequest, "send ids (up to 1000) and a status or delete")
		return
	}
	n := 0
	if in.Delete {
		for _, id := range in.IDs {
			if s.store.DeleteResponse(ctx, uid, id) == nil {
				n++
			}
		}
	} else {
		if !oneOf(in.Status, store.ResponseStatuses) {
			writeErr(w, http.StatusBadRequest, "status is new, seen, done or spam")
			return
		}
		var err error
		if n, err = s.store.SetStatus(ctx, uid, in.IDs, in.Status); err != nil {
			s.fail(w, err, "answers")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": n})
}

// ---- for the rest of the box: dashboards, Pocket Day, Search, AI ----

// handleSummary is the short state of a person's forms.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	fs, err := s.store.ListForms(ctx, uid, "")
	if err != nil {
		s.fail(w, err, "summary")
		return
	}
	open, drafts := 0, 0
	type formLine struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Emoji     string `json:"emoji"`
		Status    string `json:"status"`
		Taking    bool   `json:"taking"`
		Responses int    `json:"responses"`
		New       int    `json:"new"`
		Views     int    `json:"views"`
		URL       string `json:"url"`
		Link      string `json:"link"`
		CloseAt   string `json:"close_at,omitempty"`
	}
	lines := []formLine{}
	for _, f := range fs {
		switch {
		case f.Taking:
			open++
		case f.Status == store.FormDraft:
			drafts++
		}
		lines = append(lines, formLine{f.ID, f.Title, f.Emoji, f.Status, f.Taking, f.Responses, f.New, f.Views, s.formURL(f),
			strings.TrimRight(s.cfg.BaseURL, "/") + fmt.Sprintf("/forms/%d", f.ID), f.CloseAt})
	}
	week := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339Nano)
	_, thisWeek, _ := s.store.ListResponses(ctx, uid, store.ResponseFilter{Since: week, Limit: 1})
	latest, total, _ := s.store.ListResponses(ctx, uid, store.ResponseFilter{Limit: 5})
	for _, x := range latest {
		x.Answers = nil
	}
	newN := s.store.CountNew(ctx, uid)
	writeJSON(w, http.StatusOK, map[string]any{"forms": len(fs), "open": open, "drafts": drafts, "new": newN, "this_week": thisWeek,
		"responses": total, "latest": latest, "list": lines, "oldest_new_at": s.store.FirstNewAt(ctx, uid),
		"url": strings.TrimRight(s.cfg.BaseURL, "/") + "/", "line": dayLine(newN, thisWeek)})
}

func dayLine(newN, week int) string {
	switch {
	case newN > 0:
		return summaryLine(newN)
	case week > 0:
		return fmt.Sprintf("%d %s this week, all read.", week, plural(week, "answer", "answers"))
	}
	return "No new answers."
}

// handleDay is one day for Pocket Day's journal: the answers that came in
// (who and what about, never the whole answers) and forms opened or closed.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	day := chiParam(r, "date")
	if !validDate(day) || day == "" {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	rs, total, err := s.store.ListResponses(ctx, uid, store.ResponseFilter{Day: day, Oldest: true, Limit: 200})
	if err != nil {
		s.fail(w, err, "day")
		return
	}
	items := []map[string]any{}
	for _, x := range rs {
		t, _ := time.Parse(time.RFC3339Nano, x.CreatedAt)
		items = append(items, map[string]any{"id": x.ID, "time": t.Local().Format("15:04"), "form": x.FormTitle, "form_id": x.FormID,
			"emoji": x.FormEmoji, "name": firstNonEmpty(x.Name, x.Email, fmt.Sprintf("Answer #%d", x.Number)), "preview": x.Preview,
			"number": x.Number, "url": s.answerLink(x), "status": x.Status})
	}
	fs, _ := s.store.ListForms(ctx, uid, "")
	opened := []map[string]any{}
	for _, f := range fs {
		if t, err := time.Parse(time.RFC3339Nano, f.OpenedAt); err == nil && t.Local().Format("2006-01-02") == day {
			opened = append(opened, map[string]any{"id": f.ID, "title": f.Title, "emoji": f.Emoji, "url": s.formURL(f)})
		}
	}
	text := ""
	if total > 0 {
		text = fmt.Sprintf("%d %s to your forms", total, plural(total, "answer", "answers"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": day, "responses": items, "count": total, "opened": opened, "summary": text})
}

// handleSearch finds forms and answers.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "forms": []any{}, "responses": []any{}})
		return
	}
	lim := queryInt(r, "limit")
	if lim <= 0 || lim > 100 {
		lim = 20
	}
	fs, _ := s.store.ListForms(ctx, uid, "")
	forms := []map[string]any{}
	items := []map[string]any{}
	lq := strings.ToLower(q)
	for _, f := range fs {
		if strings.Contains(strings.ToLower(f.Title+" "+f.Description), lq) && len(forms) < lim {
			link := strings.TrimRight(s.cfg.BaseURL, "/") + fmt.Sprintf("/forms/%d", f.ID)
			forms = append(forms, map[string]any{"id": f.ID, "title": f.Title, "emoji": f.Emoji, "status": f.Status, "responses": f.Responses, "url": link})
			items = append(items, map[string]any{"kind": "form", "id": f.ID, "title": f.Title, "subtitle": fmt.Sprintf("Form · %d %s", f.Responses, plural(f.Responses, "answer", "answers")), "url": link})
		}
	}
	rs, _, err := s.store.ListResponses(ctx, uid, store.ResponseFilter{Query: q, Limit: lim})
	if err != nil {
		s.fail(w, err, "search")
		return
	}
	resp := []map[string]any{}
	for _, x := range rs {
		name := firstNonEmpty(x.Name, x.Email, fmt.Sprintf("Answer #%d", x.Number))
		resp = append(resp, map[string]any{"id": x.ID, "form": x.FormTitle, "form_id": x.FormID, "name": name, "email": x.Email,
			"preview": x.Preview, "created_at": x.CreatedAt, "status": x.Status, "url": s.answerLink(x)})
		items = append(items, map[string]any{"kind": "response", "id": x.ID, "title": name + " — " + x.FormTitle, "subtitle": x.Preview,
			"date": x.CreatedAt, "url": s.answerLink(x)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "forms": forms, "responses": resp})
}

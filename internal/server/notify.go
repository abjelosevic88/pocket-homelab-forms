package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// pushes runs every minute; once a day, at each person's own hour, it says
// which answers came in and wait (when they want that). Each answer is
// told as it arrives (arrive.go).
func (s *Server) pushes(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || !st.NotifyDigest || now.Hour() != st.NotifyHour || now.Minute() > 10 {
			continue
		}
		if !s.once(ctx, fmt.Sprintf("push:%d:%s", uid, store.Today())) {
			continue
		}
		s.dailyCheck(ctx, uid, st, now)
	}
}

// once is true the first time it is asked for a key.
func (s *Server) once(ctx context.Context, key string) bool {
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return false
	}
	_ = s.store.CachePut(ctx, key, "1")
	return true
}

// dailyCheck builds and sends one person's morning message: answers that
// came in since yesterday morning and the ones still waiting.
func (s *Server) dailyCheck(ctx context.Context, uid int64, st store.Settings, now time.Time) (string, bool) {
	since := now.Add(-24 * time.Hour).UTC().Format(time.RFC3339Nano)
	fresh, total, _ := s.store.ListResponses(ctx, uid, store.ResponseFilter{Since: since, Limit: 50})
	waiting := s.store.CountNew(ctx, uid)
	if total == 0 && waiting == 0 {
		return "", false
	}
	var lines []string
	if total > 0 {
		per := map[string]int{}
		var order []string
		for _, r := range fresh {
			if per[r.FormTitle] == 0 {
				order = append(order, r.FormTitle)
			}
			per[r.FormTitle]++
		}
		var parts []string
		for _, t := range order {
			parts = append(parts, fmt.Sprintf("%s (%d)", t, per[t]))
		}
		lines = append(lines, fmt.Sprintf("%d new %s yesterday: %s.", total, plural(total, "answer", "answers"), strings.Join(parts, ", ")))
	}
	if waiting > 0 {
		lines = append(lines, summaryLine(waiting))
	}
	body := strings.Join(lines, "\n")
	link := strings.TrimRight(s.cfg.BaseURL, "/") + "/"
	if !s.push(ctx, uid, st, "Your forms", body, link) {
		return body, false
	}
	return body, true
}

// push sends one message to a person's own notification address.
func (s *Server) push(ctx context.Context, uid int64, st store.Settings, title, body, link string) bool {
	if st.NotifyURL == "" {
		return false
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: "info", Tags: []string{"clipboard"}, Link: link}); err != nil {
		s.log.Warn("notify failed", "user", uid, "err", err)
		return false
	}
	return true
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Forms", Body: notifyTestBody, Priority: "info", Tags: []string{"clipboard"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

const notifyTestBody = "Notifications work. Each new answer to your forms, and (if you want it) a morning note about the answers waiting, will arrive here."

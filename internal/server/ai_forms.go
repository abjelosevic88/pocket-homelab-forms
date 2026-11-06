package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- Pocket AI (or the person's own key) ----
//
// Three things, each asked for: a form drafted from a sentence, the answers
// of a form summed up, and a reply to one answer. Nothing is saved by
// itself: the person sees the result and keeps it or not.

var langNames = map[string]string{"en": "English", "bs": "Bosnian", "hr": "Croatian", "sr": "Serbian (Latin script)", "de": "German"}

func langName(code string) string {
	if n, ok := langNames[code]; ok {
		return n
	}
	return "English"
}

// aiReady is the person's AI, or writes why there is none.
func (s *Server) aiReady(w http.ResponseWriter, ctx context.Context, uid int64) (*ai.Client, bool) {
	st, _ := s.store.GetSettings(ctx, uid)
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "Pocket AI help is switched off in Settings → AI")
		return nil, false
	}
	cl, _, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeErr(w, http.StatusServiceUnavailable, firstNonEmpty(why, "No AI is set up: connect Pocket AI or add a key in Settings → AI"))
		return nil, false
	}
	select {
	case s.aiSem <- struct{}{}:
		return cl, true
	default:
		writeErr(w, http.StatusTooManyRequests, "Pocket AI is busy with other requests; try again in a moment")
		return nil, false
	}
}

func (s *Server) aiDone() { <-s.aiSem }

const draftRules = `You design forms for a person who is not technical. From their description, make one form.
Answer with JSON only: {"title": "...", "description": "...", "emoji": "one emoji", "thanks": "...", "submit_label": "...", "fields": [ ... ]}.
Each field: {"id": "short_snake_case", "type": "...", "label": "...", "help": "...", "required": true|false, "options": ["..."], "other": true|false, "role": "..."}.
Types: text (one line), long (a paragraph), email, phone, number, url, date, time, choice (pick one, shown as buttons), checkboxes (pick several), dropdown (pick one from a long list), yesno, rating (1–5 stars), consent (an "I agree" box), section (a heading between questions; label = heading, help = a sentence, asks nothing).
Roles (optional, at most one field each) tell the box what an answer means: name, email, phone, company, website, subject (what it is about, one line), message (the main text), budget, deadline (a day it must be done by), date (a day the person picks for a visit or an event).
Rules: 3 to 12 fields; ask only what is needed; plain, friendly words; ask for a name and a way to reply unless it is an anonymous survey; options only for choice, checkboxes and dropdown; write everything in %s.`

func (s *Server) handleAIDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Description string `json:"description"`
		Language    string `json:"language"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Description) == "" {
		writeErr(w, http.StatusBadRequest, "say in a sentence what the form is for")
		return
	}
	cl, ok := s.aiReady(w, ctx, uid)
	if !ok {
		return
	}
	defer s.aiDone()
	var out struct {
		Title       string        `json:"title"`
		Description string        `json:"description"`
		Emoji       string        `json:"emoji"`
		Thanks      string        `json:"thanks"`
		SubmitLabel string        `json:"submit_label"`
		Fields      []store.Field `json:"fields"`
	}
	if err := cl.JSON(ctx, fmt.Sprintf(draftRules, langName(in.Language)), clip(in.Description, 2000), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket AI: "+err.Error())
		return
	}
	fs, err := cleanFields(out.Fields)
	if err != nil || len(fs) == 0 {
		writeErr(w, http.StatusBadGateway, "Pocket AI did not make usable questions; try saying it differently")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": clip(out.Title, 200), "description": clip(out.Description, 2000), "emoji": clip(out.Emoji, 8),
		"thanks": clip(out.Thanks, 1000), "submit_label": clip(out.SubmitLabel, 60), "fields": fs})
}

const summaryRules = `You read the answers people sent to a form and tell its owner, briefly and plainly, what they say.
Answer with JSON only: {"summary": "two to four sentences", "points": ["the main themes, most common first, each with how many said it"], "quotes": ["up to three short quotes that stand for many"], "next": ["up to three things the owner could do"]}.
Count, do not guess; mention numbers for choices. Never invent answers. Write in English.`

func (s *Server) handleAISummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, ok := s.ownForm(w, r, false)
	if !ok {
		return
	}
	rs, total, err := s.store.ListResponses(ctx, f.UserID, store.ResponseFilter{FormID: f.ID, Limit: 300})
	if err != nil {
		s.fail(w, err, "answers")
		return
	}
	if total == 0 {
		writeErr(w, http.StatusBadRequest, "there are no answers to sum up yet")
		return
	}
	cl, ok := s.aiReady(w, ctx, f.UserID)
	if !ok {
		return
	}
	defer s.aiDone()
	var b strings.Builder
	fmt.Fprintf(&b, "Form: %s\n%d answers", f.Title, total)
	if total > len(rs) {
		fmt.Fprintf(&b, " (the latest %d below)", len(rs))
	}
	b.WriteString("\n\nCounted choices:\n")
	for _, st := range countFields(f, rs) {
		if len(st.Options) > 0 {
			var parts []string
			for _, o := range st.Options {
				parts = append(parts, fmt.Sprintf("%s: %d", o.Option, o.Count))
			}
			fmt.Fprintf(&b, "- %s → %s", st.Label, strings.Join(parts, ", "))
			if st.Average > 0 {
				fmt.Fprintf(&b, " (average %.1f)", st.Average)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\nWritten answers:\n")
	for _, x := range rs {
		for _, fl := range f.Fields {
			if (fl.Type == "long" || fl.Type == "text") && fl.Role != "name" && fl.Role != "email" && fl.Role != "phone" {
				if t := strings.TrimSpace(x.Text(fl)); t != "" {
					fmt.Fprintf(&b, "[#%d] %s: %s\n", x.Number, fl.Label, clip(strings.Join(strings.Fields(t), " "), 600))
				}
			}
		}
		if b.Len() > 60000 {
			break
		}
	}
	var out struct {
		Summary string   `json:"summary"`
		Points  []string `json:"points"`
		Quotes  []string `json:"quotes"`
		Next    []string `json:"next"`
	}
	if err := cl.JSON(ctx, summaryRules, b.String(), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": out.Summary, "points": out.Points, "quotes": out.Quotes, "next": out.Next, "answers": total})
}

const replyRules = `You help the owner of a form answer one person who filled it in. Write the email the owner sends back.
Answer with JSON only: {"subject": "...", "body": "..."}.
The body: a greeting with their first name if known, a warm, short, concrete reply to what they wrote (2 to 5 short paragraphs), and a sign-off with the owner's name. Plain text, no Markdown. Never promise prices, dates or things the owner did not say; where the owner must decide, leave a short [placeholder]. Write in %s.`

func (s *Server) handleAIReply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, resp, ok := s.ownResponse(w, r)
	if !ok {
		return
	}
	var in struct {
		Instruction string `json:"instruction"`
	}
	_ = decode(r, &in)
	cl, ok := s.aiReady(w, ctx, resp.UserID)
	if !ok {
		return
	}
	defer s.aiDone()
	owner := ""
	if u, err := s.store.GetUser(ctx, resp.UserID); err == nil {
		owner = u.Name
	}
	prompt := fmt.Sprintf("Form: %s\nThe owner's name: %s\nThe person's answers:\n%s", f.Title, firstNonEmpty(owner, "[your name]"), answersText(f, resp))
	if t := strings.TrimSpace(in.Instruction); t != "" {
		prompt += "\n\nWhat the owner wants to say: " + clip(t, 1000)
	}
	var out struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := cl.JSON(ctx, fmt.Sprintf(replyRules, langName(f.Language)), prompt, &out); err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": clip(out.Subject, 200), "body": out.Body})
}

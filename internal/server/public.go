package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- the forms as people outside see them ----
//
// Every form lives at /f/<handle> on this app's address, and under
// PUBLIC_URL when the box has a public hostname for the forms only. The
// pages are rendered here with no JavaScript at all: a robot that does not
// wait, or fills the hidden field, is let go without its answers counting.

//go:embed site/*
var siteFS embed.FS

var siteTmpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"safeCSS": func(s string) template.CSS {
		if reHex.MatchString(s) {
			return template.CSS(s)
		}
		return template.CSS("#5f3dc4")
	},
	"paras": paras,
	"seq": func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = n - i
		}
		return out
	},
	"str": strconv.Itoa,
}).ParseFS(siteFS, "site/*.html"))

// paras splits text into paragraphs, and each into lines.
func paras(s string) [][]string {
	var out [][]string
	for _, p := range regexp.MustCompile(`\n\s*\n`).Split(strings.TrimSpace(s), -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.Split(p, "\n"))
		}
	}
	return out
}

// siteField is a question as the page shows it.
type siteField struct {
	store.Field
	N        int
	Value    string
	Checked  map[string]bool
	OtherOn  bool
	OtherVal string
	Error    string
	Input    string // the <input type>
}

type sitePage struct {
	Form     *store.Form
	Lang     string
	W        map[string]string
	Kind     string // form | thanks | closed | notfound
	Title    string
	Fields   []siteField
	Stamp    string
	Notice   string
	Bad      bool
	Embed    bool
	Preview  bool
	Again    string
	Closed   string
	Accent   string
	Action   string
	Thanks   string
	Branding bool
}

// ---- host switch ----

func hostOnly(h string) string {
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

func urlHost(u string) string {
	if p, err := url.Parse(u); err == nil {
		return strings.ToLower(p.Hostname())
	}
	return ""
}

// hostSwitch serves only the form pages on the public hostname; every other
// name (BASE_URL, host.docker.internal for Pocket Core and the apps acting
// through it, a LAN name) is the whole app.
func (s *Server) hostSwitch(next http.Handler) http.Handler {
	publicHost := urlHost(s.cfg.PublicURL)
	if publicHost == "" || publicHost == urlHost(s.cfg.BaseURL) {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hostOnly(r.Host) == publicHost {
			p := r.URL.Path
			if p != "/healthz" && !strings.HasPrefix(p, "/f/") && !strings.HasPrefix(p, "/_forms/") && p != "/favicon.svg" {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---- routes ----

// owner is whether the signed-in person reading is the form's owner.
func (s *Server) owner(r *http.Request, f *store.Form) bool {
	if s.cfg.AuthMode == "none" {
		return true
	}
	c, err := r.Cookie("pfr_session")
	if err != nil || c.Value == "" {
		return false
	}
	u, err := s.sessionUser(r.Context(), c.Value)
	return err == nil && u != nil && u.ID == f.UserID
}

func (s *Server) basePage(f *store.Form, r *http.Request) *sitePage {
	lang := "en"
	if f != nil {
		if _, ok := siteWords[f.Language]; ok {
			lang = f.Language
		}
	}
	pg := &sitePage{Form: f, Lang: lang, W: siteWords["en"], Accent: "#5f3dc4", Embed: r.URL.Query().Get("embed") == "1" || r.PostForm.Get("embed") == "1"}
	if lang != "en" {
		pg.W = map[string]string{}
		for k := range siteWords["en"] {
			pg.W[k] = word(lang, k)
		}
	}
	if f != nil {
		pg.Title, pg.Branding = f.Title, f.Branding
		pg.Action = "/f/" + f.Handle
		if f.Accent != "" {
			pg.Accent = f.Accent
		}
	}
	return pg
}

func (s *Server) handleFormPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, err := s.store.FormByHandle(ctx, chiParam(r, "handle"))
	if err != nil {
		s.siteNotFound(w, r)
		return
	}
	isOwner := s.owner(r, f)
	if f.Status == store.FormDraft && !isOwner {
		s.siteNotFound(w, r)
		return
	}
	if rest := strings.Trim(chiParam(r, "*"), "/"); rest == "thanks" {
		s.siteThanks(w, r, f)
		return
	} else if rest != "" {
		s.siteNotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		s.siteSubmit(w, r, f, isOwner)
		return
	}
	if taking, why := f.TakingState(time.Now(), s.store.CountAnswers(ctx, f.ID)); !taking && why != "draft" {
		s.siteClosed(w, r, f, why)
		return
	}
	pg := s.basePage(f, r)
	pg.Kind, pg.Preview = "form", f.Status == store.FormDraft
	pg.Fields = s.siteFields(f, r.URL.Query(), nil)
	pg.Stamp = s.formStamp(f.ID)
	if !isOwner && !isBot(r.UserAgent()) && s.firstView(r, f.ID) {
		s.store.AddView(ctx, f.UserID, f.ID)
	}
	s.renderSite(w, http.StatusOK, pg)
}

// siteFields fills the questions with what was sent (or ?prefill by id or role).
func (s *Server) siteFields(f *store.Form, vals url.Values, errs map[string]string) []siteField {
	out := make([]siteField, 0, len(f.Fields))
	n := 0
	for _, fl := range f.Fields {
		sf := siteField{Field: fl, Checked: map[string]bool{}, Error: errs[fl.ID], Input: inputType(fl.Type)}
		if fl.Asks() {
			n++
			sf.N = n
		}
		key := fl.ID
		if _, ok := vals[key]; !ok && fl.Role != "" {
			if _, ok := vals[fl.Role]; ok {
				key = fl.Role
			}
		}
		sf.Value = clip(vals.Get(key), 10000)
		for _, v := range vals[key] {
			sf.Checked[v] = true
		}
		sf.OtherVal = vals.Get(fl.ID + "__other")
		sf.OtherOn = sf.Checked["__other"]
		out = append(out, sf)
	}
	return out
}

func inputType(t string) string {
	switch t {
	case "email", "number", "url", "date", "time":
		return t
	case "phone":
		return "tel"
	}
	return "text"
}

var (
	rePhone = regexp.MustCompile(`^\+?[0-9 ()./-]{5,30}$`)
	reTime  = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

// readAnswers checks a sent form; the map has an error per question id.
func readAnswers(f *store.Form, vals url.Values, lang string) (map[string]any, map[string]string) {
	answers := map[string]any{}
	errs := map[string]string{}
	w := func(k string) string { return word(lang, k) }
	for _, fl := range f.Fields {
		if !fl.Asks() {
			continue
		}
		raw := strings.TrimSpace(vals.Get(fl.ID))
		switch fl.Type {
		case "checkboxes":
			var picked []string
			allowed := map[string]bool{}
			for _, o := range fl.Options {
				allowed[o] = true
			}
			for _, v := range vals[fl.ID] {
				switch {
				case v == "__other" && fl.Other:
					if o := clip(strings.TrimSpace(vals.Get(fl.ID+"__other")), 300); o != "" {
						picked = append(picked, o)
					}
				case allowed[v]:
					picked = append(picked, v)
				}
			}
			if len(picked) > 0 {
				answers[fl.ID] = picked
			} else if fl.Required {
				errs[fl.ID] = w("e_required")
			}
			continue
		case "choice", "dropdown":
			if raw == "__other" && fl.Other {
				raw = clip(strings.TrimSpace(vals.Get(fl.ID+"__other")), 300)
			} else if raw != "" && !oneOf(raw, fl.Options) {
				errs[fl.ID] = w("e_choice")
				continue
			}
		case "yesno":
			switch raw {
			case "yes":
				raw = w("yes")
			case "no":
				raw = w("no")
			case "":
			default:
				errs[fl.ID] = w("e_choice")
				continue
			}
		case "consent":
			if raw != "" {
				raw = w("yes")
			}
		case "rating":
			if n, err := strconv.Atoi(raw); raw != "" && (err != nil || n < 1 || n > 5) {
				errs[fl.ID] = w("e_choice")
				continue
			}
		case "email":
			if raw != "" {
				a, err := mail.ParseAddress(raw)
				if err != nil || !strings.Contains(a.Address, ".") || a.Address != raw {
					errs[fl.ID] = w("e_email")
					continue
				}
				raw = strings.ToLower(raw)
			}
		case "phone":
			if raw != "" && !rePhone.MatchString(raw) {
				errs[fl.ID] = w("e_phone")
				continue
			}
		case "number":
			if raw != "" {
				if _, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64); err != nil {
					errs[fl.ID] = w("e_number")
					continue
				}
				raw = strings.ReplaceAll(raw, ",", ".")
			}
		case "url":
			if raw != "" {
				if !strings.Contains(raw, "://") {
					raw = "https://" + raw
				}
				u, err := url.Parse(raw)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Host, ".") {
					errs[fl.ID] = w("e_url")
					continue
				}
			}
		case "date":
			if raw != "" && !validDate(raw) {
				errs[fl.ID] = w("e_date")
				continue
			}
		case "time":
			if raw != "" && !reTime.MatchString(raw) {
				errs[fl.ID] = w("e_time")
				continue
			}
		}
		limit := 500
		if fl.Type == "long" {
			limit = 10000
		}
		if utf8.RuneCountInString(raw) > limit {
			errs[fl.ID] = w("e_long")
			continue
		}
		if raw == "" {
			if fl.Required {
				errs[fl.ID] = w("e_required")
			}
			continue
		}
		answers[fl.ID] = raw
	}
	return answers, errs
}

// describe fills the name, email and a line for lists from the answers.
func describe(f *store.Form, r *store.Response) {
	r.Name = clip(r.RoleText(f, "name"), 120)
	r.Email = strings.ToLower(r.RoleText(f, "email"))
	if r.Name == "" {
		r.Name = clip(r.RoleText(f, "company"), 120)
	}
	line := firstNonEmpty(r.RoleText(f, "subject"), r.RoleText(f, "message"))
	if line == "" {
		for _, fl := range f.Fields {
			if fl.Asks() && fl.Role == "" {
				if t := strings.TrimSpace(r.Text(fl)); t != "" {
					line = fl.Label + ": " + t
					break
				}
			}
		}
	}
	r.Preview = clip(strings.Join(strings.Fields(line), " "), 160)
}

func (s *Server) siteSubmit(w http.ResponseWriter, r *http.Request, f *store.Form, isOwner bool) {
	ctx := r.Context()
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	pg := s.basePage(f, r)
	embed := ""
	if pg.Embed {
		embed = "?embed=1"
	}
	// the hidden field people never fill in: a robot; tell it all went well
	if r.PostForm.Get("website") != "" {
		http.Redirect(w, r, "/f/"+f.Handle+"/thanks"+embed, http.StatusSeeOther)
		return
	}
	if taking, why := f.TakingState(time.Now(), s.store.CountAnswers(ctx, f.ID)); !taking && why != "draft" {
		s.siteClosed(w, r, f, why)
		return
	}
	pg.Kind, pg.Preview = "form", f.Status == store.FormDraft
	answers, errs := readAnswers(f, r.PostForm, pg.Lang)
	again := func(status int, notice string) {
		pg.Fields = s.siteFields(f, r.PostForm, errs)
		pg.Stamp = s.formStamp(f.ID)
		pg.Notice, pg.Bad = notice, true
		s.renderSite(w, status, pg)
	}
	switch stamp := r.PostForm.Get("stamp"); {
	case !s.stampValid(f.ID, stamp):
		again(http.StatusUnprocessableEntity, pg.W["stamp"])
		return
	case !s.stampAged(stamp):
		// faster than a person can read the questions
		http.Redirect(w, r, "/f/"+f.Handle+"/thanks"+embed, http.StatusSeeOther)
		return
	}
	if len(errs) > 0 {
		again(http.StatusUnprocessableEntity, pg.W["fix"])
		return
	}
	resp := &store.Response{UserID: f.UserID, FormID: f.ID, Answers: answers, Origin: "web", IPHash: s.readerHash(r)}
	describe(f, resp)
	if f.Status == store.FormDraft {
		s.siteThanksPage(w, r, f, pg.W["preview_sent"], true)
		return
	}
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if s.store.RecentFrom(ctx, resp.IPHash, 10*time.Minute) >= 10 {
		again(http.StatusTooManyRequests, pg.W["wait"])
		return
	}
	if f.OnePerEmail && resp.Email != "" && s.store.EmailAnswered(ctx, f.ID, resp.Email) {
		pg.Kind = "thanks"
		pg.Thanks = pg.W["already"]
		s.renderSite(w, http.StatusOK, pg)
		return
	}
	if err := s.store.AddResponse(ctx, resp); err != nil {
		s.siteError(w, err)
		return
	}
	go s.responseArrived(context.WithoutCancel(ctx), f, resp)
	if f.RedirectURL != "" && !pg.Embed {
		http.Redirect(w, r, f.RedirectURL, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/f/"+f.Handle+"/thanks"+embed, http.StatusSeeOther)
}

func (s *Server) siteThanks(w http.ResponseWriter, r *http.Request, f *store.Form) {
	s.siteThanksPage(w, r, f, "", false)
}

func (s *Server) siteThanksPage(w http.ResponseWriter, r *http.Request, f *store.Form, notice string, preview bool) {
	pg := s.basePage(f, r)
	pg.Kind, pg.Preview = "thanks", preview
	pg.Thanks = firstNonEmpty(f.Thanks, pg.W["thanks"])
	pg.Notice = notice
	if !f.OnePerEmail {
		pg.Again = "/f/" + f.Handle
		if pg.Embed {
			pg.Again += "?embed=1"
		}
	}
	s.renderSite(w, http.StatusOK, pg)
}

func (s *Server) siteClosed(w http.ResponseWriter, r *http.Request, f *store.Form, why string) {
	pg := s.basePage(f, r)
	pg.Kind = "closed"
	switch why {
	case "date":
		t, _ := time.Parse("2006-01-02", f.CloseAt)
		pg.Closed = fmt.Sprintf(pg.W["closed_date"], t.Format("2 Jan 2006"))
	case "full":
		pg.Closed = pg.W["full"]
	default:
		pg.Closed = pg.W["closed"]
	}
	s.renderSite(w, http.StatusOK, pg)
}

func (s *Server) siteNotFound(w http.ResponseWriter, r *http.Request) {
	pg := s.basePage(nil, r)
	pg.Kind, pg.Title = "notfound", "Pocket Forms"
	s.renderSite(w, http.StatusNotFound, pg)
}

func (s *Server) siteError(w http.ResponseWriter, err error) {
	s.log.Error("form page", "err", err)
	http.Error(w, "Something went wrong. Please try again in a moment.", http.StatusInternalServerError)
}

// sitePolicy: no scripts at all, the form posts only to itself, and any
// site may show it in a frame (to put a form on your own website).
const sitePolicy = "default-src 'self'; script-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"form-action 'self'; base-uri 'none'; object-src 'none'"

func (s *Server) renderSite(w http.ResponseWriter, status int, pg *sitePage) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", sitePolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Robots-Tag", "noindex")
	w.WriteHeader(status)
	if err := siteTmpl.ExecuteTemplate(w, "page", pg); err != nil {
		s.log.Error("form page render", "err", err)
	}
}

func (s *Server) handleSiteCSS(w http.ResponseWriter, r *http.Request) {
	b, err := siteFS.ReadFile("site/form.css")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(b)
}

// ---- people opening a form: counted once a day each, never tracked ----

var botWords = []string{"bot", "crawl", "spider", "slurp", "preview", "fetch", "monitor", "curl", "wget", "python", "go-http", "headless", "lighthouse"}

func isBot(ua string) bool {
	ua = strings.ToLower(ua)
	if ua == "" {
		return true
	}
	for _, b := range botWords {
		if strings.Contains(ua, b) {
			return true
		}
	}
	return false
}

var seen = struct {
	sync.Mutex
	day  string
	keys map[string]bool
}{keys: map[string]bool{}}

func (s *Server) firstView(r *http.Request, formID int64) bool {
	day := store.Today()
	key := fmt.Sprintf("%s|%d", s.readerHash(r), formID)
	seen.Lock()
	defer seen.Unlock()
	if seen.day != day || len(seen.keys) > 100000 {
		seen.day, seen.keys = day, map[string]bool{}
	}
	if seen.keys[key] {
		return false
	}
	seen.keys[key] = true
	return true
}

// siteSecret is this box's key for form stamps and address hashes.
func (s *Server) siteSecret() []byte {
	s.secretOnce.Do(func() {
		ctx := context.Background()
		v, _, ok := s.store.CacheGet(ctx, "site-secret")
		if !ok || len(v) < 32 {
			v = store.RandomKey(32)
			_ = s.store.CachePut(ctx, "site-secret", v)
		}
		s.secret = []byte(v)
	})
	return s.secret
}

// readerHash stands for one address for one day: salted, hashed.
func (s *Server) readerHash(r *http.Request) string {
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	m := hmac.New(sha256.New, s.siteSecret())
	m.Write([]byte(ip + "|" + store.Today()))
	return hex.EncodeToString(m.Sum(nil))[:24]
}

func (s *Server) formStamp(formID int64) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	m := hmac.New(sha256.New, s.siteSecret())
	fmt.Fprintf(m, "%d|%s", formID, ts)
	return ts + "." + hex.EncodeToString(m.Sum(nil))[:20]
}

// stampValid: the form came from this page, at most a day ago.
func (s *Server) stampValid(formID int64, stamp string) bool {
	ts, sig, ok := strings.Cut(stamp, ".")
	if !ok {
		return false
	}
	m := hmac.New(sha256.New, s.siteSecret())
	fmt.Fprintf(m, "%d|%s", formID, ts)
	if !hmac.Equal([]byte(sig), []byte(hex.EncodeToString(m.Sum(nil))[:20])) {
		return false
	}
	n, err := strconv.ParseInt(ts, 10, 64)
	return err == nil && time.Since(time.Unix(n, 0)) <= 24*time.Hour
}

// stampAged: the page was open long enough for a person to fill it in.
func (s *Server) stampAged(stamp string) bool {
	ts, _, _ := strings.Cut(stamp, ".")
	n, err := strconv.ParseInt(ts, 10, 64)
	return err == nil && time.Since(time.Unix(n, 0)) >= s.formMinAge
}

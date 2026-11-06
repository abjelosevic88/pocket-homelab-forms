package server

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- your forms, out and in ----

// exportDoc is one person's forms with every answer, as JSON.
type exportDoc struct {
	App      string       `json:"app"`
	Version  string       `json:"version"`
	Exported string       `json:"exported_at"`
	Forms    []exportForm `json:"forms"`
}

type exportForm struct {
	store.Form
	Answers []*store.Response `json:"answers_list"`
}

func (s *Server) export(ctx context.Context, uid int64) exportDoc {
	doc := exportDoc{App: "pocket-forms", Version: Version, Exported: store.Now(), Forms: []exportForm{}}
	fs, _ := s.store.ListForms(ctx, uid, "")
	for _, f := range fs {
		var all []*store.Response
		for _, st := range []string{"", "spam"} {
			rs, _, _ := s.store.ListResponses(ctx, uid, store.ResponseFilter{FormID: f.ID, Status: st, Oldest: true, Limit: 5000})
			all = append(all, rs...)
		}
		if all == nil {
			all = []*store.Response{}
		}
		doc.Forms = append(doc.Forms, exportForm{Form: *f, Answers: all})
	}
	return doc
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-forms-export.json"`)
	writeJSON(w, http.StatusOK, s.export(r.Context(), actorID(r.Context())))
}

// handleImport takes a Pocket Forms export (again and again: nothing is
// doubled), or the answers of a form from another service as CSV.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var data []byte
	name := ""
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "no file")
			return
		}
		defer f.Close()
		name = fh.Filename
		if data, err = io.ReadAll(f); err != nil {
			writeErr(w, http.StatusBadRequest, "could not read the file")
			return
		}
	} else {
		b, err := s.readBody(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		data = b
		name = r.URL.Query().Get("name")
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var doc exportDoc
		if err := json.Unmarshal(data, &doc); err != nil || doc.App != "pocket-forms" {
			writeErr(w, http.StatusBadRequest, "that is not a Pocket Forms export")
			return
		}
		writeJSON(w, http.StatusOK, s.importPocket(ctx, uid, doc))
		return
	}
	out, err := s.importCSV(ctx, uid, name, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) importPocket(ctx context.Context, uid int64, doc exportDoc) map[string]any {
	forms, answers, skipped := 0, 0, 0
	for _, ef := range doc.Forms {
		f := ef.Form
		f.UserID = uid
		mine, err := s.store.FormByHandle(ctx, f.Handle)
		mapKey := fmt.Sprintf("import-handle:%d:%s", uid, f.Handle)
		if err == nil && mine.UserID != uid {
			// someone else has that address: the copy made last time, if any
			if v, _, ok := s.store.CacheGet(ctx, mapKey); ok {
				if g, gerr := s.store.GetForm(ctx, uid, int64(atoiSafe(v))); gerr == nil {
					mine, err = g, nil
				}
			}
		}
		switch {
		case err == nil && mine.UserID == uid:
			f.ID = mine.ID
		default:
			if err == nil || !store.ValidHandle(f.Handle) {
				f.Handle = s.store.NewHandle(ctx, f.Title)
			}
			fs, ferr := cleanFields(f.Fields)
			if ferr != nil {
				skipped++
				continue
			}
			f.Fields, f.ID, f.Source = fs, 0, ""
			if !oneOf(f.Status, []string{store.FormDraft, store.FormOpen, store.FormClosed}) {
				f.Status = store.FormDraft
			}
			orig := ef.Form.Handle
			if err := s.store.CreateForm(ctx, &f); err != nil {
				skipped++
				continue
			}
			if orig != f.Handle {
				_ = s.store.CachePut(ctx, fmt.Sprintf("import-handle:%d:%s", uid, orig), strconv.FormatInt(f.ID, 10))
			}
			forms++
		}
		for _, a := range ef.Answers {
			if a == nil {
				continue
			}
			resp := &store.Response{UserID: uid, FormID: f.ID, Answers: a.Answers, Name: a.Name, Email: a.Email, Preview: a.Preview,
				Status: a.Status, Starred: a.Starred, Note: a.Note, Origin: firstNonEmpty(a.Origin, "import"), CreatedAt: a.CreatedAt,
				UpdatedAt: a.UpdatedAt, ClientKey: fmt.Sprintf("pf:%s:%d", a.CreatedAt, a.Number)}
			if !oneOf(resp.Status, store.ResponseStatuses) {
				resp.Status = "seen"
			}
			if err := s.store.AddResponse(ctx, resp); err == nil {
				answers++
			} else if !errors.Is(err, store.ErrConflict) {
				skipped++
			}
		}
	}
	return map[string]any{"forms": forms, "answers": answers, "skipped": skipped}
}

// ---- answers from Google Forms, Typeform, Tally, Microsoft Forms, Jotform ----

// metaColumns are the columns other services add that are not questions.
var metaColumns = map[string]bool{"#": true, "id": true, "response id": true, "submission id": true, "respondent id": true,
	"response type": true, "network id": true, "tags": true, "stage date (utc)": true, "start date (utc)": true, "start time": true,
	"ip": true, "ip address": true, "last modified time": true, "user agent": true, "edit link": true, "score": true, "total score": true}

// timeColumns hold when the answer was sent.
var timeColumns = []string{"timestamp", "submitted at", "submit date (utc)", "completion time", "submission date", "created at", "date submitted", "zeitstempel", "vremenska oznaka"}

var (
	reEmail   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	reISODate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reNumber  = regexp.MustCompile(`^-?\d+([.,]\d+)?$`)
)

func (s *Server) importCSV(ctx context.Context, uid int64, name string, data []byte) (map[string]any, error) {
	rd := csv.NewReader(bytes.NewReader(data))
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	if bytes.Count(data[:min(len(data), 4096)], []byte(";")) > bytes.Count(data[:min(len(data), 4096)], []byte(",")) {
		rd.Comma = ';'
	}
	recs, err := rd.ReadAll()
	if err != nil || len(recs) < 2 {
		return nil, errors.New("this does not look like answers in a CSV file (a header row and at least one answer)")
	}
	head := recs[0]
	rowsIn := recs[1:]
	timeCol := -1
	type col struct {
		idx   int
		field store.Field
	}
	var cols []col
	usedRole := map[string]bool{}
	for i, h := range head {
		lh := strings.ToLower(strings.TrimSpace(h))
		if timeCol < 0 && oneOf(lh, timeColumns) {
			timeCol = i
			continue
		}
		if lh == "" || metaColumns[lh] {
			continue
		}
		var vals []string
		for _, r := range rowsIn {
			if i < len(r) && strings.TrimSpace(r[i]) != "" {
				vals = append(vals, strings.TrimSpace(r[i]))
			}
		}
		fl := guessField(h, vals)
		fl.ID = fmt.Sprintf("c%d", i+1)
		if fl.Role != "" && usedRole[fl.Role] {
			fl.Role = ""
		}
		usedRole[fl.Role] = true
		cols = append(cols, col{i, fl})
	}
	if len(cols) == 0 {
		return nil, errors.New("no questions found in the first row")
	}
	sum := sha1.Sum([]byte(strings.Join(head, "\x1f")))
	key := "import:" + hex.EncodeToString(sum[:])[:12]
	title := strings.TrimSuffix(filepath.Base(firstNonEmpty(name, "Imported answers.csv")), filepath.Ext(name))
	if i := strings.Index(title, " (Responses)"); i > 0 {
		title = title[:i]
	}
	title = clip(strings.TrimSpace(strings.NewReplacer("_", " ").Replace(title)), 200)
	f := s.formByTemplate(ctx, uid, key)
	created := false
	if f == nil {
		f = &store.Form{UserID: uid, Title: firstNonEmpty(title, "Imported answers"), Status: store.FormClosed, Language: "en",
			Branding: true, Notify: true, Template: key, Emoji: "📥"}
		for _, c := range cols {
			f.Fields = append(f.Fields, c.field)
		}
		f.Handle = s.store.NewHandle(ctx, f.Title)
		if err := s.store.CreateForm(ctx, f); err != nil {
			return nil, err
		}
		created = true
	}
	added, same := 0, 0
	for _, r := range rowsIn {
		answers := map[string]any{}
		for _, c := range cols {
			if c.idx >= len(r) {
				continue
			}
			v := strings.TrimSpace(r[c.idx])
			if v == "" {
				continue
			}
			if c.field.Type == "checkboxes" {
				answers[c.field.ID] = splitMulti(v)
			} else {
				answers[c.field.ID] = v
			}
		}
		if len(answers) == 0 {
			continue
		}
		rowSum := sha1.Sum([]byte(strings.Join(r, "\x1f")))
		resp := &store.Response{UserID: uid, FormID: f.ID, Answers: answers, Origin: "import", Status: "seen",
			ClientKey: "csv:" + hex.EncodeToString(rowSum[:])[:20]}
		if timeCol >= 0 && timeCol < len(r) {
			resp.CreatedAt = parseWhen(r[timeCol])
		}
		describe(f, resp)
		if err := s.store.AddResponse(ctx, resp); err == nil {
			added++
		} else if errors.Is(err, store.ErrConflict) {
			same++
		}
	}
	return map[string]any{"form_id": f.ID, "form": f.Title, "created": created, "answers": added, "already": same, "questions": len(cols)}, nil
}

func (s *Server) formByTemplate(ctx context.Context, uid int64, key string) *store.Form {
	fs, _ := s.store.ListForms(ctx, uid, "")
	for _, f := range fs {
		if f.Template == key {
			return f
		}
	}
	return nil
}

// guessField makes a question from a column: its header and its answers.
func guessField(head string, vals []string) store.Field {
	fl := store.Field{Type: "text", Label: clip(strings.TrimSpace(head), 300)}
	lh := strings.ToLower(head)
	all := func(re *regexp.Regexp) bool {
		if len(vals) == 0 {
			return false
		}
		for _, v := range vals {
			if !re.MatchString(v) {
				return false
			}
		}
		return true
	}
	long, uniq := 0, map[string]int{}
	multi := false
	for _, v := range vals {
		if len(v) > 120 || strings.Contains(v, "\n") {
			long++
		}
		uniq[v]++
		if strings.Contains(v, ", ") || strings.Contains(v, ";") {
			multi = true
		}
	}
	switch {
	case all(reEmail):
		fl.Type, fl.Role = "email", "email"
	case strings.Contains(lh, "phone") || strings.Contains(lh, "telefon"):
		fl.Type, fl.Role = "phone", "phone"
	case all(reISODate):
		fl.Type = "date"
	case all(reNumber):
		fl.Type = "number"
	case long > 0:
		fl.Type = "long"
	case len(vals) >= 4 && len(uniq) <= 8 && len(uniq) < len(vals):
		if multi {
			fl.Type = "checkboxes"
			seen := map[string]bool{}
			for v := range uniq {
				for _, p := range splitMulti(v) {
					if !seen[p] {
						seen[p] = true
						fl.Options = append(fl.Options, p)
					}
				}
			}
		} else {
			fl.Type = "choice"
			for v := range uniq {
				fl.Options = append(fl.Options, v)
			}
		}
		sortStrings(fl.Options)
	}
	switch {
	case fl.Role != "":
	case strings.Contains(lh, "name") || strings.Contains(lh, "ime"):
		if fl.Type == "text" {
			fl.Role = "name"
		}
	case strings.Contains(lh, "company") || strings.Contains(lh, "firma"):
		fl.Role = "company"
	}
	return fl
}

func splitMulti(v string) []string {
	sep := ", "
	if strings.Contains(v, ";") {
		sep = ";"
	}
	var out []string
	for _, p := range strings.Split(v, sep) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseWhen reads the time other services write ("" when it can't).
func parseWhen(v string) string {
	v = strings.TrimSpace(v)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006/01/02 3:04:05 PM MST", "2006/01/02 15:04:05", "1/2/2006 15:04:05",
		"1/2/2006 3:04:05 PM", "02.01.2006 15:04:05", "2.1.2006 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "1/2/2006 15:04"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t.UTC().Format(time.RFC3339Nano)
		}
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 1e9 {
		return time.Unix(n, 0).UTC().Format(time.RFC3339Nano)
	}
	return ""
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func atoiSafe(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// ---- forms ----

const formCols = `id, user_id, handle, title, description, emoji, status, fields, accent, language, submit_label, thanks,
	redirect_url, close_at, max_answers, one_per_email, branding, notify, send_copy, auto, template, source, opened_at, created_at, updated_at`

func scanForm(sc interface{ Scan(...any) error }) (*Form, error) {
	var f Form
	var fields, auto string
	var one, brand, notify, copy int
	if err := sc.Scan(&f.ID, &f.UserID, &f.Handle, &f.Title, &f.Description, &f.Emoji, &f.Status, &fields, &f.Accent, &f.Language,
		&f.SubmitLabel, &f.Thanks, &f.RedirectURL, &f.CloseAt, &f.MaxAnswers, &one, &brand, &notify, &copy, &auto, &f.Template,
		&f.Source, &f.OpenedAt, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, notFound(err)
	}
	_ = json.Unmarshal([]byte(fields), &f.Fields)
	if f.Fields == nil {
		f.Fields = []Field{}
	}
	_ = json.Unmarshal([]byte(auto), &f.Auto)
	f.OnePerEmail, f.Branding, f.Notify, f.SendCopy = one == 1, brand == 1, notify == 1, copy == 1
	for _, x := range f.Fields {
		if x.Asks() {
			f.Questions++
		}
	}
	return &f, nil
}

// TakingState says whether a form takes answers now, and why not.
func (f *Form) TakingState(now time.Time, answers int) (bool, string) {
	switch {
	case f.Status == FormDraft:
		return false, "draft"
	case f.Status == FormClosed:
		return false, "closed"
	case f.CloseAt != "" && now.Format("2006-01-02") > f.CloseAt:
		return false, "date"
	case f.MaxAnswers > 0 && answers >= f.MaxAnswers:
		return false, "full"
	}
	return true, ""
}

// CreateForm stores a new form (its handle must be free).
func (s *Store) CreateForm(ctx context.Context, f *Form) error {
	if f.Fields == nil {
		f.Fields = []Field{}
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO forms (user_id, handle, title, description, emoji, status, fields, accent, language,
		submit_label, thanks, redirect_url, close_at, max_answers, one_per_email, branding, notify, send_copy, auto, template, source, opened_at,
		created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.UserID, f.Handle, f.Title, f.Description, f.Emoji, f.Status, toJSON(f.Fields), f.Accent, f.Language, f.SubmitLabel, f.Thanks,
		f.RedirectURL, f.CloseAt, f.MaxAnswers, boolInt(f.OnePerEmail), boolInt(f.Branding), boolInt(f.Notify), boolInt(f.SendCopy),
		toJSON(f.Auto), f.Template, f.Source, f.OpenedAt, firstOr(f.CreatedAt, now), firstOr(f.UpdatedAt, now))
	if err != nil {
		return conflict(err)
	}
	f.ID, _ = res.LastInsertId()
	if f.CreatedAt == "" {
		f.CreatedAt, f.UpdatedAt = now, now
	}
	return nil
}

// UpdateForm saves every field of a form of uid.
func (s *Store) UpdateForm(ctx context.Context, f *Form) error {
	f.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE forms SET handle=?, title=?, description=?, emoji=?, status=?, fields=?, accent=?, language=?,
		submit_label=?, thanks=?, redirect_url=?, close_at=?, max_answers=?, one_per_email=?, branding=?, notify=?, send_copy=?, auto=?,
		template=?, opened_at=?, updated_at=? WHERE id=? AND user_id=?`,
		f.Handle, f.Title, f.Description, f.Emoji, f.Status, toJSON(f.Fields), f.Accent, f.Language, f.SubmitLabel, f.Thanks, f.RedirectURL,
		f.CloseAt, f.MaxAnswers, boolInt(f.OnePerEmail), boolInt(f.Branding), boolInt(f.Notify), boolInt(f.SendCopy), toJSON(f.Auto),
		f.Template, f.OpenedAt, f.UpdatedAt, f.ID, f.UserID)
	if err != nil {
		return conflict(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetForm loads one of uid's forms.
func (s *Store) GetForm(ctx context.Context, uid, id int64) (*Form, error) {
	f, err := scanForm(s.DB.QueryRowContext(ctx, `SELECT `+formCols+` FROM forms WHERE id = ? AND user_id = ?`, id, uid))
	if err != nil {
		return nil, err
	}
	s.fillCounts(ctx, []*Form{f})
	return f, nil
}

// FormByHandle loads a form by its public address (anyone's).
func (s *Store) FormByHandle(ctx context.Context, handle string) (*Form, error) {
	return scanForm(s.DB.QueryRowContext(ctx, `SELECT `+formCols+` FROM forms WHERE handle = ?`, strings.ToLower(handle)))
}

// ListForms lists uid's forms, the ones changed last first.
func (s *Store) ListForms(ctx context.Context, uid int64, status string) ([]*Form, error) {
	q := `SELECT ` + formCols + ` FROM forms WHERE user_id = ?`
	args := []any{uid}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY CASE status WHEN 'open' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	var out []*Form
	for rows.Next() {
		f, err := scanForm(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.fillCounts(ctx, out)
	return out, nil
}

// fillCounts adds answers, new answers, views and whether it takes answers.
func (s *Store) fillCounts(ctx context.Context, fs []*Form) {
	if len(fs) == 0 {
		return
	}
	byID := map[int64]*Form{}
	for _, f := range fs {
		byID[f.ID] = f
	}
	uid := fs[0].UserID
	rows, err := s.DB.QueryContext(ctx, `SELECT form_id, COUNT(1), SUM(CASE WHEN status = 'new' THEN 1 ELSE 0 END), MAX(created_at)
		FROM responses WHERE user_id = ? AND status <> 'spam' GROUP BY form_id`, uid)
	if err == nil {
		for rows.Next() {
			var id int64
			var n, nw int
			var last sql.NullString
			if rows.Scan(&id, &n, &nw, &last) == nil {
				if f := byID[id]; f != nil {
					f.Responses, f.New, f.LastAnswer = n, nw, last.String
				}
			}
		}
		rows.Close()
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT form_id, SUM(count) FROM views WHERE user_id = ? GROUP BY form_id`, uid)
	if err == nil {
		for rows.Next() {
			var id int64
			var n int
			if rows.Scan(&id, &n) == nil {
				if f := byID[id]; f != nil {
					f.Views = n
				}
			}
		}
		rows.Close()
	}
	now := time.Now()
	for _, f := range fs {
		f.Taking, f.Why = f.TakingState(now, s.countAll(ctx, f.ID))
	}
}

// countAll counts every answer a form took (spam too: the limit is about what arrived).
func (s *Store) countAll(ctx context.Context, formID int64) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM responses WHERE form_id = ? AND status <> 'spam'`, formID).Scan(&n)
	return n
}

// CountAnswers is how many answers a form has (not counting spam).
func (s *Store) CountAnswers(ctx context.Context, formID int64) int { return s.countAll(ctx, formID) }

// DeleteForm removes a form with its answers.
func (s *Store) DeleteForm(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM forms WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// HandleFree reports whether nobody's form uses this address (but form id).
func (s *Store) HandleFree(ctx context.Context, handle string, but int64) bool {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM forms WHERE handle = ? AND id <> ?`, handle, but).Scan(&n)
	return n == 0
}

var reHandle = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,58}[a-z0-9])?$`)

// ValidHandle: lower-case letters, digits and dashes, 3–60 long.
func ValidHandle(h string) bool { return len(h) >= 3 && reHandle.MatchString(h) }

// Slugify makes an address part from a title ("Café menu" → "cafe-menu").
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if t, ok := latin[r]; ok {
			b.WriteString(t)
			dash = false
			continue
		}
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

// latin spells the usual accented letters without the accent.
var latin = func() map[rune]string {
	m := map[rune]string{'đ': "dj", 'ß': "ss", 'æ': "ae", 'ø': "o", 'ł': "l", 'œ': "oe", 'þ': "th"}
	for plain, accented := range map[string]string{"a": "àáâãäåāăą", "c": "çćčĉċ", "d": "ďð", "e": "èéêëēĕėęě", "g": "ĝğġģ",
		"i": "ìíîïĩīĭįı", "n": "ñńņňŉ", "o": "òóôõöōŏő", "r": "ŕŗř", "s": "śŝşšș", "t": "ţťŧț", "u": "ùúûüũūŭůűų",
		"y": "ýÿŷ", "z": "źżž"} {
		for _, r := range accented {
			m[r] = plain
		}
	}
	return m
}()

const keyChars = "abcdefghjkmnpqrstuvwxyz23456789"

// RandomKey is n characters that are easy to read and type.
func RandomKey(n int) string {
	b := make([]byte, n)
	for i := range b {
		x, _ := rand.Int(rand.Reader, big.NewInt(int64(len(keyChars))))
		b[i] = keyChars[x.Int64()]
	}
	return string(b)
}

// NewHandle is a free address for a form: its title and four random
// characters, so a link cannot be guessed from the title alone.
func (s *Store) NewHandle(ctx context.Context, title string) string {
	base := Slugify(title)
	if base == "" {
		base = "form"
	}
	for {
		h := base + "-" + RandomKey(4)
		if s.HandleFree(ctx, h, 0) {
			return h
		}
	}
}

func firstOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// ---- views ----

// AddView counts one person opening a form today.
func (s *Store) AddView(ctx context.Context, uid, formID int64) {
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO views (user_id, form_id, day, count) VALUES (?, ?, ?, 1)
		ON CONFLICT(form_id, day) DO UPDATE SET count = count + 1`, uid, formID, Today())
}

// DayCount is a number on one day.
type DayCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// ViewsByDay is how many opened a form (0 = all of uid's) per day since from.
func (s *Store) ViewsByDay(ctx context.Context, uid, formID int64, from string) ([]DayCount, error) {
	q := `SELECT day, SUM(count) FROM views WHERE user_id = ? AND day >= ?`
	args := []any{uid, from}
	if formID > 0 {
		q += ` AND form_id = ?`
		args = append(args, formID)
	}
	return s.dayCounts(ctx, q+` GROUP BY day ORDER BY day`, args...)
}

func (s *Store) dayCounts(ctx context.Context, q string, args ...any) ([]DayCount, error) {
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DayCount{}
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

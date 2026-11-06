package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// ---- answers ----

const respCols = `r.id, r.user_id, r.form_id, r.number, r.answers, r.name, r.email, r.preview, r.status, r.starred, r.note, r.origin,
	r.ip_hash, r.client_key, r.created_at, r.updated_at, f.title, f.emoji`

func scanResponse(sc interface{ Scan(...any) error }) (*Response, error) {
	var r Response
	var answers string
	var starred int
	if err := sc.Scan(&r.ID, &r.UserID, &r.FormID, &r.Number, &answers, &r.Name, &r.Email, &r.Preview, &r.Status, &starred, &r.Note,
		&r.Origin, &r.IPHash, &r.ClientKey, &r.CreatedAt, &r.UpdatedAt, &r.FormTitle, &r.FormEmoji); err != nil {
		return nil, notFound(err)
	}
	r.Starred = starred == 1
	_ = json.Unmarshal([]byte(answers), &r.Answers)
	if r.Answers == nil {
		r.Answers = map[string]any{}
	}
	return &r, nil
}

// AddResponse stores answers with the next number of the form. With a
// client_key already there it changes nothing and returns ErrConflict.
func (s *Store) AddResponse(ctx context.Context, r *Response) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) + 1 FROM responses WHERE form_id = ?`, r.FormID).Scan(&r.Number); err != nil {
		return err
	}
	if r.Status == "" {
		r.Status = "new"
	}
	if r.Origin == "" {
		r.Origin = "web"
	}
	now := Now()
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	r.UpdatedAt = firstOr(r.UpdatedAt, r.CreatedAt)
	res, err := tx.ExecContext(ctx, `INSERT INTO responses (user_id, form_id, number, answers, name, email, preview, status, starred, note,
		origin, ip_hash, client_key, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.UserID, r.FormID, r.Number, toJSON(r.Answers), r.Name, strings.ToLower(r.Email), r.Preview, r.Status, boolInt(r.Starred), r.Note,
		r.Origin, r.IPHash, r.ClientKey, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return conflict(err)
	}
	r.ID, _ = res.LastInsertId()
	return tx.Commit()
}

// GetResponse loads one of uid's answers.
func (s *Store) GetResponse(ctx context.Context, uid, id int64) (*Response, error) {
	r, err := scanResponse(s.DB.QueryRowContext(ctx, `SELECT `+respCols+` FROM responses r JOIN forms f ON f.id = r.form_id
		WHERE r.id = ? AND r.user_id = ?`, id, uid))
	if err != nil {
		return nil, err
	}
	r.Made, _ = s.MadeFor(ctx, uid, r.ID)
	return r, nil
}

// UpdateResponse saves status, star and note.
func (s *Store) UpdateResponse(ctx context.Context, r *Response) error {
	r.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE responses SET status = ?, starred = ?, note = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		r.Status, boolInt(r.Starred), r.Note, r.UpdatedAt, r.ID, r.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus changes the status of several of uid's answers at once.
func (s *Store) SetStatus(ctx context.Context, uid int64, ids []int64, status string) (int, error) {
	n := 0
	for _, id := range ids {
		res, err := s.DB.ExecContext(ctx, `UPDATE responses SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?`, status, Now(), id, uid)
		if err != nil {
			return n, err
		}
		k, _ := res.RowsAffected()
		n += int(k)
	}
	return n, nil
}

// DeleteResponse removes one of uid's answers.
func (s *Store) DeleteResponse(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM responses WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResponseFilter narrows a list of answers.
type ResponseFilter struct {
	FormID  int64
	Status  string // new | seen | done | spam | open (new + seen) | "" (everything but spam)
	Starred bool
	Query   string
	Email   string
	Day     string // YYYY-MM-DD (local)
	Since   string // created at or after (RFC 3339)
	Oldest  bool   // oldest first (exports)
	Limit   int
	Offset  int
}

// ListResponses lists uid's answers with the total that match.
func (s *Store) ListResponses(ctx context.Context, uid int64, f ResponseFilter) ([]*Response, int, error) {
	where := []string{"r.user_id = ?"}
	args := []any{uid}
	switch f.Status {
	case "":
		where = append(where, "r.status <> 'spam'")
	case "open":
		where = append(where, "r.status IN ('new','seen')")
	default:
		where = append(where, "r.status = ?")
		args = append(args, f.Status)
	}
	if f.FormID > 0 {
		where = append(where, "r.form_id = ?")
		args = append(args, f.FormID)
	}
	if f.Starred {
		where = append(where, "r.starred = 1")
	}
	if f.Email != "" {
		where = append(where, "r.email = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(f.Email)))
	}
	if f.Since != "" {
		where = append(where, "r.created_at >= ?")
		args = append(args, f.Since)
	}
	if f.Day != "" {
		if t, err := time.ParseInLocation("2006-01-02", f.Day, time.Local); err == nil {
			where = append(where, "r.created_at >= ? AND r.created_at < ?")
			args = append(args, t.UTC().Format(time.RFC3339Nano), t.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano))
		}
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		for _, w := range strings.Fields(strings.ToLower(q)) {
			like := "%" + w + "%"
			where = append(where, "(lower(r.answers) LIKE ? OR lower(r.name) LIKE ? OR r.email LIKE ? OR lower(r.note) LIKE ? OR lower(f.title) LIKE ?)")
			args = append(args, like, like, like, like, like)
		}
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM responses r JOIN forms f ON f.id = r.form_id WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "r.created_at DESC, r.id DESC"
	if f.Oldest {
		order = "r.created_at, r.id"
	}
	lim := f.Limit
	if lim <= 0 || lim > 5000 {
		lim = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+respCols+` FROM responses r JOIN forms f ON f.id = r.form_id WHERE `+w+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, lim, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	out := []*Response{}
	for rows.Next() {
		r, err := scanResponse(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) > 0 {
		made, _ := s.madeOf(ctx, uid, out)
		for _, r := range out {
			r.Made = made[r.ID]
		}
	}
	return out, total, nil
}

// Neighbours are the answers before and after one in the inbox order.
func (s *Store) Neighbours(ctx context.Context, uid int64, r *Response, formID int64) (prev, next int64) {
	cond, args := "user_id = ? AND status <> 'spam'", []any{uid}
	if formID > 0 {
		cond += " AND form_id = ?"
		args = append(args, formID)
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT id FROM responses WHERE `+cond+` AND (created_at > ? OR (created_at = ? AND id > ?)) ORDER BY created_at, id LIMIT 1`,
		append(args, r.CreatedAt, r.CreatedAt, r.ID)...).Scan(&prev)
	_ = s.DB.QueryRowContext(ctx, `SELECT id FROM responses WHERE `+cond+` AND (created_at < ? OR (created_at = ? AND id < ?)) ORDER BY created_at DESC, id DESC LIMIT 1`,
		append(args, r.CreatedAt, r.CreatedAt, r.ID)...).Scan(&next)
	return prev, next
}

// EmailAnswered reports whether this address already answered the form.
func (s *Store) EmailAnswered(ctx context.Context, formID int64, email string) bool {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM responses WHERE form_id = ? AND email = ? AND status <> 'spam'`,
		formID, strings.ToLower(strings.TrimSpace(email))).Scan(&n)
	return n > 0
}

// RecentFrom counts answers from one address hash since a moment ago.
func (s *Store) RecentFrom(ctx context.Context, ipHash string, within time.Duration) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM responses WHERE ip_hash = ? AND created_at >= ?`, ipHash,
		time.Now().Add(-within).UTC().Format(time.RFC3339Nano)).Scan(&n)
	return n
}

// CountNew is how many of uid's answers nobody has looked at.
func (s *Store) CountNew(ctx context.Context, uid int64) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM responses WHERE user_id = ? AND status = 'new'`, uid).Scan(&n)
	return n
}

// AnswersByDay counts uid's answers per local day since from (form 0 = all).
func (s *Store) AnswersByDay(ctx context.Context, uid, formID int64, from string) ([]DayCount, error) {
	t, err := time.ParseInLocation("2006-01-02", from, time.Local)
	if err != nil {
		return nil, err
	}
	q := `SELECT created_at FROM responses WHERE user_id = ? AND status <> 'spam' AND created_at >= ?`
	args := []any{uid, t.UTC().Format(time.RFC3339Nano)}
	if formID > 0 {
		q += ` AND form_id = ?`
		args = append(args, formID)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	by := map[string]int{}
	for rows.Next() {
		var at string
		if rows.Scan(&at) == nil {
			if tt, err := time.Parse(time.RFC3339Nano, at); err == nil {
				by[tt.Local().Format("2006-01-02")]++
			}
		}
	}
	out := []DayCount{}
	for d := t; !d.After(time.Now()); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		out = append(out, DayCount{Day: k, Count: by[k]})
	}
	return out, rows.Err()
}

// ---- made in other apps ----

// AddMade remembers something made from an answer.
func (s *Store) AddMade(ctx context.Context, uid int64, m *Made) error {
	m.CreatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO made (user_id, response_id, target, app, ref, title, url, error, auto, created_at)
		SELECT ?, id, ?, ?, ?, ?, ?, ?, ?, ? FROM responses WHERE id = ? AND user_id = ?`,
		uid, m.Target, m.App, m.Ref, m.Title, m.URL, m.Error, boolInt(m.Auto), m.CreatedAt, m.ResponseID, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

// DropMadeErrors forgets the failed tries of one kind (a later try worked).
func (s *Store) DropMadeErrors(ctx context.Context, uid, responseID int64, target string) {
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM made WHERE user_id = ? AND response_id = ? AND target = ? AND error <> ''`, uid, responseID, target)
}

// MadeFor lists what was made from one answer.
func (s *Store) MadeFor(ctx context.Context, uid, responseID int64) ([]Made, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, response_id, target, app, ref, title, url, error, auto, created_at FROM made
		WHERE user_id = ? AND response_id = ? ORDER BY id`, uid, responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Made{}
	for rows.Next() {
		m, err := scanMade(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMade(sc interface{ Scan(...any) error }) (Made, error) {
	var m Made
	var auto int
	err := sc.Scan(&m.ID, &m.ResponseID, &m.Target, &m.App, &m.Ref, &m.Title, &m.URL, &m.Error, &auto, &m.CreatedAt)
	m.Auto = auto == 1
	return m, err
}

func (s *Store) madeOf(ctx context.Context, uid int64, rs []*Response) (map[int64][]Made, error) {
	ids := make([]any, 0, len(rs)+1)
	ids = append(ids, uid)
	marks := make([]string, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
		marks = append(marks, "?")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, response_id, target, app, ref, title, url, error, auto, created_at FROM made
		WHERE user_id = ? AND response_id IN (`+strings.Join(marks, ",")+`) ORDER BY id`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Made{}
	for rows.Next() {
		m, err := scanMade(rows)
		if err != nil {
			return nil, err
		}
		out[m.ResponseID] = append(out[m.ResponseID], m)
	}
	return out, rows.Err()
}

// ---- for the morning message and the summary ----

// FirstNewAt is when the oldest unread answer arrived ("" when none).
func (s *Store) FirstNewAt(ctx context.Context, uid int64) string {
	var at sql.NullString
	_ = s.DB.QueryRowContext(ctx, `SELECT MIN(created_at) FROM responses WHERE user_id = ? AND status = 'new'`, uid).Scan(&at)
	return at.String
}

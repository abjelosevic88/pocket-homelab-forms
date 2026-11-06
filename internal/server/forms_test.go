package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// newForm makes an open form from a template for a token's person.
func (a *app) newForm(auth, template string) map[string]any {
	a.t.Helper()
	f := a.ok(auth, "POST", "/api/v1/forms", map[string]any{"template": template})
	return a.ok(auth, "POST", "/api/v1/forms/"+id(f)+"/open", nil)
}

var reStamp = regexp.MustCompile(`name="stamp" value="([^"]+)"`)

// submit fills a public form like a browser: open the page, wait, send.
func (a *app) submit(handle string, vals url.Values) (int, string) {
	a.t.Helper()
	res, err := http.Get(a.url + "/f/" + handle)
	if err != nil {
		a.t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	m := reStamp.FindStringSubmatch(string(page))
	if m == nil {
		a.t.Fatalf("no form on the page (%d): %s", res.StatusCode, page)
	}
	if vals.Get("stamp") == "" {
		vals.Set("stamp", m[1])
	}
	cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err = cl.PostForm(a.url+"/f/"+handle, vals)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body) + res.Header.Get("Location")
}

func briefAnswers() url.Values {
	return url.Values{"name": {"Ana Marić"}, "email": {"Ana@Example.com"}, "subject": {"A shop for our bakery"},
		"message": {"Cakes to order online."}, "kind": {"An online shop", "__other"}, "kind__other": {"Delivery"}, "budget": {"Not sure yet"}}
}

func TestPublicSubmitAndPerPerson(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	f := a.newForm(a.alice, "project")
	handle := str(f, "handle")

	// a missing required answer and a bad email come back with the answers kept
	code, body := a.submit(handle, url.Values{"name": {"Ana"}, "email": {"not-an-email"}})
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "does not look like an email") || !strings.Contains(body, `value="Ana"`) {
		t.Fatalf("bad answers: %d %s", code, body)
	}
	code, body = a.submit(handle, briefAnswers())
	if code != http.StatusSeeOther || !strings.HasSuffix(body, "/thanks") {
		t.Fatalf("submit: %d %s", code, body)
	}
	// the honeypot is thanked and forgotten
	hp := briefAnswers()
	hp.Set("website", "http://spam.example")
	if code, _ := a.submit(handle, hp); code != http.StatusSeeOther {
		t.Fatalf("honeypot: %d", code)
	}
	// a forged stamp is not taken
	forged := briefAnswers()
	forged.Set("stamp", "123.abc")
	if code, _ := a.submit(handle, forged); code != http.StatusUnprocessableEntity {
		t.Fatalf("forged stamp: %d", code)
	}
	time.Sleep(50 * time.Millisecond)
	l := a.ok(a.alice, "GET", "/api/v1/responses", nil)
	items := l["items"].([]any)
	if len(items) != 1 || num(l, "new") != 1 {
		t.Fatalf("answers: %v", l)
	}
	r := items[0].(map[string]any)
	if str(r, "name") != "Ana Marić" || str(r, "email") != "ana@example.com" || str(r, "preview") != "A shop for our bakery" {
		t.Fatalf("roles: %v", r)
	}
	one := a.ok(a.alice, "GET", "/api/v1/responses/"+id(r), nil)
	resp := one["response"].(map[string]any)
	kinds := resp["answers"].(map[string]any)["kind"].([]any)
	if len(kinds) != 2 || kinds[1] != "Delivery" || str(resp, "status") != "seen" {
		t.Fatalf("answer: %v", resp)
	}

	// Bob sees none of it, and Alice's ids are 404 for him
	if l := a.ok(a.bob, "GET", "/api/v1/responses", nil); len(l["items"].([]any)) != 0 {
		t.Fatalf("bob sees answers: %v", l)
	}
	if fs := a.list(a.bob, "/api/v1/forms"); len(fs) != 0 {
		t.Fatalf("bob sees forms: %v", fs)
	}
	for _, p := range []string{"/api/v1/forms/" + id(f), "/api/v1/responses/" + id(r), "/api/v1/forms/" + id(f) + "/stats", "/api/v1/forms/" + id(f) + "/export.csv"} {
		a.expect(404, a.bob, "GET", p, nil)
	}
	a.expect(404, a.bob, "PUT", "/api/v1/forms/"+id(f), map[string]any{"title": "x"})
	a.expect(404, a.bob, "PUT", "/api/v1/responses/"+id(r), map[string]any{"status": "done"})
	a.expect(404, a.bob, "DELETE", "/api/v1/responses/"+id(r), nil)
	a.expect(404, a.bob, "POST", "/api/v1/responses/"+id(r)+"/make", map[string]any{"target": "task"})
	if out := a.ok(a.bob, "POST", "/api/v1/responses/bulk", map[string]any{"ids": []int64{n(id(r))}, "status": "spam"}); num(out, "changed") != 0 {
		t.Fatalf("bob changed alice's answer: %v", out)
	}
	if out := a.ok(a.bob, "GET", "/api/v1/search?q=bakery", nil); len(out["items"].([]any)) != 0 {
		t.Fatalf("bob finds alice's answer: %v", out)
	}

	// the CSV has the question texts and the answer
	req, _ := http.NewRequest("GET", a.url+"/api/v1/forms/"+id(f)+"/export.csv", nil)
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, _ := http.DefaultClient.Do(req)
	csvBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(csvBody), "In one line: what is it?") || !strings.Contains(string(csvBody), "A shop for our bakery") {
		t.Fatalf("csv: %s", csvBody)
	}
}

func TestFormLimitsAndClosing(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	f := a.newForm(a.alice, "contact")
	a.ok(a.alice, "PUT", "/api/v1/forms/"+id(f), map[string]any{"one_per_email": true, "max_answers": 2})
	h := str(f, "handle")
	ans := url.Values{"name": {"Ana"}, "email": {"ana@example.com"}, "message": {"Hi"}}
	if code, _ := a.submit(h, ans); code != http.StatusSeeOther {
		t.Fatalf("first: %d", code)
	}
	if code, body := a.submit(h, ans); code != http.StatusOK || !strings.Contains(body, "already answered") {
		t.Fatalf("same email again: %d %s", code, body)
	}
	ans.Set("email", "emir@example.com")
	a.submit(h, ans)
	// full: the page says so and takes nothing
	res, _ := http.Get(a.url + "/f/" + h)
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(page), "all the answers it needs") || strings.Contains(string(page), `name="stamp"`) {
		t.Fatalf("full form page: %s", page)
	}
	got := a.ok(a.alice, "GET", "/api/v1/forms/"+id(f), nil)
	if got["taking"] != false || str(got, "why") != "full" || num(got, "responses") != 2 {
		t.Fatalf("form state: %v", got)
	}
	// a draft is not public
	d := a.ok(a.alice, "POST", "/api/v1/forms", map[string]any{"template": "rsvp"})
	res, _ = http.Get(a.url + "/f/" + str(d, "handle"))
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("draft is public: %d", res.StatusCode)
	}
	// closing
	a.ok(a.alice, "POST", "/api/v1/forms/"+id(f)+"/close", nil)
	res, _ = http.Get(a.url + "/f/" + h)
	page, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(page), "This form is closed") {
		t.Fatalf("closed page: %s", page)
	}
}

func TestFieldsAreChecked(t *testing.T) {
	a := newApp(t, nil)
	f := a.ok(a.alice, "POST", "/api/v1/forms", map[string]any{"title": "Test"})
	a.expect(400, a.alice, "PUT", "/api/v1/forms/"+id(f), map[string]any{"fields": []map[string]any{{"type": "magic", "label": "x"}}})
	a.expect(400, a.alice, "PUT", "/api/v1/forms/"+id(f), map[string]any{"fields": []map[string]any{{"type": "text", "label": ""}}})
	out := a.ok(a.alice, "PUT", "/api/v1/forms/"+id(f), map[string]any{"fields": []map[string]any{
		{"id": "a", "type": "choice", "label": "Pick", "options": []string{" one ", "two", "One", ""}},
		{"id": "a", "type": "text", "label": "Dup id", "role": "nonsense"}}})
	fs := out["fields"].([]any)
	first, second := fs[0].(map[string]any), fs[1].(map[string]any)
	if len(first["options"].([]any)) != 2 || str(second, "id") == "a" || str(second, "role") != "" {
		t.Fatalf("cleaned: %v", fs)
	}
	// addresses are unique on the box
	g := a.ok(a.bob, "POST", "/api/v1/forms", map[string]any{"title": "Other"})
	a.expect(400, a.bob, "PUT", "/api/v1/forms/"+id(g), map[string]any{"handle": str(f, "handle")})
	a.ok(a.alice, "POST", "/api/v1/forms/"+id(f)+"/open", nil)
	empty := a.ok(a.alice, "POST", "/api/v1/forms", map[string]any{"title": "Empty"})
	a.expect(400, a.alice, "POST", "/api/v1/forms/"+id(empty)+"/open", nil)
}

func TestLinkScopes(t *testing.T) {
	req := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	cases := []struct {
		scopes []string
		r      *http.Request
		want   bool
	}{
		{[]string{"forms:read"}, req("GET", "/api/v1/forms"), true},
		{[]string{"forms:read"}, req("GET", "/api/v1/responses/7"), true},
		{[]string{"forms:read"}, req("GET", "/api/v1/day/2026-11-06"), true},
		{[]string{"forms:read"}, req("POST", "/api/v1/forms"), false},
		{[]string{"forms:read"}, req("PUT", "/api/v1/responses/7"), false},
		{[]string{"forms:write"}, req("POST", "/api/v1/forms"), true},
		{[]string{"forms:write"}, req("POST", "/api/v1/forms/3/open"), true},
		{[]string{"forms:write"}, req("DELETE", "/api/v1/forms/3"), false},
		{[]string{"forms:write", "forms:read"}, req("POST", "/api/v1/responses/3/make"), false},
		{[]string{"forms:read"}, req("GET", "/api/v1/settings"), false},
		{nil, req("GET", "/api/v1/auth/me"), true},
	}
	for _, c := range cases {
		if got := linkAllows(c.scopes, c.r); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

func TestExportImportAgain(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	f := a.newForm(a.alice, "contact")
	a.submit(str(f, "handle"), url.Values{"name": {"Ana"}, "email": {"ana@example.com"}, "message": {"Hello"}})
	a.submit(str(f, "handle"), url.Values{"name": {"Emir"}, "email": {"emir@example.com"}, "message": {"Hi"}})
	time.Sleep(50 * time.Millisecond)
	doc := a.ok(a.alice, "GET", "/api/v1/export.json", nil)
	// into Bob's account, twice: one form, two answers
	for i, want := range []float64{2, 0} {
		out := a.ok(a.bob, "POST", "/api/v1/import", doc)
		if num(out, "answers") != want {
			t.Fatalf("import %d: %v", i, out)
		}
	}
	if fs := a.list(a.bob, "/api/v1/forms"); len(fs) != 1 {
		t.Fatalf("bob's forms: %v", fs)
	}
	// Alice's form keeps its address; Bob's copy got a new one
	bf := a.list(a.bob, "/api/v1/forms")[0].(map[string]any)
	if str(bf, "handle") == str(f, "handle") {
		t.Fatal("bob's copy took alice's address")
	}
	if l := a.ok(a.bob, "GET", "/api/v1/responses", nil); num(l, "total") != 2 {
		t.Fatalf("bob's answers: %v", l)
	}
}

func TestImportGoogleFormsCSV(t *testing.T) {
	a := newApp(t, nil)
	csvData := "Timestamp,Your name,Email Address,How did you like it?,Comments\n" +
		"2026/10/01 9:15:00 AM GMT+2,Ana,ana@example.com,Very much,Great day\n" +
		"2026/10/01 10:20:00 AM GMT+2,Emir,emir@example.com,Very much,\n" +
		"2026/10/02 11:00:00 AM GMT+2,Lejla,lejla@example.com,A little,Too short\n" +
		"2026/10/03 08:00:00 AM GMT+2,Marko,marko@example.com,Very much,More coffee please\n"
	up := func() map[string]any {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		fw, _ := mw.CreateFormFile("file", "Workshop feedback (Responses) - Form responses 1.csv")
		fw.Write([]byte(csvData))
		mw.Close()
		req, _ := http.NewRequest("POST", a.url+"/api/v1/import", &b)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+a.alice)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var m map[string]any
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("import: %d %s", res.StatusCode, raw)
		}
		_ = json.Unmarshal(raw, &m)
		return m
	}
	first := up()
	if num(first, "answers") != 4 || first["created"] != true || str(first, "form") != "Workshop feedback" {
		t.Fatalf("first import: %v", first)
	}
	if again := up(); num(again, "answers") != 0 || num(again, "already") != 4 || again["created"] != false {
		t.Fatalf("again: %v", again)
	}
	f := a.ok(a.alice, "GET", "/api/v1/forms/"+itoa(int64(num(first, "form_id"))), nil)
	var roles, types []string
	for _, x := range f["fields"].([]any) {
		roles = append(roles, str(x.(map[string]any), "role"))
		types = append(types, str(x.(map[string]any), "type"))
	}
	if strings.Join(roles, ",") != "name,email,," || types[1] != "email" || types[2] != "choice" {
		t.Fatalf("guessed: %v %v", roles, types)
	}
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.bob, "PUT", "/api/v1/settings", map[string]any{"language": "bs", "task_days": 5, "notify_each": false})
	if s := a.ok(a.alice, "GET", "/api/v1/settings", nil); str(s, "language") != "en" || num(s, "task_days") != 2 || s["notify_each"] != true {
		t.Fatalf("alice got bob's settings: %v", s)
	}
	a.expect(400, a.bob, "PUT", "/api/v1/settings", map[string]any{"language": "xx"})
	// Bob's new forms speak his language
	if f := a.ok(a.bob, "POST", "/api/v1/forms", map[string]any{"title": "Upit"}); str(f, "language") != "bs" {
		t.Fatalf("bob's form language: %v", f)
	}
}

func TestDescribeUsesRoles(t *testing.T) {
	tpl, _ := templateByKey("quote")
	f := &store.Form{Fields: tpl.Fields}
	r := &store.Response{Answers: map[string]any{"name": "Ana", "phone": "+387 61 000 000", "subject": "Paint two rooms", "message": "Sizes…"}}
	describe(f, r)
	if r.Name != "Ana" || r.Preview != "Paint two rooms" || r.Email != "" {
		t.Fatalf("describe: %+v", r)
	}
	if got := who(f, r); got != "Ana" {
		t.Fatalf("who: %s", got)
	}
	_ = context.Background()
}

func TestDemoSeeds(t *testing.T) {
	a := newApp(t, nil)
	if err := a.s.seedDemo(context.Background(), a.ids["Alice"]); err != nil {
		t.Fatal(err)
	}
	s := a.ok(a.alice, "GET", "/api/v1/summary", nil)
	if num(s, "forms") != 6 || num(s, "responses") < 50 || num(s, "new") == 0 {
		t.Fatalf("summary: %v", s)
	}
	// seeding twice adds nothing
	_ = a.s.seedDemo(context.Background(), a.ids["Alice"])
	if s2 := a.ok(a.alice, "GET", "/api/v1/summary", nil); num(s2, "forms") != 6 {
		t.Fatalf("seeded twice: %v", s2)
	}
	if l := a.ok(a.bob, "GET", "/api/v1/summary", nil); num(l, "forms") != 0 {
		t.Fatalf("bob got demo data: %v", l)
	}
}

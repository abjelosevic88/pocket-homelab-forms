package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseBudget(t *testing.T) {
	for _, c := range []struct {
		in       string
		amount   float64
		currency string
		ok       bool
	}{
		{"4500", 4500, "", true},
		{"€2,000 – €5,000", 2000, "EUR", true},
		{"2.000 - 5.000 KM", 2000, "BAM", true},
		{"5k", 5000, "", true},
		{"$1,250.50", 1250.5, "USD", true},
		{"Under €2,000", 2000, "EUR", true},
		{"1 500 EUR", 1500, "EUR", true},
		{"3000EUR", 3000, "EUR", true},
		{"Not sure yet", 0, "", false},
		{"", 0, "", false},
		{"2 pages, about 300", 0, "", false},
		{"1, 2 or 3", 0, "", false},
	} {
		a, cur, ok := parseBudget(c.in)
		if a != c.amount || cur != c.currency || ok != c.ok {
			t.Errorf("parseBudget(%q) = %v %q %v, want %v %q %v", c.in, a, cur, ok, c.amount, c.currency, c.ok)
		}
	}
}

// fakeCRM answers POST /api/v1/deals like Pocket CRM: one deal per source_ref.
type fakeCRM struct {
	mu     sync.Mutex
	posted []map[string]any
	byRef  map[string]int
}

func (f *fakeCRM) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer crm-tok" {
			w.WriteHeader(401)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/deals" {
			t.Errorf("crm asked %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.posted = append(f.posted, body)
		ref, _ := body["source_ref"].(string)
		w.Header().Set("Content-Type", "application/json")
		id, seen := f.byRef[ref]
		if !seen {
			id = 7 + len(f.byRef)
			f.byRef[ref] = id
			w.WriteHeader(http.StatusCreated)
		}
		fmt.Fprintf(w, `{"id":%d,"title":%q,"status":"open"}`, id, body["title"])
	}
}

func (f *fakeCRM) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.posted)
}

func TestAnswerToCRMDeal(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	crm := &fakeCRM{byRef: map[string]int{}}
	fake := httptest.NewServer(crm.handler(t))
	defer fake.Close()

	f := a.newForm(a.alice, "project")
	vals := briefAnswers()
	vals.Set("budget", "€2,000 – €5,000")
	vals.Set("deadline", "2099-12-01")
	vals.Set("company", "Pekara Ana")
	if code, body := a.submit(str(f, "handle"), vals); code != http.StatusSeeOther {
		t.Fatalf("submit: %d %s", code, body)
	}
	time.Sleep(50 * time.Millisecond)
	l := a.ok(a.alice, "GET", "/api/v1/responses", nil)
	rid := id(l["items"].([]any)[0].(map[string]any))

	// not connected: greyed with why, and making one says so
	apps := a.list(a.alice, "/api/v1/apps")
	var crmState map[string]any
	for _, x := range apps {
		if m := x.(map[string]any); m["slug"] == "crm" {
			crmState = m
		}
	}
	if crmState == nil || crmState["connected"] != false || str(crmState, "detail") == "" {
		t.Fatalf("crm before connecting: %v", crmState)
	}
	if m := a.expect(400, a.alice, "POST", "/api/v1/responses/"+rid+"/make", map[string]any{"target": "deal"}); !strings.Contains(str(m, "error"), "Pocket CRM") {
		t.Fatalf("not connected: %v", m)
	}

	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"crm_mode": "direct", "crm_url": fake.URL, "crm_token": "crm-tok"})
	made := a.expect(201, a.alice, "POST", "/api/v1/responses/"+rid+"/make", map[string]any{"target": "deal"})
	if str(made, "url") != fake.URL+"/deals/7" || str(made, "ref") != "7" || str(made, "title") != "Pekara Ana — A shop for our bakery" || str(made, "app") != "crm" {
		t.Fatalf("made: %v", made)
	}
	body := crm.posted[0]
	want := map[string]any{"title": "Pekara Ana — A shop for our bakery", "contact_name": "Ana Marić", "contact_email": "ana@example.com",
		"company": "Pekara Ana", "amount": 2000.0, "currency": "EUR", "expected_close": "2099-12-01", "source": "form",
		"source_ref": "answer:" + rid, "next_action": "Get back to Ana Marić", "next_due": time.Now().AddDate(0, 0, 2).Format("2006-01-02"),
		"note": "From the form “Start a project with me”"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("deal %s = %v, want %v", k, body[k], v)
		}
	}
	if d, _ := body["description"].(string); !strings.Contains(d, "**In one line: what is it?:** A shop for our bakery") || !strings.Contains(d, "/answers/"+rid) {
		t.Errorf("description: %q", d)
	}
	if _, has := body["contact_phone"]; has {
		t.Errorf("an empty phone was sent: %v", body)
	}

	// again (by hand, or through Accept the brief): the same deal, remembered once
	again := a.ok(a.alice, "POST", "/api/v1/responses/"+rid+"/make", map[string]any{"target": "deal"})
	if id(again) != id(made) {
		t.Fatalf("second make: %v vs %v", again, made)
	}
	acc := a.ok(a.alice, "POST", "/api/v1/responses/"+rid+"/accept", map[string]any{"deal": true})
	steps := acc["steps"].([]any)
	if len(steps) != 1 || steps[0].(map[string]any)["target"] != "deal" || steps[0].(map[string]any)["error"] != nil {
		t.Fatalf("accept: %v", steps)
	}
	one := a.ok(a.alice, "GET", "/api/v1/responses/"+rid, nil)
	deals := 0
	for _, m := range one["response"].(map[string]any)["made"].([]any) {
		if mm := m.(map[string]any); mm["target"] == "deal" {
			deals++
		}
	}
	if deals != 1 || crm.count() != 3 {
		t.Fatalf("deals remembered %d, asked %d times", deals, crm.count())
	}

	// a budget that is not a number stays in the description only
	vals = briefAnswers()
	vals.Set("budget", "Not sure yet")
	a.ok(a.alice, "PUT", "/api/v1/forms/"+id(f), map[string]any{"auto": map[string]any{"deal": true}})
	if code, body := a.submit(str(f, "handle"), vals); code != http.StatusSeeOther {
		t.Fatalf("submit: %d %s", code, body)
	}
	for i := 0; i < 100 && crm.count() < 4; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if crm.count() != 4 {
		t.Fatalf("auto deal not made: %d", crm.count())
	}
	b := crm.posted[3]
	if _, has := b["amount"]; has || b["title"] != "Ana Marić — A shop for our bakery" || b["source_ref"] == "answer:"+rid {
		t.Fatalf("auto deal: %v", b)
	}
	if d, _ := b["description"].(string); !strings.Contains(d, "Not sure yet") {
		t.Fatalf("budget text not in the description: %q", d)
	}

	// Bob cannot make a deal from Alice's answer
	a.expect(404, a.bob, "POST", "/api/v1/responses/"+rid+"/make", map[string]any{"target": "deal"})
}

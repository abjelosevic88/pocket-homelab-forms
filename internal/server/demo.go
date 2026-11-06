package server

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- demo data (DEMO_DATA=true) ----
//
// A designer's forms after a few months: client briefs coming in, a
// feedback form, a workshop with sign-ups, a summer party that is over and
// a contact form — enough to try everything and for the screenshots.

type demoPerson struct{ name, email, company string }

var demoPeople = []demoPerson{
	{"Ana Marić", "ana@pekara-maric.ba", "Pekara Marić"}, {"Emir Hodžić", "emir.hodzic@gmail.com", ""},
	{"Lejla Begić", "lejla@studioatlas.ba", "Studio Atlas"}, {"Marko Jurić", "marko.juric@outlook.com", "Jurić Transport"},
	{"Sara Kovač", "sara@kovac-law.hr", "Kovač & partneri"}, {"Daniel Weber", "d.weber@bergholz.de", "Bergholz GmbH"},
	{"Amra Delić", "amra.delic@yahoo.com", ""}, {"Nikola Petrović", "nikola@vinarija-petrovic.rs", "Vinarija Petrović"},
	{"Maja Novak", "maja.novak@gmail.com", "Yoga Prostor"}, {"Tarik Mujić", "tarik@mujic-dent.ba", "Ordinacija Mujić"},
	{"Ivana Babić", "ivana.babic@icloud.com", ""}, {"Jonas Lindqvist", "jonas@nordtrail.se", "Nordtrail"},
	{"Selma Hadžić", "selma.hadzic@gmail.com", "Kuća Hadžić"}, {"Petra Horvat", "petra@horvat-design.hr", "Horvat Design"},
	{"Haris Omerović", "haris.omerovic@gmail.com", ""}, {"Mia Schneider", "mia.schneider@posteo.de", ""},
	{"Adnan Kurtović", "adnan@kurtovic-auto.ba", "Auto Kurtović"}, {"Lana Perić", "lana.peric@gmail.com", ""},
	{"Edin Sarić", "edin.saric@hotmail.com", ""}, {"Nina Vuković", "nina@vukovic-events.ba", "Vuković Events"},
}

var demoBriefs = []struct {
	subject, kinds, message, budget string
	deadlineDays                    int
}{
	{"An online shop for our bakery", "An online shop", "We bake bread and cakes in Sarajevo and want people to order cakes for birthdays online, pick a day and pay when they collect. We have photos of everything.", "€2,000 – €5,000", 60},
	{"A booking site for our guest house", "A new website", "Four rooms by the river in Konjic. Today people book through Booking and we pay a lot of commission. We would like our own site where guests see free days and book directly.", "€5,000 – €15,000", 90},
	{"A new website for a law office", "A new website, Advice", "Our site is from 2014 and does not work on phones. We need something calm and trustworthy, in Croatian and English, with a contact form.", "€2,000 – €5,000", 45},
	{"An app for drivers' delivery notes", "A mobile app", "Our drivers write delivery notes on paper. We want them to take a photo, get a signature on the phone and have it in the office at once.", "€5,000 – €15,000", 120},
	{"Shop for a small winery", "An online shop, A new website", "We make about 20,000 bottles a year and want to sell to people in Serbia directly, with a wine club that sends six bottles every quarter.", "€5,000 – €15,000", 0},
	{"Schedule for a yoga studio", "Changes to something I already have", "We have a WordPress site. We need a class schedule people can sign up for, with a limit of 14 per class and a waiting list.", "Under €2,000", 30},
	{"Patient booking for a dental office", "A new website", "Patients call all day to book check-ups. A page where they can ask for a time would help us a lot. Must be simple for older people.", "€2,000 – €5,000", 75},
	{"Trail maps app for hikers", "A mobile app, Advice", "We run guided hikes in Sweden and the Balkans. We are thinking about an app with offline maps of our routes. Not sure where to start — is it worth it?", "Not sure yet", 0},
	{"Our family house as a holiday rental", "A new website", "We are renovating my grandparents' house in Mostar and want a simple site with photos, a calendar of free days and how to reach us.", "Under €2,000", 50},
	{"Faster website for a car service", "Changes to something I already have", "Our site is very slow and Google shows it badly. People want to see prices for services and book a slot.", "€2,000 – €5,000", 21},
	{"Event planning portfolio", "A new website", "I plan weddings and company events. I want a beautiful portfolio with galleries and a form for couples to tell me about their day.", "€2,000 – €5,000", 40},
	{"Ordering system for a catering company", "An online shop, Advice", "Companies order lunch from us every day by phone and email. We want a menu for the week where they order by 10 am.", "More than €15,000", 100},
}

var demoFeedbackGood = []string{"Everything was clear and on time.", "Very patient with all our questions.", "The new site is so much faster — customers noticed.",
	"Great communication, always knew what was happening.", "Loved the design, exactly what we wanted.", "Quick answers, even on weekends.",
	"Explained the technical things in a way we understood.", "The shop works perfectly on phones.", "", "", "Friendly and professional."}
var demoFeedbackBetter = []string{"Some weeks were quiet; a short update every Friday would help.", "The first draft took a bit longer than we expected.",
	"Nothing really.", "More examples at the start would make choosing easier.", "", "", "The invoice could list the hours per part of the work.",
	"I would have liked a short video on how to add products.", "", "Maybe a little cheaper for small businesses :)"}

func demoTime(rng *rand.Rand, daysAgo int) string {
	t := time.Now().AddDate(0, 0, -daysAgo)
	t = time.Date(t.Year(), t.Month(), t.Day(), 8+rng.Intn(14), rng.Intn(60), rng.Intn(60), 0, time.Local)
	if t.After(time.Now()) {
		t = time.Now().Add(-time.Duration(30+rng.Intn(200)) * time.Minute)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx)
	if fs, _ := s.store.ListForms(ctx, uid, ""); len(fs) > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	mk := func(key, title, status string, opened int, tweak func(*store.Form)) (*store.Form, error) {
		t, _ := templateByKey(key)
		f := &store.Form{UserID: uid, Title: title, Description: t.Description, Emoji: t.Emoji, Status: status, Fields: append([]store.Field(nil), t.Fields...),
			Language: "en", Thanks: t.Thanks, SubmitLabel: t.Submit, Branding: true, Notify: true, Auto: t.Auto, Template: key,
			Accent: "#5f3dc4", CreatedAt: demoTime(rng, opened+3)}
		f.UpdatedAt = f.CreatedAt
		if status != store.FormDraft {
			f.OpenedAt = demoTime(rng, opened)
		}
		if tweak != nil {
			tweak(f)
		}
		f.Handle = s.store.NewHandle(ctx, f.Title)
		return f, s.store.CreateForm(ctx, f)
	}
	add := func(f *store.Form, daysAgo int, answers map[string]any, status string, starred bool, note string) *store.Response {
		r := &store.Response{UserID: uid, FormID: f.ID, Answers: answers, Status: status, Starred: starred, Note: note, CreatedAt: demoTime(rng, daysAgo), IPHash: "demo"}
		describe(f, r)
		if err := s.store.AddResponse(ctx, r); err != nil {
			s.log.Warn("demo answer", "err", err)
		}
		return r
	}
	views := func(f *store.Form, days, per int) {
		for d := days; d >= 0; d-- {
			n := rng.Intn(per + 1)
			day := time.Now().AddDate(0, 0, -d).Format("2006-01-02")
			_, _ = s.store.DB.ExecContext(ctx, `INSERT INTO views (user_id, form_id, day, count) VALUES (?,?,?,?)
				ON CONFLICT(form_id, day) DO UPDATE SET count = count + excluded.count`, uid, f.ID, day, n)
		}
	}

	// 1. client briefs
	brief, err := mk("project", "Start a project with me", store.FormOpen, 120, func(f *store.Form) { f.Emoji = "🚀" })
	if err != nil {
		return err
	}
	views(brief, 120, 9)
	heard := []string{"A recommendation", "A search", "Social media", "A recommendation", "A recommendation"}
	for i, b := range demoBriefs {
		p := demoPeople[i]
		days := 118 - i*10 + rng.Intn(4)
		if i >= len(demoBriefs)-3 {
			days = (len(demoBriefs) - 1 - i) * 2
		}
		ans := map[string]any{"name": p.name, "email": p.email, "subject": b.subject, "message": b.message, "budget": b.budget,
			"kind": strings.Split(b.kinds, ", "), "heard": heard[i%len(heard)]}
		if p.company != "" {
			ans["company"] = p.company
			ans["website"] = "https://" + strings.Split(p.email, "@")[1]
		}
		if b.deadlineDays > 0 {
			ans["deadline"] = time.Now().AddDate(0, 0, b.deadlineDays-days/2).Format("2006-01-02")
		}
		status := "done"
		switch {
		case i >= len(demoBriefs)-3:
			status = "new"
		case i >= len(demoBriefs)-5:
			status = "seen"
		}
		note := ""
		if i == 1 {
			note = "Call on Monday — they want it before the summer season."
		}
		r := add(brief, days, ans, status, i == 1 || i == 9, note)
		if status == "done" && r.ID > 0 {
			for _, m := range []store.Made{{Target: "contact", App: "contacts", Title: p.name}, {Target: "project", App: "projects", Title: firstNonEmpty(p.company, p.name) + " — " + b.subject},
				{Target: "task", App: "tasks", Title: "Get back to " + p.name}} {
				m.ResponseID, m.Ref = r.ID, strconv.Itoa(100+i)
				_ = s.store.AddMade(ctx, uid, &m)
			}
		} else if r.ID > 0 && status != "new" {
			_ = s.store.AddMade(ctx, uid, &store.Made{ResponseID: r.ID, Target: "contact", App: "contacts", Title: p.name, Auto: true, Ref: strconv.Itoa(200 + i)})
		}
	}

	// 2. feedback after a project
	fb, err := mk("feedback", "How did we do?", store.FormOpen, 90, nil)
	if err != nil {
		return err
	}
	views(fb, 90, 3)
	again := []string{"Yes", "Yes", "Yes", "Maybe", "Yes", "No"}
	for i := 0; i < 26; i++ {
		rating := 5
		switch x := rng.Intn(10); {
		case x < 2:
			rating = 3
		case x < 5:
			rating = 4
		case x == 9 && i%7 == 0:
			rating = 2
		}
		ans := map[string]any{"rating": strconv.Itoa(rating), "again": again[rng.Intn(len(again))]}
		if g := demoFeedbackGood[rng.Intn(len(demoFeedbackGood))]; g != "" {
			ans["good"] = g
		}
		if b := demoFeedbackBetter[rng.Intn(len(demoFeedbackBetter))]; b != "" {
			ans["better"] = b
		}
		if rng.Intn(3) == 0 {
			ans["email"] = demoPeople[rng.Intn(len(demoPeople))].email
		}
		status := "seen"
		if i > 23 {
			status = "new"
		}
		add(fb, 88-i*3-rng.Intn(3), ans, status, false, "")
	}

	// 3. a workshop with a limit on places
	ws, err := mk("event", "Workshop: build your first website", store.FormOpen, 20, func(f *store.Form) {
		f.Description = "A Saturday morning for beginners: by noon you have your own page on the internet. Bring a laptop; coffee is on us.\n\nSaturday 28 November, 10:00–13:00, Coworking Hub, Sarajevo."
		f.MaxAnswers = 16
		f.Emoji = "🧑‍💻"
		f.CloseAt = time.Now().AddDate(0, 0, 20).Format("2006-01-02")
	})
	if err != nil {
		return err
	}
	views(ws, 20, 12)
	food := [][]string{{"No preferences"}, {"Vegetarian"}, {"No preferences"}, {"Vegan"}, {"Gluten-free", "Vegetarian"}}
	for i := 0; i < 11; i++ {
		p := demoPeople[(i*3+5)%len(demoPeople)]
		ans := map[string]any{"name": p.name, "email": p.email, "people": strconv.Itoa(1 + rng.Intn(2)), "food": food[rng.Intn(len(food))], "consent": "Yes"}
		if i%4 == 0 {
			ans["phone"] = fmt.Sprintf("+387 6%d %03d %03d", rng.Intn(3)+1, rng.Intn(1000), rng.Intn(1000))
		}
		status := "seen"
		if i > 8 {
			status = "new"
		}
		add(ws, 19-i*2, ans, status, false, "")
	}

	// 4. a summer party that is over
	bbq, err := mk("rsvp", "Summer BBQ at ours — will you come?", store.FormClosed, 150, func(f *store.Form) {
		f.Description = "Saturday 19 July from 17:00 in the garden. Bring someone if you like!"
		f.Emoji = "🍖"
		f.CloseAt = time.Now().AddDate(0, 0, -125).Format("2006-01-02")
	})
	if err != nil {
		return err
	}
	views(bbq, 150, 4)
	coming := []string{"Yes, gladly", "Yes, gladly", "Yes, gladly", "Maybe", "Sadly not"}
	for i := 0; i < 15; i++ {
		p := demoPeople[(i*7+2)%len(demoPeople)]
		ans := map[string]any{"name": p.name, "coming": coming[rng.Intn(len(coming))], "people": strconv.Itoa(1 + rng.Intn(3))}
		if i%5 == 0 {
			ans["food"] = "No pork, please"
		}
		add(bbq, 148-i*2, ans, "done", false, "")
	}

	// 5. contact form on the website
	ct, err := mk("contact", "Get in touch", store.FormOpen, 100, func(f *store.Form) { f.Auto = store.Auto{} })
	if err != nil {
		return err
	}
	views(ct, 100, 5)
	msgs := []struct{ subject, message string }{
		{"Do you also do logos?", "Hi! I saw the bakery website you made and love it. Do you also design logos, or only websites?"},
		{"Speaking at our meetup", "We run a small web meetup in Tuzla every month. Would you like to give a 20-minute talk about building sites for small businesses?"},
		{"Invoice question", "Hello, could you send the invoice for October again with our new address? Thank you!"},
		{"Student internship", "I am in my last year of computer science and would love to learn from you this summer. I can send my small projects."},
		{"Partnership", "We are a marketing agency in Zagreb looking for a developer we can send website projects to. Can we talk?"},
	}
	for i, m := range msgs {
		p := demoPeople[(i*5+3)%len(demoPeople)]
		status := "done"
		if i >= 3 {
			status = "new"
		}
		add(ct, 90-i*21, map[string]any{"name": p.name, "email": p.email, "subject": m.subject, "message": m.message}, status, i == 4, "")
	}

	// 6. a draft being made
	if _, err := mk("appointment", "Ask for a call", store.FormDraft, 1, func(f *store.Form) {
		f.Description = "Pick a day and a time that suits you for a free 30-minute call about your project."
		f.Emoji = "📞"
		f.Auto = store.Auto{Event: true, Contact: true}
	}); err != nil {
		return err
	}
	s.log.Info("demo data ready")
	return nil
}

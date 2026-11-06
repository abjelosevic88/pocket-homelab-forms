package server

import (
	"net/http"

	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// ---- ready-made forms ----
//
// A new form can start from one of these: the questions, which answer is
// the person's name or email (roles), what happens by itself and the
// words after sending. Everything can be changed afterwards.

type formTemplate struct {
	Key         string        `json:"key"`
	Name        string        `json:"name"`
	Emoji       string        `json:"emoji"`
	Blurb       string        `json:"blurb"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Thanks      string        `json:"thanks"`
	Submit      string        `json:"submit_label"`
	Auto        store.Auto    `json:"auto"`
	Fields      []store.Field `json:"fields"`
	Links       []string      `json:"links"` // the apps it works best with
}

func fld(id, typ, label string, opts ...func(*store.Field)) store.Field {
	f := store.Field{ID: id, Type: typ, Label: label}
	for _, o := range opts {
		o(&f)
	}
	return f
}

func req(f *store.Field) { f.Required = true }
func role(r string) func(*store.Field) {
	return func(f *store.Field) { f.Role = r }
}
func help(h string) func(*store.Field) {
	return func(f *store.Field) { f.Help = h }
}
func ph(p string) func(*store.Field) {
	return func(f *store.Field) { f.Placeholder = p }
}
func opts(o ...string) func(*store.Field) {
	return func(f *store.Field) { f.Options = o }
}
func other(f *store.Field) { f.Other = true }

var formTemplates = []formTemplate{
	{Key: "project", Name: "Start a project with me", Emoji: "🚀", Links: []string{"contacts", "projects", "tasks", "mail", "sign"},
		Blurb:       "A brief from a new client: who they are, what they need, the budget and the deadline. Accept it and the contact, the project and a follow-up task are made for you.",
		Title:       "Start a project with me",
		Description: "Tell me a little about what you have in mind. It takes about three minutes, and I answer every brief within two working days.",
		Thanks:      "Thank you! I have your brief and will get back to you within two working days.",
		Submit:      "Send my brief",
		Auto:        store.Auto{Contact: true, Task: true, TaskDays: 2},
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("email", "email", "Email", req, role("email")),
			fld("company", "text", "Company or organisation", role("company"), help("Leave it empty if it is just you.")),
			fld("website", "url", "Your website, if you have one", role("website"), ph("https://")),
			fld("about", "section", "The project", help("A few words are enough; we will talk about the details.")),
			fld("subject", "text", "In one line: what is it?", req, role("subject"), ph("A booking site for our guest house")),
			fld("kind", "checkboxes", "What do you need?", opts("A new website", "An online shop", "A mobile app", "Changes to something I already have", "Advice"), other),
			fld("message", "long", "Tell me more", req, role("message"), help("Who is it for, what should it do, what do you have already?")),
			fld("budget", "dropdown", "Budget", role("budget"), opts("Under €2,000", "€2,000 – €5,000", "€5,000 – €15,000", "More than €15,000", "Not sure yet")),
			fld("deadline", "date", "When do you need it?", role("deadline"), help("Leave it empty if there is no fixed date.")),
			fld("heard", "choice", "How did you hear about me?", opts("A recommendation", "A search", "Social media"), other),
		}},
	{Key: "contact", Name: "Contact me", Emoji: "✉️", Links: []string{"contacts", "tasks", "mail"},
		Blurb:  "The simple one: name, email and a message. Put the link on your website or in your email signature.",
		Title:  "Get in touch",
		Thanks: "Thanks for your message! I will answer soon.",
		Submit: "Send",
		Auto:   store.Auto{Task: true, TaskDays: 1},
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("email", "email", "Email", req, role("email")),
			fld("subject", "text", "What is it about?", role("subject")),
			fld("message", "long", "Message", req, role("message")),
		}},
	{Key: "quote", Name: "Ask for a quote", Emoji: "🧾", Links: []string{"contacts", "tasks", "calendar", "mail"},
		Blurb:  "For a trade or a service: what needs doing, where, when, and how to reach the person.",
		Title:  "Ask for a quote",
		Thanks: "Thank you! You will have a quote from me within a few days.",
		Submit: "Ask for a quote",
		Auto:   store.Auto{Contact: true, Task: true, TaskDays: 2},
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("phone", "phone", "Phone", req, role("phone")),
			fld("email", "email", "Email", role("email")),
			fld("subject", "text", "What needs doing?", req, role("subject"), ph("Paint two rooms")),
			fld("message", "long", "Details", role("message"), help("Sizes, materials, anything that helps.")),
			fld("where", "text", "Where?", help("The address or the town.")),
			fld("date", "date", "When would suit you?", role("date")),
			fld("budget", "text", "Budget, if you have one", role("budget")),
		}},
	{Key: "appointment", Name: "Ask for an appointment", Emoji: "📅", Links: []string{"contacts", "calendar", "mail"},
		Blurb:  "People ask for a day and a time; with one click it is in your calendar (or let it go there by itself).",
		Title:  "Ask for an appointment",
		Thanks: "Thank you! I will confirm the time by email.",
		Submit: "Send my request",
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("email", "email", "Email", req, role("email")),
			fld("phone", "phone", "Phone", role("phone")),
			fld("date", "date", "Which day?", req, role("date")),
			fld("time", "time", "What time?", help("We will confirm or suggest another time.")),
			fld("message", "long", "What is it about?", role("message")),
		}},
	{Key: "feedback", Name: "Feedback", Emoji: "💬", Links: []string{"tasks", "notes"},
		Blurb:  "How did it go? A rating, what was good and what could be better. Pocket AI sums up the answers for you.",
		Title:  "How did we do?",
		Thanks: "Thank you for taking the time — it really helps.",
		Fields: []store.Field{
			fld("rating", "rating", "How happy are you overall?", req),
			fld("good", "long", "What did you like?"),
			fld("better", "long", "What could be better?", role("message")),
			fld("again", "choice", "Would you recommend us to a friend?", opts("Yes", "Maybe", "No")),
			fld("email", "email", "Your email, if we may answer you", role("email")),
		}},
	{Key: "event", Name: "Event sign-up", Emoji: "🎟️", Links: []string{"contacts", "calendar", "mail"},
		Blurb:  "Sign-ups for a workshop, a class or a trip: names, how many, food, and a limit on places.",
		Title:  "Sign up",
		Thanks: "You are on the list! We will send the details a few days before.",
		Submit: "Sign me up",
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("email", "email", "Email", req, role("email")),
			fld("phone", "phone", "Phone", role("phone")),
			fld("people", "number", "How many people?", req, ph("1")),
			fld("food", "checkboxes", "Food", opts("Vegetarian", "Vegan", "Gluten-free", "No preferences"), other),
			fld("notes", "long", "Anything we should know?", role("message")),
			fld("consent", "consent", "I agree that you keep my details for this event.", req),
		}},
	{Key: "rsvp", Name: "RSVP", Emoji: "🥂", Links: []string{"contacts"},
		Blurb:  "Are you coming? For a party, a wedding or a family lunch — with who is coming and what they eat.",
		Title:  "Will you come?",
		Thanks: "Thank you for letting us know!",
		Submit: "Send",
		Fields: []store.Field{
			fld("name", "text", "Your name", req, role("name")),
			fld("coming", "choice", "Are you coming?", req, opts("Yes, gladly", "Maybe", "Sadly not")),
			fld("people", "number", "How many of you, with you?", ph("1")),
			fld("food", "text", "Anything you do not eat?"),
			fld("message", "long", "A message for us", role("message")),
		}},
	{Key: "survey", Name: "Short survey", Emoji: "📊", Links: []string{"notes"},
		Blurb:  "A few multiple-choice questions and one open one. The answers are counted for you as they come in.",
		Title:  "A short survey",
		Thanks: "Thank you for your answers!",
		Fields: []store.Field{
			fld("q1", "choice", "How often do you …?", req, opts("Every day", "Every week", "Every month", "Rarely")),
			fld("q2", "checkboxes", "Which of these do you use?", opts("Option one", "Option two", "Option three"), other),
			fld("q3", "rating", "How much do you like it?"),
			fld("q4", "yesno", "Would you like to hear about the results?"),
			fld("q5", "long", "Anything else?", role("message")),
		}},
	{Key: "blank", Name: "Blank form", Emoji: "📝", Blurb: "Start with one question and build it your way.",
		Title: "Untitled form", Thanks: "", Links: []string{},
		Fields: []store.Field{fld("q1", "text", "Your name", role("name"))}},
}

func templateByKey(k string) (formTemplate, bool) {
	for _, t := range formTemplates {
		if t.Key == k {
			return t, true
		}
	}
	return formTemplate{}, false
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, formTemplates)
}

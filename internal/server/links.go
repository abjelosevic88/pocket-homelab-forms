package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-forms/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Forms as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "forms:read", Name: "See your forms and answers", Description: "Read your forms, the answers people sent and how many opened each form — for your journal, dashboards, search, a contact's page and Pocket AI."},
	{Scope: "forms:write", Name: "Make forms for you", Description: "Start a form as a draft for you to check (\"make me a sign-up form for Saturday\"), and open or close it when you ask. It may change only the forms that app made itself."},
}

// portable is what Pocket Move may copy out and bring in for a person
// (docs/move.md in pocket-homelab-core); Register adds move:export/import.
var portable = &core.Portable{
	Exports: []core.PortableExport{
		{Path: "/api/v1/export.json", File: "forms.json", Kind: core.KindPocket, Title: "Every form you made, with its questions and every answer it got"},
	},
	Imports: []core.PortableImport{
		{Path: "/api/v1/import", Kind: core.KindPocket, Form: "file", Title: "Adds the forms and their answers from another box; what you already have is left alone"},
		{Path: "/api/v1/import", Kind: core.KindCSV, Form: "file", Title: "Adds the answers of a form from Google Forms, Typeform, Tally, Microsoft Forms or Jotform as CSV; a second run adds nothing"},
	},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(forms|responses|summary|search|templates|day/\d{4}-\d{2}-\d{2})$`), "forms:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(forms|responses)/\d+$`), "forms:read"},
	{"GET", regexp.MustCompile(`^/api/v1/forms/\d+/(stats|export\.csv)$`), "forms:read"},
	{"POST", regexp.MustCompile(`^/api/v1/forms$`), "forms:write"},
	{"PUT", regexp.MustCompile(`^/api/v1/forms/\d+$`), "forms:write"},
	{"POST", regexp.MustCompile(`^/api/v1/forms/\d+/(open|close)$`), "forms:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	if portable.LinkAllows(scopes, r) {
		return true
	}
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.Audience != "forms" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}

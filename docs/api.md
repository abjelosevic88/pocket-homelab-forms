# API

JSON at `/api/v1`, with a session cookie or `Authorization: Bearer <token>` (Settings → API & shortcuts). Everything is per person: another person's ids are 404.

## Forms
- `GET /forms` — your forms with `responses`, `new`, `views`, `taking` and `why` (draft | closed | date | full), `url`.
- `POST /forms` — `{"template":"project"}` (see `GET /templates`) or `{"title","fields":[…]}`; starts as a draft.
- `GET|PUT|DELETE /forms/{id}` — PUT takes any of `title, description, emoji, handle, fields, accent, language, submit_label, thanks, redirect_url, close_at, max_answers, one_per_email, branding, notify, send_copy, auto`.
- `POST /forms/{id}/open`, `/close`, `/duplicate`; `GET /forms/{id}/stats`, `/forms/{id}/export.csv`; `GET /forms/check-handle?handle=`.
- A field: `{"id","type","label","help","required","options","other","placeholder","role"}`; types `text long email phone number url date time choice checkboxes dropdown yesno rating consent section`; roles `name email phone company website subject message budget deadline date`.

## Answers
- `GET /responses?form=&status=new|seen|done|spam|open&starred=1&q=&email=&day=YYYY-MM-DD&limit=&offset=` → `{items,total,new}`.
- `GET /responses/{id}` → `{response, items (each question with its answer), form, prev, next}` (marks it read; `?peek=1` does not).
- `PUT /responses/{id}` — `status`, `starred`, `note`; `DELETE /responses/{id}`; `POST /responses/bulk` `{"ids":[…],"status":"done"}` or `{"ids":[…],"delete":true}`.
- `POST /responses/{id}/make` — `{"target":"contact|project|task|event|note|email|signing", …}`; `POST /responses/{id}/accept` — `{"contact","project","task","team_id","name","due"}`.
- `GET /apps`, `GET /apps/options/teams|templates`.

## For the rest of the box
- `GET /summary` — new answers, forms taking answers, this week, the latest answers (Homepage, Pocket AI).
- `GET /day/YYYY-MM-DD` — the answers that came in that day (Pocket Day).
- `GET /search?q=` — forms and answers (Pocket Search).

## AI
- `POST /ai/draft` `{"description","language"}`, `POST /forms/{id}/ai/summary`, `POST /responses/{id}/ai/reply` `{"instruction"}`.

## Data
- `GET /export.json`, `POST /import` (multipart `file`: a Pocket Forms export or a CSV of answers), `GET /backup` (admins).

## Public
- `GET|POST /f/{address}` — the form; `GET /f/{address}/thanks`. `?embed=1` without the page around it; `?<field id or role>=…` fills an answer in.

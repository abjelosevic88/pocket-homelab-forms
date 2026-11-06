CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- ---------- Pocket Forms: everything below is per person ----------

-- A form: its questions (fields, JSON), its public address (handle;
-- box-wide unique, people open /f/<handle>), whether it takes answers and
-- what happens when one arrives.
CREATE TABLE forms (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  handle        TEXT NOT NULL UNIQUE,
  title         TEXT NOT NULL DEFAULT '',
  description   TEXT NOT NULL DEFAULT '',
  emoji         TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'draft',          -- draft | open | closed
  fields        TEXT NOT NULL DEFAULT '[]',             -- [{id,type,label,help,required,options,other,placeholder,role}]
  accent        TEXT NOT NULL DEFAULT '',
  language      TEXT NOT NULL DEFAULT 'en',
  submit_label  TEXT NOT NULL DEFAULT '',
  thanks        TEXT NOT NULL DEFAULT '',               -- shown after sending
  redirect_url  TEXT NOT NULL DEFAULT '',               -- or go here instead
  close_at      TEXT NOT NULL DEFAULT '',               -- YYYY-MM-DD: takes answers until the end of that day
  max_answers   INTEGER NOT NULL DEFAULT 0,             -- 0 = no limit
  one_per_email INTEGER NOT NULL DEFAULT 0,
  branding      INTEGER NOT NULL DEFAULT 1,             -- "Made with Pocket Forms" under the form
  notify        INTEGER NOT NULL DEFAULT 1,             -- a push for each answer
  send_copy     INTEGER NOT NULL DEFAULT 0,             -- email the person a copy (Pocket Mail)
  auto          TEXT NOT NULL DEFAULT '{}',             -- what is made by itself: {"contact","task","task_days","event","note"}
  template      TEXT NOT NULL DEFAULT '',
  source        TEXT NOT NULL DEFAULT '',               -- the app that made it through a Core link
  opened_at     TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX forms_user ON forms(user_id, status, updated_at);

-- One set of answers someone sent. user_id is the form's owner.
CREATE TABLE responses (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  form_id    INTEGER NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  number     INTEGER NOT NULL,                          -- #1, #2… per form
  answers    TEXT NOT NULL DEFAULT '{}',                -- {field id: "text" | ["a","b"]}
  name       TEXT NOT NULL DEFAULT '',                  -- from the fields used as name / email
  email      TEXT NOT NULL DEFAULT '',
  preview    TEXT NOT NULL DEFAULT '',                  -- a line for lists
  status     TEXT NOT NULL DEFAULT 'new',               -- new | seen | done | spam
  starred    INTEGER NOT NULL DEFAULT 0,
  note       TEXT NOT NULL DEFAULT '',                  -- the owner's own note
  origin     TEXT NOT NULL DEFAULT 'web',               -- web | import | app
  ip_hash    TEXT NOT NULL DEFAULT '',
  client_key TEXT NOT NULL DEFAULT '',                  -- imports: the same row is never added twice
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX responses_user ON responses(user_id, created_at);
CREATE INDEX responses_form ON responses(form_id, created_at);
CREATE INDEX responses_email ON responses(user_id, email);
CREATE INDEX responses_ip ON responses(ip_hash, created_at);
CREATE UNIQUE INDEX responses_number ON responses(form_id, number);
CREATE UNIQUE INDEX responses_client ON responses(user_id, form_id, client_key) WHERE client_key <> '';

-- What was made from an answer in the other Pocket apps (a contact, a
-- project, a task…), by hand or by itself, with a link to it — or why not.
CREATE TABLE made (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  response_id INTEGER NOT NULL REFERENCES responses(id) ON DELETE CASCADE,
  target      TEXT NOT NULL,                            -- contact | project | task | event | note | email | signing
  app         TEXT NOT NULL DEFAULT '',
  ref         TEXT NOT NULL DEFAULT '',
  title       TEXT NOT NULL DEFAULT '',
  url         TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT '',
  auto        INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX made_user ON made(user_id, response_id);

-- How many people opened a form, per day (no cookies, no addresses kept).
CREATE TABLE views (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  form_id INTEGER NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  day     TEXT NOT NULL,
  count   INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (form_id, day)
);
CREATE INDEX views_user ON views(user_id, day);

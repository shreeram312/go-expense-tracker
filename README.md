# Expense Tracker (Go)

My first Go backend project, built to learn Go properly by building something real.
Users sign up, log in, record their expenses in categories, filter them by month and
see how much they spent per category.

- **The backend is mine:** I write every line of Go myself.
- **The frontend is AI's job:** it gets vibe-coded later from the API contract below.
- **The goal is learning, not shipping.** This won't be production-grade at first, and that's fine.

What I knew when I started: structs, interfaces, channels, mutexes.
The main things I want out of it: **auth** and **CRUD**.

---

## How I learn each phase

Every phase follows the same four steps. I don't move on until all four are done.

1. **Theory:** read the concepts and the **key questions with their answers** before writing any code.
2. **Build:** write the code myself. AI gives hints, not solutions, unless I ask.
3. **Review:** get the code reviewed and fix what comes up.
4. **Recap:** reread the answered questions now that I've built it; they make more sense after the code.
   Optional: jot anything that surprised me in `notes/phase-N.md`.

Each phase also has a **"break it on purpose"** step, because seeing the error teaches
more than reading about it.

No hurry. One phase might take a few days or a week, and that's fine.

---

## Progress

- [ ] Phase 0: Setup and project structure
- [ ] Phase 1: How HTTP works, in-memory CRUD
- [ ] Phase 2: A real database (SQLite)
- [ ] Phase 3: Users: signup and login
- [ ] Phase 4: Protecting routes with JWT
- [ ] Phase 5: Ownership: users only see their own data
- [ ] Phase 6: Categories, filters and the monthly summary
- [ ] Phase 7: Tests
- [ ] Phase 8 (stretch): Polish, sessions and Docker

---

## Project structure (where it ends up)

```
expense-tracker-go/
├── README.md                  ← this file
├── notes/                     ← my answers to each phase's checkpoint questions
├── backend/
│   ├── go.mod
│   ├── cmd/
│   │   └── api/
│   │       └── main.go        ← starts the server, wires everything together
│   ├── internal/
│   │   ├── httpapi/           ← handlers, routes, middleware, JSON helpers   (Phase 1)
│   │   ├── store/             ← database code: users, expenses, categories  (Phase 2)
│   │   ├── auth/              ← password hashing + JWT                       (Phases 3–4)
│   │   └── config/            ← settings from env vars                       (Phase 8)
│   └── migrations/            ← .sql files that create the tables           (Phase 2)
└── frontend/                  ← built by AI later
```

**Rule: only create a folder in the phase that needs it.** No empty folders on day one.

Each layer only talks to the one below it:

```
HTTP request → httpapi (handlers) → store (SQL) → SQLite
                    ↑
                  auth (hashing, tokens)
```

---

## Data model

```
users       (id, email, password_hash, created_at)
categories  (id, user_id, name)
expenses    (id, user_id, category_id, amount_paise, note, spent_on, created_at)
```

Money is stored as **whole paise in an `int64`**, never `float64`.
₹249.50 is stored as `24950`. (Floats can't represent amounts like 0.10 exactly.)

---

## Dependency rule

Use the standard library unless there's a good reason not to. The only outside packages are:

| Package | Why | Added in |
|---|---|---|
| A SQLite driver (`modernc.org/sqlite`, pure Go, no C compiler needed) | `database/sql` needs a driver | Phase 2 |
| `golang.org/x/crypto/bcrypt` | password hashing | Phase 3 |
| `github.com/golang-jwt/jwt/v5` | signing and verifying tokens | Phase 4 |

No web framework (Gin, Echo, Fiber). Go 1.22+ `net/http` routing does everything this project needs,
and learning the standard library first makes every framework easy later.

---

## Phase 0: Setup and project structure

**Goal:** a Go module that compiles and runs, and an understanding of how Go organises code.

**Theory**
- Module vs package: `go.mod` defines the module; each folder is one package
- `package main` + `func main()` = a runnable program; the `cmd/<name>/` convention
- Exported (`Capital`) vs unexported (`lower`) names
- `internal/` and how the compiler enforces it
- Commands: `go mod init`, `go run`, `go build`, `gofmt`, `go vet`, `./...`

**Build**
1. In `backend/`, run `go mod init expense-tracker-go`
2. `cmd/api/main.go` prints `starting expense tracker...` (`go run ./cmd/api`)
3. `internal/httpapi/hello.go` has `Greeting() string`; call it from `main.go`

**Break it on purpose**
- Rename `Greeting` → `greeting` and read the error
- Add an unused import or variable
- Mess up the formatting, then run `gofmt -w .`

**Key questions (answered)**

1. **What's the difference between a module and a package?**
   A *module* is the whole project: it's defined by `go.mod`, which holds its name (the module path) and
   its dependencies. A *package* is one folder of `.go` files that all start with the same `package xyz`
   line. A module contains many packages; you import a package as `<module path>/<folder>`.

2. **Why does `greeting()` fail when called from `main`?**
   A lowercase first letter means *unexported*: only code inside `package httpapi` can see it.
   `main` is a different package, so the compiler stops with an error like
   `undefined: httpapi.greeting`. Capitalise it (`Greeting`) to export it.

3. **What does `internal/` protect against?**
   Other projects importing your code. Anything under `internal/` can only be imported by code inside
   the folder that contains `internal/`. That means you can change or delete it freely without breaking
   anyone else, and the compiler enforces it.

4. **Why are unused imports a compile error instead of a warning?**
   Go has no warnings: something is either a problem (error) or fine. Unused imports slow compilation
   and usually mean leftover or half-finished code, so Go keeps code clean by refusing to build.
   If you really need one for its side effects, write `import _ "pkg"`.

**Reading:** [Tour of Go: Basics](https://go.dev/tour/basics/1) · [Organizing a Go module](https://go.dev/doc/modules/layout)

---

## Phase 1: How HTTP works, in-memory CRUD

**Goal:** full CRUD for expenses over HTTP, with the data kept in a Go map (no database yet).

**Theory**
- What an HTTP request and response contain: method, path, headers, body, status code
- REST conventions: `GET` reads, `POST` creates, `PUT` updates, `DELETE` removes
- Status codes worth knowing: 200, 201, 204, 400, 404, 405, 500
- The `http.Handler` interface and `http.HandlerFunc`, the core idea of Go web servers
- `http.ServeMux` routing with method and path patterns: `mux.HandleFunc("GET /expenses/{id}", ...)`, `r.PathValue("id")`
- JSON: `encoding/json`, struct tags (`json:"amount_paise"`), why fields must be exported
- Every request runs in its own goroutine, so the shared map **needs a mutex** (`sync.RWMutex`)

**Build**
| Method | Path | Does |
|---|---|---|
| `GET` | `/health` | returns `{"status":"ok"}` |
| `POST` | `/expenses` | creates an expense → 201 |
| `GET` | `/expenses` | lists all expenses |
| `GET` | `/expenses/{id}` | gets one expense, or 404 |
| `PUT` | `/expenses/{id}` | updates an expense |
| `DELETE` | `/expenses/{id}` | deletes it → 204 |

- Validation: amount must be > 0, note under 200 characters, bad JSON → 400
- Helpers `writeJSON(w, status, data)` and `writeError(w, status, msg)`, so handlers stay short
- Test with `curl` (or a REST client)

**Break it on purpose**
- Remove the mutex and hit the server with many requests at once while running `go run -race`
- Make a struct field lowercase and see it vanish from the JSON
- Send invalid JSON and make sure you get a 400, not a crash

**Key questions (answered)**

1. **What does the `http.Handler` interface look like, and why is it so powerful?**
   ```go
   type Handler interface {
       ServeHTTP(w http.ResponseWriter, r *http.Request)
   }
   ```
   It has one method, so almost anything can be a handler: a plain function (via `http.HandlerFunc`),
   a struct that holds dependencies like a store, the router (`ServeMux` is itself a Handler), or
   middleware that wraps another handler. Because everything shares one shape, pieces snap together.

2. **Why do struct fields need to be exported for `encoding/json`?**
   `encoding/json` is a different package and reads your struct using reflection, which can only see
   exported fields. Lowercase fields are **silently skipped**: no error, just missing from the JSON.
   Use struct tags to control the JSON name: `AmountPaise int64 \`json:"amount_paise"\``.

3. **Why does the map need a mutex even though you never started a goroutine?**
   `net/http` starts one goroutine per incoming request. Two requests at the same time means two
   goroutines touching the same map, which is a data race, and Go maps crash with
   `fatal error: concurrent map writes`. `sync.RWMutex` allows many readers at once (`RLock`) or a single
   writer (`Lock`).

4. **When do you return 201 vs 200 vs 204?**
   - **201 Created:** a `POST` created something new (return the new object).
   - **200 OK:** a successful read or update that returns a body.
   - **204 No Content:** success with nothing to send back, typical for `DELETE`.

**Reading:** [Go by Example: HTTP server](https://gobyexample.com/http-server) · [Go by Example: JSON](https://gobyexample.com/json) · [net/http routing patterns](https://go.dev/blog/routing-enhancements)

---

## Phase 2: A real database (SQLite)

**Goal:** expenses survive a server restart. Swap the map for SQLite **without touching the handlers**.

**Theory**
- SQL basics: `CREATE TABLE`, `INSERT`, `SELECT`, `UPDATE`, `DELETE`, `WHERE`, `ORDER BY`
- Primary keys and `AUTOINCREMENT`; storing dates
- `database/sql`: `sql.Open`, `DB.QueryContext`, `QueryRowContext`, `ExecContext`, `rows.Scan`, `defer rows.Close()`
- **Placeholders (`?`) vs string building, and why string building is a SQL injection hole**
- `sql.ErrNoRows` → 404
- Migrations: SQL files that create and change tables, run in order
- **Interfaces in practice:** define an `ExpenseStore` interface; the map version and the SQLite version
  both satisfy it, and `main.go` picks which one to use
- Files (the first time you need them): reading `.sql` files with `os.ReadFile`, or `embed`

**Build**
- `migrations/001_create_expenses.sql`
- `internal/store/`: an `ExpenseStore` interface plus a SQLite implementation
- Run migrations at startup
- Handlers depend on the interface, not on SQLite

**Break it on purpose**
- Build a query with `fmt.Sprintf` and inject `1 OR 1=1`; then fix it with `?`
- Forget `rows.Close()` and think through what leaks
- Ask for an id that doesn't exist and check that you get a 404, not a 500

**Key questions (answered)**

1. **Why are placeholders safe and string concatenation not?**
   With `?`, the SQL text and the values travel to the database separately. The database parses the
   query first, so a value is always treated as *data*, never as SQL. With `fmt.Sprintf`, input like
   `1 OR 1=1` becomes part of the SQL itself and returns every row (SQL injection).

2. **What did the `ExpenseStore` interface buy you when switching from map to SQLite?**
   Handlers depend on the interface, not on a concrete type, so switching storage changed one line in
   `main.go` and zero lines in the handlers. It also lets tests pass in a fake store.

3. **What happens if you forget `defer rows.Close()`?**
   Each open `Rows` holds a database connection from the pool. If you return early without closing it,
   that connection is never given back. Leak enough of them and the pool runs dry, so new requests hang
   waiting for a connection.

4. **What is a migration, and why not just edit the table by hand?**
   A migration is a numbered SQL file that changes the schema (`001_create_expenses.sql`,
   `002_create_users.sql`, …), run in order. It lives in git, so every machine and environment ends up
   with the same tables and you can see how the schema evolved. Hand edits are forgotten and drift
   between machines.

**Reading:** [Go database/sql tutorial](https://go.dev/doc/tutorial/database-access) · [SQLBolt](https://sqlbolt.com/) (interactive SQL lessons)

---

## Phase 3: Users: signup and login

**Goal:** people can create an account and log in with email + password.

**Theory**
- **Authentication** (who are you?) vs **authorization** (what are you allowed to do?)
- Why passwords are **hashed**, never encrypted or stored as plain text
- How bcrypt works: salt, cost factor, why "slow" is a feature
- `UNIQUE` constraints, and turning a duplicate email into 409 Conflict
- Login error messages that don't leak information: always "invalid email or password"
- Validating email and password length

**Build**
- `migrations/002_create_users.sql`
- `internal/auth/password.go`: `HashPassword`, `CheckPassword`
- `POST /auth/signup` → 201 (never return `password_hash` in JSON: `json:"-"`)
- `POST /auth/login` → 200 if the credentials are right (the token comes in Phase 4)

**Break it on purpose**
- Sign up twice with the same email
- Look at the stored hash: hash the same password twice and compare the results
- Return "user not found" vs "wrong password" and think about why that helps an attacker

**Key questions (answered)**

1. **Hashing vs encryption: what's the difference, and why hash passwords?**
   Encryption is reversible: anyone with the key gets the original back. Hashing is one-way. The server
   never needs your actual password, only to check that a guess matches, so it stores a hash. If the
   database leaks, attackers can't reverse the hashes; they have to guess passwords one by one, and
   bcrypt makes each guess slow.

2. **Why does bcrypt produce a different hash for the same password each time?**
   bcrypt generates a random *salt* per hash and stores it inside the hash string
   (`$2a$10$<salt><hash>`). So identical passwords get different hashes: attackers can't use
   precomputed tables, and can't spot two users sharing a password. `CompareHashAndPassword` reads the
   salt back out of the stored hash to check a login.

3. **Why shouldn't login say *which* part was wrong?**
   "No account with that email" tells an attacker which emails are registered (user enumeration). They
   can then focus password guessing on real accounts, and it leaks who uses your app. Always say
   "invalid email or password".

4. **Authentication vs authorization, in one line each.**
   - *Authentication:* proving who you are (logging in).
   - *Authorization:* deciding what you're allowed to do (may you edit expense #7?).

**Reading:** [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) · [bcrypt package docs](https://pkg.go.dev/golang.org/x/crypto/bcrypt)

---

## Phase 4: Protecting routes with JWT

**Goal:** login returns a token; expense routes reject requests without a valid one.

**Theory**
- Stateless vs stateful auth: tokens vs server-side sessions
- What a JWT is: `header.payload.signature`, signed JSON (decode one at [jwt.io](https://jwt.io))
- Claims: `sub` (user id), `exp` (expiry), `iat` (issued at)
- Signing secret: keep it in an env var, never in code
- The `Authorization: Bearer <token>` header
- **Middleware:** a function that takes an `http.Handler` and returns a new one
- **`context.Context`:** passing the logged-in user's id from middleware to handlers
- 401 Unauthorized (who are you?) vs 403 Forbidden (I know you, but no)

**Build**
- `internal/auth/token.go`: `CreateToken(userID)`, `ParseToken(token)`
- `POST /auth/login` now returns `{"token": "..."}`
- `requireAuth` middleware: reads the header, verifies the token, puts the user id in the context
- `GET /me` returns the logged-in user
- A `loggingMiddleware` too (method, path, status, duration), for practice with middleware

**Break it on purpose**
- Call `/expenses` with no token, an expired token, and a token you edited by hand
- Change one character of the payload at jwt.io and watch verification fail
- Sign with one secret and verify with another

**Key questions (answered)**

1. **What are the three parts of a JWT, and which part stops tampering?**
   `header.payload.signature`. The header says which algorithm was used, the payload holds the claims
   (`sub`, `exp`, …), and the signature is `HMAC-SHA256(header + "." + payload, secret)`. **The signature
   stops tampering:** change one byte of the payload and the signature no longer matches, and nobody
   can make a new valid one without the secret.

2. **Can anyone read what's inside a JWT? What does that mean for what you put in it?**
   Yes. The header and payload are only base64url-*encoded*, not encrypted; paste one into jwt.io and
   read it. Signed means "can't be changed", not "can't be read". Only put non-secret things in it:
   user id, expiry. Never passwords or private data.

3. **Explain middleware. Why is it a function returning a function?**
   Middleware is code that runs before and/or after a handler: checking auth, logging, recovering
   from panics. Its shape is `func(next http.Handler) http.Handler`. It takes the next handler and
   returns a new handler that does its own work and then calls `next`. The returned function remembers
   `next` (a closure), which lets you chain them: `logging(requireAuth(mux))`.

4. **Why use `context` to pass the user id instead of a global variable?**
   Many requests run at once, each in its own goroutine. A global would be shared by all of them, so
   requests would overwrite each other's user (a data race, and a security bug). A context belongs to
   one request and flows down through every function it calls. It also carries cancellation, for
   example when the client disconnects.

**Reading:** [Introduction to JWT](https://jwt.io/introduction) · [Go by Example: Context](https://gobyexample.com/context) · [golang-jwt docs](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)

---

## Phase 5: Ownership: users only see their own data

**Goal:** user A can never read, edit or delete user B's expenses.

**Theory**
- Authorization at the data layer: every query is scoped with `WHERE user_id = ?`
- Foreign keys (`expenses.user_id → users.id`) and `PRAGMA foreign_keys = ON` in SQLite
- IDOR (Insecure Direct Object Reference), the most common real-world API bug
- Why you return **404, not 403**, for someone else's expense (don't reveal that it exists)

**Build**
- `migrations/003_add_user_id_to_expenses.sql`
- Every store method takes a `userID`
- Handlers read the user id from the context, **never from the request body**

**Break it on purpose**
- Create two users. As user B, try `GET`, `PUT` and `DELETE` on user A's expense id
- Send `"user_id": <someone else>` in the body and make sure it's ignored

**Key questions (answered)**

1. **What is IDOR? Give an example from this app.**
   Insecure Direct Object Reference: the API trusts an id sent by the client without checking who owns
   it. Here: user B calls `GET /expenses/7`, expense 7 belongs to user A, and the query was
   `WHERE id = ?` without `AND user_id = ?`, so B sees A's data.

2. **Why should the user id come from the token and not the request body?**
   The body is written by the client, and anyone can type any `user_id` into it. The token was signed
   by your server, so the user id inside it can't be faked.

3. **Why 404 instead of 403 here?**
   403 says "this exists, but it isn't yours", which confirms the id is real and lets someone map out
   other users' data. 404 says "from where you stand, this doesn't exist", which reveals nothing.

**Reading:** [OWASP API Security: Broken Object Level Authorization](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/)

---

## Phase 6: Categories, filters and the monthly summary

**Goal:** the features that make it a real expense tracker.

**Theory**
- One-to-many relationships: user → categories → expenses
- `JOIN`s: show the category name with each expense
- Query parameters: `r.URL.Query().Get("month")`, parsing and validating them
- Dates in Go: `time.Parse`, the reference layout `2006-01-02`, time zones
- Aggregation: `SUM`, `COUNT`, `GROUP BY`
- Pagination: `limit` and `offset`, with a maximum limit
- Indexes: why `(user_id, spent_on)` makes month queries fast

**Build**
- CRUD for `/categories` (owned by the user, unique name per user)
- `GET /expenses?month=2026-11&category_id=3&limit=20&offset=0`
- `GET /summary?month=2026-11` → total for the month and per category
- Deleting a category that still has expenses → 409 (or pick another rule and write down why)

**Break it on purpose**
- `?month=banana`, `?limit=-5`, `?limit=100000`
- Use a category id that belongs to another user when creating an expense

**Key questions (answered)**

1. **What does `GROUP BY` do? What's the summary query?**
   It collapses rows that share a value into one row per group, and aggregate functions (`SUM`,
   `COUNT`) are computed per group:
   ```sql
   SELECT c.name, SUM(e.amount_paise) AS total_paise
   FROM expenses e
   JOIN categories c ON c.id = e.category_id
   WHERE e.user_id = ? AND e.spent_on >= ? AND e.spent_on < ?   -- first day of month, first day of next
   GROUP BY c.id, c.name
   ORDER BY total_paise DESC;
   ```

2. **Why does Go use `2006-01-02` as its date format?**
   Go doesn't use `YYYY-MM-DD` codes. Instead you write how one fixed reference moment would look:
   `Mon Jan 2 15:04:05 MST 2006`, which is 01/02 03:04:05 PM '06 (counting 1, 2, 3, 4, 5, 6).
   `2006-01-02` means "year-month-day".

3. **Why cap `limit`?**
   Without a cap, a client can ask for `?limit=1000000`, making the database read everything, the
   server hold it all in memory and send a huge response. That's an easy way to slow down or crash the
   server. Clamp it to a maximum such as 100.

4. **What does an index cost you, and what does it buy?**
   It costs disk space and slightly slower writes, because every insert or update also updates the
   index. It buys fast lookups: the database jumps straight to the matching rows instead of scanning
   the whole table. Index the columns you filter on often, like `(user_id, spent_on)`.

**Reading:** [SQLBolt lessons 6–12](https://sqlbolt.com/) (joins, aggregates) · [Go by Example: Time formatting](https://gobyexample.com/time-formatting-parsing)

---

## Phase 7: Tests

**Goal:** confidence that it works without clicking through it by hand.

**Theory**
- `go test`, `_test.go` files, `t.Run` subtests
- **Table-driven tests**, the standard Go testing style
- `httptest.NewRecorder` (test one handler) and `httptest.NewServer` (test the whole server)
- Testing against a throwaway SQLite database (`:memory:` or a temp file)
- `go test -race` and `go test -cover`

**Build**
- Unit tests: password hashing, token create/parse, input validation
- Handler tests: each status code path (201, 400, 401, 404)
- One end-to-end test: signup → login → create expense → list → other user can't see it

**Key questions (answered)**

1. **What does a table-driven test look like, and why does Go favour it?**
   ```go
   tests := []struct {
       name    string
       amount  int64
       wantErr bool
   }{
       {"positive amount", 2500, false},
       {"zero amount", 0, true},
       {"negative amount", -10, true},
   }
   for _, tc := range tests {
       t.Run(tc.name, func(t *testing.T) {
           err := validateAmount(tc.amount)
           if (err != nil) != tc.wantErr {
               t.Errorf("got err=%v, wantErr=%v", err, tc.wantErr)
           }
       })
   }
   ```
   Adding a case is one line, the checking code is written once, and a failure prints the case name.

2. **When do you use `NewRecorder` vs `NewServer`?**
   - `httptest.NewRecorder`: call one handler directly with a fake `ResponseWriter`. Fast, no network;
     good for testing one handler's logic.
   - `httptest.NewServer`: start a real HTTP server on a random port and send real requests. Tests the
     full stack: routing, middleware, JSON over the wire.

3. **Why test the 400/401/404 paths and not just the happy path?**
   You'll click through the happy path by hand anyway. Bugs and security holes live in the failure
   paths: bad input, missing token, someone else's expense. Tests there also catch regressions when you
   refactor later.

**Reading:** [Go by Example: Testing](https://gobyexample.com/testing-and-benchmarking) · [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests)

---

## Phase 8 (stretch): Polish, sessions and Docker

Pick any, in any order:
- **Config:** `internal/config` reads `PORT`, `DB_PATH` and `JWT_SECRET` from env vars, with defaults
- **Structured logging:** `log/slog`
- **Graceful shutdown:** `http.Server.Shutdown` + `signal.NotifyContext`
- **Panic recovery middleware** and **CORS** for the frontend
- **Sessions:** rebuild login with a cookie + `sessions` table, then compare it with JWT (logout, revoking access)
- **Refresh tokens:** short-lived access token + long-lived refresh token
- **Docker:** a multi-stage `Dockerfile`
- **CSV export:** `GET /expenses/export?month=…` (practice with `encoding/csv` and `io.Writer`)

---

## API contract (for the AI-built frontend)

All responses are JSON. Errors look like `{"error": "message"}`.
Protected routes need `Authorization: Bearer <token>`.

| Method | Path | Auth | Body / query | Response |
|---|---|---|---|---|
| `GET` | `/health` | no | | `{"status":"ok"}` |
| `POST` | `/auth/signup` | no | `{email, password}` | 201 `{id, email}` |
| `POST` | `/auth/login` | no | `{email, password}` | 200 `{token}` |
| `GET` | `/me` | yes | | `{id, email}` |
| `GET` | `/categories` | yes | | `[{id, name}]` |
| `POST` | `/categories` | yes | `{name}` | 201 `{id, name}` |
| `PUT` | `/categories/{id}` | yes | `{name}` | `{id, name}` |
| `DELETE` | `/categories/{id}` | yes | | 204 |
| `GET` | `/expenses` | yes | `?month=YYYY-MM&category_id=&limit=&offset=` | `[{id, amount_paise, note, spent_on, category}]` |
| `POST` | `/expenses` | yes | `{amount_paise, note, spent_on, category_id}` | 201 expense |
| `GET` | `/expenses/{id}` | yes | | expense |
| `PUT` | `/expenses/{id}` | yes | same as POST | expense |
| `DELETE` | `/expenses/{id}` | yes | | 204 |
| `GET` | `/summary` | yes | `?month=YYYY-MM` | `{month, total_paise, by_category: [{category, total_paise}]}` |

**Frontend (later, AI-built):** login/signup pages, an expense list with a month picker and
category filter, an add/edit form, and a summary page with a pie chart.

---

## Commands

```bash
cd backend
go run ./cmd/api          # run the server
go build -o bin/api ./cmd/api
go fmt ./...              # format
go vet ./...              # catch likely bugs
go test ./...             # run tests (add -race -cover)
go mod tidy               # tidy dependencies
```

---

## Go-wide reading (dip in alongside any phase)

- [A Tour of Go](https://go.dev/tour/)
- [Go by Example](https://gobyexample.com/)
- [Effective Go](https://go.dev/doc/effective_go)
- *Let's Go* by Alex Edwards (paid book, covers almost exactly this kind of project)

## Next project

**An uptime monitor:** it checks websites on a schedule using goroutines, worker pools and tickers.
It was set aside as too much for a first backend; it's the natural follow-up once this one is done.

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

1. **Theory:** read and understand the concepts before writing any code.
2. **Build:** write the code myself. AI gives hints, not solutions, unless I ask.
3. **Review:** get the code reviewed and fix what comes up.
4. **Checkpoint:** answer the phase's questions in my own words, in `notes/phase-N.md`.

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

**Checkpoint**
1. What's the difference between a module and a package?
2. Why did `greeting()` fail when called from `main`?
3. What does `internal/` protect against?
4. Why are unused imports a compile error instead of a warning?

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

**Checkpoint**
1. What does the `http.Handler` interface look like, and why is it so powerful?
2. Why do struct fields need to be exported for `encoding/json`?
3. Why does the map need a mutex even though you never started a goroutine?
4. When do you return 201 vs 200 vs 204?

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

**Checkpoint**
1. Why are placeholders safe and string concatenation not?
2. What did the `ExpenseStore` interface buy you when switching from map to SQLite?
3. What happens if you forget `defer rows.Close()`?
4. What is a migration, and why not just edit the table by hand?

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

**Checkpoint**
1. Hashing vs encryption: what's the difference, and why hash passwords?
2. Why does bcrypt produce a different hash for the same password each time?
3. Why shouldn't login say *which* part was wrong?
4. Authentication vs authorization, in one line each.

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

**Checkpoint**
1. What are the three parts of a JWT, and which part stops tampering?
2. Can anyone read what's inside a JWT? What does that mean for what you put in it?
3. Explain middleware in your own words. Why is it a function returning a function?
4. Why use `context` to pass the user id instead of a global variable?

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

**Checkpoint**
1. What is IDOR? Give an example from this app.
2. Why should the user id come from the token and not the request body?
3. Why 404 instead of 403 here?

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

**Checkpoint**
1. What does `GROUP BY` do? Write the summary query from memory.
2. Why does Go use `2006-01-02` as its date format?
3. Why cap `limit`?
4. What does an index cost you, and what does it buy?

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

**Checkpoint**
1. What does a table-driven test look like, and why does Go favour it?
2. When do you use `NewRecorder` vs `NewServer`?
3. What did the tests catch that manual testing missed?

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

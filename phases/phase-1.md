# Phase 1 guide: HTTP + in-memory CRUD, step by step

Nine small steps. Every step has the same four parts:

- **Build:** what this step adds
- **Theory:** only what you need for this step, explained simply
- **Code:** the shape to write, with comments for each line you fill in
- **Check:** how to prove it works before moving on

**Rule: don't start a step until the previous step's Check passes.** Commit after every step
(`git add -A && git commit -m "Phase 1 step N: ..."`). Small commits = easy to undo a mistake.

| Step | Build | New ideas |
|---|---|---|
| 1 | `Expense` type + `MemoryStore` with `Create` and `List` | maps, **`nextID`**, **`sync.RWMutex`**, pointer receivers |
| 2 | `Get` and `Delete` | comma-ok lookup, sentinel errors |
| 3 | `Update` | why a function must not lock twice (deadlock) |
| 4 | A server that answers `GET /health` | `Handler`, `HandlerFunc`, `ServeMux`, `http.Server` |
| 5 | JSON helpers + the `Server` struct | response order, `encoding/json`, dependency injection |
| 6 | `POST /expenses` and `GET /expenses` | request structs, reading a JSON body safely |
| 7 | `GET`, `PUT`, `DELETE /expenses/{id}` | path values, `errors.Is` → 404, 204 |
| 8 | Validation | bytes vs characters, parsing dates |
| 9 | `api.http`, break it on purpose, final commit | |

Steps 1–3 are **plain Go, no HTTP at all**. You test them by printing from `main`.
HTTP starts in Step 4.

---

## Step 1: `Expense` + `MemoryStore` (`Create`, `List`)

**File:** `internal/store/memory.go`

### Theory

#### 1a. Why do we need `nextID`?

Every expense needs a unique ID so you can say "give me expense 3" (`GET /expenses/3`).
A real database generates IDs for you (Phase 2). A Go map doesn't: it just stores whatever key you give it.
So **you** keep a counter that remembers the last ID handed out:

```
nextID starts at 0

Create("lunch")   → nextID becomes 1 → lunch gets ID 1
Create("taxi")    → nextID becomes 2 → taxi gets ID 2
Delete(1)         → lunch is gone, nextID stays 2
Create("coffee")  → nextID becomes 3 → coffee gets ID 3   (NOT 1)
```

**IDs are never reused.** If coffee got ID 1, anyone who saved a link to `/expenses/1` (meaning
lunch) would silently get coffee instead. Real databases follow the same rule.

`nextID` lives inside the store struct, next to the map, because it's part of the store's state.

#### 1b. Why does anything need a lock?

In Phase 1, your server handles each request in its own goroutine, so **many requests run at the
same time**. They all touch the same map and the same `nextID`.

`nextID++` looks like one step, but the CPU does three:

```
1. read nextID       (get 5)
2. add 1             (make 6)
3. write nextID      (store 6)
```

Two requests creating an expense at the same moment:

```
time   request A                request B
 1     reads nextID → 5
 2                              reads nextID → 5      ← B read before A wrote
 3     writes 6, uses ID 6
 4                              writes 6, uses ID 6   ← same ID!
 5     map[6] = lunch
 6                              map[6] = taxi         ← lunch is overwritten and lost
```

I ran exactly this on your machine: 1000 goroutines each did `nextID++`, and the result was
**931–988 unique IDs instead of 1000**. Dozens of duplicates, with no error, just wrong data.
A Go map is worse still: two goroutines writing to it at once crash the whole program with
`fatal error: concurrent map writes`.

A **lock** fixes this: only one goroutine at a time is allowed into the code that touches the
shared data.

#### 1c. `sync.Mutex`: the lock with one key

Think of a **single-person room with one key**:

```go
var mu sync.Mutex

mu.Lock()     // take the key. If someone else has it, WAIT here until they give it back
nextID++      // only one goroutine can ever be here at a time
mu.Unlock()   // give the key back so the next one can enter
```

With the lock, the timeline above becomes:

```
time   request A                       request B
 1     Lock()  ← gets the key
 2     reads 5, writes 6, uses ID 6    Lock()  ← waits...
 3     Unlock()
 4                                     ← gets the key now
 5                                     reads 6, writes 7, uses ID 7   ✓ different IDs
 6                                     Unlock()
```

The code between `Lock()` and `Unlock()` is called the **critical section**. Keep it short.

#### 1d. `sync.RWMutex`: the lock that lets readers share

A plain `Mutex` makes **everyone** wait, even two requests that only want to *read* the list.
Two readers can't break anything, though; only writers can.

`RWMutex` is like a **library reading room**:
- **Many readers** can be inside at the same time (`RLock` / `RUnlock`).
- A **writer** needs the room **empty**, and while the writer is inside, nobody else can enter
  (`Lock` / `Unlock`).

| Situation | Allowed? |
|---|---|
| reader + reader | ✅ at the same time |
| reader + writer | ❌ one waits |
| writer + writer | ❌ one waits |

So in the store:

| Method | Changes data? | Use |
|---|---|---|
| `List`, `Get` | no | `s.mu.RLock()` + `defer s.mu.RUnlock()` |
| `Create`, `Update`, `Delete` | yes | `s.mu.Lock()` + `defer s.mu.Unlock()` |

Most API traffic is reads, so letting readers share keeps the server fast. (Honestly, a plain
`Mutex` would also be fine for this app. `RWMutex` is just the right tool, and it's good to learn.)

#### 1e. `defer`: never forget to unlock

```go
s.mu.Lock()
defer s.mu.Unlock()   // runs automatically when the function returns, whichever return it is
```

If you forget `Unlock()` (for example by returning early from an `if`), the key is never given
back. Every later request waits forever and the server **hangs** (a *deadlock*). Writing
`defer` on the very next line makes that impossible.

#### 1f. Three rules about mutexes

1. **The zero value is ready to use.** `mu sync.RWMutex` as a struct field needs no setup.
2. **Never copy a mutex.** A copy is a different lock that protects nothing. That's why
   `NewMemoryStore` returns a **pointer** (`*MemoryStore`) and every method has a **pointer
   receiver** (`func (s *MemoryStore) ...`). `go vet` warns you if you copy one by mistake.
3. **Go mutexes are not re-entrant:** a goroutine that holds the lock and calls `Lock()` again
   waits for *itself*, forever. This matters in Step 3.

#### 1g. Other things in this step
- **The map must be created** with `make(map[int64]Expense)`. Writing to a nil map panics.
  That's what the constructor `NewMemoryStore()` is for.
- **Store values, not pointers** (`map[int64]Expense`). Returning a value hands the caller a
  *copy*, so nobody can change the stored expense without going through the lock.
- **Map order is random** (Go does it on purpose). Sort the list by ID before returning it.
- **A nil slice turns into JSON `null`**, so build the list with `make([]Expense, 0, len(s.expenses))`
  to get `[]` when empty.

### Code

```go
package store

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

type Expense struct {
	ID          int64     `json:"id"`
	AmountPaise int64     `json:"amount_paise"`
	Note        string    `json:"note"`
	SpentOn     string    `json:"spent_on"` // "2026-11-24"
	CreatedAt   time.Time `json:"created_at"`
}

type MemoryStore struct {
	mu       sync.RWMutex        // protects nextID and expenses
	nextID   int64               // last ID handed out
	expenses map[int64]Expense   // ID → expense
}

func NewMemoryStore() *MemoryStore {
	// return a pointer to a MemoryStore whose map is made with make(...)
	// (mu and nextID can stay at their zero values)
}

func (s *MemoryStore) Create(e Expense) Expense {
	// 1. take the WRITE lock, and defer the unlock on the next line
	// 2. increase s.nextID by 1 and put it in e.ID
	// 3. set e.CreatedAt to time.Now()
	// 4. save e in the map under e.ID
	// 5. return e  (the caller now knows its ID)
}

func (s *MemoryStore) List() []Expense {
	// 1. take the READ lock (RLock), defer RUnlock
	// 2. list := make([]Expense, 0, len(s.expenses))
	// 3. loop over the map and append each expense to list
	// 4. sort: slices.SortFunc(list, func(a, b Expense) int { return cmp.Compare(a.ID, b.ID) })
	// 5. return list
}
```

### Check
Temporarily replace the body of `main()` in `cmd/api/main.go`:

```go
st := store.NewMemoryStore()
st.Create(store.Expense{AmountPaise: 24950, Note: "lunch", SpentOn: "2026-11-24"})
st.Create(store.Expense{AmountPaise: 15000, Note: "taxi", SpentOn: "2026-11-24"})
fmt.Println(st.List())
```

`go run ./cmd/api` should print two expenses with IDs **1 and 2**, in that order.
Also run `go vet ./...`; it should print nothing.

---

## Step 2: `Get` and `Delete`

### Theory
- **The comma-ok lookup.** Reading a missing key from a map doesn't fail; it returns the zero
  value (an empty `Expense`). To know whether the key existed:
  ```go
  e, ok := s.expenses[id]   // ok is false if id isn't in the map
  ```
- **Sentinel errors.** A named error value other code can check for:
  ```go
  var ErrNotFound = errors.New("expense not found")
  ```
  The store only says "not found". It knows nothing about HTTP. Later the handler turns
  `ErrNotFound` into a 404. Keeping layers ignorant of each other is what lets you swap the
  store in Phase 2.
- **Deleting from a map:** the built-in `delete(s.expenses, id)`. It does nothing if the key is
  missing, so check with comma-ok first if you want to report `ErrNotFound`.
- Get **reads** → `RLock`. Delete **writes** → `Lock`.

### Code
```go
var ErrNotFound = errors.New("expense not found")

func (s *MemoryStore) Get(id int64) (Expense, error) {
	// 1. read lock + defer
	// 2. comma-ok lookup
	// 3. not found → return Expense{}, ErrNotFound
	// 4. found → return e, nil
}

func (s *MemoryStore) Delete(id int64) error {
	// 1. write lock + defer
	// 2. comma-ok check; missing → return ErrNotFound
	// 3. delete(s.expenses, id); return nil
}
```

### Check
Add to the temporary `main`:
```go
fmt.Println(st.Get(1))    // the lunch expense, <nil>
fmt.Println(st.Get(99))   // an empty expense, "expense not found"
fmt.Println(st.Delete(1)) // <nil>
fmt.Println(st.Delete(1)) // expense not found
st.Create(store.Expense{AmountPaise: 500, Note: "coffee", SpentOn: "2026-11-24"})
fmt.Println(st.List())    // taxi (ID 2) and coffee (ID 3). ID 1 is NOT reused
```

---

## Step 3: `Update`

### Theory
- `Update` replaces an expense's fields **but keeps its `ID` and `CreatedAt`**. Those belong to
  the server, not the caller.
- **The deadlock trap.** It's tempting to write `Update` like this:
  ```go
  func (s *MemoryStore) Update(id int64, e Expense) (Expense, error) {
      s.mu.Lock()
      defer s.mu.Unlock()
      old, err := s.Get(id)   // ❌ Get calls RLock while we already hold Lock
      ...
  ```
  `Get` tries to take the read lock, but *this same goroutine* already holds the write lock, so
  it waits for itself forever. The program freezes, with no error message. (Rule 3 from Step 1.)
  **Inside a locked method, read the map directly** (comma-ok), never call another method that
  locks.

### Code
```go
func (s *MemoryStore) Update(id int64, e Expense) (Expense, error) {
	// 1. write lock + defer
	// 2. old, ok := s.expenses[id]; missing → return Expense{}, ErrNotFound
	// 3. e.ID = old.ID and e.CreatedAt = old.CreatedAt
	// 4. save e in the map; return e, nil
}
```

### Check
```go
fmt.Println(st.Update(2, store.Expense{AmountPaise: 18000, Note: "taxi + tip", SpentOn: "2026-11-24"}))
// ID 2, the new amount and note, the SAME created_at as before
fmt.Println(st.Update(99, store.Expense{}))   // expense not found
```

**Try the trap:** call `s.Get(id)` inside `Update` and run it. The program freezes, or Go
reports `fatal error: all goroutines are asleep - deadlock!`. Undo it.

When this passes, delete the temporary test code from `main` and commit. **The store is done.**

---

## Step 4: A server that answers `GET /health`

**Files:** `internal/httpapi/health.go` (delete `hello.go`), `cmd/api/main.go`

### Theory
- **A handler** is anything that can answer a request. Go defines it as an interface with
  one method:
  ```go
  type Handler interface {
      ServeHTTP(w http.ResponseWriter, r *http.Request)
  }
  ```
  `r` is what the client sent (method, URL, headers, body). `w` is how you write the reply.
- **`http.HandlerFunc`** is a *function type with a method*. Its `ServeHTTP` just calls the
  function. So any function shaped `func(w http.ResponseWriter, r *http.Request)` can be used as a
  handler. That's why handlers are usually plain functions.
- **`http.ServeMux`** is the router. It looks at the method and path and picks a handler:
  ```go
  mux := http.NewServeMux()
  mux.HandleFunc("GET /health", handleHealth)
  ```
  If the path matches but the method doesn't, it replies **405** by itself. An unknown path gets **404**.
- **`http.Server`** listens on a port and passes every request to the mux, each request in its
  **own goroutine** (this is why Step 1 needed the lock). Set `ReadHeaderTimeout` so a client
  that sends very slowly can't hold a connection open forever.

### Code
```go
// internal/httpapi/health.go
package httpapi

import "net/http"

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// set the header Content-Type to application/json
	// write the body {"status":"ok"}  →  w.Write([]byte(`{"status":"ok"}`))
}

// temporary, until Step 5 builds the Server struct:
func Routes() http.Handler {
	// mux := http.NewServeMux()
	// register "GET /health" → handleHealth
	// return mux
}
```

```go
// cmd/api/main.go
func main() {
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())   // blocks forever; only returns on an error
}
```

### Check
Start it with `go run ./cmd/api`, then in a **second terminal**:
```bash
curl -i localhost:8080/health
```
→ `HTTP/1.1 200 OK`, `Content-Type: application/json`, body `{"status":"ok"}`

```bash
curl -i -X DELETE localhost:8080/health
```
→ `405 Method Not Allowed` with `Allow: GET, HEAD`. You didn't write any code for that.

```bash
curl -i localhost:8080/nope
```
→ `404 page not found`

Stop the server with Ctrl+C.

---

## Step 5: JSON helpers + the `Server` struct

**Files:** `internal/httpapi/json.go`, `internal/httpapi/server.go`

### Theory
- **A response goes out in a fixed order:** headers → status → body.
  ```go
  w.Header().Set("Content-Type", "application/json")   // 1
  w.WriteHeader(http.StatusCreated)                    // 2
  json.NewEncoder(w).Encode(data)                      // 3
  ```
  Once the body starts, the status and headers are already sent, so changing them does nothing.
  If you never call `WriteHeader`, the first write sends 200.
- **`encoding/json` only sees exported (Capitalised) fields.** Struct tags (`json:"amount_paise"`)
  set the key names.
- **One helper for every response** means every handler replies the same way, and errors always
  look like `{"error": "..."}`.
- **Dependency injection:** handlers will need the store. Instead of a global variable, put the
  store in a struct and make the handlers **methods** on it:
  ```go
  type Server struct {
      expenses *store.MemoryStore
  }
  mux.HandleFunc("POST /expenses", s.handleCreateExpense)   // s.method = a function bound to s
  ```
  In Phase 2 the field becomes an interface and the handlers don't change at all.

### Code
```go
// internal/httpapi/json.go
func writeJSON(w http.ResponseWriter, status int, data any) {
	// 1. set Content-Type
	// 2. w.WriteHeader(status)
	// 3. json.NewEncoder(w).Encode(data)  (if it returns an error, just log it; the status is already sent)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	// writeJSON(w, status, map[string]string{"error": msg})
}
```

```go
// internal/httpapi/server.go
type Server struct {
	expenses *store.MemoryStore
}

func NewServer(expenses *store.MemoryStore) *Server {
	// return &Server{...}
}

func (s *Server) Routes() http.Handler {
	// mux := http.NewServeMux()
	// "GET /health" → handleHealth   (health needs no store, so it stays a plain function)
	// return mux
}
```

Change `handleHealth` to use `writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})`,
and delete the temporary `Routes()` from `health.go`.

In `main`: `st := store.NewMemoryStore()` → `api := httpapi.NewServer(st)` → `Handler: api.Routes()`.

### Check
Same `curl -i localhost:8080/health` as Step 4, same result. (A refactor that changes nothing
visible is a successful refactor.)

---

## Step 6: `POST /expenses` and `GET /expenses`

**File:** `internal/httpapi/expenses.go`

### Theory
- **A separate request struct.** The client may only send `amount_paise`, `note` and `spent_on`.
  The server decides `id` and `created_at`.
  ```go
  type expenseRequest struct {
      AmountPaise int64  `json:"amount_paise"`
      Note        string `json:"note"`
      SpentOn     string `json:"spent_on"`
  }
  ```
  If you decoded straight into `store.Expense`, a client could send `"id": 999` and you'd trust it.
  That bug is called **mass assignment**.
- **Reading a body safely:**
  - `http.MaxBytesReader` caps the body (1 MB) so nobody can send you 5 GB.
  - `dec.DisallowUnknownFields()`: by default a typo like `"amout_paise"` is silently ignored
    and you'd store 0. This turns it into an error.
  - Bad JSON or a wrong type (`"amount_paise": "abc"`) makes `Decode` return an error → **400**.
- **`return` after every `writeError`**, or the handler keeps going and writes a second response.
- Create → **201** with the new expense. List → **200** with the array.

### Code
```go
// add to json.go
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	// dec := json.NewDecoder(r.Body)
	// dec.DisallowUnknownFields()
	// return dec.Decode(dst)
}
```

```go
// expenses.go
func (s *Server) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	// 1. var req expenseRequest
	// 2. if err := readJSON(w, r, &req); err != nil → writeError 400 with err.Error(), return
	// 3. build a store.Expense from req's three fields
	// 4. created := s.expenses.Create(e)
	// 5. writeJSON 201 with created
}

func (s *Server) handleListExpenses(w http.ResponseWriter, r *http.Request) {
	// writeJSON 200 with s.expenses.List()
}
```
Register both in `Routes()`: `"POST /expenses"` and `"GET /expenses"`.

### Check
```bash
curl -i localhost:8080/expenses
```
→ 200, body `[]` (not `null`)

```bash
curl -i -X POST localhost:8080/expenses -H 'Content-Type: application/json' -d '{"amount_paise": 24950, "note": "lunch", "spent_on": "2026-11-24"}'
```
→ 201, the expense with `"id":1` and a `created_at`

```bash
curl -i -X POST localhost:8080/expenses -d '{"amount": 100}'
```
→ 400 `json: unknown field "amount"`

```bash
curl -i -X POST localhost:8080/expenses -d '{"amount_paise": "abc"}'
```
→ 400 `json: cannot unmarshal string into Go struct field ...`

```bash
curl -i -X POST localhost:8080/expenses -d '{"amount_paise": 1, "id": 999}'
```
→ 400 (unknown field `id`): mass assignment blocked

---

## Step 7: `GET`, `PUT`, `DELETE /expenses/{id}`

### Theory
- **Path values:** with the pattern `"GET /expenses/{id}"`, `r.PathValue("id")` gives you the
  text after `/expenses/`. It's **always a string**; convert it with
  `strconv.ParseInt(text, 10, 64)` (base 10, 64-bit). Not a number → **400**.
- **Turning store errors into status codes:**
  ```go
  if errors.Is(err, store.ErrNotFound) {
      writeError(w, http.StatusNotFound, "expense not found")
      return
  }
  ```
  `errors.Is` is how you compare errors (it also works when errors get wrapped later).
- **DELETE replies 204 No Content:** just `w.WriteHeader(http.StatusNoContent)`, no body.
- **PUT means replace:** the client sends all three fields, the same body as POST.
- All three handlers start by parsing the id, so write a small helper once.

### Code
```go
func parseID(r *http.Request) (int64, error) {
	// return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func (s *Server) handleGetExpense(w http.ResponseWriter, r *http.Request) {
	// 1. id, err := parseID(r); err → 400 "invalid id", return
	// 2. e, err := s.expenses.Get(id)
	// 3. errors.Is(err, store.ErrNotFound) → 404, return
	// 4. writeJSON 200 with e
}

func (s *Server) handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	// parse id → read body into expenseRequest → build store.Expense
	// → s.expenses.Update(id, e) → 404 if not found → 200 with the result
}

func (s *Server) handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	// parse id → s.expenses.Delete(id) → 404 if not found → w.WriteHeader(204)
}
```
Register `"GET /expenses/{id}"`, `"PUT /expenses/{id}"` and `"DELETE /expenses/{id}"`.

### Check
After creating one expense:

| Request | Expected |
|---|---|
| `curl -i localhost:8080/expenses/1` | 200 + the expense |
| `curl -i localhost:8080/expenses/99` | 404 `{"error":"expense not found"}` |
| `curl -i localhost:8080/expenses/abc` | 400 `{"error":"invalid id"}` |
| `curl -i -X PUT localhost:8080/expenses/1 -d '{"amount_paise": 30000, "note": "dinner", "spent_on": "2026-11-24"}'` | 200, same id and created_at, new fields |
| `curl -i -X DELETE localhost:8080/expenses/1` | 204, empty body |
| the same DELETE again | 404 |

---

## Step 8: Validation

### Theory
- **Validate at the edge:** check input in the handler, *before* it reaches the store, and answer
  **400** with a clear message.
- **Bytes vs characters:** `len("₹")` is **3**, because `len` counts bytes and `₹` takes 3 bytes in
  UTF-8. To count characters, use `utf8.RuneCountInString(s)` (a *rune* is one character).
- **Checking a date:** `time.Parse(time.DateOnly, s)`, where `time.DateOnly` is the layout
  `"2006-01-02"`. It returns an error for `"2026-13-45"` or `"yesterday"`.
- Put the rules in **one method** so POST and PUT share them.

### Code
```go
// returns "" when everything is fine, otherwise the message for the client
func (req expenseRequest) validate() string {
	// amount_paise <= 0                         → "amount_paise must be greater than 0"
	// utf8.RuneCountInString(req.Note) > 200    → "note must be at most 200 characters"
	// req.SpentOn == ""                         → "spent_on is required"
	// time.Parse(time.DateOnly, req.SpentOn) err → "spent_on must be YYYY-MM-DD"
	// otherwise ""
}
```
In create and update, right after `readJSON`:
```go
if msg := req.validate(); msg != "" {
	writeError(w, http.StatusBadRequest, msg)
	return
}
```

### Check
| Body | Expected |
|---|---|
| `{"amount_paise": 0, "note": "x", "spent_on": "2026-11-24"}` | 400 amount message |
| `{"amount_paise": 100, "note": "x", "spent_on": "24-11-2026"}` | 400 date message |
| `{"amount_paise": 100, "note": "x"}` | 400 spent_on required |
| a valid body | 201 |

---

## Step 9: `api.http`, break it on purpose, commit

### `api.http` at the repo root
One request per endpoint and per error case, so you can retest everything in seconds. Install the
VS Code **REST Client** extension; a "Send Request" link appears above each `###`.

```http
@base = http://localhost:8080

### Health
GET {{base}}/health

### Create
POST {{base}}/expenses
Content-Type: application/json

{"amount_paise": 24950, "note": "lunch", "spent_on": "2026-11-24"}

### List
GET {{base}}/expenses

### Get one
GET {{base}}/expenses/1

### Update
PUT {{base}}/expenses/1
Content-Type: application/json

{"amount_paise": 30000, "note": "dinner", "spent_on": "2026-11-24"}

### Delete
DELETE {{base}}/expenses/1

### Error: bad amount
POST {{base}}/expenses
Content-Type: application/json

{"amount_paise": 0, "note": "x", "spent_on": "2026-11-24"}
```

### Break it on purpose
| Do this | What you'll see |
|---|---|
| Comment out the lock in `Create`, run `go run -race ./cmd/api`, send 50 POSTs at once (command below) | `WARNING: DATA RACE` pointing at `nextID` and the map. Put the lock back. |
| Make `Note` lowercase in `Expense` | `note` vanishes from every response, with no error |
| Delete a `return` after a `writeError` | two JSON bodies glued together + `superfluous response.WriteHeader call` in the log |
| Make `List` return a nil slice | an empty list prints `null` |

50 requests at once:
```bash
for i in $(seq 1 50); do curl -s -o /dev/null -X POST localhost:8080/expenses -H 'Content-Type: application/json' -d '{"amount_paise":100,"note":"x","spent_on":"2026-11-24"}' & done; wait
```
Then `curl localhost:8080/expenses` and check that the IDs run 1–50 with no gaps or duplicates (with the lock in place).

### Finish
- [ ] Every Check above passes
- [ ] `go vet ./...` prints nothing
- [ ] Imports grouped (standard library, blank line, `expense-tracker/...`)
- [ ] `bin/` added to `.gitignore`
- [ ] Committed

Then say **"review phase 1"**.

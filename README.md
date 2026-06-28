# students-api

A small REST API for managing students, written in Go using **only the standard library** for HTTP (plus a few focused third-party packages for config, validation, and the SQLite driver).

It's intentionally structured like a real production service so it doubles as a learning project: layered architecture, an interface-based storage layer, dependency injection, structured logging, input validation, and graceful shutdown.

---

## What it does

Exposes a JSON HTTP API. Today it supports creating a student:

```
POST /api/students
Content-Type: application/json

{ "name": "Ikshant", "email": "a@b.com", "age": 22 }
```

On success it returns `201 Created` with the new id:

```json
{ "id": 1 }
```

Data is stored in a local **SQLite** database file.

---

## Folder & file structure

```
students-api/
├── cmd/
│   └── students-api/
│       └── main.go              # Entry point ("composition root"): wires everything together
│
├── config/
│   └── local.yaml               # Runtime settings (env, db path, http address)
│
├── internal/                    # Private application code (cannot be imported by other modules)
│   ├── config/
│   │   └── config.go            # Loads + parses config from YAML / env vars / defaults
│   │
│   ├── http/
│   │   └── handlers/
│   │       └── student/
│   │           └── student.go   # HTTP handler for /api/students (decode -> validate -> store)
│   │
│   ├── storage/
│   │   ├── storage.go           # The Storage INTERFACE (the contract)
│   │   └── sqlite/
│   │       └── sqlite.go        # SQLite IMPLEMENTATION of the Storage interface
│   │
│   ├── types/
│   │   └── types.go             # Shared domain model(s): the Student struct
│   │
│   └── utils/
│       └── response/
│           └── response.go      # Helpers for writing consistent JSON responses
│
├── storage/
│   └── storage.db               # The SQLite database file (created at runtime)
│
├── go.mod                       # Module path + dependencies
├── go.sum                       # Dependency checksums (integrity/security)
└── README.md
```

### Why it's organized this way

The project follows the widely-used **Standard Go Project Layout** and a **layered architecture** where each layer has one job and depends only on the layer below it through an interface.

| Path | Responsibility | Knows about |
|------|----------------|-------------|
| `cmd/students-api/` | Program entry point; constructs and wires all components. | everything (it's the only place that does) |
| `internal/config/` | Read settings into a typed struct. | nothing else in the app |
| `internal/types/` | Pure data structures shared across layers. | nothing (leaf package, avoids import cycles) |
| `internal/storage/` | The data-access **contract** (interface). | only `types`-level data |
| `internal/storage/sqlite/` | One concrete data store behind the contract. | `database/sql`, the interface |
| `internal/http/handlers/` | HTTP concerns: request/response, status codes, validation. | the `Storage` interface — **not** SQL |
| `internal/utils/response/` | Consistent JSON response writing. | `net/http`, `encoding/json` |

Two Go conventions worth calling out:

- **`cmd/`** — each subfolder is one runnable program (a `main` package). Keeping `main` thin and separate from business logic is idiomatic.
- **`internal/`** — a **compiler-enforced** private directory. Code outside this module cannot import anything under `internal/`. It marks implementation as private.

### The core design idea

The HTTP handler depends on the `Storage` **interface**, not on SQLite directly:

```
handler  ──depends on──▶  storage.Storage (interface)  ◀──implemented by──  sqlite.Sqlite
```

This decouples the layers, so the database can be swapped (e.g. SQLite → Postgres) or replaced with a fake for testing — without changing handler code. The concrete implementation is chosen in exactly one place: `main.go`.

---

## Request lifecycle

A `POST /api/students` flows through the layers like this:

```
1. main.go starts an http.Server on the configured address
2. ServeMux (router) matches "POST /api/students"
3. student handler decodes the JSON body into a types.Student
4. handler validates the struct (go-playground/validator)
5. handler calls storage.CreateStudent(...)   ← interface call
6. the sqlite implementation runs the INSERT and returns the new id
7. handler writes a JSON response { "id": ... } with 201 Created
```

---

## Configuration

Settings are read with the following precedence (highest first):

1. Environment variables (e.g. `HTTP_ADDRESS`, `ENV`)
2. The YAML config file
3. Built-in defaults

`config/local.yaml`:

```yaml
env: "dev"
storage_path: "storage/storage.db"
http_server:
  address: "localhost:8080"
```

The config file path is provided via the `CONFIG_PATH` environment variable or the `-config` flag.

---

## Running the project

Prerequisites: a recent Go toolchain.

```bash
# From the project root
go run cmd/students-api/main.go -config config/local.yaml
```

The server starts on the address from your config (default `localhost:8080`).

Create a student:

```bash
curl -i -X POST http://localhost:8080/api/students \
  -H "Content-Type: application/json" \
  -d '{"name":"Ikshant","email":"a@b.com","age":22}'
```

Stop the server with `Ctrl+C` — it shuts down gracefully, letting in-flight requests finish (up to a 5s deadline).

---

## Build, format, and vet

```bash
go build ./...   # compile everything
go vet ./...     # static checks
gofmt -l .       # list files that need formatting
```

---

## Tech stack

| Concern | Choice |
|---------|--------|
| HTTP routing | `net/http` `ServeMux` (Go 1.22+ method-based routing) |
| Database | SQLite via `github.com/mattn/go-sqlite3` + `database/sql` |
| Config | `github.com/ilyakaznacheev/cleanenv` |
| Validation | `github.com/go-playground/validator/v10` |
| Logging | `log/slog` (structured logging) |

---

## Roadmap

- [ ] `GET /api/students/{id}` — fetch one student
- [ ] `GET /api/students` — list students
- [ ] `PUT /api/students/{id}` — update
- [ ] `DELETE /api/students/{id}` — delete
- [ ] Unit tests using a fake `Storage`
- [ ] Logging / recovery middleware
- [ ] Server timeouts and hardened error responses

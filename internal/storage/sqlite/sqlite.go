// Package sqlite is ONE concrete implementation of the storage.Storage
// interface, backed by a SQLite database file on disk.
//
// The rest of the app never imports this package's type directly for its logic;
// it talks to the storage.Storage interface. main.go is the only place that
// picks "sqlite" specifically, which keeps the database swappable.
package sqlite

import (
	"database/sql" // standard-library generic SQL API (the "interface")
	"errors"
	"fmt"

	"github.com/ikshantshukla123/students-api/internal/config"
	"github.com/ikshantshukla123/students-api/internal/storage"
	"github.com/ikshantshukla123/students-api/internal/types"

	// Blank import: we never reference this package by name, we import it ONLY
	// for its side effect. Its init() calls sql.Register("sqlite3", ...), which
	// plugs the real SQLite driver into database/sql. Without this line,
	// sql.Open("sqlite3", ...) would fail at runtime with `unknown driver`.
	// The `_` avoids Go's "imported and not used" compile error.
	_ "github.com/mattn/go-sqlite3"
)

// Sqlite wraps a *sql.DB. Note: *sql.DB is NOT a single connection — it is a
// connection POOL that is safe for concurrent use by many goroutines. Create it
// once at startup and share it for the whole program's life.
type Sqlite struct {
	Db *sql.DB
}

// Compile-time guarantee that *Sqlite satisfies the storage.Storage interface.
// This line produces no runtime code; it only forces the compiler to error out
// here (with a clear message) if *Sqlite ever stops matching the interface,
// e.g. if we typo a method signature. Reads as: "a nil *Sqlite must be usable
// as a storage.Storage".
var _ storage.Storage = (*Sqlite)(nil)

// New is the constructor for Sqlite (Go has no built-in constructors; the
// convention is a function named New/NewXxx). It returns a concrete *Sqlite
// (remember: "accept interfaces, return structs") plus an error, letting the
// caller (main.go) decide how to handle failure rather than crashing here.
func New(cfg *config.Config) (*Sqlite, error) {
	// IMPORTANT: sql.Open does NOT actually connect or verify anything. It only
	// validates arguments and prepares the pool lazily; the first real
	// connection happens on first use.
	db, err := sql.Open("sqlite3", cfg.StoragePath)
	if err != nil {
		return nil, err
	}

	// Because sql.Open is lazy, we Ping to force a real connection now, so a
	// bad DB path/permission fails loudly at startup instead of on first request.
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Create the table if it doesn't already exist. "IF NOT EXISTS" makes this
	// safe (idempotent) to run on every startup. In larger apps you'd use a
	// dedicated migrations tool instead of creating schema in code.
	// Exec is used for statements that don't return rows (CREATE/INSERT/UPDATE/DELETE).
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS students (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT,
	age INTEGER,
	email TEXT
	)`)
	if err != nil {
		return nil, err
	}

	return &Sqlite{
		Db: db,
	}, nil
}

// CreateStudent inserts one student and returns the new auto-generated id.
//
// The receiver `(s *Sqlite)` makes this a METHOD on *Sqlite — `s` is like
// this/self. It's a pointer receiver so every call shares the same Db pool
// instead of copying the struct. Having this method is what makes *Sqlite
// satisfy the storage.Storage interface.
func (s *Sqlite) CreateStudent(name string, email string, age int) (int64, error) {
	// The `?` are placeholders (parameterized query). Values are sent to the DB
	// SEPARATELY from the SQL text, so user input can never be executed as SQL.
	// This is the defense against SQL INJECTION — never build SQL via string
	// concatenation. We also always name the columns so table column order
	// doesn't matter.
	//
	// Prepare compiles the statement once; it's ideal when running the same
	// statement repeatedly. For a one-off insert, s.Db.Exec(query, args...)
	// would be equally fine and simpler.
	stmt, err := s.Db.Prepare("INSERT INTO students (name,email,age) VALUES (?,?,?)")
	if err != nil {
		return 0, err
	}
	// defer schedules this to run when CreateStudent returns, on ANY path
	// (success or error). It's Go's guaranteed-cleanup mechanism; deferred
	// calls run in LIFO order. Here it ensures the statement is always released.
	defer stmt.Close()

	result, err := stmt.Exec(name, email, age)
	if err != nil {
		return 0, err
	}

	// Exec returns a sql.Result; LastInsertId gives the auto-increment primary
	// key the DB just generated for this row.
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetStudentById fetches exactly one student by id.
func (s *Sqlite) GetStudentById(id int64) (types.Student, error) {
	// QueryRow is used when we expect AT MOST one row (vs Query for many).
	// It still uses a ? placeholder + separate value for SQL-injection safety.
	// LIMIT 1 is a small safety net in case of unexpected duplicates.
	stmt, err := s.Db.Prepare("SELECT id, name, email, age FROM students WHERE id = ? LIMIT 1")
	if err != nil {
		return types.Student{}, err
	}
	defer stmt.Close()

	var student types.Student

	// Scan copies the columns of the result row INTO our variables, in order.
	// We pass POINTERS (&student.Id, ...) so Scan can write into the fields.
	// The order here must match the SELECT column order above.
	err = stmt.QueryRow(id).Scan(&student.Id, &student.Name, &student.Email, &student.Age)
	if err != nil {
		// sql.ErrNoRows is the SPECIAL error meaning "the query found no row".
		// It's not a real failure — it's a valid "not found", which the handler
		// turns into a 404. We wrap it with the id for a clearer message; %w
		// preserves the original error so errors.Is can still detect it.
		if errors.Is(err, sql.ErrNoRows) {
			return types.Student{}, fmt.Errorf("no student found with id %d", id)
		}
		return types.Student{}, err
	}

	return student, nil
}

// GetStudents returns all students.
func (s *Sqlite) GetStudents() ([]types.Student, error) {
	stmt, err := s.Db.Prepare("SELECT id, name, email, age FROM students")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Query (not QueryRow) returns MANY rows as a *sql.Rows cursor.
	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	// rows holds a DB connection open until closed — ALWAYS defer rows.Close()
	// or you leak connections from the pool.
	defer rows.Close()

	var students []types.Student

	// rows.Next() advances the cursor one row at a time, returning false when
	// done (or on error). Scan reads the current row into a struct.
	for rows.Next() {
		var student types.Student
		if err := rows.Scan(&student.Id, &student.Name, &student.Email, &student.Age); err != nil {
			return nil, err
		}
		students = append(students, student)
	}

	// IMPORTANT: the loop can stop because of an error mid-iteration, not just
	// because we ran out of rows. rows.Err() surfaces any such error.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return students, nil
}




// This is your storage layer implementation. Current state (with the two lines we just added):


// package sqlite

// import (
// 	"database/sql"
// 	"github.com/ikshantshukla123/students-api/internal/config"
// 	"github.com/ikshantshukla123/students-api/internal/storage"
// 	_ "github.com/mattn/go-sqlite3"
// )

// type Sqlite struct{
// 	Db *sql.DB
// }

// var _ storage.Storage = (*Sqlite)(nil)

// func New(cfg *config.Config) (*Sqlite, error){ ... }
// func (s *Sqlite) CreateStudent(...) (int64, error){ ... }
// Lots of essential Go-meets-databases here. Let me go piece by piece.

// 1. database/sql — Go's database abstraction
// database/sql is a standard library package that defines a generic API for SQL databases — DB, Row, Stmt, Exec, Query, etc. But crucially, it contains no actual database code. It's like the Storage interface idea, but at the standard-library level: it defines the contract, and a driver provides the real implementation.

// 2. The blank import — _ "github.com/mattn/go-sqlite3"
// This is the single most confusing line for beginners, and a classic interview question.


// _ "github.com/mattn/go-sqlite3"
// The _ is the blank identifier. Normally, importing a package and not using it is a compile error in Go. The _ says "import this for its side effects only; I won't reference it by name."
// What side effect? When this package loads, its init() function runs and calls sql.Register("sqlite3", ...). That registers the SQLite driver into database/sql's internal registry.
// That's why later you can write sql.Open("sqlite3", ...) — the string "sqlite3" is looked up in that registry. Without the blank import, sql.Open("sqlite3", ...) would fail at runtime with unknown driver "sqlite3".
// So: database/sql is the interface, go-sqlite3 is the driver that plugs into it via init(). Same decoupling pattern as your Storage interface, one level down.

// Interview soundbite: "The blank import runs the driver's init() to register it with database/sql; the package isn't referenced directly, so we use _ to avoid the unused-import error."

// 3. type Sqlite struct { Db *sql.DB }
// Your struct wraps a *sql.DB. Two things to know:

// *sql.DB is NOT a single connection — it's a connection pool. It's safe for concurrent use by many goroutines, manages opening/closing/reusing connections automatically, and is designed to be created once and shared for the whole program's life. You should not open a new sql.DB per request. (Your code does it once in New — correct.)

// 4. func New(cfg *config.Config) (*Sqlite, error)
// This is a constructor — Go has no new/constructors built in, so the convention is a function named New (or NewXxx) that builds and returns your type.


// db, err := sql.Open("sqlite3", cfg.StoragePath)
// Gotcha worth knowing: sql.Open does not actually connect or verify the database! It just validates arguments and prepares the pool lazily. The first real connection happens on first use. To verify connectivity eagerly, you call db.Ping(). Many production codebases add a Ping here — a great improvement we can make.


// _, err = db.Exec(`CREATE TABLE IF NOT EXISTS students (...)`)
// Exec runs a statement that doesn't return rows (CREATE, INSERT, UPDATE, DELETE). IF NOT EXISTS makes it safe to run on every startup — idempotent. (In bigger projects you'd use a migrations tool instead of creating tables in code; good to mention in interviews.)

// Note the two return values (*Sqlite, error) — the ubiquitous Go pattern: return the result and an error, let the caller decide what to do. The constructor doesn't log.Fatal itself; it bubbles the error up to main. Good separation.

// 5. func (s *Sqlite) CreateStudent(...) — the method

// func (s *Sqlite) CreateStudent(name string, email string, age int) (int64, error)
// The (s *Sqlite) part is the receiver — it's what makes this a method on *Sqlite rather than a free function. s is like this/self. It's a pointer receiver (*Sqlite) — used so the method shares the same Db pool rather than copying the struct. (Pointer vs value receivers is its own interview topic; rule of thumb: use pointer receivers for types holding resources/state, and be consistent across a type's methods.)

// This method is what makes *Sqlite satisfy storage.Storage.

// Prepared statements & SQL injection (very important)

// stmt, err := s.Db.Prepare("INSERT INTO students (name,email,age) VALUES (?,?,?)")
// ...
// defer stmt.Close()
// result, err := stmt.Exec(name, email, age)
// The ? are placeholders / parameterized queries. You pass the values separately to Exec. The driver sends the query and the data apart, so user input can never be interpreted as SQL. This is your defense against SQL injection — the canonical web vuln. Never build SQL by string concatenation like "... VALUES ('" + name + "')".
// Prepare compiles the statement once; stmt.Exec runs it. For a single insert, you could also just call s.Db.Exec("INSERT ... VALUES (?,?,?)", name, email, age) directly — simpler. Prepare shines when you run the same statement many times in a loop.
// defer stmt.Close()

// defer stmt.Close()
// defer schedules a function call to run when the surrounding function returns — no matter how it returns (normal or error path). It's Go's tool for guaranteed cleanup (closing files, statements, connections, unlocking mutexes). Here it ensures the prepared statement is always released, even if stmt.Exec errors out. Deferred calls run in LIFO order (last deferred, first run).

// result.LastInsertId()
// Exec returns a sql.Result, from which LastInsertId() gives the auto-generated primary key — the new student's id, which you return up to the handler.

// 6. One real bug hiding in your code 🐞
// Look carefully at the column order:


// "INSERT INTO students (name,email,age) VALUES (?,?,?)"
// stmt.Exec(name, email, age)
// That's correct ✅. But your table was created as (id, name, age, email) — and your interface method signature is CreateStudent(name, email, age). The named columns in the INSERT protect you here, so data lands correctly. Good — but it's a reminder to always name your columns in INSERTs (never INSERT INTO students VALUES (?,?,?)), precisely so column order in the table doesn't matter. You're already doing the safe thing.

// Let me now write the fully documented version, and add a db.Ping() for eager connection verification:






// Exec

// Used for:

// CREATE
// INSERT
// UPDATE
// DELETE

// Example:

// _, err := db.Exec(
//     "DELETE FROM students WHERE id=?",
//     id,
// )



//Query 
// Used when you want data back.

// Example:

// SELECT * FROM students;

// Go:

// rows, err := db.Query(
//     "SELECT * FROM students",
// )

// because you get:

// id | name | age
// ----------------
// 1  | Amit | 20
// 2  | Rahul| 22
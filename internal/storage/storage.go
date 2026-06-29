// Package storage defines the CONTRACT for how the rest of the app talks to a
// data store — without saying anything about WHICH data store.
//
// This is the central design idea of the project. The HTTP handlers depend on
// the Storage interface below, not on SQLite directly. That means we can swap
// the database (SQLite -> Postgres -> an in-memory fake for tests) without
// touching any handler code.
package storage

import "github.com/ikshantshukla123/students-api/internal/types"

// Storage is an interface: a contract made of method signatures with no
// implementation. ANY type that has all these methods automatically satisfies
// Storage — in Go you never write "implements Storage". This is called
// implicit/structural typing.
//
// Today the only implementation is *sqlite.Sqlite (see internal/storage/sqlite),
// but a test could provide a fake type with the same method to avoid a real DB.
//
// Design notes (idiomatic Go):
//   - Keep interfaces small. Small interfaces are easy to implement and to mock.
//   - "Accept interfaces, return structs": handlers accept this interface;
//     sqlite.New returns a concrete *Sqlite.
type Storage interface {
	// CreateStudent inserts a new student and returns the database-generated id,
	// or an error if the insert failed.
	CreateStudent(name string, email string, age int) (int64, error)

	// GetStudentById fetches one student by its id. Returns an error if no row
	// matches (so the handler can answer 404).
	GetStudentById(id int64) (types.Student, error)

	// GetStudents returns all students (a slice, possibly empty).
	GetStudents() ([]types.Student, error)
}

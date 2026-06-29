// Package student holds the HTTP handler(s) for the /students endpoints.
//
// This is the HTTP layer: it knows about requests, JSON, status codes and
// validation — but NOTHING about SQL. It talks to the database only through the
// storage.Storage interface, which keeps it decoupled and testable.
package student

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/ikshantshukla123/students-api/internal/storage"
	"github.com/ikshantshukla123/students-api/internal/types"
	"github.com/ikshantshukla123/students-api/internal/utils/response"
)

// validate is created ONCE and reused for every request. validator.New() builds
// an internal cache of reflection info and is relatively expensive, so calling
// it per request (as the original code did) wastes work. A single instance is
// safe for concurrent use.
var validate = validator.New()

// New is a FACTORY: it takes the dependencies the handler needs (here, a
// storage.Storage) and returns the actual handler function.
//
// Why this shape? An http handler's signature is fixed as func(w, r) — there's
// no slot for a `storage` argument. So we use a CLOSURE: the returned inner
// function "captures" the `storage` variable from this enclosing scope and
// remembers it after New returns. This is dependency injection via closures —
// the canonical Go pattern. main.go calls student.New(storage) once at startup;
// tests can call it with a fake storage to test the handler without a real DB.
//
// http.HandlerFunc is just `type HandlerFunc func(ResponseWriter, *Request)`,
// so the returned function satisfies what the router expects.
func New(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("creating a student")

		var student types.Student

		// r.Body is a stream of the request bytes. Decode reads it and fills the
		// struct. We pass &student (a POINTER) because Go is pass-by-value:
		// Decode must write into our original variable, not a copy.
		err := json.NewDecoder(r.Body).Decode(&student)

		// io.EOF means the body was empty (client sent no JSON). errors.Is is the
		// correct way to compare errors — it unwraps wrapped errors, unlike `==`.
		if errors.Is(err, io.EOF) {
			response.WriteJson(w, http.StatusBadRequest,
				response.GeneralError(errors.New("empty request body")))
			return
		}
		// Any other decode error (malformed JSON, wrong types, ...). Returning
		// early on errors keeps the happy path un-indented below.
		if err != nil {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(err))
			return
		}

		// Request validation. validate.Struct reads the `validate:"..."` struct
		// tags on types.Student and enforces them. Returns nil when valid.
		if err := validate.Struct(student); err != nil {
			// Safe (comma-ok) type assertion: err is the `error` interface; we
			// check whether the concrete type underneath is ValidationErrors.
			// Using the plain form err.(...) would PANIC on a different error type.
			validateErrs, ok := err.(validator.ValidationErrors)
			if !ok {
				response.WriteJson(w, http.StatusBadRequest, response.GeneralError(err))
				return
			}
			response.WriteJson(w, http.StatusBadRequest, response.ValidationError(validateErrs))
			return
		}

		// The interface call: the handler has no idea this is SQLite underneath.
		id, err := storage.CreateStudent(
			student.Name,
			student.Email,
			student.Age,
		)
		// 500 = server's fault (the DB failed), not the client's.
		if err != nil {
			response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
			return
		}

		// Log success only AFTER we know it actually succeeded.
		slog.Info("student created successfully", slog.Int64("id", id))

		// 201 Created is the correct status for a successful resource creation.
		response.WriteJson(w, http.StatusCreated, map[string]int64{"id": id})
	}
}



// GetById handles GET /api/students/{id} — fetch a single student.
func GetById(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// r.PathValue("id") reads the {id} wildcard from the route pattern
		// (a Go 1.22+ feature). It's ALWAYS a string, so we must convert it.
		idStr := r.PathValue("id")
		slog.Info("getting a student", slog.String("id", idStr))

		// Convert the path string to int64. base 10, 64-bit. If the client sent
		// something non-numeric (e.g. /students/abc), this errors -> 400.
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			response.WriteJson(w, http.StatusBadRequest, response.GeneralError(err))
			return
		}

		student, err := storage.GetStudentById(id)
		if err != nil {
			// Not found is the client asking for something that doesn't exist:
			// 404, not 500. (We keep it simple here; a more precise version would
			// distinguish "not found" from a real DB failure.)
			response.WriteJson(w, http.StatusNotFound, response.GeneralError(err))
			return
		}

		response.WriteJson(w, http.StatusOK, student)
	}
}

// GetList handles GET /api/students — fetch all students.
func GetList(storage storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("getting all students")

		students, err := storage.GetStudents()
		if err != nil {
			// A failure here is the server's fault (DB error) -> 500.
			response.WriteJson(w, http.StatusInternalServerError, response.GeneralError(err))
			return
		}

		response.WriteJson(w, http.StatusOK, students)
	}
}





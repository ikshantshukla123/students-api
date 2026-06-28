// Package response centralizes how the API writes JSON to clients.
//
// Why have this at all? Without it, every handler would repeat the same steps
// (set Content-Type, write status code, encode JSON) — duplication that's easy
// to get wrong. Centralizing keeps responses consistent (DRY) and gives clients
// one predictable error shape.
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Response is the standard "envelope" we send for errors (and could reuse for
// simple status messages). Clients can always rely on this shape.
type Response struct {
	Status string `json:"status"`
	// omitempty: when Error is the empty string "", the field is left out of the
	// JSON entirely, so success responses don't carry an empty "error":"".
	Error string `json:"error,omitempty"`
}

// Named constants instead of sprinkling raw "OK"/"Error" strings around the
// codebase — typo-proof and easy to change in one place.
const (
	StatusOK    = "OK"
	StatusError = "Error"
)

// WriteJson is the single place that turns a Go value into a JSON HTTP response.
//
// data is `interface{}` (a.k.a. `any`) — the empty interface, which EVERY type
// satisfies — so this can encode a Response, a map, a Student, anything.
//
// ORDER MATTERS and is a classic bug source:
//  1. set headers, 2. write the status code, 3. write the body.
// Once the body (or WriteHeader) is written, headers are flushed and locked;
// setting a header after that is silently ignored.
func WriteJson(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// NewEncoder(w).Encode streams JSON straight to the response writer (an
	// io.Writer), avoiding the intermediate []byte that json.Marshal allocates.
	return json.NewEncoder(w).Encode(data)
}

// GeneralError adapts ANY Go error into our JSON error envelope.
//
// `error` is itself an interface: `interface{ Error() string }`. Calling
// err.Error() returns its human-readable message.
//
// Production note: returning raw internal errors (e.g. SQL errors) to clients
// can leak implementation details. A hardened version logs the real error
// server-side and returns a generic message to the client.
func GeneralError(err error) Response {
	return Response{
		Status: StatusError,
		Error:  err.Error(),
	}
}

// ValidationError turns the validator package's per-field errors into one
// friendly, human-readable message.
func ValidationError(errs validator.ValidationErrors) Response {
	// A nil slice is fine to append to in Go (unlike a nil map). append grows it.
	var errMsgs []string

	// range iterates the slice; `_` discards the index since we only want each
	// individual field error.
	for _, err := range errs {
		// ActualTag() reports WHICH validation rule failed (required, gt, ...).
		// Go's switch needs no break; cases don't fall through by default.
		switch err.ActualTag() {
		case "required":
			// Field() is the struct field name that failed. Sprintf builds a string.
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is required field", err.Field()))
		default:
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is invalid", err.Field()))
		}
	}

	return Response{
		Status: StatusError,
		// Join glues all messages into one comma-separated string.
		Error: strings.Join(errMsgs, ", "),
	}
}

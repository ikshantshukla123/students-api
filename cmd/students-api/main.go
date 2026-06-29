// Command students-api is the entry point of the application.
//
// This file is the "composition root": the ONE place where all the pieces are
// constructed and wired together (config -> storage -> router -> handlers ->
// server). Keeping the wiring here is what lets the other packages stay
// decoupled and individually testable.
package main

import (
	"context" // carries deadlines/cancellation; used for graceful shutdown
	"log"     // Fatal logging (logs then os.Exit)
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ikshantshukla123/students-api/internal/config"
	"github.com/ikshantshukla123/students-api/internal/http/handlers/student"
	"github.com/ikshantshukla123/students-api/internal/storage/sqlite"
)

func main() {
	// 1) Load configuration (or exit). See internal/config.
	cfg := config.MustLoad()

	// 2) Build the storage layer. We get back a concrete *sqlite.Sqlite, but
	// everything downstream will use it through the storage.Storage interface.
	storage, err := sqlite.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("storage initialized", slog.String("env", cfg.Env), slog.String("version", "1.0.0"))

	// 3) Set up the router (request multiplexer). Since Go 1.22 the pattern can
	// include the HTTP METHOD, so only POST requests to this path match.
	// student.New(storage) runs now and returns the handler closure with the
	// storage dependency baked in.
	router := http.NewServeMux() //A ServeMux is a router (also called a "request multiplexer" — that's the "Mux"). Its one job: look at an incoming request's method + URL path, and decide which handler function to run.
 
	router.HandleFunc("POST /api/students", student.New(storage))
	// {id} is a path-parameter wildcard, read in the handler via r.PathValue("id").
	router.HandleFunc("GET /api/students/{id}", student.GetById(storage))
	router.HandleFunc("GET /api/students", student.GetList(storage))

	// 4) Build the server explicitly (rather than http.ListenAndServe) so we can
	// call Shutdown() later for a graceful stop. Production tip: also set
	// ReadTimeout/WriteTimeout/IdleTimeout here to defend against slow clients.
	server := http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}

	// 5) Prepare for graceful shutdown.
	// A buffered channel (capacity 1) to receive OS signals. The buffer matters:
	// signal.Notify sends non-blockingly, so the buffer ensures a signal that
	// arrives before we're parked on the receive is not dropped.
	done := make(chan os.Signal, 1)
	// Route these OS signals to our channel instead of letting them kill us:
	//   os.Interrupt / SIGINT -> Ctrl+C; SIGTERM -> docker/k8s "please stop".
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// 6) Run the server in a goroutine, because ListenAndServe BLOCKS forever.
	// If we called it directly, the shutdown code below would never be reached.
	go func() {
		slog.Info("server started", slog.String("address", cfg.Addr))

		err := server.ListenAndServe()
		// When Shutdown() is called, ListenAndServe returns ErrServerClosed —
		// that's the NORMAL "asked to stop" signal, not a real error, so we
		// ignore it. Any other error is a genuine startup failure.
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %s", err.Error())
		}
	}()

	// 7) Block here until an OS signal arrives. main idles efficiently while the
	// server keeps serving in its goroutine.
	<-done

	slog.Info("shutting down the server")

	// 8) Graceful shutdown: stop accepting new connections but give in-flight
	// requests up to 5 seconds to finish. context.WithTimeout enforces that
	// deadline so shutdown can't hang forever.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// cancel MUST always be called to release the context's resources; defer
	// guarantees it runs no matter how main returns.
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		slog.Error("failed to shutdown server", slog.String("error", err.Error()))
	}

	slog.Info("server shutdown successfully")
}

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ikshantshukla123/students-api/internal/config"
)


func main(){
	//load config
	cfg := config.MustLoad()
	//database setup
	//setup router
	 router := http.NewServeMux()
	 router.HandleFunc("GET /home",func(w http.ResponseWriter,r *http.Request){
		w.Write([]byte("welcome to students api"))
	 })

	 
	//setup server


	server := http.Server{
		Addr: cfg.Addr,
		Handler: router,
	}


done := make(chan os.Signal,1)
signal.Notify(done,os.Interrupt,syscall.SIGINT)

go func (){
	slog.Info("server started",slog.String("address",cfg.Addr))
	fmt.Printf("Server started %v", cfg.Addr)
	err:= server.ListenAndServe()
	if err != nil{
	log.Fatal("Failed to start server")
	}
}()
<-done

slog.Info("Shutting down the server")


ctx,cancel := context.WithTimeout(context.Background(),5 * time.Second)

defer cancel()

err := server.Shutdown(ctx)
if err != nil{
	slog.Error("Failed to shutdown server",slog.String("error",err.Error()))
}
slog.Info("Server shutdown successfully")
}

 
//use logger in place of println/print : log/slog
// Because logs are not just for printing messages. They help you:

// Debug errors
// Monitor servers
// Know what users are doing
// Find why your API crashed
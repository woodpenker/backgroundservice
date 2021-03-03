package main

import (
	"backgroundservice"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var flags = backgroundservice.Flags{
	BinPath: "nc",
	LogPath: "run.log",
	Args:    []string{"-l", "9999"},
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

	am := backgroundservice.New(backgroundservice.WithFlags(flags))
	go func() {
		err := am.Start()
		if err != nil {
			fmt.Println(err)
		}
	}()

	go func() {
		http.ListenAndServe(":8080", http.FileServer(http.Dir("/usr/share/doc")))
	}()

	<-stop
	err := am.Stop()
	if err != nil {
		fmt.Println(err)
	}
	os.Remove("run.log")

}

// ╰─$ go run example.go
// ^Cafter stop: false

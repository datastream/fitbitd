package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	s := SyncTask{
		exitChannel: make(chan int),
	}
	go s.Run()
	termchan := make(chan os.Signal, 1)
	signal.Notify(termchan, syscall.SIGINT, syscall.SIGTERM)
	<-termchan
	s.Stop()
}

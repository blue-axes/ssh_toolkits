package main

import (
	"github.com/creack/pty"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func handleResize(ptmx *os.File) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH
}

func getAccessTime(stat os.FileInfo) time.Time {
	switch val := stat.Sys().(type) {
	case syscall.Stat_t:
		return time.Unix(val.Atim.Sec, 0)
	}
	return time.Now()
}

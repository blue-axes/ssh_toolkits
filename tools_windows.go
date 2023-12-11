package main

import (
	"os"
	"syscall"
	"time"
)

func handleResize(ptmx *os.File) {

}

func getAccessTime(stat os.FileInfo) time.Time {
	switch val := stat.Sys().(type) {
	case *syscall.Win32FileAttributeData:
		return time.Unix(val.LastAccessTime.Nanoseconds()/1000000000, 0)
	}
	return time.Now()
}

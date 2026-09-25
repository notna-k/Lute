package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lute/worker/internal/joblog"
)

// showLog prints the last lines of path and, with follow, keeps printing what is appended.
func showLog(path string, lines int, follow bool) error {
	var offset int64
	if lines > 0 {
		res := joblog.ReadTail(path, lines, 0)
		if res.Err != "" {
			return errors.New(res.Err)
		}
		for _, line := range res.Lines {
			fmt.Println(line)
		}
		offset = res.FileSize
	}
	if !follow {
		return nil
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	return followFrom(path, offset, stop)
}

// followFrom copies path to stdout from offset as it grows, until stop fires.
// The file may not exist yet: setup creates it when it starts the agent.
func followFrom(path string, offset int64, stop <-chan os.Signal) error {
	for {
		f, err := os.Open(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			err = tail(f, offset, stop)
			_ = f.Close()
			return err
		}
		select {
		case <-stop:
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func tail(f *os.File, offset int64, stop <-chan os.Signal) error {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = os.Stdout.Write(buf[:n])
		}
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			select {
			case <-stop:
				return nil
			case <-time.After(300 * time.Millisecond):
			}
		}
	}
}

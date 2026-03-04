package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"bucket/internal/app"
	"bucket/internal/config"
)

var version = "dev"

func main() {
	exitCode := 0
	defer func() {
		if recovered := recover(); recovered != nil {
			logPath := "~/.config/bucket/log.txt"
			if resolved, err := config.LogPath(); err == nil {
				logPath = resolved
				_ = os.MkdirAll(filepathDir(logPath), 0o700)
				file, fileErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
				if fileErr == nil {
					_, _ = fmt.Fprintf(file, "panic: %v\n%s\n", recovered, debug.Stack())
					_ = file.Close()
				}
			}
			fmt.Fprintf(os.Stderr, "buckets crashed. See %s\n", logPath)
			exitCode = 1
		}
		os.Exit(exitCode)
	}()

	if err := app.Run(version); err != nil {
		fmt.Fprintf(os.Stderr, "buckets: %v\n", err)
		exitCode = 1
	}
}

func filepathDir(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' || path[index] == '\\' {
			if index == 0 {
				return string(path[0])
			}
			return path[:index]
		}
	}
	return "."
}

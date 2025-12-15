package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var Log *log.Logger

func init() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	logPath := filepath.Join(dir, "pandabot.log")

	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		Log = log.New(os.Stderr, "PandaBot: ", log.LstdFlags|log.Lmicroseconds)
		return
	}

	multi := io.MultiWriter(f, os.Stdout)
	Log = log.New(multi, "PandaBot: ", log.LstdFlags|log.Lmicroseconds)

	Log.Printf("PandaBot started - log initialized at %s\n", logPath)
}

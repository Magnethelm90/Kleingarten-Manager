//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealPath zeigt eine Datei im Dateimanager an (macOS: markiert im Finder, sonst der Ordner).
func revealPath(p string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-R", p)
	} else {
		cmd = exec.Command("xdg-open", filepath.Dir(p))
	}
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}

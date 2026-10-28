//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// revealPath öffnet den Ordner einer Datei im Explorer und markiert die Datei, damit sie sich
// bequem in eine E-Mail ziehen lässt. Die Befehlszeile wird selbst gebaut, weil der Explorer
// /select nur mit dem Pfad in eigenen Anführungszeichen versteht.
func revealPath(p string) {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + p + `"`}
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}

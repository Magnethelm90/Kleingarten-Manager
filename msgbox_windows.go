//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const mbIconError = 0x10

// fatalBox zeigt einen Fehler als Windows-Meldungsfenster an. Nötig, weil das
// Programmfenster kein Konsolenfenster mehr hat, in dem fmt.Println sichtbar wäre.
func fatalBox(msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	msgBoxW := user32.NewProc("MessageBoxW")
	title, errT := syscall.UTF16PtrFromString(appName)
	text, errM := syscall.UTF16PtrFromString(msg)
	if errT != nil || errM != nil {
		return
	}
	_, _, _ = msgBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
}

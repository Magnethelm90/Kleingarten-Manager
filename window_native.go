//go:build windows || darwin

package main

import (
	"errors"

	webview "github.com/webview/webview_go"
)

const hasNativeWindow = true

// runNativeWindow zeigt die Oberfläche in einem eigenen Programmfenster statt im
// Browser. Blockiert bis das Fenster geschlossen wird.
func runNativeWindow(url string) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle(appName)
	w.SetSize(1280, 860, webview.HintNone)
	// Externe Links nicht im Programmfenster laden (dort fehlen Adressleiste und
	// die Schutzanzeigen eines Browsers), sondern im Systembrowser öffnen.
	_ = w.Bind("kgmOpenExternal", func(target string) error {
		if !externalURLAllowed(target) {
			return errors.New("Adresse nicht erlaubt")
		}
		openBrowser(target)
		return nil
	})
	w.Navigate(url)
	w.Run()
}

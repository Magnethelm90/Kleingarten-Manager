//go:build windows || darwin

package main

import webview "github.com/webview/webview_go"

const hasNativeWindow = true

// runNativeWindow zeigt die Oberfläche in einem eigenen Programmfenster statt im
// Browser. Blockiert bis das Fenster geschlossen wird.
func runNativeWindow(url string) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle(appName)
	w.SetSize(1280, 860, webview.HintNone)
	w.Navigate(url)
	w.Run()
}

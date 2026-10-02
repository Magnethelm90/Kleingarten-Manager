//go:build !windows && !darwin

package main

// Auf anderen Plattformen (z. B. beim Entwickeln unter Linux) gibt es keine
// gebündelte WebView, dort bleibt es beim Öffnen im Standardbrowser.
const hasNativeWindow = false

func runNativeWindow(url string) {}

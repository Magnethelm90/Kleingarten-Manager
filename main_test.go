package main

import "testing"

func TestExternalURLAllowed(t *testing.T) {
	erlaubt := []string{
		"https://github.com/Magnethelm90/Kleingarten-Manager/releases/tag/v1.0",
		"https://github.com/Magnethelm90/Kleingarten-Manager/releases/latest",
	}
	for _, u := range erlaubt {
		if !externalURLAllowed(u) {
			t.Errorf("sollte erlaubt sein: %q", u)
		}
	}
	verboten := []string{
		"",
		"https://github.com/Magnethelm90/Kleingarten-Manager",
		"https://github.com/Andere/Kleingarten-Manager/releases/tag/v1.0",
		"https://github.com/Magnethelm90/Kleingarten-Manager-Fake/releases",
		"https://github.com.evil.example/Magnethelm90/Kleingarten-Manager/",
		"https://evil.example/https://github.com/Magnethelm90/Kleingarten-Manager/",
		"http://github.com/Magnethelm90/Kleingarten-Manager/releases",
		"file:///C:/Windows/System32/calc.exe",
		"ms-settings:",
		"javascript:alert(1)",
		"https://github.com/Magnethelm90/Kleingarten-Manager/releases\" & calc",
		"https://github.com/Magnethelm90/Kleingarten-Manager/releases calc.exe",
		"https://github.com/Magnethelm90/Kleingarten-Manager/\\..\\x",
		"https://github.com/Magnethelm90/Kleingarten-Manager/\nx",
	}
	for _, u := range verboten {
		if externalURLAllowed(u) {
			t.Errorf("sollte abgelehnt werden: %q", u)
		}
	}
}

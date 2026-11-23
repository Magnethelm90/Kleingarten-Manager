package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAushang(t *testing.T) {
	_, h := newTestApp(t)
	for _, in := range []map[string]any{
		{"titel": "Wasser abgestellt", "text": "Kurz.", "datum": "2026-04-01", "zusatz": "Rückfragen: Vorstand"},
		{"titel": strings.Repeat("Sehr lange Überschrift ", 5), "text": strings.Repeat("Viel Text äöü ß – \"<&>\" 🌱 ", 80)},
		{"titel": "Viele Zeilen", "text": strings.Repeat("Zeile\n", 300)},
	} {
		rec := do(h, "POST", "/api/admin/aushang", in)
		if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("%v: %d %s", in["titel"], rec.Code, rec.Body.String())
		}
	}
	for _, in := range []map[string]any{{"titel": "", "text": "x"}, {"titel": "x", "text": " "}, {"titel": "x", "text": "y", "datum": "kaputt"}} {
		if rec := do(h, "POST", "/api/admin/aushang", in); rec.Code != 400 {
			t.Errorf("%v: %d", in, rec.Code)
		}
	}
}

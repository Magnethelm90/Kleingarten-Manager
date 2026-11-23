package main

import (
	"bytes"
	"testing"
)

func TestMaengelListe(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Maengel = []Mangel{{ID: "m1", Gartennr: "2", Datum: "2026-05-01", Art: "Hecke", Beschreibung: "zu hoch"}, {ID: "m2", Gartennr: "10", Datum: "2026-05-01", Art: "Laube", Beschreibung: "Dach", ErledigtAm: "2026-06-01"}}
	app.st.mu.Unlock()
	if rec := do(h, "GET", "/api/admin/maengel-liste?ids=m1,m2", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Liste: %d", rec.Code)
	}
	for _, q := range []string{"", "?ids=", "?ids=gibtsnicht"} {
		if rec := do(h, "GET", "/api/admin/maengel-liste"+q, nil); rec.Code < 400 {
			t.Errorf("%q: %d", q, rec.Code)
		}
	}
}

package main

import (
	"bytes"
	"os"
	"testing"
)

func TestWillkommensmappeUndExpose(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	dir := os.Getenv("GA_PDF_DIR")
	pdf := func(name, pfad string) {
		t.Helper()
		rec := do(h, "GET", pfad, nil)
		if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if dir != "" {
			_ = os.WriteFile(dir+"/"+name+".pdf", rec.Body.Bytes(), 0o600)
		}
	}
	pdf("willkommen", "/api/admin/paechter/"+st.Paechter[2].ID+"/willkommen")
	pdf("expose", "/api/admin/expose?gartennr=11") // mit Wertermittlung
	pdf("expose3", "/api/admin/expose?gartennr=3")
	if rec := do(h, "GET", "/api/admin/expose?gartennr=999", nil); rec.Code != 404 {
		t.Errorf("unbekannter Garten: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/paechter/gibtesnicht/willkommen", nil); rec.Code != 404 {
		t.Errorf("unbekannter Pächter: %d", rec.Code)
	}
	// Texte speichern und bereinigen
	v := decode[Vereinsinfo](t, do(h, "PUT", "/api/admin/vereinsinfo", map[string]string{"ansprechpartner": "  A  \n\n B ", "hinweise": "x"}))
	if v.Ansprechpartner != "A\nB" {
		t.Errorf("Vereinsinfo: %q", v.Ansprechpartner)
	}
	// ohne Lageplan, Termine und Texte geht es auch
	_, h2 := newTestApp(t)
	p := decode[Paechter](t, do(h2, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "gartennr": "7", "name": "Ohne Alles", "gartengroesse": 300}))
	if rec := do(h2, "GET", "/api/admin/paechter/"+p.ID+"/willkommen", nil); rec.Code != 200 {
		t.Errorf("leere Mappe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h2, "GET", "/api/admin/expose?gartennr=7", nil); rec.Code != 200 {
		t.Errorf("Exposé ohne Wertermittlung: %d %s", rec.Code, rec.Body.String())
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	for _, pfad := range []string{"/api/admin/expose?gartennr=11", "/api/admin/paechter/" + st.Paechter[2].ID + "/willkommen", "/api/admin/vereinsinfo"} {
		if rec := do(h, "GET", pfad, nil); rec.Code != 401 {
			t.Errorf("%s ohne Anmeldung: %d", pfad, rec.Code)
		}
	}
}

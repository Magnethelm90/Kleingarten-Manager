package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestZaehlerErinnerung(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	for _, nr := range []string{"91", "92"} { // zwei Pächter mit Zählern, aber ohne aktuelle Stände
		if rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": nr, "gartennr": nr, "name": "Neu " + nr, "gartengroesse": 100,
			"wasserzaehlerNr": "W" + nr, "stromzaehlerNr": "S" + nr}); rec.Code != 200 {
			t.Fatalf("Pächter anlegen: %d %s", rec.Code, rec.Body.String())
		}
	}
	d := decode[erinnerungDaten](t, do(h, "GET", "/api/admin/zaehler-fehlen", nil))
	if d.Gesamt == 0 || len(d.Fehlend) == 0 || len(d.Fehlend) > d.Gesamt {
		t.Fatalf("fehlende Zählerstände: %+v", d)
	}
	for _, f := range d.Fehlend {
		if f.Gartennr == "" || len(f.Fehlende) == 0 {
			t.Errorf("Eintrag unvollständig: %+v", f)
		}
	}
	// Stand nachtragen: der Pächter verschwindet aus der Liste
	erster := d.Fehlend[0]
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	a := st.Ablesungen[erster.ID]
	w, s := 100.0, 200.0
	a.WasserAkt, a.StromAkt = &w, &s
	if rec := do(h, "PUT", "/api/ablesung/"+erster.ID, a); rec.Code != 200 {
		t.Fatalf("Ablesung speichern: %d %s", rec.Code, rec.Body.String())
	}
	d2 := decode[erinnerungDaten](t, do(h, "GET", "/api/admin/zaehler-fehlen", nil))
	if len(d2.Fehlend) != len(d.Fehlend)-1 {
		t.Errorf("nach dem Nachtragen: %d statt %d", len(d2.Fehlend), len(d.Fehlend)-1)
	}
	// Aushang: PDF, ohne Namen im Inhalt kaum prüfbar, aber ohne Fehler und nur für den Admin
	rec := do(h, "GET", "/api/admin/zaehler-aushang?frist=2030-05-31", nil)
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Aushang: %d %s", rec.Code, rec.Body.String())
	}
	if dir := os.Getenv("GA_PDF_DIR"); dir != "" {
		_ = os.WriteFile(dir+"/zaehler_aushang.pdf", rec.Body.Bytes(), 0o600)
	}
	// E-Mail: Empfänger im Bcc, Nachricht ohne Namen
	var urls []string
	alt := openMailFn
	openMailFn = func(u string) { urls = append(urls, u) }
	defer func() { openMailFn = alt }()
	for _, p := range st.Paechter { // allen eine Adresse geben
		if p.Email == "" {
			p.Email = strings.ToLower(strings.ReplaceAll(p.Name, " ", ".")) + "@example.org"
			do(h, "PUT", "/api/admin/paechter/"+p.ID, p)
		}
	}
	rec = do(h, "POST", "/api/admin/zaehler-mail?frist=2030-05-31", nil)
	if rec.Code != 200 || len(urls) != 1 {
		t.Fatalf("E-Mail: %d %s (%d Aufrufe)", rec.Code, rec.Body.String(), len(urls))
	}
	if !strings.HasPrefix(urls[0], "mailto:?subject=") || !strings.Contains(urls[0], "&bcc=") || !strings.Contains(urls[0], "%40example.org") || len(urls[0]) > maxMailtoLaenge {
		t.Errorf("mailto-Link: %s", urls[0])
	}
	if strings.Contains(urls[0], "To=") {
		t.Error("Empfänger gehören ins Bcc")
	}

	// alles gemeldet → kein Aushang nötig
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	for _, f := range d2.Fehlend {
		a := st.Ablesungen[f.ID]
		a.WasserAkt, a.StromAkt = &w, &s
		do(h, "PUT", "/api/ablesung/"+f.ID, a)
	}
	if rec := do(h, "GET", "/api/admin/zaehler-aushang", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("Aushang ohne fehlende Stände: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/zaehler-mail", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("E-Mail ohne Empfänger: %d", rec.Code)
	}

	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/zaehler-fehlen", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/zaehler-mail", nil); rec.Code != 401 {
		t.Errorf("Mail ohne Anmeldung: %d", rec.Code)
	}
}

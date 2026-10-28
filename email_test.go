package main

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestRechnungsMailText(t *testing.T) {
	s := defaultSettings()
	r := &Rechnung{Jahr: 2025, Nummer: "100-7", Settings: s, Paechter: Paechter{Anrede: "Frau", Name: "Erika Muster"}, Result: Result{Gesamt: 123.4}}
	betreff, text := rechnungsMail(r)
	if !strings.Contains(betreff, "2025") || !strings.Contains(betreff, "100-7") {
		t.Errorf("Betreff: %q", betreff)
	}
	for _, want := range []string{"Sehr geehrte Frau Erika Muster,", "123,40 €", "15.02.2026", s.IBAN, "Verwendungszweck: 100-7", s.VereinName} {
		if !strings.Contains(text, want) {
			t.Errorf("Text enthält %q nicht:\n%s", want, text)
		}
	}
	r.Result.Gesamt, r.Paechter.Anrede = -20, "Familie"
	_, text = rechnungsMail(r)
	if !strings.Contains(text, "Guten Tag Erika Muster,") || !strings.Contains(text, "Guthaben von 20,00 €") || strings.Contains(text, "IBAN") {
		t.Errorf("Guthaben-Text:\n%s", text)
	}
}

func TestMailtoKodierung(t *testing.T) {
	enc := mailtoEscape("Grüße & mehr?\nZeile 2 + 3 = 5 %")
	if strings.ContainsAny(enc, " &?+\n") {
		t.Fatalf("unkodierte Zeichen: %q", enc)
	}
	if !strings.Contains(enc, "%0D%0A") || !strings.Contains(enc, "%20") {
		t.Errorf("Zeilenumbruch/Leerzeichen: %q", enc)
	}
	if dec, err := url.PathUnescape(enc); err != nil || dec != "Grüße & mehr?\r\nZeile 2 + 3 = 5 %" {
		t.Errorf("Rückwandlung: %q %v", dec, err)
	}
}

func TestRechnungEmailVorbereiten(t *testing.T) {
	app, h := newTestApp(t)
	var gemailt, gezeigt []string
	oldMail, oldReveal := openMailFn, revealPathFn
	openMailFn = func(u string) { gemailt = append(gemailt, u) }
	revealPathFn = func(p string) { gezeigt = append(gezeigt, p) }
	t.Cleanup(func() { openMailFn, revealPathFn = oldMail, oldReveal })

	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna", "gartengroesse": 300}))
	abl := Ablesung{WasserVJ: fp(1), WasserAkt: fp(2), StromVJ: fp(1), StromAkt: fp(2), Stunden: fp(12)}
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	inf := st.Issued[p.ID]

	// ohne Adresse: verständliche Meldung, nichts wird geöffnet
	if rec := do(h, "POST", "/api/invoice-email/"+inf.ID, nil); rec.Code != 400 || !strings.Contains(rec.Body.String(), "keine E-Mail-Adresse") {
		t.Fatalf("ohne Adresse: %d %s", rec.Code, rec.Body.String())
	}
	if len(gemailt)+len(gezeigt) != 0 {
		t.Fatal("ohne Adresse darf nichts geöffnet werden")
	}

	// Adresse nachtragen: die ausgestellte Rechnung gilt deshalb nicht als geändert
	p.Email = "anna@example.org"
	do(h, "PUT", "/api/admin/paechter/"+p.ID, p)
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Issued[p.ID].Geaendert {
		t.Error("E-Mail-Adresse nachgetragen: Rechnung darf nicht als geändert gelten")
	}

	// PDF-Datei fehlt (z. B. von Hand gelöscht): wird neu erzeugt
	full, _ := app.rechnungsDatei(st.Archive[0].Datei)
	_ = os.Remove(full)
	rec := do(h, "POST", "/api/invoice-email/"+inf.ID, nil)
	if rec.Code != 200 {
		t.Fatalf("E-Mail vorbereiten: %d %s", rec.Code, rec.Body.String())
	}
	if len(gemailt) != 1 || !strings.HasPrefix(gemailt[0], "mailto:anna@example.org?subject=") || !strings.Contains(gemailt[0], "&body=") {
		t.Fatalf("mailto: %v", gemailt)
	}
	if len(gezeigt) != 1 || gezeigt[0] != full {
		t.Fatalf("angezeigte Datei: %v, erwartet %s", gezeigt, full)
	}
	if _, err := os.Stat(full); err != nil {
		t.Errorf("PDF wurde nicht neu erzeugt: %v", err)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Issued[p.ID].EmailAm == "" || st.Archive[0].EmailAm == "" {
		t.Error("Zeitpunkt der Vorbereitung wird nicht angezeigt")
	}

	// ersetzte Rechnung: nicht mehr verschicken
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{IDs: []string{p.ID}, Replace: true})
	if rec := do(h, "POST", "/api/invoice-email/"+inf.ID, nil); rec.Code != 400 {
		t.Errorf("ersetzte Rechnung: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/invoice-email/gibtsnicht", nil); rec.Code != 404 {
		t.Errorf("unbekannte Rechnung: %d", rec.Code)
	}
}

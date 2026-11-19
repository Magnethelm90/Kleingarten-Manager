package main

import (
	"strings"
	"testing"
	"time"
)

func TestAbschlussPruefung(t *testing.T) {
	app, h := newTestApp(t)
	hat := func(d abschlussAntwort, stufe, teil string) bool {
		for _, p := range d.Punkte {
			if p.Stufe == stufe && strings.Contains(p.Titel, teil) {
				return true
			}
		}
		return false
	}
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "A", "gartennr": "1", "gartengroesse": 300, "wasserzaehlerNr": "W1"}))
	d := decode[abschlussAntwort](t, do(h, "GET", "/api/admin/abschlusspruefung", nil))
	if d.Rot == 0 || !hat(d, "err", "fehlenden Angaben") {
		t.Fatalf("fehlende Zählerstände müssen rot sein: %+v", d)
	}
	if !hat(d, "warn", "Keine Sicherung außer Haus") {
		t.Fatalf("fehlende Sicherung außer Haus muss gelb sein: %+v", d)
	}
	for _, x := range d.Punkte {
		if x.Stufe == "ok" && x.Ziel != "" {
			t.Fatalf("grüne Punkte brauchen keinen Knopf: %+v", x)
		}
	}
	// vollständig, aber ohne Rechnung
	app.st.mu.Lock()
	jahr := app.st.d.Settings.Jahr
	null, zehn := 0.0, 10.0
	app.st.d.Jahre[yearKey(jahr)].Ablesungen[p.ID] = Ablesung{WasserVJ: &null, WasserAkt: &zehn, StromVJ: &null, StromAkt: &zehn, Stunden: &zehn}
	d = app.st.abschlussPruefungLocked(time.Now())
	voll := calculate(app.st.d.Settings, app.st.d.Paechter[0], app.st.d.Jahre[yearKey(jahr)].Ablesungen[p.ID]).Vollstaendig
	app.st.mu.Unlock()
	if !voll {
		t.Fatal("Testdaten müssen einen vollständigen Pächter ergeben")
	}
	if !hat(d, "err", "noch nicht ausgestellt") || hat(d, "err", "fehlenden Angaben") {
		t.Fatalf("Rechnung fehlt → rot, Angaben vollständig → nicht rot: %+v", d)
	}
	// negative Kasse
	if rec := do(h, "POST", "/api/admin/ausgaben", Ausgabe{Datum: "2025-03-01", Beschreibung: "Pumpe", Betrag: 500}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	d = decode[abschlussAntwort](t, do(h, "GET", "/api/admin/abschlusspruefung", nil))
	if !hat(d, "err", "Kassenbestand im Minus") || !hat(d, "warn", "ohne Beleg") {
		t.Fatalf("Kasse im Minus und Ausgabe ohne Beleg erwartet: %+v", d)
	}
}

func TestSicherungAnsehen(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika Alt", "gartennr": "7", "gartengroesse": 300})
	app.st.mu.Lock()
	app.st.snapshotBackup("test")
	namen := app.st.sicherungsNamen()
	app.st.mu.Unlock()
	if len(namen) == 0 {
		t.Fatal("keine Sicherung angelegt")
	}
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Neu Dazu", "gartennr": "8", "gartengroesse": 300})
	d := decode[sicherungInhalt](t, do(h, "POST", "/api/admin/sicherungen/ansehen", map[string]string{"name": namen[0]}))
	if len(d.Paechter) != 1 || d.Paechter[0][1] != "Erika Alt" || d.Zahlen[2][1] != "1" || d.Heute[2][1] != "2" {
		t.Fatalf("Inhalt der Sicherung falsch: %+v", d)
	}
	// Ansehen ändert nichts
	app.st.mu.Lock()
	n := len(app.st.d.Paechter)
	app.st.mu.Unlock()
	if n != 2 {
		t.Fatal("Ansehen darf den Bestand nicht verändern")
	}
	// nur Namen aus dem Sicherungsordner
	for _, name := range []string{"../" + dataFileName, "/etc/passwd", "gibtsnicht.json", ""} {
		if rec := do(h, "POST", "/api/admin/sicherungen/ansehen", map[string]string{"name": name}); rec.Code != 404 {
			t.Errorf("%q: %d", name, rec.Code)
		}
	}
}

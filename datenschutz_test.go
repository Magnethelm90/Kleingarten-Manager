package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFristAbgelaufen(t *testing.T) {
	cases := []struct {
		ausgestellt, aktuell int
		want                 bool
	}{
		{2025, 2035, false}, // Frist endet am 31.12.2035
		{2025, 2036, true},
		{2010, 2026, true},
		{2026, 2026, false},
		{0, 2026, false}, // unlesbares Datum: lieber behalten als versehentlich löschen
	}
	for _, c := range cases {
		if got := fristAbgelaufen(c.ausgestellt, c.aktuell); got != c.want {
			t.Errorf("fristAbgelaufen(%d, %d) = %v, erwartet %v", c.ausgestellt, c.aktuell, got, c.want)
		}
	}
}

func TestDatenauskunft(t *testing.T) {
	app, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
		"mitgliedsnr": "35-95", "gartennr": "35", "name": "Erika Beispiel", "strasse": "Gartenweg 2", "gartengroesse": 300, "notiz": "ruft nur abends an"}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20)})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})

	rec := do(h, "GET", "/api/admin/paechter/"+p.ID+"/auskunft", nil)
	if rec.Code != 200 {
		t.Fatalf("Auskunft: %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("Auskunft sollte als Datei herunterladbar sein: %q", rec.Header().Get("Content-Disposition"))
	}
	a := decode[auskunft](t, rec)
	if a.Stammdaten.Name != "Erika Beispiel" || a.Stammdaten.Notiz != "ruft nur abends an" || a.Stammdaten.Strasse != "Gartenweg 2" {
		t.Errorf("Stammdaten unvollständig: %+v", a.Stammdaten)
	}
	if len(a.Jahre) != 1 || a.Jahre[0].Jahr != 2025 || a.Jahre[0].Zaehler.WasserAkt == nil {
		t.Errorf("Zählerstände fehlen: %+v", a.Jahre)
	}
	if len(a.Rechnungen) != 1 || a.Rechnungen[0].Gesamtbetrag <= 0 {
		t.Errorf("Rechnung fehlt: %+v", a.Rechnungen)
	}
	if len(a.Garten) != 1 {
		t.Errorf("Garten-Historie fehlt: %+v", a.Garten)
	}
	if len(a.Protokoll) == 0 {
		t.Error("Eintrag im Änderungsprotokoll fehlt")
	}

	if rec := do(h, "GET", "/api/admin/paechter/gibtsnicht/auskunft", nil); rec.Code != 404 {
		t.Errorf("unbekannter Pächter: %d, erwartet 404", rec.Code)
	}

	// mit Passwort nur für den Admin
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/paechter/"+p.ID+"/auskunft", nil); rec.Code != 401 {
		t.Errorf("Auskunft ohne Anmeldung: %d, erwartet 401", rec.Code)
	}
	for _, path := range []string{"/api/admin/datenschutz"} {
		if rec := do(h, "GET", path, nil); rec.Code != 401 {
			t.Errorf("%s ohne Anmeldung: %d, erwartet 401", path, rec.Code)
		}
	}
	if rec := do(h, "POST", "/api/admin/datenschutz/bereinigen", nil); rec.Code != 401 {
		t.Errorf("Bereinigen ohne Anmeldung: %d, erwartet 401", rec.Code)
	}
	_ = app
}

func TestEndgueltigEntferntPersonenbezug(t *testing.T) {
	app, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
		"mitgliedsnr": "35-95", "gartennr": "35", "name": "Erika Beispiel", "gartengroesse": 300, "notiz": "GEHEIMNOTIZ-PAECHTER"}))
	abl := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20)}
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	abl.Hinweis = "GEHEIMNOTIZ-HINWEIS" // erst nach der Rechnung, ein Hinweis füllt die Rechnungsseite
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	do(h, "PUT", "/api/admin/paechter/"+p.ID, map[string]any{"mitgliedsnr": "35-95", "gartennr": "35", "name": "Erika Beispiel", "gartengroesse": 300, "notiz": "GEHEIMNOTIZ-PAECHTER"})
	do(h, "DELETE", "/api/admin/paechter/"+p.ID, nil)

	rec := do(h, "DELETE", "/api/admin/paechter/"+p.ID+"/endgueltig", nil)
	if rec.Code != 200 {
		t.Fatalf("endgültig löschen: %d %s", rec.Code, rec.Body)
	}

	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	d := app.st.d
	if len(d.Paechter) != 0 {
		t.Errorf("Stammdatensatz noch vorhanden: %+v", d.Paechter)
	}
	for _, e := range d.GartenHistorie {
		if strings.Contains(e.Name, "Erika") || e.Mitgliedsnr != "" {
			t.Errorf("Garten-Historie enthält noch Personenbezug: %+v", e)
		}
	}
	for _, e := range d.AuditLog {
		if strings.Contains(e.Aktion, "Erika") || strings.Contains(e.Aktion, "35-95") {
			t.Errorf("Änderungsprotokoll enthält noch Personenbezug: %q", e.Aktion)
		}
	}
	if len(d.AuditLog) == 0 || !strings.Contains(d.AuditLog[len(d.AuditLog)-1].Aktion, "endgültig gelöscht") {
		t.Error("das Löschen selbst muss (ohne Namen) protokolliert sein")
	}
	// Rechnung bleibt wegen der Aufbewahrungspflicht, interne Notizen nicht
	if len(d.Rechnungen) != 1 || d.Rechnungen[0].Paechter.Name != "Erika Beispiel" {
		t.Errorf("Rechnung muss bis zum Fristende unverändert bleiben: %+v", d.Rechnungen)
	}
	if d.Rechnungen[0].Paechter.Notiz != "" {
		t.Error("interne Notiz darf nicht in der aufbewahrten Rechnung bleiben")
	}
	if a := d.Jahre["2025"].Ablesungen[p.ID]; a.Hinweis != "" {
		t.Errorf("Hinweistext der Ablesung noch vorhanden: %q", a.Hinweis)
	}
	// in keiner Sicherung darf der Stammdatensatz oder die Notiz überleben
	dateien := app.st.sicherungsDateien()
	if len(dateien) == 0 {
		t.Fatal("Test braucht mindestens eine Sicherung")
	}
	for _, f := range dateien {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "GEHEIMNOTIZ") {
			t.Errorf("%s enthält noch Notizen der gelöschten Person", filepath.Base(f))
		}
		var bd Data
		if err := json.Unmarshal(raw, &bd); err != nil {
			t.Fatalf("%s nicht mehr lesbar: %v", filepath.Base(f), err)
		}
		for _, bp := range bd.Paechter {
			if bp.ID == p.ID {
				t.Errorf("%s enthält noch den Stammdatensatz", filepath.Base(f))
			}
		}
	}
}

func TestBereinigungNachAufbewahrungsfrist(t *testing.T) {
	app, h := newTestApp(t)
	mk := func(nr, name string) string {
		return decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": nr, "name": name, "gartengroesse": 300})).ID
	}
	alt, neu := mk("1", "Alt Pächter"), mk("2", "Neu Pächter")
	abl := Ablesung{WasserVJ: fp(1), WasserAkt: fp(2), StromVJ: fp(1), StromAkt: fp(2), Stunden: fp(20)}
	do(h, "PUT", "/api/ablesung/"+alt+"?year=2025", abl)
	do(h, "PUT", "/api/ablesung/"+neu+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})

	app.st.mu.Lock()
	var altRechnung *Rechnung
	for _, r := range app.st.d.Rechnungen {
		if r.PaechterID == alt {
			altRechnung = r
			r.Ausgestellt = "2010-01-05T10:00:00+01:00"
		}
	}
	datei := filepath.Join(app.st.dir, filepath.FromSlash(altRechnung.Datei))
	altID := altRechnung.ID
	app.st.mu.Unlock()
	if _, err := os.Stat(datei); err != nil {
		t.Fatalf("PDF der Rechnung sollte als Datei existieren: %v", err)
	}

	info := decode[datenschutzInfo](t, do(h, "GET", "/api/admin/datenschutz", nil))
	if info.FaelligRechnungen != 1 || info.AufbewahrungJahre != aufbewahrungJahre || info.NaechsteFrist == 0 {
		t.Fatalf("Übersicht: %+v", info)
	}

	res := decode[map[string]int](t, do(h, "POST", "/api/admin/datenschutz/bereinigen", nil))
	if res["rechnungen"] != 1 || res["dateien"] != 1 {
		t.Fatalf("Bereinigung: %+v", res)
	}
	if _, err := os.Stat(datei); !os.IsNotExist(err) {
		t.Error("PDF-Datei der abgelaufenen Rechnung muss gelöscht sein")
	}
	app.st.mu.Lock()
	for _, r := range app.st.d.Rechnungen {
		if r.PaechterID == alt && (!r.Bereinigt || r.Paechter.Name != fristAbgelaufenText || r.Paechter.Strasse != "" || r.Datei != "") {
			t.Errorf("abgelaufene Rechnung nicht bereinigt: %+v", r.Paechter)
		}
		if r.PaechterID == neu && (r.Bereinigt || r.Paechter.Name != "Neu Pächter") {
			t.Errorf("Rechnung innerhalb der Frist darf nicht angefasst werden: %+v", r.Paechter)
		}
	}
	app.st.mu.Unlock()

	if rec := do(h, "GET", "/api/archive/"+altID+".pdf", nil); rec.Code != 410 {
		t.Errorf("bereinigte Rechnung drucken: %d, erwartet 410", rec.Code)
	}
	// zweiter Lauf: nichts mehr zu tun
	if res := decode[map[string]int](t, do(h, "POST", "/api/admin/datenschutz/bereinigen", nil)); res["rechnungen"] != 0 {
		t.Errorf("zweiter Lauf sollte nichts mehr ändern: %+v", res)
	}
}

func TestBereinigungJahresunterlagen(t *testing.T) {
	d := &Data{Jahre: map[string]*Jahr{
		"2010": {Abgeschlossen: true, Paechter: []Paechter{{ID: "a", Name: "Alt", Strasse: "Weg 1"}}, Ablesungen: map[string]Ablesung{"a": {Hinweis: "privat"}}},
		"2025": {Abgeschlossen: true, Paechter: []Paechter{{ID: "b", Name: "Jung"}}, Ablesungen: map[string]Ablesung{}},
		"2026": {Paechter: nil, Ablesungen: map[string]Ablesung{}},
	}}
	_, jahre, _ := d.bereinigeAbgelaufene(2026)
	if jahre != 1 {
		t.Fatalf("Jahre bereinigt: %d, erwartet 1", jahre)
	}
	if p := d.Jahre["2010"].Paechter[0]; p.Name != fristAbgelaufenText || p.Strasse != "" {
		t.Errorf("Jahreskopie 2010 nicht anonymisiert: %+v", p)
	}
	if d.Jahre["2010"].Ablesungen["a"].Hinweis != "" {
		t.Error("Hinweistext 2010 nicht entfernt")
	}
	if d.Jahre["2025"].Paechter[0].Name != "Jung" {
		t.Error("Jahr 2025 liegt innerhalb der Frist und darf nicht angefasst werden")
	}
	if _, jahre, _ := d.bereinigeAbgelaufene(2026); jahre != 0 {
		t.Error("zweiter Lauf muss idempotent sein")
	}
}

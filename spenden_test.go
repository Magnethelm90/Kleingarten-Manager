package main

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestBetragInWorten(t *testing.T) {
	for b, want := range map[float64]string{
		1: "ein Euro", 2: "zwei Euro", 11: "elf Euro", 21: "einundzwanzig Euro", 100: "einhundert Euro", 101: "einhunderteins Euro",
		250.5: "zweihundertfünfzig Euro fünfzig Cent", 1000: "eintausend Euro", 1234.56: "eintausendzweihundertvierunddreißig Euro sechsundfünfzig Cent",
		0.01: "null Euro ein Cent", 1000001: "einemillioneins Euro", 2500000: "zweimillionenfünfhunderttausend Euro", 31: "einunddreißig Euro",
	} {
		if got := betragInWorten(b); got != want {
			t.Errorf("%v: %q, erwartet %q", b, got, want)
		}
	}
}

func TestGemeinnuetzigkeitPruefen(t *testing.T) {
	heute := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	g := Gemeinnuetzigkeit{Anschrift: "Am Garten 1\n12345 Musterstadt", Zweck: "der Kleingärtnerei", Finanzamt: "Musterstadt", Steuernummer: "12/345/67890",
		Bescheid: "freistellung", BescheidDatum: "2022-03-01", Veranlagung: "2019–2021"}
	if err := pruefeGemeinnuetzigkeit(g, heute); err != nil {
		t.Errorf("gültiger Freistellungsbescheid: %v", err)
	}
	alt := g
	alt.BescheidDatum = "2021-03-01"
	if pruefeGemeinnuetzigkeit(alt, heute) == nil {
		t.Error("Freistellungsbescheid älter als 5 Jahre muss abgelehnt werden")
	}
	fest := g
	fest.Bescheid, fest.Veranlagung, fest.BescheidDatum = "feststellung", "", "2023-10-01"
	if pruefeGemeinnuetzigkeit(fest, heute) == nil {
		t.Error("Feststellung nach § 60a AO älter als 3 Jahre muss abgelehnt werden")
	}
	fest.BescheidDatum = "2024-01-15"
	if err := pruefeGemeinnuetzigkeit(fest, heute); err != nil {
		t.Errorf("gültige Feststellung: %v", err)
	}
	unvollst := g
	unvollst.Steuernummer = ""
	if pruefeGemeinnuetzigkeit(unvollst, heute) == nil {
		t.Error("ohne Steuernummer")
	}
}

func TestSpendenUndZuwendungsbestaetigung(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	p := st.Paechter[0]
	jahr := time.Now().Year()
	tag := func(m, d int) string {
		return time.Date(jahr, time.Month(m), d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	neu := func(body map[string]any) Spende {
		rec := do(h, "POST", "/api/admin/spenden", body)
		if rec.Code != 200 {
			t.Fatalf("Spende anlegen: %d %s", rec.Code, rec.Body.String())
		}
		return decode[Spende](t, rec)
	}
	s1 := neu(map[string]any{"datum": tag(3, 1), "betrag": 150, "name": p.Name, "strasse": "Weg 1", "plzOrt": "12345 Musterstadt", "paechterId": p.ID})
	s2 := neu(map[string]any{"datum": tag(6, 15), "betrag": 49.9, "name": p.Name, "strasse": "Weg 1", "plzOrt": "12345 Musterstadt", "paechterId": p.ID, "verzicht": true})
	fremd := neu(map[string]any{"datum": tag(5, 2), "betrag": 20, "name": "Bäckerei Korn", "strasse": "Markt 2", "plzOrt": "12345 Musterstadt"})
	ohneAdresse := neu(map[string]any{"datum": tag(5, 3), "betrag": 5, "name": "Anonym"})
	if s1.Jahr != st.CurrentYear {
		t.Errorf("Kassenjahr: %d, erwartet %d", s1.Jahr, st.CurrentYear)
	}
	for name, body := range map[string]map[string]any{
		"ohne Betrag": {"datum": tag(1, 1), "betrag": 0, "name": "X"},
		"ohne Name":   {"datum": tag(1, 1), "betrag": 5},
		"ohne Datum":  {"betrag": 5, "name": "X"},
	} {
		if rec := do(h, "POST", "/api/admin/spenden", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	// Kassenbericht zählt die Spenden als Einnahme
	app.st.mu.Lock()
	v, _ := app.st.viewLocked(st.CurrentYear)
	k := app.st.kassenberichtLocked(v)
	app.st.mu.Unlock()
	if k.Spenden != 224.9+170 || round2(k.Kassenbestand-k.Anfangsbestand-k.EinnahmenBezahlt+k.GuthabenAusgezahlt+k.AusgabenSumme) != k.Spenden {
		t.Errorf("Kassenbericht: Spenden %v, Bestand %v", k.Spenden, k.Kassenbestand)
	}

	// ohne Angaben zur Gemeinnützigkeit keine Bestätigung (die Beispieldaten bringen erfundene mit, also leeren)
	do(h, "PUT", "/api/admin/spenden/gemeinnuetzigkeit", map[string]any{})
	if rec := do(h, "POST", "/api/admin/spenden/bestaetigen", map[string]any{"ids": []string{s1.ID}}); rec.Code != http.StatusBadRequest {
		t.Errorf("ohne Gemeinnützigkeit: %d", rec.Code)
	}
	g := map[string]any{"anschrift": "Am Garten 1\n12345 Musterstadt", "zweck": "der Kleingärtnerei", "finanzamt": "Musterstadt", "steuernummer": "12/345/67890",
		"bescheid": "freistellung", "bescheidDatum": time.Now().AddDate(-1, 0, 0).Format("2006-01-02"), "veranlagung": "2022–2024"}
	if rec := do(h, "PUT", "/api/admin/spenden/gemeinnuetzigkeit", g); rec.Code != 200 {
		t.Fatalf("Gemeinnützigkeit: %d %s", rec.Code, rec.Body.String())
	}
	// Einzel- und Sammelbestätigung
	dir := os.Getenv("GA_PDF_DIR")
	for name, ids := range map[string]string{"einzel": s1.ID, "sammel": s1.ID + "," + s2.ID} {
		rec := do(h, "GET", "/api/admin/zuwendungsbestaetigung?ids="+ids, nil)
		if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if dir != "" {
			_ = os.WriteFile(dir+"/zuwendung_"+name+".pdf", rec.Body.Bytes(), 0o600)
		}
	}
	for name, ids := range map[string]string{"anderer Spender": s1.ID + "," + fremd.ID, "ohne Anschrift": ohneAdresse.ID, "unbekannt": "gibtesnicht", "doppelt": s1.ID + "," + s1.ID} {
		if rec := do(h, "GET", "/api/admin/zuwendungsbestaetigung?ids="+ids, nil); rec.Code == 200 {
			t.Errorf("%s: Bestätigung hätte abgelehnt werden müssen", name)
		}
	}
	// ausstellen: danach sind Betrag und Löschen gesperrt
	if rec := do(h, "POST", "/api/admin/spenden/bestaetigen", map[string]any{"ids": []string{s1.ID, s2.ID}}); rec.Code != 200 {
		t.Fatalf("bestätigen: %d %s", rec.Code, rec.Body.String())
	}
	s1.Betrag = 999
	if rec := do(h, "PUT", "/api/admin/spenden/"+s1.ID, s1); rec.Code != http.StatusBadRequest {
		t.Errorf("Betrag nach Bestätigung geändert: %d", rec.Code)
	}
	s1.Betrag, s1.Notiz = 150, "Dank per Brief"
	if rec := do(h, "PUT", "/api/admin/spenden/"+s1.ID, s1); rec.Code != 200 {
		t.Errorf("Notiz nach Bestätigung: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/admin/spenden/"+s1.ID, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("bestätigte Spende gelöscht: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/admin/spenden/"+ohneAdresse.ID, nil); rec.Code != 200 {
		t.Errorf("unbestätigte Spende löschen: %d", rec.Code)
	}
	// Datenschutz: Auskunft enthält die Spenden; nach dem Löschen der Person bleiben bestätigte erhalten
	app.st.mu.Lock()
	a, _ := app.st.auskunftLocked(p.ID)
	app.st.d.entfernePerson(p.ID)
	var rest []Spende
	for _, x := range app.st.d.Spenden {
		if x.ID == s1.ID || x.ID == s2.ID {
			rest = append(rest, x)
		}
	}
	app.st.mu.Unlock()
	if len(a.Spenden) != 2 {
		t.Errorf("Auskunft: %d Spenden", len(a.Spenden))
	}
	if len(rest) != 2 || rest[0].PaechterID != "" || rest[0].Name != p.Name {
		t.Errorf("bestätigte Spenden nach dem Löschen der Person: %+v", rest)
	}
	// nach Ablauf der Aufbewahrungsfrist ohne Personenbezug
	app.st.mu.Lock()
	n := app.st.d.spendenBereinigen(jahr + aufbewahrungJahre + 1)
	app.st.mu.Unlock()
	if n != 5 { // drei eigene und die zwei Spenden aus den Beispieldaten
		t.Errorf("bereinigt: %d", n)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/spenden", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}

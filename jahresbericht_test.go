package main

import (
	"bytes"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestJahresbericht(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	do(h, "POST", "/api/invoices/issue", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	for _, a := range st.Archive[:5] {
		do(h, "PUT", "/api/payment/"+a.ID, map[string]any{"bezahltAm": time.Now().Format("2006-01-02")})
	}
	app.st.mu.Lock()
	d, ok := app.st.jahresberichtLocked(st.Year, time.Now())
	app.st.mu.Unlock()
	if !ok || d.Paechter != len(demoListe) || d.Flaeche <= 0 || d.Stunden <= 0 || d.K.EinnahmenBezahlt <= 0 || d.Offen <= 0 || d.Warteliste != 3 {
		t.Fatalf("Jahresbericht-Daten: %+v", d)
	}
	if d.PflichtOK > d.MitStunden || d.MitStunden != len(demoListe)-1 {
		t.Errorf("Pflichtstunden: %d von %d", d.PflichtOK, d.MitStunden)
	}
	rec := do(h, "GET", "/api/admin/jahresbericht?year="+strconv.Itoa(st.Year), nil)
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("PDF: %d", rec.Code)
	}
	// nach dem Jahreswechsel: Vorjahresvergleich und Verlauf
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	app.st.mu.Lock()
	d2, _ := app.st.jahresberichtLocked(st.Year+1, time.Now())
	app.st.mu.Unlock()
	if d2.VK == nil || len(d2.Verlauf) != 2 || d2.Verlauf[0].Jahr != st.Year {
		t.Errorf("Vorjahr/Verlauf: %+v %+v", d2.VK, d2.Verlauf)
	}
	if rec := do(h, "GET", "/api/admin/jahresbericht?year="+strconv.Itoa(st.Year+1), nil); rec.Code != 200 {
		t.Errorf("Folgejahr: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/jahresbericht", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}

func TestVeraenderung(t *testing.T) {
	for _, c := range []struct {
		a, v float64
		want string
	}{{110, 100, "+10 % ggü. 2024"}, {90, 100, "-10 % ggü. 2024"}, {100.2, 100, "wie 2024"}, {0, 100, ""}, {50, 0, ""}} {
		if got := veraenderung(c.a, c.v, 2024); got != c.want {
			t.Errorf("%v/%v: %q, erwartet %q", c.a, c.v, got, c.want)
		}
	}
}

func TestFreienGartenVergebenUndLageplanDruck(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "gartennr": "1", "name": "Erna Eins", "gartengroesse": 200})
	do(h, "PUT", "/api/admin/lageplan", map[string]any{"positionen": map[string]any{"1": map[string]int{"x": 20, "y": 20}, "2": map[string]int{"x": 160, "y": 20}}})
	b := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Zora Zwei", "email": "zora@example.org", "seit": "2025-01-01"}))
	if st := decode[stateResp](t, do(h, "GET", "/api/state", nil)); st.WartelisteAnzahl != 1 {
		t.Errorf("Warteliste-Anzahl im Zustand: %d", st.WartelisteAnzahl)
	}

	// Aushang: ohne Namen; mit Namen nur auf Wunsch
	ohne := do(h, "GET", "/api/lageplan-druck", nil)
	mit := do(h, "GET", "/api/lageplan-druck?namen=1", nil)
	if ohne.Code != 200 || mit.Code != 200 || !bytes.HasPrefix(ohne.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Lageplan-Druck: %d %d", ohne.Code, mit.Code)
	}

	// belegter Garten lässt sich nicht vergeben
	if rec := do(h, "POST", "/api/admin/warteliste/"+b.ID+"/vergeben", map[string]any{"gartennr": "1", "mitgliedsnr": "2", "gartengroesse": 150}); rec.Code != 400 || !strings.Contains(rec.Body.String(), "Erna Eins") {
		t.Errorf("belegter Garten: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "POST", "/api/admin/warteliste/"+b.ID+"/vergeben", map[string]any{"gartennr": "2", "mitgliedsnr": "1", "gartengroesse": 150}); rec.Code != 400 {
		t.Errorf("doppelte Mitgliedsnummer: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/warteliste/"+b.ID+"/vergeben", map[string]any{"gartennr": "2", "mitgliedsnr": "2"}); rec.Code != 400 {
		t.Errorf("ohne Größe: %d", rec.Code)
	}
	rec := do(h, "POST", "/api/admin/warteliste/"+b.ID+"/vergeben", map[string]any{"gartennr": "2", "mitgliedsnr": "2", "gartengroesse": 150, "seit": "2025-05-01"})
	if rec.Code != 200 {
		t.Fatalf("Vergeben: %d %s", rec.Code, rec.Body.String())
	}
	p := decode[Paechter](t, rec)
	if p.Name != "Zora Zwei" || p.Email != "zora@example.org" || p.Versand != "Emailsendung" || p.Gartennr != "2" {
		t.Errorf("neuer Pächter: %+v", p)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.WartelisteAnzahl != 0 || len(st.Paechter) != 2 {
		t.Errorf("nach dem Vergeben: %d auf der Liste, %d Pächter", st.WartelisteAnzahl, len(st.Paechter))
	}
	hist := decode[[]GartenEintrag](t, do(h, "GET", "/api/garten-historie?gartennr=2", nil))
	if len(hist) != 1 || hist[0].Seit != "2025-05-01" {
		t.Errorf("Gartenhistorie: %+v", hist)
	}

	// mit Passwort: Druck ohne Anmeldung nie mit Namen
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	req := httptest.NewRequest("GET", "/api/lageplan-druck?namen=1", nil)
	if app.lageplanMitNamen(req) {
		t.Error("Namen ohne Anmeldung")
	}
	if rec := do(h, "GET", "/api/lageplan-druck?namen=1", nil); rec.Code != 200 {
		t.Errorf("Aushang ohne Anmeldung: %d", rec.Code)
	}
	app.st.mu.Lock()
	for _, g := range app.st.lageplanGaertenLocked(false) {
		if g.Name != "" {
			t.Errorf("Name im Aushang: %+v", g)
		}
	}
	app.st.mu.Unlock()
}

func TestLageplanGaertenFrei(t *testing.T) {
	st, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st.d.Paechter = []Paechter{{ID: "a", Gartennr: "1", Name: "A"}, {ID: "b", Gartennr: "2", Name: "B", AusgeschiedenAm: "2025-01-01"}, {ID: "c", Gartennr: "3", Name: "C", Geloescht: true}}
	st.d.Lageplan = map[string]LagePos{"1": {X: 0, Y: 0}, "2": {X: 140, Y: 0}, "3": {X: 280, Y: 0}}
	frei := map[string]bool{}
	for _, g := range st.lageplanGaertenLocked(true) {
		frei[g.Nr] = g.Frei
	}
	if frei["1"] || !frei["2"] || !frei["3"] {
		t.Errorf("freie Gärten: %v", frei)
	}
}

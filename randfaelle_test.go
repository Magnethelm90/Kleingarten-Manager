package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Zwei Übergaben am selben Garten im selben Jahr (A → B → C): jeder bekommt seine Schlussrechnung.
func TestPaechterwechselKette(t *testing.T) {
	_, h := newTestApp(t)
	a := wechselAnlegen(t, h)
	req := wechselReqFuer(a)
	req["neu"].(map[string]any)["mitgliedsnr"] = "8"
	b := decode[struct{ Neu Paechter }](t, do(h, "POST", "/api/admin/paechterwechsel", req)).Neu
	if b.ID == "" {
		t.Fatal("erster Wechsel fehlgeschlagen")
	}
	req2 := map[string]any{
		"altId": b.ID, "datum": "2025-09-01", "wasserStand": 140, "stromStand": 1300, "stunden": 2,
		"neu": map[string]any{"mitgliedsnr": "9", "name": "Dritter Pächter"},
	}
	rec := do(h, "POST", "/api/admin/paechterwechsel", req2)
	if rec.Code != 200 {
		t.Fatalf("zweiter Wechsel: %d %s", rec.Code, rec.Body.String())
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if r := st.Results[b.ID]; !r.Vollstaendig || r.PachtGarten != 0 || *r.WasserVerbrauch != 10 {
		t.Errorf("Schlussrechnung des Zwischenpächters: %+v", r)
	}
	// beide ausgeschiedenen Pächter lassen sich abrechnen; der dritte hat noch keine Zählerstände
	rec = do(h, "POST", "/api/invoices/issue", issueReq{})
	if res := decode[struct {
		Created []string
		Skipped []issueSkip
	}](t, rec); len(res.Created) != 2 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Name, "Dritter") {
		t.Errorf("Ausstellen: %s", rec.Body.String())
	}
	// Die Übergabe (Juni 2025) liegt vor dem Eintragen des ersten Pächters (heute): kein Zeitraum mit Ende vor Beginn
	hist := decode[[]GartenEintrag](t, do(h, "GET", "/api/garten-historie?gartennr=7", nil))
	if len(hist) != 3 || hist[2].Name != "Dritter Pächter" || hist[2].Bis != "" {
		t.Fatalf("Gartenhistorie: %+v", hist)
	}
	for _, e := range hist {
		if e.Bis != "" && e.Bis < e.Seit {
			t.Errorf("Ende vor Beginn: %+v", e)
		}
	}
}

// Jahreswechsel, bevor die Schlussrechnung ausgestellt wurde: sie lässt sich danach im alten Jahr noch ausstellen.
func TestSchlussrechnungNachJahreswechsel(t *testing.T) {
	_, h := newTestApp(t)
	a := wechselAnlegen(t, h)
	req := wechselReqFuer(a)
	req["neu"].(map[string]any)["mitgliedsnr"] = "8"
	do(h, "POST", "/api/admin/paechterwechsel", req)
	if rec := do(h, "POST", "/api/admin/jahreswechsel", nil); rec.Code != 200 {
		t.Fatalf("Jahreswechsel: %d", rec.Code)
	}
	rec := do(h, "POST", "/api/invoices/issue?year=2025", issueReq{IDs: []string{a}})
	if rec.Code != 200 || len(decode[struct{ Created []string }](t, rec).Created) != 1 {
		t.Fatalf("Schlussrechnung im Vorjahr: %d %s", rec.Code, rec.Body.String())
	}
	old := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if info := old.Issued[a]; info.Geaendert || info.Gesamt <= 0 {
		t.Errorf("ausgestellte Schlussrechnung im abgeschlossenen Jahr: %+v", info)
	}
	if rec := do(h, "GET", "/api/invoice/"+a+".pdf?year=2025", nil); rec.Code != 200 {
		t.Errorf("Vorschau im abgeschlossenen Jahr: %d", rec.Code)
	}
}

// Die neuen Zugänge verlangen Admin (bis auf die absichtlich öffentlichen Termine und den Aushang).
func TestNeueZugaengeRechte(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	admin := []struct{ m, u string }{
		{"POST", "/api/admin/paechterwechsel"}, {"GET", "/api/admin/uebergaben"}, {"GET", "/api/admin/uebergabe/x/protokoll"},
		{"POST", "/api/admin/uebergabe/x/protokoll/oeffnen"}, {"GET", "/api/admin/warteliste"}, {"POST", "/api/admin/warteliste"},
		{"PUT", "/api/admin/warteliste/x"}, {"DELETE", "/api/admin/warteliste/x"}, {"POST", "/api/admin/warteliste/x/vergeben"},
		{"POST", "/api/admin/termine"}, {"PUT", "/api/admin/termine/x"}, {"DELETE", "/api/admin/termine/x"},
		{"POST", "/api/admin/termine-uebernehmen"}, {"GET", "/api/admin/jahresbericht"}, {"POST", "/api/admin/jahresbericht/oeffnen"},
		{"GET", "/api/admin/gartenakte?gartennr=1"}, {"POST", "/api/admin/dokumente"}, {"GET", "/api/admin/dokumente/x"}, {"DELETE", "/api/admin/dokumente/x"},
		{"POST", "/api/admin/wertermittlungen"}, {"PUT", "/api/admin/wertermittlungen/x"}, {"DELETE", "/api/admin/wertermittlungen/x"},
		{"GET", "/api/admin/wertermittlungen/x/pdf"}, {"GET", "/api/admin/jahresmeldung"}, {"POST", "/api/admin/warteliste/x/vergeben"},
	}
	for _, c := range admin {
		if rec := do(h, c.m, c.u, map[string]any{}); rec.Code != 401 {
			t.Errorf("%s %s ohne Anmeldung: %d", c.m, c.u, rec.Code)
		}
	}
	// öffentlich, aber ohne Personendaten
	for _, u := range []string{"/api/termine.ics", "/api/termine-druck?jahr=2026", "/api/lageplan-druck"} {
		if rec := do(h, "GET", u, nil); rec.Code == 401 || rec.Code == 403 {
			t.Errorf("%s sollte öffentlich sein: %d", u, rec.Code)
		}
	}
	// Schreibzugriff ohne Schutzheader (CSRF) wird abgelehnt
	req := httptest.NewRequest("POST", "/api/admin/termine", strings.NewReader(`{"titel":"x","datum":"2026-01-01"}`))
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.routes(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Errorf("POST ohne X-GA-Request: %d", rec.Code)
	}
}

// Zählerstände dürfen die Schluss-Markierung nicht fälschen und der Import darf sie nicht entfernen.
func TestSchlussMarkierungGeschuetzt(t *testing.T) {
	app, h := newTestApp(t)
	a := wechselAnlegen(t, h)
	// von außen gesendetes »schluss« beim normalen Speichern der Ablesung wird ignoriert
	do(h, "PUT", "/api/ablesung/"+a, map[string]any{"wasserVJ": 100, "wasserAkt": 110, "stromVJ": 1000, "stromAkt": 1100, "stunden": 12, "schluss": "2025-01-01"})
	app.st.mu.Lock()
	got := app.st.d.Jahre["2025"].Ablesungen[a].Schluss
	app.st.mu.Unlock()
	if got != "" {
		t.Errorf("Schluss-Markierung von außen gesetzt: %q", got)
	}
	// ebenso »ausgeschiedenAm« beim Anlegen
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "50", "name": "X", "gartengroesse": 100, "ausgeschiedenAm": "2025-01-01", "geloescht": true}))
	if p.AusgeschiedenAm != "" || p.Geloescht {
		t.Errorf("Anlegen übernimmt interne Felder: %+v", p)
	}
}

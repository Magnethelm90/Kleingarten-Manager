package main

import (
	"bytes"
	"net/http"
	"strconv"
	"testing"
)

func TestArbeitseinsatzStunden(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	var einsatz, fest Termin
	for _, x := range st.Termine {
		if x.Titel == "Frühjahrsarbeitseinsatz" {
			einsatz = x
		}
		if x.Art == "Fest" {
			fest = x
		}
	}
	v := decode[einsatzAnsicht](t, do(h, "GET", "/api/admin/termine/"+einsatz.ID+"/stunden", nil))
	if v.Jahr != st.CurrentYear || v.Dauer != 4 || len(v.Zeilen) != len(demoListe) || v.Gebucht {
		t.Fatalf("Ansicht: jahr %d dauer %v zeilen %d", v.Jahr, v.Dauer, len(v.Zeilen))
	}
	stunden := func(pid string) float64 {
		s := decode[stateResp](t, do(h, "GET", "/api/state", nil))
		if a := s.Ablesungen[pid].Stunden; a != nil {
			return *a
		}
		return 0
	}
	p1, p2 := v.Zeilen[0].PaechterID, v.Zeilen[1].PaechterID
	b1, b2 := stunden(p1), stunden(p2)
	jahr := v.Jahr
	speichern := func(e []map[string]any) *bytes.Buffer {
		rec := do(h, "PUT", "/api/admin/termine/"+einsatz.ID+"/stunden", map[string]any{"jahr": jahr, "eintraege": e})
		if rec.Code != 200 {
			t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body
	}
	speichern([]map[string]any{{"paechterId": p1, "stunden": 4}, {"paechterId": p2, "stunden": 4}})
	if stunden(p1) != b1+4 || stunden(p2) != b2+4 {
		t.Errorf("nach dem Buchen: %v %v (vorher %v %v)", stunden(p1), stunden(p2), b1, b2)
	}
	// Korrektur: nur der Unterschied wird gebucht, nichts zählt doppelt
	speichern([]map[string]any{{"paechterId": p1, "stunden": 3}})
	if stunden(p1) != b1+3 || stunden(p2) != b2 {
		t.Errorf("nach der Korrektur: %v %v (vorher %v %v)", stunden(p1), stunden(p2), b1, b2)
	}
	v = decode[einsatzAnsicht](t, do(h, "GET", "/api/admin/termine/"+einsatz.ID+"/stunden", nil))
	if !v.Gebucht || v.Zeilen[0].Stunden == nil || *v.Zeilen[0].Stunden != 3 || v.Zeilen[1].Stunden != nil {
		t.Errorf("Ansicht nach dem Buchen: %+v", v.Zeilen[:2])
	}
	// ungültige Eingaben
	for name, body := range map[string]map[string]any{
		"zu viele Stunden": {"jahr": jahr, "eintraege": []map[string]any{{"paechterId": p1, "stunden": 25}}},
		"negativ":          {"jahr": jahr, "eintraege": []map[string]any{{"paechterId": p1, "stunden": -1}}},
		"fremder Pächter":  {"jahr": jahr, "eintraege": []map[string]any{{"paechterId": "gibtesnicht", "stunden": 2}}},
		"unbekanntes Jahr": {"jahr": 1999, "eintraege": []map[string]any{}},
	} {
		if rec := do(h, "PUT", "/api/admin/termine/"+einsatz.ID+"/stunden", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if stunden(p1) != b1+3 {
		t.Errorf("abgelehnte Eingabe hat gebucht: %v", stunden(p1))
	}
	if rec := do(h, "GET", "/api/admin/termine/"+fest.ID+"/stunden", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("Fest statt Arbeitseinsatz: %d", rec.Code)
	}
	// Stundenliste zum Ausdrucken
	if rec := do(h, "GET", "/api/admin/termine/"+einsatz.ID+"/stundenliste", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("Stundenliste: %d", rec.Code)
	}
	// nach dem Jahreswechsel ist das alte Jahr gesperrt
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	if rec := do(h, "PUT", "/api/admin/termine/"+einsatz.ID+"/stunden", map[string]any{"jahr": jahr + 1, "eintraege": []map[string]any{}}); rec.Code != http.StatusBadRequest {
		t.Errorf("Änderung im abgeschlossenen Jahr: %d", rec.Code)
	}
	// Termin löschen: die Stunden bleiben gebucht
	if rec := do(h, "DELETE", "/api/admin/termine/"+einsatz.ID, nil); rec.Code != 200 {
		t.Fatalf("löschen: %d", rec.Code)
	}
	alt := decode[stateResp](t, do(h, "GET", "/api/state?year="+strconv.Itoa(jahr), nil))
	if a := alt.Ablesungen[p1].Stunden; a == nil || *a != b1+3 {
		t.Errorf("Stunden nach dem Löschen des Termins: %v", a)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/termine/"+fest.ID+"/stunden", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}

func TestTerminDauer(t *testing.T) {
	for _, c := range []struct {
		von, bis string
		want     float64
	}{{"09:00", "13:00", 4}, {"09:30", "12:15", 2.75}, {"10:00", "", 0}, {"", "", 0}} {
		if got := terminDauer(Termin{Von: c.von, Bis: c.bis}); got != c.want {
			t.Errorf("%s–%s: %v", c.von, c.bis, got)
		}
	}
}

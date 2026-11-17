package main

import (
	"net/url"
	"testing"
)

func TestWahlAuswerten(t *testing.T) {
	fall := []struct {
		name string
		in   wahlEingabe
		ok   bool
		wer  string
	}{
		{"einer angenommen", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 10}}, Nein: 3}, true, "A"},
		{"einer abgelehnt", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 3}}, Nein: 3}, false, "A"},
		{"einer absolut verfehlt", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 6}}, Nein: 2, Enthaltung: 5, Absolut: true}, false, "A"},
		{"zwei klar", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 7}, {"B", 12}}}, true, "B"},
		{"zwei gleich", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 7}, {"B", 7}}}, false, ""},
		{"drei absolut verfehlt", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 8}, {"B", 7}, {"C", 6}}, Absolut: true}, false, "A"},
		{"keine Stimmen", wahlEingabe{Kandidaten: []wahlKandidat{{"A", 0}}}, false, ""},
		{"kein Kandidat", wahlEingabe{}, false, ""},
	}
	for _, f := range fall {
		wer, _, ok, hinweis := wahlAuswerten(f.in)
		if ok != f.ok || wer != f.wer || (!ok && hinweis == "") {
			t.Errorf("%s: ok=%v wer=%q hinweis=%q", f.name, ok, wer, hinweis)
		}
	}
}

func TestWahlHandler(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Aemter = []Amt{{ID: "a1", Amt: "Kassenwart/in", Name: "Alt Person", Seit: "2020-03-01"}}
	vorher := len(app.st.d.Beschluesse)
	app.st.mu.Unlock()
	ein := map[string]any{"amt": "Kassenwart/in", "amtId": "a1", "datum": "2026-03-15", "uebernehmen": true, "amtszeit": 3,
		"kandidaten": []map[string]any{{"name": "Neu Person", "stimmen": 12}}, "nein": 2, "enthaltung": 1, "ungueltig": 0}
	res := decode[wahlErgebnis](t, do(h, "POST", "/api/admin/wahl", ein))
	if !res.Ok || res.Gewaehlt != "Neu Person" || res.Beschluss == "" || !res.AmtEingetr || res.Protokoll == "" {
		t.Fatalf("%+v", res)
	}
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	if len(app.st.d.Beschluesse) != vorher+1 {
		t.Fatal("Beschluss fehlt")
	}
	var alt, neu *Amt
	for i := range app.st.d.Aemter {
		if app.st.d.Aemter[i].Name == "Alt Person" {
			alt = &app.st.d.Aemter[i]
		}
		if app.st.d.Aemter[i].Name == "Neu Person" {
			neu = &app.st.d.Aemter[i]
		}
	}
	if alt == nil || !alt.Ehemalig || alt.Bis != "2026-03-15" || neu == nil || neu.Bis != "2029-03-15" || neu.Checkliste == nil {
		t.Fatalf("Amtswechsel: %+v %+v", alt, neu)
	}
}

func TestWahlWiederwahlUndFehler(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Aemter = []Amt{{ID: "a1", Amt: "Vorsitz", Name: "Dieselbe Person", Seit: "2020-03-01", Bis: "2026-03-01"}}
	app.st.mu.Unlock()
	do(h, "POST", "/api/admin/wahl", map[string]any{"amt": "Vorsitz", "amtId": "a1", "datum": "2026-03-15", "uebernehmen": true, "amtszeit": 2,
		"kandidaten": []map[string]any{{"name": "dieselbe person", "stimmen": 9}}, "nein": 1})
	app.st.mu.Lock()
	if len(app.st.d.Aemter) != 1 || app.st.d.Aemter[0].Bis != "2028-03-15" {
		t.Fatalf("Wiederwahl: %+v", app.st.d.Aemter)
	}
	app.st.mu.Unlock()
	for _, schlecht := range []map[string]any{
		{"amt": "", "datum": "2026-03-15", "kandidaten": []map[string]any{{"name": "A", "stimmen": 1}}},
		{"amt": "X", "datum": "kein Datum", "kandidaten": []map[string]any{{"name": "A", "stimmen": 1}}},
		{"amt": "X", "datum": "2026-03-15", "kandidaten": []map[string]any{}},
		{"amt": "X", "datum": "2026-03-15", "kandidaten": []map[string]any{{"name": "A", "stimmen": -1}}},
	} {
		if rec := do(h, "POST", "/api/admin/wahl", schlecht); rec.Code < 400 {
			t.Fatalf("angenommen: %v", schlecht)
		}
	}
	q := url.Values{"amt": {"Vorsitz"}, "kandidat": {"A", "B"}, "anzahl": {"9"}}
	if rec := do(h, "GET", "/api/admin/wahl/stimmzettel?"+q.Encode(), nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("Stimmzettel: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/wahl/stimmzettel?amt=X", nil); rec.Code < 400 {
		t.Fatal("ohne Kandidaten angenommen")
	}
}

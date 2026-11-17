package main

import "testing"

func TestWunschPrueft(t *testing.T) {
	passt, nicht := wunschPrueft("klein, mit Laube, gern nahe am Eingang", 200, true)
	if len(passt) != 2 || len(nicht) != 0 {
		t.Fatalf("%v %v", passt, nicht)
	}
	passt, nicht = wunschPrueft("großer Garten ohne Laube", 200, true)
	if len(nicht) != 2 || len(passt) != 0 {
		t.Fatalf("%v %v", passt, nicht)
	}
	if p, n := wunschPrueft("nahe am Eingang", 300, false); len(p)+len(n) != 0 {
		t.Fatal("kein Wunsch zu prüfen")
	}
	if p, n := wunschPrueft("klein", 0, false); len(p)+len(n) != 0 {
		t.Fatal("ohne Größe nichts prüfbar")
	}
}

func TestFreierVorschlag(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "p8", Name: "Vorher", Gartennr: "8", Gartengroesse: 400})
	app.st.d.Bauwerke = append(app.st.d.Bauwerke, Bauwerk{ID: "b1", Gartennr: "8", Art: "Laube", Flaeche: 20})
	app.st.d.Warteliste = []Bewerber{
		{ID: "w1", Seit: "2020-01-01", Name: "Erster", Wunsch: "klein"},
		{ID: "w2", Seit: "2021-01-01", Name: "Zweiter", Wunsch: "groß mit Laube"},
		{ID: "w3", Seit: "2019-01-01", Name: "Absage", BesichtigungErgebnis: "kein Interesse"},
	}
	app.st.mu.Unlock()
	v := decode[freierVorschlag](t, do(h, "GET", "/api/admin/freie-gaerten/8/vorschlag", nil))
	if v.Groesse != 400 || !v.Laube || len(v.Zeilen) != 2 {
		t.Fatalf("%+v", v)
	}
	if v.Zeilen[0].Name != "Erster" || v.Zeilen[0].Empfohlen || v.Zeilen[1].Name != "Zweiter" || !v.Zeilen[1].Empfohlen {
		t.Fatalf("Reihenfolge/Empfehlung: %+v", v.Zeilen)
	}
}

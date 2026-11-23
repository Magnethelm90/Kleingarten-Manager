package main

import (
	"fmt"
	"io/fs"
	"strconv"
	"testing"
	"time"
)

func TestNaechsterGeburtstag(t *testing.T) {
	heute := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	tage, alter, ok := naechsterGeburtstag(heute, "1956-11-20")
	if !ok || tage != 2 || alter != 70 {
		t.Fatalf("%d %d %v", tage, alter, ok)
	}
	if tage, _, _ := naechsterGeburtstag(heute, "1956-11-18"); tage != 0 {
		t.Fatalf("heute: %d", tage)
	}
	if tage, alter, _ := naechsterGeburtstag(heute, "1956-11-17"); tage != 364 || alter != 71 {
		t.Fatalf("Vorjahr: %d %d", tage, alter)
	}
	if _, _, ok := naechsterGeburtstag(heute, "kaputt"); ok {
		t.Fatal("ungültig")
	}
	// 29. Februar im Nichtschaltjahr: 28.
	if tage, _, _ := naechsterGeburtstag(time.Date(2027, 2, 26, 0, 0, 0, 0, time.UTC), "1960-02-29"); tage != 2 {
		t.Fatalf("Schalttag: %d", tage)
	}
}

func TestWartungsplanUndGeburtstag(t *testing.T) {
	app, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika Muster", "gartennr": "1", "gartengroesse": 300}))
	if p.ID == "" {
		t.Fatal("Pächter nicht angelegt")
	}
	// Wartungsplan
	heute := time.Now().Format("2006-01-02")
	w := decode[Wartungsaufgabe](t, do(h, "POST", "/api/admin/wartungsplan", map[string]any{"titel": "Hecke schneiden", "monate": 6, "faellig": heute}))
	if w.ID == "" {
		t.Fatal("nicht angelegt")
	}
	if rec := do(h, "POST", "/api/admin/wartungsplan", map[string]any{"titel": "", "monate": 6}); rec.Code != 400 {
		t.Errorf("leerer Name: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/wartungsplan", map[string]any{"titel": "x", "monate": 0}); rec.Code != 400 {
		t.Errorf("Abstand 0: %d", rec.Code)
	}
	e := decode[Wartungsaufgabe](t, do(h, "POST", "/api/admin/wartungsplan/"+w.ID+"/erledigt", map[string]any{"datum": "2026-01-31"}))
	if e.Faellig != "2026-07-31" {
		t.Errorf("Fälligkeit %s", e.Faellig)
	}
	// Geburtstag nur mit Einwilligung
	do(h, "PUT", "/api/admin/paechter/"+p.ID+"/geburtstag", map[string]any{"datum": "1956-11-20", "einwilligung": false})
	app.st.mu.Lock()
	if len(app.st.d.Geburtstage) != 0 {
		t.Error("ohne Einwilligung darf nichts gespeichert werden")
	}
	app.st.mu.Unlock()
	do(h, "PUT", "/api/admin/paechter/"+p.ID+"/geburtstag", map[string]any{"datum": "1956-11-20", "einwilligung": true})
	app.st.mu.Lock()
	g := app.st.d.Geburtstage[p.ID]
	n := 0
	app.st.geburtstagAufgabenLocked(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), func(_, _, _, _, _ string) { n++ })
	app.st.mu.Unlock()
	if !g.Einwilligung || n != 1 {
		t.Errorf("Geburtstag %+v, Hinweise %d", g, n)
	}
	// Löschen entfernt das Datum
	app.st.mu.Lock()
	weg := app.st.d.geburtstagPersonEntfernen(p.ID)
	app.st.mu.Unlock()
	if !weg {
		t.Error("Datum nicht entfernt")
	}
}

func TestHaushaltHinweis(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika Muster", "gartennr": "1", "gartengroesse": 300})
	app.st.mu.Lock()
	jahr := app.st.d.Settings.Jahr
	app.st.mu.Unlock()
	kat := AusgabenKategorien[0]
	do(h, "PUT", "/api/admin/haushalt?year="+strconv.Itoa(jahr), map[string]any{"ausgaben": map[string]float64{kat: 100}})
	do(h, "POST", "/api/admin/ausgaben?year="+strconv.Itoa(jahr), map[string]any{"datum": strconv.Itoa(jahr) + "-05-01", "beschreibung": "x", "kategorie": kat, "betrag": 95})
	app.st.mu.Lock()
	var stufen []string
	app.st.haushaltAufgabenLocked(time.Now(), func(stufe, _, _, _, _ string) { stufen = append(stufen, stufe) })
	app.st.mu.Unlock()
	if len(stufen) != 1 || stufen[0] != "info" {
		t.Fatalf("90 %%: %v", stufen)
	}
}

func TestProbeAutomatisch(t *testing.T) {
	app, _ := newTestApp(t)
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	jetzt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if app.st.probeAutomatischLocked(jetzt) {
		t.Fatal("ohne Daten keine Probe")
	}
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "p1", Name: "A"})
	if !app.st.probeAutomatischLocked(jetzt) {
		t.Fatal("erste Probe fehlt")
	}
	if app.st.probeAutomatischLocked(jetzt.AddDate(0, 0, 10)) {
		t.Fatal("zu früh wiederholt")
	}
	if !app.st.probeAutomatischLocked(jetzt.AddDate(0, 0, 31)) {
		t.Fatal("nach 31 Tagen nicht wiederholt")
	}
}

func TestEinFehler(t *testing.T) {
	ja := [][2]string{{"meyer", "meier"}, {"hekce", "hecke"}, {"hecke", "heckee"}, {"hecke", "hece"}, {"hecke", "hecke"}}
	nein := [][2]string{{"hecke", "hacke2"}, {"hecke", "heckeee"}, {"hecke", "wiese"}, {"hecke", "hcekk"}}
	for _, p := range ja {
		if !einFehler(p[0], p[1]) {
			t.Errorf("%v sollte passen", p)
		}
	}
	for _, p := range nein {
		if einFehler(p[0], p[1]) {
			t.Errorf("%v sollte nicht passen", p)
		}
	}
	if tippfehlerTreffer("hecke am hauptweg", "hekce") != true || tippfehlerTreffer("hecke", "hek") {
		t.Error("Treffer")
	}
}

func TestEinstellungsVerlaufUndSelbsttest(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	s := app.st.d.Settings
	app.st.mu.Unlock()
	neu := s
	neu.WasserPreis = s.WasserPreis + 1
	if rec := do(h, "PUT", "/api/admin/settings", neu); rec.Code != 200 {
		t.Fatalf("speichern %d %s", rec.Code, rec.Body.String())
	}
	l := decode[[]einstellungsZeile](t, do(h, "GET", "/api/admin/einstellungen/verlauf", nil))
	if len(l) != 1 || len(l[0].Aenderungen) != 1 {
		t.Fatalf("Verlauf %+v", l)
	}
	if rec := do(h, "POST", "/api/admin/einstellungen/verlauf/0/zurueck", nil); rec.Code != 200 {
		t.Fatalf("zurück %d %s", rec.Code, rec.Body.String())
	}
	app.st.mu.Lock()
	if app.st.d.Settings.WasserPreis != s.WasserPreis {
		t.Error("Preis nicht zurückgesetzt")
	}
	if len(app.st.d.EinstellungsVerlauf) != 2 {
		t.Errorf("Verlauf muss das Zurücknehmen selbst merken: %d", len(app.st.d.EinstellungsVerlauf))
	}
	// Selbsttest: neue Installation = nur Version merken; mit Daten und anderer Version = Test.
	app.st.d.LetzteVersion = ""
	if !app.st.selbsttestLocked(time.Now(), webFS) || app.st.d.Selbsttest != nil {
		t.Error("neue Installation")
	}
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "p", Name: "A"})
	app.st.d.LetzteVersion = "0.0.1"
	sub, _ := fs.Sub(webFS, "web")
	if !app.st.selbsttestLocked(time.Now(), sub) || app.st.d.Selbsttest == nil || app.st.d.Selbsttest.Version != appVersion {
		t.Error("Selbsttest nach Update")
	}
	if app.st.selbsttestLocked(time.Now(), sub) {
		t.Error("nur einmal je Version")
	}
	app.st.mu.Unlock()
}

// Der älteste Stand bei vollem Verlauf lässt sich zurücknehmen, ohne dass das Programm abstürzt.
func TestEinstellungsVerlaufAeltesterStand(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	s := app.st.d.Settings
	app.st.mu.Unlock()
	for i := 1; i <= maxEinstellungsStaende+2; i++ {
		neu := s
		neu.WasserPreis = s.WasserPreis + float64(i)
		if rec := do(h, "PUT", "/api/admin/settings", neu); rec.Code != 200 {
			t.Fatalf("speichern %d %s", rec.Code, rec.Body.String())
		}
	}
	letzter := maxEinstellungsStaende - 1
	if rec := do(h, "POST", fmt.Sprintf("/api/admin/einstellungen/verlauf/%d/zurueck", letzter), nil); rec.Code != 200 {
		t.Fatalf("zurück %d %s", rec.Code, rec.Body.String())
	}
}

package main

import (
	"strings"
	"testing"
	"time"
)

func TestVorstandsCockpit(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	heute := time.Now()
	tag := func(n int) string { return heute.AddDate(0, 0, n).Format("2006-01-02") }
	app.st.mu.Lock()
	app.st.d.Termine = []Termin{
		{ID: "a", Datum: tag(10), Titel: "Ablesung", Art: "Ablesung"},
		{ID: "v", Datum: tag(20), Titel: "Jahreshauptversammlung", Art: "Versammlung"},
		{ID: "e", Datum: tag(-5), Titel: "Herbsteinsatz", Art: "Arbeitseinsatz"},
		{ID: "e2", Datum: tag(-6), Titel: "gebuchter Einsatz", Art: "Arbeitseinsatz"},
		{ID: "x", Datum: tag(60), Titel: "Sommerfest", Art: "Fest"},
	}
	app.st.d.Einsatzstunden = []EinsatzStunden{{TerminID: "e2", Jahr: app.st.d.Settings.Jahr, PaechterID: "p", Stunden: 2}}
	app.st.d.Gemeinnuetzigkeit = Gemeinnuetzigkeit{Steuernummer: "1", BescheidDatum: heute.AddDate(-5, 2, 0).Format("2006-01-02"), Bescheid: "freistellung"}
	l := app.st.aufgabenLocked(heute)
	app.st.mu.Unlock()
	titel := []string{}
	for _, a := range l {
		titel = append(titel, a.Stufe+": "+a.Titel)
	}
	alle := strings.Join(titel, "\n")
	for _, erwartet := range []string{"Zählerablesung in 10 Tagen", "Jahreshauptversammlung in 20 Tagen", "Stunden vom Herbsteinsatz eintragen",
		"Mängelfrist abgelaufen", "Spenden ohne Bestätigung", "Bescheid zur Gemeinnützigkeit läuft bald ab", "Warteliste nachfragen"} {
		if !strings.Contains(alle, erwartet) {
			t.Errorf("fehlt: %q\n%s", erwartet, alle)
		}
	}
	for _, nicht := range []string{"gebuchter Einsatz", "Sommerfest"} {
		if strings.Contains(alle, nicht) {
			t.Errorf("sollte nicht erscheinen: %q", nicht)
		}
	}
	if l[0].Stufe == "info" {
		t.Error("Warnungen gehören nach oben")
	}
	rec := do(h, "GET", "/api/admin/aufgaben", nil)
	if rec.Code != 200 {
		t.Errorf("Endpunkt: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/aufgaben", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}

func TestCockpitLeererVerein(t *testing.T) {
	_, h := newTestApp(t)
	if l := decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben", nil)); len(l) != 0 {
		t.Errorf("leerer Verein: %+v", l)
	}
}

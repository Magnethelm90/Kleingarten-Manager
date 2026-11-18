package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestKassenvorlagen(t *testing.T) {
	_, h := newTestApp(t)
	if rec := do(h, "PUT", "/api/admin/kassenvorlagen", []KassenVorlage{{Beschreibung: "Versicherung", Kategorie: "Versicherung & Gebühren", Betrag: 120}}); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	v := decode[[]KassenVorlage](t, do(h, "GET", "/api/admin/kassenvorlagen", nil))
	if len(v) != 1 || v[0].ID == "" || v[0].Betrag != 120 {
		t.Fatalf("Vorlagen: %+v", v)
	}
	if rec := do(h, "PUT", "/api/admin/kassenvorlagen", []KassenVorlage{{Beschreibung: " ", Betrag: 1}}); rec.Code != 400 {
		t.Errorf("leere Beschreibung: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/admin/kassenvorlagen", []KassenVorlage{{Beschreibung: "x", Betrag: -5}}); rec.Code != 400 {
		t.Errorf("negativer Betrag: %d", rec.Code)
	}
}

func TestDatencheck(t *testing.T) {
	a, h := newTestApp(t)
	a.st.mu.Lock()
	a.st.d.Paechter = []Paechter{
		{ID: "a", Mitgliedsnr: "1", Gartennr: "5", Name: "Anna Muster", Gartengroesse: 300, Versand: "Postversand", Strasse: "X", PLZOrt: "1 Y"},
		{ID: "b", Mitgliedsnr: "1", Gartennr: "5", Name: "anna  muster", Gartengroesse: 0, Versand: "Postversand"},
		{ID: "c", Mitgliedsnr: "3", Gartennr: "7", Name: "Bert", Gartengroesse: 200, Email: "kaputt", Versand: "Emailsendung"},
	}
	a.st.d.Lageplan = map[string]LagePos{"5": {}, "9": {}}
	a.st.d.Mandate = []Mandat{{ID: "m", PaechterID: "c", IBAN: "DE00"}}
	a.st.mu.Unlock()
	r := decode[datencheckErgebnis](t, do(h, "GET", "/api/admin/datencheck", nil))
	arten := map[string]int{}
	var alle []string
	for _, f := range r.Funde {
		arten[f.Art]++
		alle = append(alle, f.Text)
	}
	text := strings.Join(alle, "\n")
	for _, w := range []string{"Der Name kommt mehrfach vor", "Mitgliedsnummer 1", "Garten 5 hat mehrere", "Garten 9", "IBAN im Lastschrift-Mandat", "sieht nicht richtig aus", "Gartengröße"} {
		if !strings.Contains(text, w) {
			t.Errorf("Fund fehlt: %q\n%s", w, text)
		}
	}
	if r.Geprueft != 3 {
		t.Errorf("geprüft: %d", r.Geprueft)
	}
}

func TestDatencheckSauber(t *testing.T) {
	_, h := newTestApp(t)
	r := decode[datencheckErgebnis](t, do(h, "GET", "/api/admin/datencheck", nil))
	if r.Funde == nil || len(r.Funde) != 0 {
		t.Fatalf("leerer Bestand: %+v", r)
	}
}

func TestAbgleich(t *testing.T) {
	a, h := newTestApp(t)
	a.st.mu.Lock()
	a.st.d.Termine = []Termin{{ID: "t1", Datum: "2026-05-01", Titel: "Einsatz"}}
	a.st.mu.Unlock()
	rec := do(h, "GET", "/api/admin/abgleich", nil)
	if rec.Code != 200 {
		t.Fatalf("Export: %d", rec.Code)
	}
	d := decode[abgleichDatei](t, rec)
	if d.Kennung != abgleichKennung || len(d.Termine) != 1 {
		t.Fatalf("Export: %+v", d)
	}
	// Zweiter Rechner: neuer Termin und Beschluss, ein vorhandener Termin
	d.Termine = append(d.Termine, Termin{ID: "t2", Datum: "2026-06-01", Titel: "Fest"})
	d.Beschluesse = []Beschluss{{ID: "b1", Datum: "2026-03-01", Gremium: "Vorstand", Nummer: "V 2026/1", Titel: "Zaun"}}
	v := decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich", d))
	if v.Neu != 2 || v.Angewandt {
		t.Fatalf("Vorschau: %+v", v)
	}
	a.st.mu.Lock()
	n := len(a.st.d.Termine)
	a.st.mu.Unlock()
	if n != 1 {
		t.Fatalf("Vorschau hat verändert: %d Termine", n)
	}
	v = decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich?anwenden=1", d))
	if v.Neu != 2 || !v.Angewandt {
		t.Fatalf("Übernahme: %+v", v)
	}
	a.st.mu.Lock()
	nt, nb := len(a.st.d.Termine), len(a.st.d.Beschluesse)
	a.st.mu.Unlock()
	if nt != 2 || nb != 1 {
		t.Fatalf("Termine %d Beschlüsse %d", nt, nb)
	}
	// zweimal einlesen ändert nichts
	v = decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich?anwenden=1", d))
	if v.Neu != 0 {
		t.Errorf("doppelt übernommen: %+v", v)
	}
	// falsche Datei
	if rec := do(h, "POST", "/api/admin/abgleich", map[string]any{"kennung": "x"}); rec.Code != 400 {
		t.Errorf("fremde Datei: %d", rec.Code)
	}
}

func TestStatistikUndJahresabrechnung(t *testing.T) {
	_, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil); rec.Code != 200 {
		t.Fatalf("Beispieldaten: %d", rec.Code)
	}
	st := decode[statistikErgebnis](t, do(h, "GET", "/api/admin/statistik", nil))
	if len(st.Kosten.Verein) == 0 || st.Zahlung == nil {
		t.Fatalf("Statistik: %+v", st)
	}
	rec := do(h, "GET", "/api/admin/jahresabrechnung/pdf", nil)
	// ohne ausgestellte Rechnungen: verständlicher Fehler, kein Absturz
	if rec.Code != 404 && rec.Code != 200 {
		t.Fatalf("Jahresabrechnung: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Code == 200 && !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Error("kein PDF")
	}
}

// Vorschau und Übernahme zählen gleich, Gelöschtes kommt nicht zurück, doppelte IDs nur einmal,
// Spenden gelöschter Pächter bleiben draußen.
func TestAbgleichVorschauUndGeloeschtes(t *testing.T) {
	a, h := newTestApp(t)
	rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartennr": "1", "gartengroesse": 300})
	p := decode[Paechter](t, rec)
	a.st.mu.Lock()
	a.st.d.Termine = []Termin{{ID: "weg", Datum: "2026-05-01", Titel: "Alt"}}
	a.st.mu.Unlock()
	if rec := do(h, "DELETE", "/api/admin/termine/weg", nil); rec.Code != 200 {
		t.Fatalf("Termin löschen: %d %s", rec.Code, rec.Body.String())
	}
	d := decode[abgleichDatei](t, do(h, "GET", "/api/admin/abgleich", nil))
	d.Termine = []Termin{{ID: "weg", Datum: "2026-05-01", Titel: "Alt"}, {ID: "neu", Datum: "2026-06-01", Titel: "Einsatz"}, {ID: "neu", Datum: "2026-06-01", Titel: "Einsatz"}}
	d.Stunden = []EinsatzStunden{{TerminID: "neu", PaechterID: p.ID, Stunden: 3}}
	d.Spenden = []Spende{{ID: "s1", Datum: "2026-04-01", Jahr: 2026, Betrag: 10, PaechterID: "gibtsnicht", Name: "Gelöscht"}}
	v := decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich", d))
	w := decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich?anwenden=1", d))
	if v.Neu != w.Neu || w.Neu != 2 {
		t.Fatalf("Vorschau %+v, Übernahme %+v (erwartet je 2: Termin + Stunden)", v, w)
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if len(a.st.d.Termine) != 1 || a.st.d.Termine[0].ID != "neu" || len(a.st.d.Spenden) != 0 || len(a.st.d.Einsatzstunden) != 1 {
		t.Fatalf("Termine %+v, Spenden %d, Stunden %d", a.st.d.Termine, len(a.st.d.Spenden), len(a.st.d.Einsatzstunden))
	}
}

// Zählerstände vom Zweitgerät kommen an, auch wenn hier schon der Vorjahresstand und eine Vorauszahlung stehen.
// Was hier schon eingetragen ist, wird nie überschrieben.
func TestAbgleichErgaenztZaehlerstaende(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartennr": "1", "gartengroesse": 300}))
	q := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Hans", "gartennr": "2", "gartengroesse": 300}))
	a.st.mu.Lock()
	jahr := yearKey(a.st.d.Settings.Jahr)
	a.st.d.Jahre[jahr].Ablesungen[p.ID] = Ablesung{WasserVJ: fp(100), StromVJ: fp(1000), Abschlag: 50}
	a.st.d.Jahre[jahr].Ablesungen[q.ID] = Ablesung{WasserVJ: fp(10), WasserAkt: fp(15)}
	a.st.mu.Unlock()
	d := decode[abgleichDatei](t, do(h, "GET", "/api/admin/abgleich", nil))
	d.Jahre[jahr].Ablesungen[p.ID] = Ablesung{WasserVJ: fp(100), StromVJ: fp(1000), WasserAkt: fp(130), StromAkt: fp(1200), Stunden: fp(8)}
	d.Jahre[jahr].Ablesungen[q.ID] = Ablesung{WasserVJ: fp(10), WasserAkt: fp(99), Stunden: fp(4)}
	w := decode[abgleichErgebnis](t, do(h, "POST", "/api/admin/abgleich?anwenden=1", d))
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	x, y := a.st.d.Jahre[jahr].Ablesungen[p.ID], a.st.d.Jahre[jahr].Ablesungen[q.ID]
	if x.WasserAkt == nil || *x.WasserAkt != 130 || x.StromAkt == nil || *x.StromAkt != 1200 || x.Stunden == nil || *x.Stunden != 8 || x.Abschlag != 50 {
		t.Errorf("Erika nicht ergänzt (Ergebnis %+v): %+v", w, x)
	}
	if *y.WasserAkt != 15 || y.Stunden == nil || *y.Stunden != 4 {
		t.Errorf("Hans: vorhandener Stand darf nicht überschrieben, fehlende Stunden müssen ergänzt werden: %+v", y)
	}
}

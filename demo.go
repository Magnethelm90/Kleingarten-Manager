package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Beispieldaten: Wer das Programm zum ersten Mal öffnet, kann es mit einem erfundenen Verein
// ausprobieren, ohne eigene Daten einzugeben. Beispieldaten lassen sich nur in einen leeren
// Datenbestand laden und mit einem Klick wieder vollständig entfernen.

// datenbestandLeer meldet, ob noch nichts eingegeben wurde (keine Pächter, Rechnungen, Ausgaben,
// kein abgeschlossenes Jahr). Der Aufrufer hält s.mu.
func (s *Store) datenbestandLeer() bool {
	if len(s.d.Paechter) > 0 || len(s.d.Rechnungen) > 0 || len(s.d.Jahre) > 1 || len(s.d.Warteliste) > 0 || len(s.d.Termine) > 0 || len(s.d.Dokumente) > 0 || len(s.d.Wertermittlungen) > 0 || len(s.d.Maengel) > 0 || len(s.d.Einsatzstunden) > 0 || len(s.d.Spenden) > 0 || len(s.d.Vorauszahlungen) > 0 ||
		len(s.d.Rundschreiben) > 0 || len(s.d.Schluessel) > 0 || len(s.d.Geraete) > 0 ||
		len(s.d.Kuendigungen) > 0 || len(s.d.Beschluesse) > 0 || len(s.d.Vertraege) > 0 || len(s.d.Haushalt) > 0 || s.d.LageKarte.Bild != "" || len(s.d.LageKarte.Objekte) > 0 {
		return false
	}
	for _, j := range s.d.Jahre {
		if j.Abgeschlossen || len(j.Ablesungen) > 0 || len(j.Ausgaben) > 0 || j.Versorger != nil || j.Anfangsbestand != nil {
			return false
		}
	}
	return true
}

type demoPaechter struct {
	anrede, name, strasse string
	groesse               float64
	wasser, strom         [2]float64 // Vorjahr, aktuell (aktuell < 0: noch nicht abgelesen)
	stunden               float64    // < 0: noch nicht eingetragen
	post                  bool
	jahreDabei            int // Pächter seit so vielen Jahren (nur einer mit rundem Jubiläum)
}

var demoListe = []demoPaechter{
	{"Frau", "Brigitte Sommer", "Lindenweg 3", 312, [2]float64{148, 191}, [2]float64{2136, 2302}, 14, false, 23},
	{"Herr", "Klaus Winter", "Am Bach 12", 285, [2]float64{96, 121}, [2]float64{1480, 1555}, 12, true, 12},
	{"Familie", "Yılmaz", "Rosenstraße 7", 410, [2]float64{233, 301}, [2]float64{3105, 3390}, 24, false, 15},
	{"Frau", "Ingrid Herbst", "Gartenstraße 1", 260, [2]float64{55, 70}, [2]float64{805, 851}, 6, true, 31},
	{"Herr", "Tobias Frühling", "Ahornallee 22", 330, [2]float64{120, 158}, [2]float64{1990, 2120}, 18, false, 4},
	{"Frau", "Agnieszka Nowak", "Birkenweg 5", 298, [2]float64{77, 102}, [2]float64{1210, 1288}, 12, false, 19},
	{"Herr", "Peter Grün", "Lindenweg 9", 355, [2]float64{180, 236}, [2]float64{2650, 2805}, 10, false, 9},
	{"Frau", "Sabine Blume", "Am Bach 4", 275, [2]float64{64, 88}, [2]float64{990, 1043}, 16, false, 6},
	{"Herr und Frau", "Becker", "Kastanienweg 18", 390, [2]float64{205, 260}, [2]float64{2870, 3040}, 20, true, 16},
	{"Herr", "Mehmet Demir", "Rosenstraße 11", 305, [2]float64{110, 143}, [2]float64{1655, 1749}, 12, false, 2},
	{"Frau", "Helga Wiese", "Birkenweg 2", 268, [2]float64{90, -1}, [2]float64{1300, -1}, 12, false, 21},
	{"Herr", "Jan Sonnenberg", "Ahornallee 3", 320, [2]float64{132, 170}, [2]float64{2010, 2133}, -1, false, 1},
}

// ladeBeispieldaten füllt einen leeren Datenbestand mit einem erfundenen Verein. Der Aufrufer hält s.mu.
func (s *Store) ladeBeispieldaten() {
	set := &s.d.Settings
	set.VereinName = "Kleingartenverein Sonnenschein e. V."
	set.Ort = "12345 Musterstadt"
	set.Absenderzeile = "Kleingartenverein Sonnenschein e. V. – Kassenwart – kasse@example.org"
	// abgerechnet wird das Vorjahr, ausgestellt heute mit vier Wochen Zahlungsziel: so ist am Anfang
	// noch nichts überfällig, egal wann die Beispieldaten geladen werden
	heute := time.Now()
	jahr := heute.Year() - 1
	if set.Jahr != jahr {
		delete(s.d.Jahre, yearKey(set.Jahr))
		set.Jahr = jahr
	}
	set.Rechnungsdatum = heute.Format("2006-01-02")
	set.Zahlungsziel = heute.AddDate(0, 0, 28).Format("2006-01-02")
	j := s.ensureYear(jahr)
	for i, x := range demoListe {
		id := newID()
		versand := "Emailsendung"
		if x.post {
			versand = "Postversand"
		}
		p := Paechter{
			ID: id, Mitgliedsnr: fmt.Sprintf("%d-%02d", i+1, (heute.Year()-x.jahreDabei)%100), Gartennr: fmt.Sprint(i + 1), Anrede: x.anrede, Name: x.name,
			Strasse: x.strasse, PLZOrt: set.Ort, Versand: versand, Gartengroesse: x.groesse,
			WasserzaehlerNr: fmt.Sprintf("W-%04d", 1001+i), StromzaehlerNr: fmt.Sprintf("S-%04d", 2001+i),
		}
		if !x.post {
			p.Email = fmt.Sprintf("garten%d@example.org", i+1)
		}
		s.d.Paechter = append(s.d.Paechter, p)
		a := Ablesung{WasserVJ: fp(x.wasser[0]), StromVJ: fp(x.strom[0])}
		if x.wasser[1] >= 0 {
			a.WasserAkt = fp(x.wasser[1])
		}
		if x.strom[1] >= 0 {
			a.StromAkt = fp(x.strom[1])
		}
		if x.stunden >= 0 {
			a.Stunden = fp(x.stunden)
		}
		j.Ablesungen[id] = a
		s.d.GartenHistorie = append(s.d.GartenHistorie, GartenEintrag{
			Gartennr: p.Gartennr, PaechterID: id, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name, Seit: fmt.Sprintf("%d-04-01", heute.Year()-x.jahreDabei),
		})
	}
	bestand := 2480.0
	j.Anfangsbestand = &bestand
	j.Ausgaben = []Ausgabe{
		{ID: newID(), Datum: fmt.Sprintf("%d-03-31", jahr), Beschreibung: "Kontoführung 1. Quartal", Kategorie: "Versicherung & Gebühren", Betrag: 15, Wiederkehrend: true},
		{ID: newID(), Datum: fmt.Sprintf("%d-05-14", jahr), Beschreibung: "Reparatur Vereinsrasenmäher", Kategorie: "Instandhaltung", Betrag: 145.5},
		{ID: newID(), Datum: fmt.Sprintf("%d-09-02", jahr), Beschreibung: "Druckerpapier und Briefumschläge", Kategorie: "Verwaltung", Betrag: 18.99},
	}
	// Lageplan: zwei Reihen Gärten links und rechts eines Weges, wie auf einem kleinen Vereinsgelände
	s.d.Lageplan = map[string]LagePos{}
	for i := range demoListe {
		x, y := 20+(i/2)*140, 20
		if i%2 == 1 {
			y = 160
		}
		s.d.Lageplan[fmt.Sprint(i+1)] = LagePos{X: x, Y: y}
	}
	// Warteliste: drei Interessenten, einer schon lange nicht mehr nachgefragt
	s.d.Warteliste = []Bewerber{
		{ID: newID(), Seit: heute.AddDate(-2, -3, 0).Format("2006-01-02"), Anrede: "Herr", Name: "Lukas Morgenroth", Strasse: "Feldweg 8", PLZOrt: set.Ort,
			Telefon: "0123 456789", Wunsch: "gern mit Laube"},
		{ID: newID(), Seit: heute.AddDate(-1, -1, 0).Format("2006-01-02"), Anrede: "Familie", Name: "Kaya", Strasse: "Wiesenstraße 15", PLZOrt: set.Ort,
			Email: "familie.kaya@example.org", Wunsch: "großer Garten für Kinder",
			BesichtigungAm: heute.AddDate(0, 0, 4).Format("2006-01-02"), BesichtigungUm: "17:30", BesichtigungGarten: "11"},
		{ID: newID(), Seit: heute.AddDate(0, -2, 0).Format("2006-01-02"), Anrede: "Frau", Name: "Emma Regen", Strasse: "Lindenweg 20", PLZOrt: set.Ort,
			Email: "emma.regen@example.org"},
	}
	// Jahresplaner: typische Vereinstermine im laufenden Jahr
	j2 := heute.Year()
	tag := func(monat time.Month, wt time.Weekday, n int) string {
		return nterWochentag(j2, monat, wt, n).Format("2006-01-02")
	}
	s.d.Termine = []Termin{
		{ID: newID(), Datum: tag(time.March, time.Saturday, 2), Von: "15:00", Bis: "17:00", Titel: "Jahreshauptversammlung", Art: "Versammlung", Ort: "Vereinsheim"},
		{ID: newID(), Datum: tag(time.April, time.Saturday, 1), Von: "09:00", Bis: "13:00", Titel: "Frühjahrsarbeitseinsatz", Art: "Arbeitseinsatz", Ort: "Treffpunkt am Tor", Notiz: "Bitte Arbeitshandschuhe mitbringen"},
		{ID: newID(), Datum: tag(time.April, time.Saturday, 3), Von: "10:00", Titel: "Wasser anstellen", Art: "Sonstiges"},
		{ID: newID(), Datum: tag(time.June, time.Saturday, 4), Von: "14:00", Titel: "Sommerfest", Art: "Fest", Ort: "Festwiese"},
		{ID: newID(), Datum: tag(time.September, time.Saturday, 2), Von: "09:00", Bis: "13:00", Titel: "Herbstarbeitseinsatz", Art: "Arbeitseinsatz", Ort: "Treffpunkt am Tor"},
		{ID: newID(), Datum: tag(time.October, time.Saturday, 4), Von: "10:00", Titel: "Zählerablesung und Wasser abstellen", Art: "Ablesung", Notiz: "Zähler bitte frei zugänglich machen"},
		{ID: newID(), Datum: tag(time.November, time.Tuesday, 2), Von: "19:00", Bis: "21:00", Titel: "Vorstandssitzung", Art: "Vorstand", Ort: "Vereinsheim"},
		{ID: newID(), Datum: tag(time.December, time.Saturday, 2), Von: "16:00", Titel: "Adventsfeier", Art: "Fest", Ort: "Vereinsheim"},
	}
	// Wertermittlung für Garten 11: die Pächterin möchte aufhören, der Nachfolger steht auf der Warteliste
	s.d.Wertermittlungen = []Wertermittlung{{
		ID: newID(), Gartennr: "11", Datum: heute.AddDate(0, 0, -6).Format("2006-01-02"), Anlass: "Pächterwechsel", Wertermittler: "Wertermittler des Bezirksverbands",
		Positionen: []WertPosition{
			{Kategorie: "Baulichkeiten", Bezeichnung: "Gartenlaube, Holz, Dach 2019 erneuert", Menge: 16, Einheit: "m²", Einzelwert: 95, Abschlag: 35},
			{Kategorie: "Baulichkeiten", Bezeichnung: "Gewächshaus Alu/Glas", Menge: 1, Einheit: "Stück", Einzelwert: 450, Abschlag: 40},
			{Kategorie: "Anpflanzungen", Bezeichnung: "Apfelbaum, Halbstamm", Menge: 3, Einheit: "Stück", Einzelwert: 60},
			{Kategorie: "Anpflanzungen", Bezeichnung: "Johannisbeersträucher", Menge: 6, Einheit: "Stück", Einzelwert: 12},
			{Kategorie: "Einrichtungen", Bezeichnung: "Regentonnen", Menge: 2, Einheit: "Stück", Einzelwert: 25, Abschlag: 20},
			{Kategorie: "Abzug", Bezeichnung: "Entsorgung Altholz hinter der Laube", Menge: 1, Einheit: "pauschal", Einzelwert: 80},
		},
		Bemerkung: "Laube trocken, Fenster etwas undicht.",
	}}
	if w, err := cleanWertermittlung(s.d.Wertermittlungen[0]); err == nil {
		s.d.Wertermittlungen[0] = w
	}
	// Mängel aus der letzten Begehung
	for _, m := range []Mangel{
		{Gartennr: "3", Art: "Hecke und Bäume", Beschreibung: "Hecke zur Wegseite höher als 1,25 m, bitte zurückschneiden", Datum: heute.AddDate(0, 0, -9).Format("2006-01-02"), Frist: heute.AddDate(0, 0, 14).Format("2006-01-02")},
		{Gartennr: "3", Art: "Pflege und Unkraut", Beschreibung: "Beete am Weg stark verunkrautet", Datum: heute.AddDate(0, 0, -9).Format("2006-01-02"), Frist: heute.AddDate(0, 0, 14).Format("2006-01-02")},
		{Gartennr: "7", Art: "Abfall und Lagerung", Beschreibung: "Altholz und Bauschutt hinter der Laube lagern", Datum: heute.AddDate(0, 0, -9).Format("2006-01-02"), Frist: heute.AddDate(0, 0, -2).Format("2006-01-02")},
		{Gartennr: "5", Art: "Wege und Zäune", Beschreibung: "Zaunfeld zum Nachbarn hängt durch", Datum: heute.AddDate(0, 0, -9).Format("2006-01-02"), ErledigtAm: heute.AddDate(0, 0, -3).Format("2006-01-02")},
	} {
		if p, ok := s.aktiverPaechterLocked(m.Gartennr); ok {
			m.ID, m.PaechterID = newID(), p.ID
			s.d.Maengel = append(s.d.Maengel, m)
		}
	}
	// Spenden und erfundene Angaben zur Gemeinnützigkeit, damit sich die Zuwendungsbestätigung ausprobieren lässt
	s.d.Gemeinnuetzigkeit = Gemeinnuetzigkeit{Anschrift: "Am Vereinsheim 1\n" + set.Ort, Zweck: "der Kleingärtnerei", Finanzamt: "Musterstadt",
		Steuernummer: "000/000/00000", Bescheid: "freistellung", BescheidDatum: heute.AddDate(-1, -2, 0).Format("2006-01-02"),
		Veranlagung: fmt.Sprintf("%d–%d", heute.Year()-4, heute.Year()-2)}
	if p, ok := s.aktiverPaechterLocked("2"); ok {
		s.d.Spenden = append(s.d.Spenden, Spende{ID: newID(), Datum: fmt.Sprintf("%d-05-10", jahr), Jahr: jahr, Betrag: 50, PaechterID: p.ID, Name: p.Name, Strasse: p.Strasse, PLZOrt: p.PLZOrt})
	}
	s.d.Spenden = append(s.d.Spenden, Spende{ID: newID(), Datum: fmt.Sprintf("%d-06-21", jahr), Jahr: jahr, Betrag: 120, Name: "Gartencenter Grünfink", Strasse: "Gewerbestraße 4", PLZOrt: set.Ort, Notiz: "für das Sommerfest"})
	// Vorauszahlungen per Dauerauftrag: fünf Gärten zahlen monatlich, Garten 4 ist seit drei Monaten im Rückstand
	s.d.VorauszahlungSoll = map[string]float64{}
	for i, nr := range []string{"1", "2", "4", "6", "9"} {
		p, ok := s.aktiverPaechterLocked(nr)
		if !ok {
			continue
		}
		soll := []float64{20, 15, 20, 15, 20}[i] // unter der Jahresrechnung, damit ein Restbetrag bleibt
		s.d.VorauszahlungSoll[p.ID] = soll
		monate := 12
		if nr == "4" {
			monate = 9
		}
		for m := 1; m <= monate; m++ {
			v := Vorauszahlung{ID: newID(), PaechterID: p.ID, Jahr: jahr, Datum: fmt.Sprintf("%d-%02d-01", jahr, m), Betrag: soll, Quelle: "kontoauszug"}
			s.d.Vorauszahlungen = append(s.d.Vorauszahlungen, v)
			s.d.vorauszahlungBuchen(&s.d.Vorauszahlungen[len(s.d.Vorauszahlungen)-1], +1)
		}
	}
	// Lageplan: Hauptweg mit Wasserleitung, Tor, Zähler, Absperrhahn, Verteiler, Vereinsheim und Kompostplatz
	s.d.LageKarte.Leitungen = []LageLinie{{Art: "wasser", Punkte: []Punkt{{10, 122}, {860, 122}}, Bez: "Hauptleitung unter dem Weg"}}
	s.d.LageKarte.Objekte = []LageObjekt{
		{Art: "tor", X: 10, Y: 122, Bez: "Haupttor"}, {Art: "wasserzaehler", X: 45, Y: 122, Bez: "Hauptzähler"},
		{Art: "absperr", X: 370, Y: 122, Bez: "Hahn Mitte"}, {Art: "strom", X: 580, Y: 104, Bez: "Verteiler"},
		{Art: "vereinsheim", X: 830, Y: 60}, {Art: "kompost", X: 830, Y: 190},
	}
	// Schlüssel und Geräte: ein Rasenmäher ist überfällig, der Häcksler muss zur Wartung
	vorTagen := func(t int) string { return heute.AddDate(0, 0, t).Format("2006-01-02") }
	s.d.Schluessel = []Schluessel{
		{ID: newID(), Nummer: "1", Art: "Tor", Inhaber: "Vorstand (Vorsitz)", Seit: vorTagen(-900)},
		{ID: newID(), Nummer: "2", Art: "Tor", Inhaber: "Vorstand (Kasse)", Seit: vorTagen(-900)},
		{ID: newID(), Nummer: "3", Art: "Tor", Pfand: 20},
		{ID: newID(), Nummer: "1", Art: "Vereinsheim", Inhaber: "Vorstand (Vorsitz)", Seit: vorTagen(-900)},
		{ID: newID(), Nummer: "1", Art: "Wasserschacht", Bezeichnung: "Hauptschacht am Tor", Pfand: 0},
	}
	if p, ok := s.aktiverPaechterLocked("3"); ok {
		s.d.Schluessel[2].PaechterID, s.d.Schluessel[2].Seit = p.ID, vorTagen(-400)
		s.d.Schluessel[2].Verlauf = []Vorgang{{Datum: vorTagen(-400), Aktion: "ausgegeben", PaechterID: p.ID, Notiz: "20 € Pfand bar"}}
	}
	s.d.Geraete = []Geraet{
		{ID: newID(), Name: "Gartenhäcksler", Inventarnr: "G-01", Standort: "Geräteschuppen", Angeschafft: vorTagen(-1500), Wert: 450, WartungMonate: 12, LetzteWartung: vorTagen(-380)},
		{ID: newID(), Name: "Rasenmäher (Benzin)", Inventarnr: "G-02", Standort: "Geräteschuppen", Angeschafft: vorTagen(-900), Wert: 380, WartungMonate: 12, LetzteWartung: vorTagen(-120)},
		{ID: newID(), Name: "Leiter 3 m", Inventarnr: "G-03", Standort: "Vereinsheim", Wert: 90},
	}
	if p, ok := s.aktiverPaechterLocked("5"); ok {
		g := &s.d.Geraete[1]
		g.PaechterID, g.Seit, g.Bis = p.ID, vorTagen(-10), vorTagen(-3)
		g.Verlauf = []Vorgang{{Datum: vorTagen(-10), Aktion: "verliehen", PaechterID: p.ID}}
	}
	// Kündigung: Garten 8 kündigt zum 30. November, Bestätigung ist raus
	if p, ok := s.aktiverPaechterLocked("8"); ok {
		s.d.Kuendigungen = []Kuendigung{{ID: newID(), PaechterID: p.ID, Gartennr: "8", Eingang: fmt.Sprintf("%d-08-12", jahr+1), Zum: fmt.Sprintf("%d-11-30", jahr+1),
			Von: "paechter", Grund: "Umzug", Erledigt: map[string]string{"bestaetigung": fmt.Sprintf("%d-08-15", jahr+1)}}}
	}
	// Beschlussbuch, Verträge, Haushaltsplan und Hauptzähler (für den Schwund)
	s.d.Beschluesse = []Beschluss{
		{ID: newID(), Datum: fmt.Sprintf("%d-03-14", jahr), Gremium: "Mitgliederversammlung", Nummer: fmt.Sprintf("MV %d/1", jahr), Titel: "Wasserpreis ab dem nächsten Jahr 2,04 € je m³", Ja: 31, Nein: 4, Enthaltung: 2, Ergebnis: "angenommen", Stichworte: "Wasser, Preise"},
		{ID: newID(), Datum: fmt.Sprintf("%d-03-14", jahr), Gremium: "Mitgliederversammlung", Nummer: fmt.Sprintf("MV %d/2", jahr), Titel: "12 Pflichtstunden Gemeinschaftsarbeit, 10 € je fehlende Stunde", Ja: 29, Nein: 6, Enthaltung: 2, Ergebnis: "angenommen", Stichworte: "Gemeinschaftsarbeit"},
		{ID: newID(), Datum: fmt.Sprintf("%d-05-02", jahr), Gremium: "Vorstand", Nummer: fmt.Sprintf("V %d/1", jahr), Titel: "Neuer Rasenmäher für den Verein bis 400 €", Text: "Angebote von zwei Händlern eingeholt, der günstigere wird gekauft.", Ergebnis: "angenommen", Stichworte: "Anschaffung, Geräte"},
	}
	s.d.Vertraege = []Vertrag{
		{ID: newID(), Art: "Versicherung", Bezeichnung: "Vereinshaftpflicht", Partner: "Musterversicherung AG", Nummer: "VH-123456", Beginn: "2019-01-01", Ende: fmt.Sprintf("%d-12-31", jahr), FristMonate: 3, Verlaengert: 12, Kosten: 186.4, Turnus: "jährlich"},
		{ID: newID(), Art: "Strom", Bezeichnung: "Stromliefervertrag Anlage", Partner: "Stadtwerke Musterstadt", Beginn: vorTagen(-640), Ende: heute.AddDate(0, 3, 20).Format("2006-01-02"), FristMonate: 3, Verlaengert: 12, Kosten: 38, Turnus: "monatlich"},
		{ID: newID(), Art: "Pacht (Zwischenpacht)", Bezeichnung: "Zwischenpachtvertrag mit dem Verband", Partner: "Verband der Gartenfreunde", Beginn: "1998-04-01"},
	}
	s.d.Haushalt = map[string]Haushaltsplan{fmt.Sprint(jahr): {
		Einnahmen: map[string]float64{"Pacht": 220, "Mitgliedsbeiträge und Umlage": 1050, "Wasser und Strom": 1600, "Spenden": 100},
		Ausgaben:  map[string]float64{"Instandhaltung": 300, "Anschaffung": 200, "Verwaltung": 60, "Versicherung & Gebühren": 250},
		Notiz:     "Größere Anschaffung geplant: Rasenmäher (Beschluss V 1).",
	}}
	if v, ok := s.viewLocked(jahr); ok {
		summe := 0.0 // alle abgelesenen Gärten, auch solche, deren Rechnung noch unvollständig ist
		for _, p := range v.Paechter {
			if w := calculate(v.Settings, p, v.Ablesungen[p.ID]).WasserVerbrauch; w != nil {
				summe += *w
			}
		}
		haupt, eur := round2(summe*1.14+3), round2((summe*1.14+3)*1.9)
		if j := s.d.Jahre[yearKey(jahr)]; j != nil {
			j.Versorger = &Versorger{WasserM3: &haupt, WasserEUR: &eur}
		}
	}
	s.d.Rundschreiben = []Rundschreiben{{ID: newID(), Datum: fmt.Sprintf("%d-03-20", jahr), An: "alle", Betreff: "Wasser wird angestellt – bitte Leitungen prüfen",
		Text:       "Liebe Gartenfreundinnen und Gartenfreunde,\n\nam ersten Samstag im April stellen wir das Wasser in der Anlage wieder an. Bitte schließt vorher die Entleerungshähne in euren Gärten und prüft die Leitungen auf Frostschäden.\n\nWer beim Anstellen helfen möchte, ist um 9 Uhr am Haupttor herzlich willkommen.",
		Verschickt: fmt.Sprintf("%d-03-20", jahr)}}
	s.d.Vereinsinfo = Vereinsinfo{
		Ansprechpartner: "Vorsitz: Erika Beispiel, Tel. 0123 4567\nKasse: Max Muster, kasse@example.org\nSprechstunde: samstags 10–12 Uhr im Vereinsheim",
		Hinweise:        "Mittagsruhe 13–15 Uhr, sonntags keine lauten Arbeiten\nWasser wird im April angestellt und im Oktober abgestellt\nGrünschnitt bitte zum Kompostplatz am Tor, Müll nimmt jeder selbst mit",
	}
	s.d.Demo = true
	s.audit("Beispieldaten geladen")
}

// nterWochentag liefert z. B. den zweiten Samstag im März.
func nterWochentag(jahr int, monat time.Month, wt time.Weekday, n int) time.Time {
	d := time.Date(jahr, monat, 1, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != wt {
		d = d.AddDate(0, 0, 1)
	}
	return d.AddDate(0, 0, 7*(n-1))
}

func fp(x float64) *float64 { return &x }

// entferneBeispieldaten setzt den Datenbestand auf den Anfang zurück und löscht die in der Zwischenzeit
// erzeugten Rechnungs- und Belegdateien. Passwörter, der zweite Sicherungsordner und die Einführung
// bleiben, wie sie sind. Der Aufrufer hält s.mu.
func (s *Store) entferneBeispieldaten() {
	for _, r := range s.d.Rechnungen {
		s.entferneDatei("Rechnungen", r.Datei)
	}
	for _, d := range s.d.Dokumente {
		s.entferneDokumentDatei(d.Datei)
	}
	for _, j := range s.d.Jahre {
		for _, a := range j.Ausgaben {
			for _, b := range a.Belege {
				s.entferneBeleg(b)
			}
		}
	}
	alt := s.d
	s.d = newData()
	s.d.Admin, s.d.PruefAuth, s.d.TutorialGesehen = alt.Admin, alt.PruefAuth, alt.TutorialGesehen
	s.d.Settings.ZweiteSicherung = alt.Settings.ZweiteSicherung
	s.audit("Beispieldaten entfernt")
	s.leereDruckOrdner()
}

// entferneDatei löscht eine gewöhnliche Datei, deren gespeicherter relativer Pfad im Unterordner
// ordner des Datenordners liegt (der Pfad stammt aus der Datendatei und wird deshalb geprüft).
func (s *Store) entferneDatei(ordner, rel string) {
	if rel == "" {
		return
	}
	base := filepath.Join(s.dir, ordner) + string(filepath.Separator)
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if len(full) <= len(base) || full[:len(base)] != base {
		return
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode().IsRegular() {
		_ = os.Remove(full)
	}
}

func (a *App) handleDemoLaden(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.datenbestandLeer() {
		writeErr(w, bad("Beispieldaten lassen sich nur laden, solange noch keine eigenen Daten eingegeben sind"))
		return
	}
	a.st.ladeBeispieldaten()
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"paechter": len(a.st.d.Paechter)})
}

func (a *App) handleDemoEntfernen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.d.Demo {
		writeErr(w, bad("Es sind keine Beispieldaten geladen"))
		return
	}
	a.st.snapshotBackup("vor-Entfernen-der-Beispieldaten")
	a.st.entferneBeispieldaten()
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"stand": time.Now().Format(time.RFC3339)})
}

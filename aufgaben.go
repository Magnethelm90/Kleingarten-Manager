package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Vorstands-Cockpit: Die Übersicht sagt dem Vorstand von selbst, was gerade ansteht – aus den Daten, die das
// Programm ohnehin hat (Termine, Mängel, Rechnungen, Spenden, Warteliste, Bescheid). Nichts davon wird
// gespeichert; die Liste entsteht bei jedem Aufruf neu.

type aufgabe struct {
	Stufe string `json:"stufe"` // err | warn | info
	Titel string `json:"titel"`
	Text  string `json:"text,omitempty"`
	Ziel  string `json:"ziel"`  // admin:<bereich> oder tab:<reiter>
	Knopf string `json:"knopf"` // Beschriftung des Knopfs
}

func tageBis(von time.Time, datum string) int {
	d, err := parseDate(datum)
	if err != nil {
		return 1 << 30
	}
	a := time.Date(von.Year(), von.Month(), von.Day(), 0, 0, 0, 0, time.UTC)
	return int(d.Sub(a).Hours() / 24)
}

func inTagen(n int) string {
	switch n {
	case 0:
		return "heute"
	case 1:
		return "morgen"
	default:
		return fmt.Sprintf("in %d Tagen", n)
	}
}

func mehrzahl(n int, eins, viele string) string {
	if n == 1 {
		return "1 " + eins
	}
	return fmt.Sprintf("%d %s", n, viele)
}

// aufgabenLocked stellt die anstehenden Aufgaben zusammen. Der Aufrufer hält s.mu.
func (s *Store) aufgabenLocked(heute time.Time) []aufgabe {
	var l []aufgabe
	add := func(stufe, titel, text, ziel, knopf string) {
		l = append(l, aufgabe{stufe, titel, text, ziel, knopf})
	}

	// Termine: Ablesung und Versammlung vorbereiten, Stunden nach dem Arbeitseinsatz eintragen
	gebucht := map[string]bool{}
	for _, e := range s.d.Einsatzstunden {
		gebucht[e.TerminID] = true
	}
	for _, t := range s.d.Termine {
		n := tageBis(heute, t.Datum)
		switch {
		case t.Art == "Ablesung" && n >= 0 && n <= 21:
			add("info", fmt.Sprintf("Zählerablesung %s (%s)", inTagen(n), germanDate(t.Datum)), "Ablesebogen für den Rundgang drucken.", "admin:berichte", "Ablesebogen")
		case t.Art == "Versammlung" && n >= 0 && n <= 28:
			add("warn", fmt.Sprintf("%s %s", t.Titel, inTagen(n)), "Einladung mit Tagesordnung rechtzeitig verschicken und Anwesenheitsliste drucken.", "tab:termine", "Einladung")
		case t.Art == "Arbeitseinsatz" && n < 0 && n >= -30 && !gebucht[t.ID]:
			add("warn", fmt.Sprintf("Stunden vom %s eintragen", t.Titel), fmt.Sprintf("Der Einsatz war am %s – wer war dabei?", germanDate(t.Datum)), "tab:termine", "Stunden eintragen")
		}
	}

	// Zählerstände: Ablesung ist vorbei, aber es fehlen noch Stände
	ablesungVorbei := false
	praefix := fmt.Sprint(heute.Year()) + "-"
	for _, t := range s.d.Termine {
		if t.Art == "Ablesung" && strings.HasPrefix(t.Datum, praefix) && tageBis(heute, t.Datum) < 0 {
			ablesungVorbei = true
		}
	}
	if ablesungVorbei {
		if d, ok := s.zaehlerFehlenLocked(s.d.Settings.Jahr); ok && len(d.Fehlend) > 0 {
			add("warn", mehrzahl(len(d.Fehlend), "Garten ohne Zählerstand", "Gärten ohne Zählerstand"), "Die Ablesung ist vorbei. Aushang oder E-Mail-Erinnerung verschicken.", "admin:berichte", "Erinnern")
		}
	}

	// Rechnungen: überfällige Zahlungen, ausstehende Schlussrechnungen
	heuteISO := heute.Format("2006-01-02")
	ueberfaellig, summe := 0, 0.0
	hatRechnung := map[string]bool{}
	for _, r := range s.d.Rechnungen {
		if r.Status != statusGueltig {
			continue
		}
		if r.Jahr == s.d.Settings.Jahr {
			hatRechnung[r.PaechterID] = true
		}
		if r.Result.Gesamt > 0 && openAmount(r) > 0.005 && r.Settings.Zahlungsziel != "" && r.Settings.Zahlungsziel < heuteISO && !r.Bereinigt {
			ueberfaellig++
			summe += openAmount(r)
		}
	}
	if ueberfaellig > 0 {
		add("warn", mehrzahl(ueberfaellig, "Rechnung überfällig", "Rechnungen überfällig"), fmt.Sprintf("Zusammen %s offen. Kontoauszug einlesen oder Mahnung erzeugen.", fmtEUR(round2(summe))), "tab:zahlungen", "Zahlungen")
	}
	schluss := 0
	for _, p := range s.d.Paechter {
		if p.AusgeschiedenAm != "" && !p.Geloescht && !hatRechnung[p.ID] {
			schluss++
		}
	}
	if schluss > 0 {
		add("info", mehrzahl(schluss, "Schlussrechnung steht aus", "Schlussrechnungen stehen aus"), "Nach einem Pächterwechsel bekommt der bisherige Pächter eine Rechnung bis zur Übergabe.", "tab:rechnungen", "Rechnungen")
	}

	// Mängel und Drittelregelung
	fristAb := 0
	for _, m := range s.d.Maengel {
		if m.ErledigtAm == "" && m.Frist != "" && m.Frist < heuteISO {
			fristAb++
		}
	}
	if fristAb > 0 {
		add("warn", mehrzahl(fristAb, "Mängelfrist abgelaufen", "Mängelfristen abgelaufen"), "Nachsehen, ob die Mängel beseitigt sind, oder eine Aufforderung schreiben.", "admin:maengel", "Mängel")
	}
	unter := 0
	for _, z := range s.nutzungUebersichtLocked() {
		if z.Unter {
			unter++
		}
	}
	if unter > 0 {
		add("info", mehrzahl(unter, "Garten unter einem Drittel Anbau", "Gärten unter einem Drittel Anbau"), "Laut letzter Schätzung bei der Begehung – Gespräch mit dem Pächter suchen.", "admin:maengel", "Ansehen")
	}

	// Gemeinnützigkeit und Spenden
	if g := s.d.Gemeinnuetzigkeit; g.Steuernummer != "" && g.BescheidDatum != "" {
		if d, err := parseDate(g.BescheidDatum); err == nil {
			jahre := freistellungGueltig
			if g.Bescheid == "feststellung" {
				jahre = feststellungGueltig
			}
			ende := d.AddDate(jahre, 0, 0)
			switch n := tageBis(heute, ende.Format("2006-01-02")); {
			case n < 0:
				add("err", "Bescheid zur Gemeinnützigkeit abgelaufen", "Zuwendungsbestätigungen sind damit nicht mehr möglich. Neuen Bescheid beim Finanzamt anfordern und eintragen.", "admin:spenden", "Spenden")
			case n <= 180:
				add("warn", "Bescheid zur Gemeinnützigkeit läuft bald ab", fmt.Sprintf("Am %s – rechtzeitig die Steuererklärung abgeben, damit ein neuer Bescheid kommt.", ende.Format("02.01.2006")), "admin:spenden", "Spenden")
			}
		}
	}
	offen := 0
	for _, x := range s.d.Spenden {
		if x.BestaetigtAm == "" && !x.Bereinigt && (tageBis(heute, x.Datum) < -60 || jahrVon(x.Datum) < fmt.Sprint(heute.Year())) {
			offen++
		}
	}
	if offen > 0 {
		add("info", mehrzahl(offen, "Spende ohne Bestätigung", "Spenden ohne Bestätigung"), "Spender brauchen die Zuwendungsbestätigung für ihre Steuererklärung.", "admin:spenden", "Spenden")
	}

	// Vorauszahlungen per Überweisung: Rückstände im laufenden Jahr
	if len(s.d.VorauszahlungSoll) > 0 {
		rueck, summe := 0, 0.0
		for _, z := range s.vzAnsichtLocked(s.d.Settings.Jahr, heute).Zeilen {
			if z.Rueckstand > 0 {
				rueck++
				summe += z.Rueckstand
			}
		}
		if rueck > 0 {
			add("warn", mehrzahl(rueck, "Pächter mit Rückstand bei der Vorauszahlung", "Pächter mit Rückstand bei der Vorauszahlung"),
				fmt.Sprintf("Zusammen %s – Kontoauszug einlesen oder eine Erinnerung schicken.", fmtEUR(round2(summe))), "admin:vorauszahlung", "Vorauszahlungen")
		}
	}

	// Lastschrift: nächster Monatslauf und verfallene Mandate
	vorauszahlung, verfallen := false, 0
	for _, m := range s.d.Mandate {
		if m.Widerrufen != "" {
			continue
		}
		if m.verfallen(heute) {
			verfallen++
		} else if m.Monatsbetrag > 0 {
			vorauszahlung = true
		}
	}
	if verfallen > 0 {
		add("warn", mehrzahl(verfallen, "Lastschriftmandat verfallen", "Lastschriftmandate verfallen"), "Nach 36 Monaten ohne Einzug gilt ein Mandat nicht mehr – neues Mandat unterschreiben lassen.", "admin:sepa", "Lastschrift")
	}
	if vorauszahlung {
		tag := max(1, s.d.Sepa.Faelligkeitstag)
		naechster := naechsterBankarbeitstag(time.Date(heute.Year(), heute.Month(), tag, 0, 0, 0, 0, time.UTC))
		if tageBis(heute, naechster.Format("2006-01-02")) < 1 {
			naechster = naechsterBankarbeitstag(time.Date(heute.Year(), heute.Month()+1, tag, 0, 0, 0, 0, time.UTC))
		}
		schon := false
		for _, l := range s.d.Lastschriften {
			schon = schon || (l.Art == "vorauszahlung" && strings.HasPrefix(l.Faellig, naechster.Format("2006-01")))
		}
		if n := tageBis(heute, naechster.Format("2006-01-02")); !schon && n <= 10 {
			add("warn", fmt.Sprintf("Lastschrift für %s %d erstellen", monatsnamen[naechster.Month()-1], naechster.Year()),
				fmt.Sprintf("Die Vorauszahlungen sind am %s fällig (%s) – Datei rechtzeitig ans Online-Banking geben.", naechster.Format("02.01.2006"), inTagen(n)), "admin:sepa", "Lastschrift")
		}
	}

	s.inventarAufgabenLocked(heute, add)
	s.schwundAufgabenLocked(heute, add)
	s.kuendigungAufgabenLocked(heute, add)
	s.vertragAufgabenLocked(heute, add)

	// Warteliste
	nachfragen := 0
	for _, b := range s.d.Warteliste {
		if b.nachfragen(heute) {
			nachfragen++
		}
	}
	if nachfragen > 0 {
		add("info", mehrzahl(nachfragen, "Eintrag auf der Warteliste nachfragen", "Einträge auf der Warteliste nachfragen"), "Seit über zwei Jahren nicht bestätigt: Besteht noch Interesse?", "admin:warteliste", "Warteliste")
	}

	rang := map[string]int{"err": 0, "warn": 1, "info": 2}
	sort.SliceStable(l, func(i, j int) bool { return rang[l[i].Stufe] < rang[l[j].Stufe] })
	return l
}

func (a *App) handleAufgaben(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	l := a.st.aufgabenLocked(time.Now())
	a.st.mu.Unlock()
	if l == nil {
		l = []aufgabe{}
	}
	writeJSON(w, 200, l)
}

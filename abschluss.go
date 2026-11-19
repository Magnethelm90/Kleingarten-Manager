package main

import (
	"fmt"
	"net/http"
	"time"
)

// Abschlussprüfung: Ampel vor dem Jahreswechsel. Rot = sollte vorher erledigt sein, Gelb = bitte ansehen,
// Grün = in Ordnung. Sie blockiert nichts; nach dem Jahreswechsel bleibt alles weiter bearbeitbar.

type abschlussPunkt struct {
	Stufe string `json:"stufe"` // ok | warn | err
	Titel string `json:"titel"`
	Text  string `json:"text,omitempty"`
	Ziel  string `json:"ziel,omitempty"`
	Knopf string `json:"knopf,omitempty"`
}

type abschlussAntwort struct {
	Jahr   int              `json:"jahr"`
	Rot    int              `json:"rot"`
	Gelb   int              `json:"gelb"`
	Punkte []abschlussPunkt `json:"punkte"`
}

func (s *Store) abschlussPruefungLocked(heute time.Time) abschlussAntwort {
	jahr := s.d.Settings.Jahr
	a := abschlussAntwort{Jahr: jahr, Punkte: []abschlussPunkt{}}
	add := func(stufe, titel, text, ziel, knopf string) {
		if stufe == "ok" {
			ziel, knopf = "", ""
		}
		a.Punkte = append(a.Punkte, abschlussPunkt{stufe, titel, text, ziel, knopf})
		switch stufe {
		case "err":
			a.Rot++
		case "warn":
			a.Gelb++
		}
	}
	v, ok := s.viewLocked(jahr)
	if !ok {
		return a
	}

	// 1. Angaben vollständig?
	aktiv, fertig := 0, map[string]bool{}
	for _, p := range v.Paechter {
		if p.Geloescht {
			continue
		}
		aktiv++
		if calculate(v.Settings, p, v.Ablesungen[p.ID]).Vollstaendig {
			fertig[p.ID] = true
		}
	}
	if n := aktiv - len(fertig); n > 0 {
		add("err", mehrzahl(n, "Pächter mit fehlenden Angaben", "Pächter mit fehlenden Angaben"), "Ohne Zählerstände und Stunden lässt sich keine Rechnung ausstellen. Nach dem Jahreswechsel werden die heutigen Stände zu Vorjahresständen.", "tab:eingabe", "Zählerstände")
	} else {
		add("ok", fmt.Sprintf("Angaben vollständig bei allen %d Pächtern", aktiv), "", "", "")
	}

	// 2. Rechnungen ausgestellt und noch passend?
	issued, _ := s.archiveViewLocked(v)
	ohne, geaendert := 0, 0
	for id := range fertig {
		inf, da := issued[id]
		switch {
		case !da:
			ohne++
		case inf.Geaendert:
			geaendert++
		}
	}
	switch {
	case ohne > 0:
		add("err", mehrzahl(ohne, "Rechnung noch nicht ausgestellt", "Rechnungen noch nicht ausgestellt"), "Erst ausstellen, dann abschließen – sonst fehlt die Rechnung im Archiv des Jahres.", "tab:rechnungen", "Rechnungen")
	case len(fertig) > 0:
		add("ok", fmt.Sprintf("Alle %d Rechnungen ausgestellt", len(fertig)), "", "", "")
	}
	if geaendert > 0 {
		add("warn", mehrzahl(geaendert, "Rechnung passt nicht mehr zu den Daten", "Rechnungen passen nicht mehr zu den Daten"), "Nach dem Ausstellen wurden Werte geändert. Prüfen, ob eine korrigierte Rechnung nötig ist.", "tab:rechnungen", "Rechnungen")
	}

	// 3. Zahlungen und Schlussrechnungen
	offen, summe := 0, 0.0
	hatRechnung := map[string]bool{}
	for _, r := range s.d.Rechnungen {
		if r.Status != statusGueltig || r.Jahr != jahr {
			continue
		}
		hatRechnung[r.PaechterID] = true
		if r.Result.Gesamt > 0 && openAmount(r) > 0.005 && !r.Bereinigt {
			offen++
			summe += openAmount(r)
		}
	}
	if offen > 0 {
		add("warn", mehrzahl(offen, "Rechnung noch nicht bezahlt", "Rechnungen noch nicht bezahlt"), fmt.Sprintf("Zusammen %s. Offene Rechnungen bleiben nach dem Jahreswechsel unter »Zahlungen« sichtbar.", fmtEUR(round2(summe))), "tab:zahlungen", "Zahlungen")
	} else if len(hatRechnung) > 0 {
		add("ok", "Alle Rechnungen bezahlt", "", "", "")
	}
	schluss := 0
	for _, p := range s.d.Paechter {
		if p.AusgeschiedenAm != "" && !p.Geloescht && !hatRechnung[p.ID] {
			schluss++
		}
	}
	if schluss > 0 {
		add("warn", mehrzahl(schluss, "Schlussrechnung steht aus", "Schlussrechnungen stehen aus"), "Ausgeschiedene Pächter bekommen eine Rechnung bis zur Übergabe.", "tab:rechnungen", "Rechnungen")
	}

	// 4. Kasse
	k := s.kassenberichtLocked(v)
	ohneBeleg := 0
	for _, x := range k.Ausgaben {
		if len(x.Belege) == 0 && x.Betrag != 0 {
			ohneBeleg++
		}
	}
	switch {
	case ohneBeleg > 0:
		add("warn", mehrzahl(ohneBeleg, "Ausgabe ohne Beleg", "Ausgaben ohne Beleg"), "Die Kassenprüfer brauchen zu jeder Ausgabe einen Beleg.", "admin:kassenbericht:ohnebeleg", "Belege ergänzen")
	case len(k.Ausgaben) > 0:
		add("ok", "Jede Ausgabe hat einen Beleg", "", "", "")
	}
	if k.Kassenbestand < 0 {
		add("err", "Kassenbestand im Minus", fmt.Sprintf("Laut Kassenbericht %s. Fehlt der Anfangsbestand oder eine Einnahme? Der Endbestand wird zum Anfangsbestand des neuen Jahres.", fmtEUR(k.Kassenbestand)), "admin:kassenbericht", "Kassenbericht")
	} else {
		add("ok", "Kassenbestand "+fmtEUR(k.Kassenbestand), "Bitte mit dem Kontoauszug vergleichen – er wird zum Anfangsbestand des neuen Jahres.", "", "")
	}
	if s.modulAn("barkasse") && s.barkasseAktivLocked(jahr) {
		if b := s.barkasseLocked(jahr, 20); b.Bestand < 0 {
			add("err", "Barkasse im Minus", fmt.Sprintf("Laut Kassenbuch %s – das geht bei Bargeld nicht.", fmtEUR(b.Bestand)), "admin:kassenbericht", "Barkasse")
		} else {
			add("ok", "Barkasse "+fmtEUR(b.Bestand), "Bitte das Bargeld nachzählen.", "", "")
		}
	}

	// 5. Sicherung außer Haus
	switch z := s.zweiteSicherungLocked(); {
	case !z.Eingerichtet:
		add("warn", "Keine Sicherung außer Haus", "Alle Sicherungen liegen nur auf diesem Rechner. Vor dem Abschluss eine Kopie auf einen USB-Stick legen.", "admin:daten", "Sicherung")
	case z.Fehler != "":
		add("warn", "Zweiter Sicherungsordner nicht erreichbar", z.Fehler+". USB-Stick einstecken und »Jetzt sichern« wählen.", "admin:daten", "Sicherung")
	case z.Zuletzt == "" || tageBis(heute, z.Zuletzt) < -30:
		add("warn", "Sicherung außer Haus ist älter als 30 Tage", "USB-Stick einstecken und »Jetzt sichern« wählen.", "admin:daten", "Sicherung")
	default:
		add("ok", "Sicherung außer Haus vom "+germanDate(z.Zuletzt), "", "", "")
	}
	return a
}

func (a *App) handleAbschlussPruefung(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.abschlussPruefungLocked(time.Now()))
}

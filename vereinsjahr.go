package main

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Vereinsjahr: was im Kleingartenverein typischerweise wann ansteht, nach Monaten geordnet. Für neue
// Vorstandsmitglieder der rote Faden durchs Jahr. Punkte, die das Programm selbst erkennen kann (Rechnungen
// erstellt, Zähler abgelesen, Kassenprüfung fertig …), haken sich von allein ab; die übrigen hakt man von Hand ab.
// Punkte ausgeblendeter Bereiche erscheinen nicht.

type planVorlage struct {
	key, titel, text, ziel, knopf, modul string
	monat                                int // 0 = je nach Abrechnungsrhythmus (siehe rhythmusLocked)
	erkannt                              func(s *Store, jahr int) bool
}

// rhythmusLocked: Wann rechnet der Verein ab? Aus den Einstellungen: Monat des Rechnungsdatums und des Zahlungsziels
// sowie, ob die Rechnungen im Folgejahr des Abrechnungsjahres kommen (versatz 1, z. B. Januar 2026 für 2025) oder im
// selben Jahr (versatz 0, z. B. November). Im Kalenderjahr j werden also die Rechnungen für j - versatz verschickt.
func (s *Store) rhythmusLocked() (versatz, mRechnung, mZahlung int) {
	versatz, mRechnung, mZahlung = 0, 11, 12
	if d, err := parseDate(s.d.Settings.Rechnungsdatum); err == nil {
		mRechnung, mZahlung = int(d.Month()), min(int(d.Month())+1, 12)
		versatz = min(max(d.Year()-s.d.Settings.Jahr, 0), 1)
	}
	if d, err := parseDate(s.d.Settings.Zahlungsziel); err == nil && int(d.Month()) >= mRechnung {
		mZahlung = int(d.Month())
	}
	return
}

type planPunkt struct {
	Key      string `json:"key"`
	Titel    string `json:"titel"`
	Text     string `json:"text"`
	Ziel     string `json:"ziel"`
	Knopf    string `json:"knopf"`
	Erledigt bool   `json:"erledigt"`
	Erkannt  bool   `json:"erkannt"` // vom Programm erkannt, nicht von Hand abgehakt
}

type planTermin struct {
	Datum string `json:"datum"`
	Titel string `json:"titel"`
	Art   string `json:"art"`
}

type planMonat struct {
	Monat   int          `json:"monat"`
	Punkte  []planPunkt  `json:"punkte"`
	Termine []planTermin `json:"termine"`
}

type vereinsjahrAntwort struct {
	Jahr   int         `json:"jahr"`
	Heute  string      `json:"heute"`
	Monate []planMonat `json:"monate"`
}

// terminImJahr: gibt es einen Termin dieser Art im Jahr (vorbei = nur vergangene)?
func (s *Store) terminImJahr(jahr int, art string, vorbei bool) bool {
	heute := time.Now().Format("2006-01-02")
	for _, t := range s.d.Termine {
		if t.Art == art && strings.HasPrefix(t.Datum, strconv.Itoa(jahr)+"-") && (!vorbei || t.Datum < heute) {
			return true
		}
	}
	return false
}

var vereinsjahrVorlagen = []planVorlage{
	{monat: 1, key: "kassenabschluss", titel: "Kassenbuch des Vorjahres abschließen", text: "Alle Buchungen eingetragen und jede Ausgabe mit Beleg? Dann ist der Kassenbericht bereit für die Prüfung.", ziel: "admin:kassenbericht", knopf: "Kassenbericht"},
	{monat: 1, key: "spendenbest", modul: "spenden", titel: "Zuwendungsbestätigungen verschicken", text: "Spender brauchen sie für ihre Steuererklärung.", ziel: "admin:spenden", knopf: "Spenden",
		erkannt: func(s *Store, jahr int) bool {
			n := 0
			for _, x := range s.d.Spenden {
				if strings.HasPrefix(x.Datum, strconv.Itoa(jahr-1)+"-") && !x.Bereinigt {
					if x.BestaetigtAm == "" {
						return false
					}
					n++
				}
			}
			return n > 0
		}},
	{monat: 1, key: "jahresmeldung", titel: "Jahresmeldung an den Verband", text: "Mitglieder, Flächen und Vergaben des Vorjahres – die Zahlen stehen unter »Berichte & Meldungen«.", ziel: "admin:berichte", knopf: "Berichte"},
	{monat: 2, key: "kassenpruefung", titel: "Kassenprüfung des Vorjahres", text: "Die Kassenprüfer prüfen vor der Mitgliederversammlung. Die geführte Prüfung im Kassenbericht erstellt den Bericht gleich mit.", ziel: "admin:kassenbericht", knopf: "Zur Prüfung",
		erkannt: func(s *Store, jahr int) bool { return s.d.Kassenpruefungen[yearKey(jahr-1)].Ergebnis != "" }},
	{monat: 2, key: "haushalt", modul: "haushalt", titel: "Haushaltsplan aufstellen", text: "Einnahmen und Ausgaben planen – die Versammlung beschließt ihn.", ziel: "admin:haushalt", knopf: "Haushaltsplan",
		erkannt: func(s *Store, jahr int) bool { _, ok := s.d.Haushalt[yearKey(jahr)]; return ok }},
	{monat: 2, key: "einladung", titel: "Mitgliederversammlung ansetzen und einladen", text: "Termin anlegen und die Einladung mit Tagesordnung fristgerecht nach Satzung verschicken.", ziel: "tab:termine", knopf: "Termine",
		erkannt: func(s *Store, jahr int) bool { return s.terminImJahr(jahr, "Versammlung", false) }},
	{monat: 3, key: "versammlung", titel: "Mitgliederversammlung durchführen", text: "Kassenbericht, Entlastung des Vorstands, Haushaltsplan, ggf. Wahlen. Das Protokoll lässt sich direkt im Termin schreiben.", ziel: "tab:termine", knopf: "Termine",
		erkannt: func(s *Store, jahr int) bool { return s.terminImJahr(jahr, "Versammlung", true) }},
	{monat: 3, key: "arbeitseinsaetze", titel: "Arbeitseinsätze fürs Jahr planen", text: "Als Termine anlegen – dann lassen sich die Stunden später mit wenigen Klicks buchen.", ziel: "tab:termine", knopf: "Termine",
		erkannt: func(s *Store, jahr int) bool { return s.terminImJahr(jahr, "Arbeitseinsatz", false) }},
	{monat: 4, key: "wasseran", titel: "Wasser anstellen", text: "Leitungen auf Frostschäden prüfen, Stand der Hauptwasseruhr notieren und die Pächter vorher informieren.", ziel: "admin:rundschreiben", knopf: "Rundschreiben"},
	{monat: 5, key: "begehung1", titel: "Gartenbegehung im Frühjahr", text: "Mängel und Nutzung (Drittelregelung) im Begehungsmodus direkt am Garten erfassen.", ziel: "admin:maengel", knopf: "Begehung"},
	{monat: 6, key: "maengelfristen", titel: "Mängelfristen nachhalten", text: "Abgelaufene Fristen prüfen und bei Bedarf ein Mahnschreiben erstellen.", ziel: "admin:maengel", knopf: "Mängel"},
	{monat: 6, key: "eichung", modul: "eichung", titel: "Zählertausch planen", text: "Zähler mit ablaufender Eichfrist rechtzeitig bestellen; getauscht wird am besten zur Ablesung im Herbst.", ziel: "tab:eingabe:eichung", knopf: "Eichfristen"},
	{monat: 7, key: "sommerfest", modul: "barkasse", titel: "Sommerfest abrechnen", text: "Bargeld-Einnahmen und -Ausgaben mit Belegnummer ins Kassenbuch der Barkasse.", ziel: "admin:kassenbericht", knopf: "Barkasse"},
	{monat: 7, key: "warteliste", modul: "warteliste", titel: "Warteliste durchsehen", text: "Nachfragen, ob noch Interesse besteht, und Besichtigungen für frei werdende Gärten vereinbaren.", ziel: "admin:warteliste", knopf: "Warteliste"},
	{monat: 8, key: "kuendigung", titel: "Kündigungen zum 30. November", text: "Kündigt der Verein wegen Pflichtverletzungen, muss die Kündigung spätestens am dritten Werktag im August zugehen (§ 9 BKleingG).", ziel: "admin:kuendigungen", knopf: "Kündigungen"},
	{monat: 9, key: "begehung2", titel: "Herbstbegehung", text: "Mängel aus dem Frühjahr abnehmen, frei werdende Gärten für die Wertermittlung ansehen.", ziel: "admin:maengel", knopf: "Begehung"},
	{monat: 9, key: "ablesungtermin", titel: "Zählerablesung ankündigen", text: "Termin anlegen und den Ablesebogen für den Rundgang drucken.", ziel: "tab:termine", knopf: "Termine",
		erkannt: func(s *Store, jahr int) bool { return s.terminImJahr(jahr, "Ablesung", false) }},
	{monat: 10, key: "ablesung", titel: "Zählerstände eintragen", text: "Nach dem Rundgang alle Stände eintragen – fehlende zeigt das Programm an.", ziel: "tab:eingabe", knopf: "Zählerstände",
		erkannt: func(s *Store, jahr int) bool {
			d, ok := s.zaehlerFehlenLocked(jahr)
			return ok && d.Gesamt > 0 && len(d.Fehlend) == 0
		}},
	{monat: 10, key: "wasserab", titel: "Wasser abstellen", text: "Leitungen entleeren und die Hauptwasseruhr ablesen. Der Wasserschwund im Kassenbericht zeigt, ob Leitungen undicht sind.", ziel: "admin:kassenbericht", knopf: "Kassenbericht"},
	{key: "rechnungen", titel: "Jahresrechnungen erstellen und verschicken", text: "Arbeitsstunden eintragen, Rechnungen prüfen und ausstellen.", ziel: "tab:rechnungen", knopf: "Rechnungen",
		erkannt: func(s *Store, jahr int) bool {
			versatz, _, _ := s.rhythmusLocked()
			for _, r := range s.d.Rechnungen {
				if r.Jahr == jahr-versatz && r.Status == statusGueltig {
					return true
				}
			}
			return false
		}},
	{monat: 11, key: "wechsel", titel: "Pächterwechsel zum 30. November", text: "Übergabe, Wertermittlung und Schlussrechnung für gekündigte Gärten.", ziel: "admin:kuendigungen", knopf: "Kündigungen"},
	{key: "zahlungen", titel: "Zahlungseingänge prüfen", text: "Kontoauszug einlesen; für offene Rechnungen Erinnerung oder Mahnung erstellen.", ziel: "tab:zahlungen", knopf: "Zahlungen"},
	{monat: 12, key: "sicherung", titel: "Sicherung außer Haus", text: "Eine Datensicherung auf USB-Stick oder Netzlaufwerk – getrennt vom Vereinsrechner aufbewahren.", ziel: "admin:daten", knopf: "Sicherung"},
	{key: "jahreswechsel", titel: "Abrechnungsjahr abschließen", text: "Wenn alle Rechnungen raus sind, das neue Jahr im Programm beginnen.", ziel: "admin:jahreswechsel", knopf: "Jahreswechsel",
		erkannt: func(s *Store, jahr int) bool {
			versatz, _, _ := s.rhythmusLocked()
			return s.d.Settings.Jahr > jahr-versatz
		}},
}

func planVorlageGueltig(key string) bool {
	return slices.ContainsFunc(vereinsjahrVorlagen, func(v planVorlage) bool { return v.key == key })
}

func (s *Store) vereinsjahrLocked(jahr int, heute time.Time) vereinsjahrAntwort {
	a := vereinsjahrAntwort{Jahr: jahr, Heute: heute.Format("2006-01-02"), Monate: make([]planMonat, 12)}
	for i := range a.Monate {
		a.Monate[i] = planMonat{Monat: i + 1, Punkte: []planPunkt{}, Termine: []planTermin{}}
	}
	haken := s.d.Vereinsjahr[yearKey(jahr)]
	versatz, mRechnung, mZahlung := s.rhythmusLocked()
	for _, v := range vereinsjahrVorlagen {
		if v.modul != "" && !s.modulAn(v.modul) {
			continue
		}
		p := planPunkt{Key: v.key, Titel: v.titel, Text: v.text, Ziel: v.ziel, Knopf: v.knopf}
		if p.Ziel == "admin:rundschreiben" && !s.modulAn("rundschreiben") {
			p.Ziel, p.Knopf = "tab:termine", "Termine"
		}
		monat := v.monat
		switch v.key {
		case "rechnungen":
			monat = mRechnung
			p.Titel = fmt.Sprintf("Jahresrechnungen %d erstellen und verschicken", jahr-versatz)
		case "jahreswechsel":
			monat = mRechnung
			p.Titel = fmt.Sprintf("Abrechnungsjahr %d abschließen", jahr-versatz)
		case "zahlungen":
			monat = mZahlung
		}
		if v.erkannt != nil && v.erkannt(s, jahr) {
			p.Erledigt, p.Erkannt = true, true
		} else {
			p.Erledigt = slices.Contains(haken, v.key)
		}
		a.Monate[monat-1].Punkte = append(a.Monate[monat-1].Punkte, p)
	}
	praefix := strconv.Itoa(jahr) + "-"
	for _, t := range s.d.Termine {
		if strings.HasPrefix(t.Datum, praefix) && len(t.Datum) >= 7 {
			if m, err := strconv.Atoi(t.Datum[5:7]); err == nil && m >= 1 && m <= 12 {
				a.Monate[m-1].Termine = append(a.Monate[m-1].Termine, planTermin{Datum: t.Datum, Titel: t.Titel, Art: t.Art})
			}
		}
	}
	for i := range a.Monate {
		slices.SortFunc(a.Monate[i].Termine, func(x, y planTermin) int { return strings.Compare(x.Datum, y.Datum) })
	}
	return a
}

func planJahr(r *http.Request) int {
	if j, err := strconv.Atoi(r.URL.Query().Get("jahr")); err == nil && j >= 2000 && j <= 2100 {
		return j
	}
	return time.Now().Year()
}

func (a *App) handleVereinsjahr(w http.ResponseWriter, r *http.Request) {
	jahr := planJahr(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.vereinsjahrLocked(jahr, time.Now()))
}

// handleVereinsjahrHaken hakt einen Punkt von Hand ab oder nimmt den Haken zurück.
func (a *App) handleVereinsjahrHaken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Jahr     int    `json:"jahr"`
		Key      string `json:"key"`
		Erledigt bool   `json:"erledigt"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Jahr < 2000 || in.Jahr > 2100 || !planVorlageGueltig(in.Key) {
		writeErr(w, bad("Unbekannter Punkt im Vereinsjahr"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.st.d.Vereinsjahr == nil {
		a.st.d.Vereinsjahr = map[string][]string{}
	}
	k := yearKey(in.Jahr)
	l := slices.DeleteFunc(a.st.d.Vereinsjahr[k], func(x string) bool { return x == in.Key })
	if in.Erledigt {
		l = append(l, in.Key)
	}
	if len(l) == 0 {
		delete(a.st.d.Vereinsjahr, k)
	} else {
		a.st.d.Vereinsjahr[k] = l
	}
	a.speichernUndAntworten(w, a.st.vereinsjahrLocked(in.Jahr, time.Now()))
}

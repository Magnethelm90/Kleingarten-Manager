package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Jahresmeldung: die Zahlen, die Landes- und Regionalverbände jedes Jahr von ihren Vereinen abfragen
// (Parzellen, Flächen, Pächterwechsel, Warteliste, Gemeinschaftsarbeit, Abgaben). Das Programm kennt sie
// ohnehin; hier stehen sie gesammelt zum Abschreiben in das Formular des Verbands oder als Excel-Datei.

type meldeZeile struct {
	Gruppe  string  `json:"gruppe"`
	Angabe  string  `json:"angabe"`
	Wert    float64 `json:"wert"`
	Einheit string  `json:"einheit"` // "", "m²", "€", "h", "m³", "kWh", "%"
}

type jahresmeldung struct {
	Jahr   int          `json:"jahr"`
	Verein string       `json:"verein"`
	Stand  string       `json:"stand"`
	Zeilen []meldeZeile `json:"zeilen"`
}

func (s *Store) jahresmeldungLocked(year int, jetzt time.Time) (jahresmeldung, bool) {
	v, ok := s.viewLocked(year)
	if !ok {
		return jahresmeldung{}, false
	}
	k := s.kassenberichtLocked(v)
	m := jahresmeldung{Jahr: year, Verein: v.Settings.VereinName, Stand: jetzt.Format("2006-01-02")}
	add := func(gruppe, angabe string, wert float64, einheit string) {
		m.Zeilen = append(m.Zeilen, meldeZeile{gruppe, angabe, round2(wert), einheit})
	}
	praefix := strconv.Itoa(year) + "-"

	// Parzellen: verpachtet = Pächter mit Gartennummer am Jahresende (ausgeschiedene zählen beim Nachfolger),
	// frei = Gärten im Lageplan ohne Pächter (nur im laufenden Jahr bekannt)
	verpachtet, flaeche := map[string]bool{}, 0.0
	mitglieder := 0
	for _, p := range v.Paechter {
		if p.AusgeschiedenAm != "" || (p.Geloescht && !v.ReadOnly) {
			continue
		}
		mitglieder++
		if nr := strings.TrimSpace(p.Gartennr); nr != "" && !verpachtet[nr] {
			verpachtet[nr] = true
			flaeche += p.Gartengroesse
		}
	}
	frei := 0
	if !v.ReadOnly {
		for nr := range s.d.Lageplan {
			if !verpachtet[nr] {
				frei++
			}
		}
	}
	add("Parzellen", "Parzellen insgesamt", float64(len(verpachtet)+frei), "")
	add("Parzellen", "davon verpachtet", float64(len(verpachtet)), "")
	add("Parzellen", "davon frei (laut Lageplan)", float64(frei), "")
	add("Parzellen", "Fläche der verpachteten Parzellen", flaeche, "m²")
	if len(verpachtet) > 0 {
		add("Parzellen", "durchschnittliche Parzellengröße", flaeche/float64(len(verpachtet)), "m²")
	}
	add("Parzellen", "Gemeinschaftsfläche (Anteil Verein)", v.Settings.VereinsflaecheQm, "m²")
	add("Parzellen", "Fläche freier Gärten (Pachtanteil)", v.Settings.FreieGaertenQm, "m²")

	// Mitglieder und Bewegungen im Kalenderjahr
	// gezählt werden nur echte Vergaben, nicht das erstmalige Eintragen langjähriger Pächter ins Programm
	neu, wechsel, ausgetreten := 0, 0, 0
	for _, e := range s.d.GartenHistorie {
		if e.Vergeben && strings.HasPrefix(e.Seit, praefix) {
			neu++
		}
	}
	for _, u := range s.d.Uebergaben {
		if strings.HasPrefix(u.Datum, praefix) {
			wechsel++
		}
	}
	for _, p := range s.d.Paechter {
		if p.Geloescht && p.AusgeschiedenAm == "" && strings.HasPrefix(p.GeloeschtAm, praefix) {
			ausgetreten++ // ohne Pächterwechsel in den Papierkorb gelegt, z. B. Kündigung
		}
	}
	add("Pächter", "Pächter (Mitglieder mit Garten)", float64(mitglieder), "")
	add("Pächter", "neu vergebene Gärten im Jahr", float64(neu), "")
	add("Pächter", "davon durch Pächterwechsel", float64(wechsel), "")
	add("Pächter", "Pächter ausgeschieden ohne Nachfolger", float64(ausgetreten), "")
	add("Pächter", "Interessenten auf der Warteliste (heute)", float64(len(s.d.Warteliste)), "")

	// Gemeinschaftsarbeit
	stunden, mit, erfuellt := 0.0, 0, 0
	for _, p := range v.Paechter {
		if a := v.Ablesungen[p.ID]; a.Stunden != nil && p.AusgeschiedenAm == "" {
			stunden += *a.Stunden
			mit++
			if *a.Stunden >= v.Settings.Pflichtstunden {
				erfuellt++
			}
		}
	}
	add("Gemeinschaftsarbeit", "geleistete Stunden insgesamt", stunden, "h")
	add("Gemeinschaftsarbeit", "Pflichtstunden je Garten", v.Settings.Pflichtstunden, "h")
	add("Gemeinschaftsarbeit", "Gärten mit erfüllter Pflicht", float64(erfuellt), "")
	add("Gemeinschaftsarbeit", "Ersatzzahlungen für fehlende Stunden", k.Summen.NachzahlungStunden, "€")

	// Abgaben und Verbrauch laut Abrechnung
	add("Abrechnung", "Pacht (Gärten, Vereins- und freie Flächen)", k.Summen.PachtGarten+k.Summen.PachtVerein+k.Summen.PachtFrei, "€")
	add("Abrechnung", "Pacht je m²", v.Settings.PachtJeQm, "€")
	add("Abrechnung", "Mitgliedsbeiträge einschließlich Verband", k.Summen.Mitgliedsbeitrag, "€")
	add("Abrechnung", "darin Beitrag an den Territorialverband", v.Settings.Territorialverband*float64(k.Ausgestellt+k.Berechnet), "€")
	add("Abrechnung", "Umlagen", k.Summen.Umlage, "€")
	add("Abrechnung", "Versicherungen", k.Versicherung, "€")
	add("Abrechnung", "Wasserverbrauch der Pächter", k.WasserVerbrauch, "m³")
	add("Abrechnung", "Stromverbrauch der Pächter", k.StromVerbrauch, "kWh")
	add("Abrechnung", "Rechnungssumme", k.Summen.Gesamt, "€")

	// Kasse
	add("Kasse", "Kassenbestand zu Jahresbeginn", k.Anfangsbestand, "€")
	add("Kasse", "Einnahmen von Pächtern", k.EinnahmenBezahlt, "€")
	add("Kasse", "Spenden", k.Spenden, "€")
	add("Kasse", "sonstige Ausgaben", k.AusgabenSumme, "€")
	add("Kasse", "Kassenbestand heute", k.Kassenbestand, "€")
	return m, true
}

func (a *App) handleJahresmeldung(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	m, ok := a.st.jahresmeldungLocked(year, time.Now())
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Dieses Jahr gibt es nicht"))
		return
	}
	if r.URL.Query().Get("format") != "xlsx" {
		writeJSON(w, 200, m)
		return
	}
	rows := [][]xCell{
		{{"Jahresmeldung " + strconv.Itoa(m.Jahr) + " – " + m.Verein, stBold}},
		{{"Stand " + germanDate(m.Stand) + " · erstellt mit dem Kleingarten-Manager", stNormal}},
		{},
		{{"Bereich", stHeader}, {"Angabe", stHeader}, {"Wert", stHeader}, {"Einheit", stHeader}},
	}
	for _, z := range m.Zeilen {
		st := stNum
		if z.Einheit == "€" {
			st = stEUR
		}
		rows = append(rows, []xCell{{z.Gruppe, stNormal}, {z.Angabe, stNormal}, {z.Wert, st}, {z.Einheit, stNormal}})
	}
	data, err := writeXLSX("Jahresmeldung", []float64{20, 46, 16, 10}, rows)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Jahresmeldung_%d.xlsx"`, m.Jahr))
	_, _ = w.Write(data)
}

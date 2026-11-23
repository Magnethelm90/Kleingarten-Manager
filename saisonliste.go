package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Saison-Liste für den Wasserwart: Frühjahr (Wasser an) und Herbst (Wasser ab, winterfest) als Vordruck für den
// Rundgang – eine Zeile je Garten mit Feldern zum Abhaken, für den Zählerstand und für Schäden.

var saisonArten = map[string]struct{ Titel, Hinweis string }{
	"fruehjahr": {"Wasser anstellen im Frühjahr",
		"Ablauf: 1. Gartenleitungen und Zapfstellen auf Frostschäden prüfen. 2. Hauptabsperrhahn langsam öffnen, Stand der Hauptwasseruhr notieren. " +
			"3. Je Garten Hahn öffnen und auf Tropfen oder Nässe achten. 4. Schäden in der Spalte Bemerkung festhalten und dem Vorstand melden."},
	"herbst": {"Wasser abstellen im Herbst",
		"Ablauf: 1. Pächter vorher informieren, Zählerstand (Ablesung) notieren. 2. Hauptwasseruhr ablesen, Hauptabsperrhahn schließen. " +
			"3. Leitungen und Zapfstellen entleeren, Hähne offen lassen. 4. Wasserzähler und Außenhähne frostsicher machen; Bemerkungen festhalten."},
}

func (a *App) saisonlistePDF(r *http.Request) ([]byte, string, error) {
	art := r.URL.Query().Get("art")
	v, ok := saisonArten[art]
	if !ok {
		return nil, "", bad("Bitte Frühjahr oder Herbst wählen")
	}
	a.st.mu.Lock()
	s := a.st.d.Settings
	var l []Paechter
	for _, p := range a.st.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" && strings.TrimSpace(p.Gartennr) != "" {
			l = append(l, p)
		}
	}
	a.st.mu.Unlock()
	if len(l) == 0 {
		return nil, "", bad("Es sind keine Gärten mit Gartennummer vorhanden")
	}
	sort.SliceStable(l, func(i, j int) bool { return naturalLess(l[i].Gartennr, l[j].Gartennr) })
	titel := fmt.Sprintf("%s %d", v.Titel, time.Now().Year())
	pdf := neuesPDF(s, titel, true)
	kopf := []string{"Garten", "Pächter", "Wasserzähler", "Erledigt", "Datum", "Zählerstand", "Bemerkung / Schaden"}
	breiten := []float64{16, 48, 34, 18, 22, 28, 91}
	zeilen := make([][]string, 0, len(l))
	for _, p := range l {
		zeilen = append(zeilen, []string{p.Gartennr, p.Name, p.WasserzaehlerNr, "☐", "", "", ""})
	}
	pdfListe(pdf, s, titel, v.Hinweis, kopf, breiten, zeilen, 9.5)
	out, err := pdfBytes(pdf)
	return out, "Saisonliste-" + art + ".pdf", err
}

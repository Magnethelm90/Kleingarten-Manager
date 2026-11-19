package main

import (
	"net/http"
	"sort"
)

// Ausgewählte Mängel als Liste drucken (z. B. für den Nachkontroll-Rundgang): GET /api/admin/maengel-liste?ids=a,b
func (a *App) maengelListePDF(r *http.Request) ([]byte, string, error) {
	ids := idsParam(r.URL.Query().Get("ids"))
	if len(ids) == 0 || len(ids) > 2000 {
		return nil, "", bad("Bitte Mängel auswählen")
	}
	gewaehlt := map[string]bool{}
	for _, id := range ids {
		gewaehlt[id] = true
	}
	a.st.mu.Lock()
	s := a.st.d.Settings
	namen := map[string]string{}
	for _, p := range a.st.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" && p.Gartennr != "" {
			namen[p.Gartennr] = p.Name
		}
	}
	var l []Mangel
	for _, m := range a.st.d.Maengel {
		if gewaehlt[m.ID] {
			m.FotoBytes = nil
			l = append(l, m)
		}
	}
	a.st.mu.Unlock()
	if len(l) == 0 {
		return nil, "", notFound("Die ausgewählten Mängel gibt es nicht mehr")
	}
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Gartennr != l[j].Gartennr {
			return naturalLess(l[i].Gartennr, l[j].Gartennr)
		}
		return l[i].Datum < l[j].Datum
	})
	titel := "Mängelliste"
	pdf := neuesPDF(s, titel, true)
	kopf := []string{"Garten", "Pächter", "Art", "Mangel", "Festgestellt", "Frist", "Behoben", "Bemerkung"}
	breiten := []float64{16, 38, 32, 74, 22, 20, 18, 37}
	var zeilen [][]string
	for _, m := range l {
		frist := ""
		if m.Frist != "" {
			frist = germanDate(m.Frist)
		}
		behoben := "☐"
		if m.ErledigtAm != "" {
			behoben = germanDate(m.ErledigtAm)
		}
		zeilen = append(zeilen, []string{m.Gartennr, namen[m.Gartennr], m.Art, m.Beschreibung, germanDate(m.Datum), frist, behoben, ""})
	}
	pdfListe(pdf, s, titel, "Ausgewählte Mängel zur Nachkontrolle. Vertraulich – nicht aushängen.", kopf, breiten, zeilen, 9)
	out, err := pdfBytes(pdf)
	return out, "Maengelliste.pdf", err
}

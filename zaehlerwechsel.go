package main

import (
	"fmt"
	"net/http"
	"sort"
)

// Zählerwechsel-Protokoll: Die Wechsel selbst stehen bei der Ablesung (ZaehlerWechsel). Das Protokoll listet alle
// getauschten Zähler eines Jahres mit Endstand des alten und Anfangsstand des neuen Zählers – zum Abheften und
// zum Gegenzeichnen durch Wart und Pächter. Es verändert nichts.

type wechselZeile struct {
	Gartennr, Name, Art, NeueNr, AltEnde, NeuStart string
}

func fmtStand(p *float64) string {
	if p == nil {
		return "–"
	}
	return fmtFlex(*p)
}

// zaehlerwechselLocked sammelt die Wechsel eines Jahres. Der Aufrufer hält s.mu.
func (s *Store) zaehlerwechselLocked(jahr int) ([]wechselZeile, Settings, bool) {
	v, ok := s.viewLocked(jahr)
	if !ok {
		return nil, Settings{}, false
	}
	var l []wechselZeile
	for _, p := range v.Paechter {
		a := v.Ablesungen[p.ID]
		for _, e := range []struct {
			art string
			w   *ZaehlerWechsel
		}{{"Wasser", a.WasserWechsel}, {"Strom", a.StromWechsel}} {
			if e.w == nil {
				continue
			}
			l = append(l, wechselZeile{p.Gartennr, p.Name, e.art, e.w.NeueNr, fmtStand(e.w.AltEnde), fmtStand(e.w.NeuStart)})
		}
	}
	sort.SliceStable(l, func(i, j int) bool { return naturalLess(l[i].Gartennr, l[j].Gartennr) })
	return l, v.Settings, true
}

func (a *App) wechselProtokollPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	l, s, ok := a.st.zaehlerwechselLocked(jahr)
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Das Jahr gibt es nicht")
	}
	if len(l) == 0 {
		return nil, "", bad(fmt.Sprintf("Im Jahr %d wurde kein Zähler als getauscht eingetragen", jahr))
	}
	titel := fmt.Sprintf("Zählerwechsel-Protokoll %d", jahr)
	pdf := neuesPDF(s, titel, true)
	kopf := []string{"Garten", "Pächter", "Zähler", "Neue Nummer", "Endstand alt", "Anfangsstand neu", "Unterschrift Wart", "Unterschrift Pächter"}
	breiten := []float64{16, 48, 18, 34, 26, 28, 44, 43}
	zeilen := make([][]string, 0, len(l))
	for _, z := range l {
		zeilen = append(zeilen, []string{z.Gartennr, z.Name, z.Art, z.NeueNr, z.AltEnde, z.NeuStart, "", ""})
	}
	pdfListe(pdf, s, titel, "Die Stände wurden beim Tausch abgelesen und stimmen mit den Angaben im Programm überein.", kopf, breiten, zeilen, 9.5)
	out, err := pdfBytes(pdf)
	return out, fmt.Sprintf("Zaehlerwechsel-%d.pdf", jahr), err
}

package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Fristenkalender: Kalenderdatei (.ics) für den Vorstand mit den Fristen, die sonst nur im Cockpit auftauchen –
// Mängelfristen, Kündigungstermine von Verträgen und Zahlungsfristen laufender Mahnungen. Ganztägige Einträge mit
// Erinnerung am Vortag. Die Datei enthält Namen nur dort, wo sie für den Vorstand nötig sind (Gartennummer, keine Namen).

type fristEintrag struct {
	Datum, Titel, Text, Kennung string
}

// fristenLocked sammelt die künftigen und gerade fälligen Fristen. Der Aufrufer hält s.mu.
func (s *Store) fristenLocked(heute time.Time) []fristEintrag {
	ab := heute.AddDate(0, 0, -7).Format("2006-01-02")
	var l []fristEintrag
	for _, m := range s.d.Maengel {
		if m.ErledigtAm == "" && m.Frist >= ab && m.Frist != "" {
			l = append(l, fristEintrag{m.Frist, "Mängelfrist Garten " + m.Gartennr, m.Art + ": " + m.Beschreibung, "mangel-" + m.ID})
		}
	}
	for _, v := range s.d.Vertraege {
		a := v.ansicht(heute)
		if a.Gekuendigt == "" && a.KuendigenBis != "" && a.KuendigenBis >= ab {
			l = append(l, fristEintrag{a.KuendigenBis, "Kündigungsfrist: " + v.Bezeichnung, "Letzter Tag, um den Vertrag zum " + germanDate(a.LaeuftBis) + " zu kündigen.", "vertrag-" + v.ID})
		}
	}
	for _, r := range s.d.Rechnungen {
		if r.Status != "gueltig" || r.BezahltAm != "" || len(r.Mahnungen) == 0 {
			continue
		}
		m := r.Mahnungen[len(r.Mahnungen)-1]
		if m.Frist >= ab {
			l = append(l, fristEintrag{m.Frist, "Zahlungsfrist " + mahnTitel[min(max(m.Stufe, 1), 3)] + " (Rechnung " + r.Nummer + ")", "Danach nächste Mahnstufe prüfen.", "mahnung-" + r.ID})
		}
	}
	sort.SliceStable(l, func(i, j int) bool { return l[i].Datum < l[j].Datum })
	return l
}

func (a *App) handleFristenICS(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	l := a.st.fristenLocked(time.Now())
	verein := a.st.d.Settings.VereinName
	a.st.mu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//%s//DE\r\nCALSCALE:GREGORIAN\r\nX-WR-CALNAME:%s\r\n", appName, icsEscape("Fristen – "+verein))
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, f := range l {
		d, err := parseDate(f.Datum)
		if err != nil {
			continue
		}
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:frist-%s@%s\r\nDTSTAMP:%s\r\n", f.Kennung, appID, stamp)
		fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\n", d.Format("20060102"), d.AddDate(0, 0, 1).Format("20060102"))
		fmt.Fprintf(&b, "SUMMARY:%s\r\nDESCRIPTION:%s\r\n", icsEscape(f.Titel), icsEscape(f.Text))
		b.WriteString("BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-P1D\r\nDESCRIPTION:" + icsEscape(f.Titel) + "\r\nEND:VALARM\r\n")
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Vereinsfristen.ics"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}

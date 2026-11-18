package main

import (
	"fmt"
	"time"
)

// Jahresplaner-Erinnerungen: Punkte des Vereinsjahres (vereinsjahr.go), die noch nicht erledigt sind, erscheinen im
// Cockpit, sobald ihr Monat zwei Wochen entfernt ist – und bleiben bis Monatsende. Was das Programm selbst erkennt
// (Rechnungen erstellt, Zähler abgelesen …) oder schon abgehakt ist, taucht nicht mehr auf. Höchstens vier Hinweise,
// damit das Cockpit übersichtlich bleibt; mehr zeigt der Reiter »Vereinsjahr« auf der Übersicht.

const (
	planerVorlaufTage = 14
	planerMaxHinweise = 4
)

func (s *Store) planerAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	if len(s.d.Paechter) == 0 {
		return // ein leerer Verein braucht erst die Einrichtung, keinen Jahresplan
	}
	tag := time.Date(heute.Year(), heute.Month(), heute.Day(), 0, 0, 0, 0, time.Local)
	n := 0
	for _, jahr := range []int{heute.Year(), heute.Year() + 1} {
		plan := s.vereinsjahrLocked(jahr, heute)
		for _, m := range plan.Monate {
			start := time.Date(jahr, time.Month(m.Monat), 1, 0, 0, 0, 0, time.Local)
			ende := start.AddDate(0, 1, -1)
			if tag.Before(start.AddDate(0, 0, -planerVorlaufTage)) || tag.After(ende) {
				continue
			}
			for _, p := range m.Punkte {
				if p.Erledigt || n >= planerMaxHinweise {
					continue
				}
				wann := fmt.Sprintf("Im %s", monatsnamen[m.Monat-1])
				if tag.Before(start) {
					wann = fmt.Sprintf("Ab %s", monatsnamen[m.Monat-1])
				}
				add("info", fmt.Sprintf("%s: %s", wann, p.Titel), p.Text, p.Ziel, p.Knopf)
				n++
			}
		}
	}
}

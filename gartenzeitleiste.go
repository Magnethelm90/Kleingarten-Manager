package main

import (
	"fmt"
	"sort"
)

// Zeitleiste der Gartenakte: Alles, was zu einem Garten geschah, in einer Reihe nach Datum (neueste zuerst).

type zeitEintrag struct {
	Datum string `json:"datum"`
	Art   string `json:"art"` // paechter | uebergabe | wertermittlung | dokument | mangel | kuendigung | nutzung
	Titel string `json:"titel"`
	Text  string `json:"text,omitempty"`
}

// zeitleisteLocked sammelt die Ereignisse eines Gartens. Der Aufrufer hält s.mu.
func (s *Store) zeitleisteLocked(nr string) []zeitEintrag {
	out := []zeitEintrag{}
	add := func(datum, art, titel, text string) {
		if datum != "" {
			out = append(out, zeitEintrag{datum, art, titel, text})
		}
	}
	for _, e := range s.d.GartenHistorie {
		if e.Gartennr != nr {
			continue
		}
		add(e.Seit, "paechter", "Pächter: "+e.Name, "Mitgliedsnr. "+e.Mitgliedsnr)
		if e.Bis != "" {
			add(e.Bis, "paechter", "Pacht beendet: "+e.Name, "")
		}
	}
	for _, u := range s.d.Uebergaben {
		if u.Gartennr == nr {
			add(u.Datum, "uebergabe", "Garten übergeben", fmt.Sprintf("%s an %s", u.AltName, u.NeuName))
		}
	}
	for _, w := range s.d.Wertermittlungen {
		if w.Gartennr == nr {
			add(w.Datum, "wertermittlung", "Wertermittlung ("+w.Anlass+")", "Summe "+fmtEUR(w.Summe))
		}
	}
	for _, d := range s.d.Dokumente {
		if d.Gartennr == nr {
			add(d.Datum, "dokument", d.Art+": "+d.Titel, "")
		}
	}
	for _, m := range s.d.Maengel {
		if m.Gartennr != nr {
			continue
		}
		add(m.Datum, "mangel", "Mangel festgestellt: "+m.Art, m.Beschreibung)
		add(m.ErledigtAm, "mangel", "Mangel behoben: "+m.Art, "")
	}
	for _, k := range s.d.Kuendigungen {
		if k.Gartennr == nr {
			add(k.Eingang, "kuendigung", "Kündigung eingegangen", "zum "+germanDate(k.Zum))
		}
	}
	for _, n := range s.d.Nutzungen {
		if n.Gartennr == nr {
			add(n.Datum, "nutzung", "Nutzung erhoben", fmt.Sprintf("Anbau %s %%, Erholung %s %%, Bebauung %s %%", fmtFlex(n.Anbau), fmtFlex(n.Erholung), fmtFlex(n.Bebauung)))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Datum > out[j].Datum })
	return out
}

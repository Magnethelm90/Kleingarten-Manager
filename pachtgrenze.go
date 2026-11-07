package main

import (
	"fmt"
	"math"
	"net/http"
	"time"
)

// Pachtobergrenze nach § 5 Abs. 1 Bundeskleingartengesetz: Die Pacht darf höchstens das Vierfache der
// ortsüblichen Pacht im erwerbsmäßigen Obst- und Gemüseanbau betragen (der Wert kommt vom Gutachterausschuss
// oder vom Verband). Das Programm vergleicht den eigenen Pachtsatz je m² damit.

type Pachtgrenze struct {
	ErwerbsobstbauJeQm float64 `json:"erwerbsobstbauJeQm"` // ortsübliche Pacht in €/m² und Jahr
	Quelle             string  `json:"quelle,omitempty"`   // z. B. »Gutachterausschuss 2024«
}

type pachtgrenzeAnsicht struct {
	Pachtgrenze
	PachtJeQm  float64  `json:"pachtJeQm"`
	Hoechst    *float64 `json:"hoechst,omitempty"`    // 4 × ortsübliche Pacht
	Auslastung *float64 `json:"auslastung,omitempty"` // eigener Satz in % der Höchstpacht
	Ueber      bool     `json:"ueber"`
}

func (s *Store) pachtgrenzeLocked() pachtgrenzeAnsicht {
	g := s.d.Pachtgrenze
	a := pachtgrenzeAnsicht{Pachtgrenze: g, PachtJeQm: s.d.Settings.PachtJeQm}
	if g.ErwerbsobstbauJeQm > 0 {
		h := math.Round(g.ErwerbsobstbauJeQm*4*10000) / 10000
		p := math.Round(a.PachtJeQm/h*1000) / 10
		a.Hoechst, a.Auslastung = &h, &p
		a.Ueber = a.PachtJeQm > h+0.00005
	}
	return a
}

func (a *App) handlePachtgrenze(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if r.Method == http.MethodPut {
		var in Pachtgrenze
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, err)
			return
		}
		if !validNum(in.ErwerbsobstbauJeQm) || in.ErwerbsobstbauJeQm > 10 {
			writeErr(w, bad("Bitte die ortsübliche Pacht in Euro je m² und Jahr eintragen (z. B. 0,03)"))
			return
		}
		in.Quelle = trim(in.Quelle, 120)
		a.st.d.Pachtgrenze = in
		a.st.audit("Pachtobergrenze: ortsübliche Pacht %s €/m²", fmtFlex(in.ErwerbsobstbauJeQm))
		a.speichernUndAntworten(w, a.st.pachtgrenzeLocked())
		return
	}
	writeJSON(w, 200, a.st.pachtgrenzeLocked())
}

func (s *Store) pachtgrenzeAufgabenLocked(_ time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	if g := s.pachtgrenzeLocked(); g.Ueber {
		add("warn", "Pacht über der gesetzlichen Höchstgrenze",
			fmt.Sprintf("Der Pachtsatz von %s €/m² liegt über dem Vierfachen der ortsüblichen Pacht (%s €/m², § 5 BKleingG).", fmtFlex(g.PachtJeQm), fmtFlex(*g.Hoechst)),
			"admin:einstellungen", "Einstellungen")
	}
}

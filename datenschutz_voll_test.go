package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Endgültig gelöschte Pächter: weder Name noch interne ID dürfen außerhalb der aufbewahrungspflichtigen Unterlagen
// stehen bleiben – weder im Bestand noch in einer Datei im Datenordner (Sicherungen eingeschlossen).
func TestEndgueltigGeloeschtUeberall(t *testing.T) {
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", map[string]any{})
	a.st.mu.Lock()
	type person struct{ id, name string }
	var alle []person
	for _, p := range a.st.d.Paechter {
		alle = append(alle, person{p.ID, p.Name})
	}
	a.st.mu.Unlock()
	for _, p := range alle {
		do(h, "DELETE", "/api/admin/paechter/"+p.id, nil)
		if rec := do(h, "DELETE", "/api/admin/paechter/"+p.id+"/endgueltig", nil); rec.Code != 200 {
			t.Errorf("endgültig %s: %d %s", p.name, rec.Code, rec.Body.String())
		}
	}
	a.st.mu.Lock()
	rest := *a.st.d
	rest.Rechnungen, rest.Jahre, rest.Lastschriften, rest.Spenden = nil, nil, nil, nil
	ohne, _ := json.Marshal(&rest)
	var felder map[string]json.RawMessage
	_ = json.Unmarshal(ohne, &felder)
	a.st.mu.Unlock()
	for _, p := range alle {
		for k, v := range felder {
			for _, s := range []string{p.name, p.id} {
				if i := strings.Index(string(v), s); i >= 0 {
					t.Errorf("%s: %q in %s: …%s…", p.name, s, k, string(v)[max(0, i-80):min(len(v), i+40)])
				}
			}
		}
	}
	// Dateien im Datenordner (ohne Sicherungen und Rechnungs-PDFs)
	_ = filepath.Walk(a.st.dir, func(pfad string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(a.st.dir, pfad)
		raw, _ := os.ReadFile(pfad)
		for _, p := range alle {
			if strings.Contains(string(raw), p.name) {
				t.Errorf("Datei %s enthält noch %q", rel, p.name)
			}
		}
		return nil
	})
}

// Auskunft: Papierkorb und gelernter Bankabgleich gehören dazu; Bewerber der Warteliste bekommen eine eigene Auskunft.
func TestAuskunftVollstaendig(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika Muster", "gartennr": "1", "gartengroesse": 300}))
	a.st.mu.Lock()
	a.st.kontoLernenLocked("ERIKA MUSTER", p.ID)
	a.st.papierkorbLegeLocked("mangel", "Hecke zu hoch", p.ID, Mangel{ID: "m1"})
	a.st.d.Warteliste = append(a.st.d.Warteliste, Bewerber{ID: "b1", Name: "Hans Wartend", Seit: "2025-01-01"})
	a.st.mu.Unlock()
	x := decode[auskunft](t, do(h, "GET", "/api/admin/paechter/"+p.ID+"/auskunft", nil))
	if len(x.Papierkorb) != 1 || x.Bankabgl == "" {
		t.Errorf("Papierkorb %v, Bankabgleich %q", x.Papierkorb, x.Bankabgl)
	}
	rec := do(h, "GET", "/api/admin/warteliste/b1/auskunft", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Hans Wartend") {
		t.Errorf("Bewerber-Auskunft: %d %s", rec.Code, rec.Body.String())
	}
}

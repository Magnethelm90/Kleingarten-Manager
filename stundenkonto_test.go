package main

import (
	"strings"
	"testing"
	"time"
)

func TestStundenkonto(t *testing.T) {
	a, h := newTestApp(t)
	jahr := a.st.d.Settings.Jahr
	do(h, "PUT", "/api/admin/settings", map[string]any{})
	p1 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Stunde", "gartennr": "1", "gartengroesse": 100}))
	p2 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Bernd Stunde", "gartennr": "2", "gartengroesse": 100}))
	a.st.mu.Lock()
	a.st.d.Settings.Pflichtstunden, a.st.d.Settings.NachzahlungJeStd = 12, 10
	a.st.mu.Unlock()

	buchen := func(id string, j int, std float64) int {
		return do(h, "POST", "/api/admin/stundenkonto/buchen", map[string]any{"paechterId": id, "jahr": j, "stunden": std}).Code
	}
	if buchen(p1.ID, jahr, 5) != 200 || buchen(p1.ID, jahr, 4.5) != 200 || buchen(p2.ID, jahr, 12) != 200 {
		t.Fatal("Buchung fehlgeschlagen")
	}
	for _, c := range []int{buchen(p1.ID, jahr, 0), buchen(p1.ID, jahr, 99), buchen("gibtesnicht", jahr, 1), buchen(p1.ID, 1999, 1)} {
		if c < 400 {
			t.Errorf("ungültige Buchung angenommen: %d", c)
		}
	}
	k := decode[stundenKonto](t, do(h, "GET", "/api/admin/stundenkonto", nil))
	if len(k.Zeilen) != 2 || k.Zeilen[0].Ist != 9.5 || k.Zeilen[0].Fehlt != 2.5 || k.Zeilen[0].Ersatz != 25 || !k.Zeilen[1].Erfuellt || k.Erfuellt != 1 || k.ErsatzSumme != 25 {
		t.Fatalf("Konto: %+v", k)
	}
	// zurücknehmen
	buchen(p1.ID, jahr, -9.5)
	k = decode[stundenKonto](t, do(h, "GET", "/api/admin/stundenkonto", nil))
	if k.Zeilen[0].Ist != 0 || k.Zeilen[0].Ersatz != 120 {
		t.Errorf("zurückgenommen: %+v", k.Zeilen[0])
	}
	// Buchung steht im Protokoll
	if !strings.Contains(a.st.d.AuditLog[len(a.st.d.AuditLog)-1].Aktion, "Garten 1") {
		t.Error("Buchung fehlt im Protokoll")
	}
	// Cockpit: erst ab Oktober
	var l []string
	add := func(stufe, titel, text, ziel, knopf string) { l = append(l, titel) }
	a.st.mu.Lock()
	a.st.stundenAufgabeLocked(time.Date(jahr, 6, 1, 0, 0, 0, 0, time.UTC), add)
	a.st.stundenAufgabeLocked(time.Date(jahr, 11, 1, 0, 0, 0, 0, time.UTC), add)
	a.st.mu.Unlock()
	if len(l) != 1 || !strings.Contains(l[0], "fehlenden Pflichtstunden") {
		t.Errorf("Cockpit: %v", l)
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWiederherstellungsProbe(t *testing.T) {
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Probe", "gartengroesse": 100})
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Bernd Probe", "gartengroesse": 100})

	// ohne Sicherung: Fehler in der Zeile, nichts bricht
	for _, n := range a.st.sicherungsNamen() {
		_ = os.Remove(filepath.Join(a.st.backupDir(), n))
	}
	rec := do(h, "POST", "/api/admin/wiederherstellungsprobe", nil)
	if rec.Code != 200 {
		t.Fatalf("Probe: %d %s", rec.Code, rec.Body)
	}
	if r := decode[probeErgebnis](t, rec); r.Ok {
		t.Error("ohne Sicherung darf die Probe nicht bestehen")
	}
	if a.st.d.Probe == nil {
		t.Fatal("Ergebnis nicht gespeichert")
	}

	// mit Sicherung: besteht
	do(h, "POST", "/api/admin/sicherung-jetzt", nil)
	r := decode[probeErgebnis](t, do(h, "POST", "/api/admin/wiederherstellungsprobe", nil))
	if !r.Ok || r.Zeilen[0].Stufe == "err" {
		t.Fatalf("Probe mit Sicherung: %+v", r)
	}

	// beschädigte Sicherung wird erkannt
	name, _ := neuesteSicherungIn(a.st.backupDir())
	if err := os.WriteFile(filepath.Join(a.st.backupDir(), name), []byte("{kaputt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := decode[probeErgebnis](t, do(h, "POST", "/api/admin/wiederherstellungsprobe", nil)); r.Ok {
		t.Error("beschädigte Sicherung muss auffallen")
	}
	if a.st.d.Probe.Ok {
		t.Error("fehlgeschlagene Probe muss gespeichert werden")
	}

	// Cockpit: Fehlschlag meldet sich, nach 90 Tagen Erinnerung, davor Ruhe
	var l []string
	add := func(stufe, titel, text, ziel, knopf string) { l = append(l, stufe+" "+titel) }
	a.st.mu.Lock()
	a.st.probeAufgabeLocked(time.Now(), add)
	a.st.d.Probe = &Wiederherstellungsprobe{Datum: time.Now().Format("2006-01-02"), Ok: true}
	n := len(l)
	a.st.probeAufgabeLocked(time.Now(), add)
	a.st.probeAufgabeLocked(time.Now().AddDate(0, 0, 91), add)
	a.st.mu.Unlock()
	if n != 1 || len(l) != 2 {
		t.Errorf("Cockpit-Hinweise: %v", l)
	}
}

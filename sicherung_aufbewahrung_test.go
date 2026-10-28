package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// legeSicherungenAn erzeugt Tages-, Ereignis- und Jahreswechsel-Sicherungen in dir.
func legeSicherungenAn(t *testing.T, dir string, tage, ereignisse, jahre int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	schreib := func(n string) {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for d := 0; d < tage; d++ {
		schreib(fmt.Sprintf("%s%s.json", backupPrefix, time.Date(2020, 1, 1+d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")))
	}
	for i := 0; i < ereignisse; i++ {
		schreib(fmt.Sprintf("%s2021-06-01_%06d-vor-Import.json", backupPrefix, i))
	}
	for y := 0; y < jahre; y++ {
		schreib(fmt.Sprintf("%s%d-01-05_100000-%s.json", backupPrefix, 2010+y, jahreswechselAnlass))
	}
}

func zaehleSicherungen(t *testing.T, dir string) (tage, ereignisse, jahre int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		switch n := e.Name(); {
		case isDailyBackup(n):
			tage++
		case isJahresSicherung(n):
			jahre++
		case istSicherung(n):
			ereignisse++
		}
	}
	return
}

// Viele Ereignissicherungen (Löschen, Import) dürfen die Sicherungen vor dem Jahreswechsel nicht
// verdrängen: der Stand jedes Abrechnungsjahres muss über die ganze Aufbewahrungsfrist bleiben.
func TestJahressicherungenBleibenErhalten(t *testing.T) {
	dir := t.TempDir()
	legeSicherungenAn(t, dir, 70, 75, 15)
	raeumeSicherungenAuf(dir)
	tage, ereignisse, jahre := zaehleSicherungen(t, dir)
	if tage != behalteTagessicherungen || ereignisse != behalteEreignissicherung || jahre != behalteJahressicherungen {
		t.Errorf("nach dem Aufräumen: %d Tages-, %d Ereignis-, %d Jahressicherungen", tage, ereignisse, jahre)
	}
	if behalteJahressicherungen < aufbewahrungJahre+1 {
		t.Errorf("Jahressicherungen (%d) decken die Aufbewahrungsfrist von %d Jahren nicht ab", behalteJahressicherungen, aufbewahrungJahre)
	}
	// die jüngsten bleiben, die ältesten gehen
	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%s2024-01-05_100000-%s.json", backupPrefix, jahreswechselAnlass))); err != nil {
		t.Error("jüngste Jahressicherung fehlt")
	}
	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%s2010-01-05_100000-%s.json", backupPrefix, jahreswechselAnlass))); err == nil {
		t.Error("älteste Jahressicherung hätte entfernt werden müssen")
	}
}

// Ältere Jahressicherungen erscheinen in der Auswahl zum Wiederherstellen, auch wenn sie nicht zu den
// neuesten 40 Dateien gehören.
func TestJahressicherungenInDerAuswahl(t *testing.T) {
	app, _ := newTestApp(t)
	legeSicherungenAn(t, app.st.backupDir(), 50, 0, 3)
	jahre := 0
	for _, s := range app.st.sicherungen() {
		if isJahresSicherung(s.Name) {
			jahre++
		}
	}
	if jahre != 3 {
		t.Errorf("Jahressicherungen in der Auswahl: %d, erwartet 3", jahre)
	}
}

// Im zweiten Sicherungsordner (USB-Stick) wurde früher nie aufgeräumt: über Jahre sammelten sich
// dort Tausende Dateien.
func TestZweiterSicherungsordnerWirdAufgeraeumt(t *testing.T) {
	app, _ := newTestApp(t)
	zweiter := t.TempDir()
	legeSicherungenAn(t, zweiter, 100, 0, 0)
	app.st.d.Settings.ZweiteSicherung = zweiter
	app.st.snapshotBackup("vor-Test")
	if tage, ereignisse, _ := zaehleSicherungen(t, zweiter); tage != behalteTagessicherungen || ereignisse != 1 {
		t.Errorf("zweiter Ordner nach dem Spiegeln: %d Tages-, %d Ereignissicherungen", tage, ereignisse)
	}
}

func TestZweiteSicherungStandUndNeuerVersuch(t *testing.T) {
	app, h := newTestApp(t)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.ZweiteSicherung.Eingerichtet {
		t.Fatal("ohne zweiten Ordner darf nichts eingerichtet sein")
	}

	// USB-Stick »nicht eingesteckt«: an der Stelle des Ordners liegt eine Datei
	stick := filepath.Join(t.TempDir(), "stick")
	if err := os.WriteFile(stick, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.st.mu.Lock()
	app.st.d.Settings.ZweiteSicherung = filepath.Join(stick, "Sicherung")
	app.st.zweiteSicherungGeaendert()
	app.st.mu.Unlock()
	if rec := do(h, "POST", "/api/admin/sicherung-jetzt", nil); rec.Code != 200 {
		t.Fatalf("Jetzt sichern: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if !st.ZweiteSicherung.Eingerichtet || st.ZweiteSicherung.Fehler == "" || st.ZweiteSicherung.Zuletzt != "" {
		t.Fatalf("nicht erreichbarer Ordner: %+v", st.ZweiteSicherung)
	}

	// Stick eingesteckt: die nächste Änderung (nach ein paar Minuten) holt die Kopie nach
	if err := os.Remove(stick); err != nil {
		t.Fatal(err)
	}
	app.st.mu.Lock()
	app.st.zweiteVersuch = time.Now().Add(-10 * time.Minute)
	app.st.mu.Unlock()
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "X", "gartengroesse": 100})
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.ZweiteSicherung.Fehler != "" || st.ZweiteSicherung.Zuletzt != time.Now().Format("2006-01-02") {
		t.Fatalf("nach dem Einstecken: %+v", st.ZweiteSicherung)
	}
	if entries, _ := os.ReadDir(filepath.Join(stick, "Sicherung")); len(entries) == 0 {
		t.Error("keine Kopie im zweiten Ordner")
	}

	// nach einem Neustart wird der Stand aus dem Ordner gelesen
	st2, err := openStore(app.st.dir)
	if err != nil {
		t.Fatal(err)
	}
	st2.mu.Lock()
	z := st2.zweiteSicherungLocked()
	st2.mu.Unlock()
	if z.Zuletzt != time.Now().Format("2006-01-02") {
		t.Errorf("nach Neustart: %+v", z)
	}
}

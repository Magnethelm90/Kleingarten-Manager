package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMehrplatzSperre(t *testing.T) {
	dir := t.TempDir()
	a, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	jetzt := time.Now()
	if err := a.sperreHolen(jetzt); err != nil {
		t.Fatalf("erste Sperre: %v", err)
	}
	// zweiter Rechner: frische Sperre → abgelehnt, mit Rechnernamen in der Meldung
	b, _ := openStore(dir)
	err = b.sperreHolen(jetzt.Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "in Benutzung") {
		t.Fatalf("zweiter Rechner: %v", err)
	}
	// verwaiste Sperre (z. B. Absturz) darf übernommen werden
	if err := b.sperreHolen(jetzt.Add(3 * time.Minute)); err != nil {
		t.Fatalf("verwaiste Sperre übernehmen: %v", err)
	}
	// das erste Programm darf danach nicht mehr speichern
	a.mu.Lock()
	err = a.saveLocked()
	hinweis := a.sperrHinweisLocked()
	a.mu.Unlock()
	if err == nil || hinweis == "" {
		t.Fatalf("erstes Programm speichert trotz fremder Sperre: %v", err)
	}
	// der neue Besitzer speichert normal und gibt die Sperre beim Beenden frei
	b.mu.Lock()
	if err := b.saveLocked(); err != nil {
		t.Errorf("Besitzer speichert nicht: %v", err)
	}
	b.mu.Unlock()
	b.sperreFreigeben()
	if _, err := os.Stat(b.sperrPfad()); !os.IsNotExist(err) {
		t.Error("Sperre nach dem Beenden nicht freigegeben")
	}
	// gelöschte Sperrdatei (Stick kurz abgezogen) wird beim Speichern neu angelegt
	c, _ := openStore(dir)
	_ = c.sperreHolen(time.Now())
	_ = os.Remove(c.sperrPfad())
	c.mu.Lock()
	err = c.saveLocked()
	c.mu.Unlock()
	if err != nil {
		t.Errorf("Speichern nach gelöschter Sperre: %v", err)
	}
	raw, _ := os.ReadFile(c.sperrPfad())
	var si sperrInfo
	if json.Unmarshal(raw, &si) != nil || si.Token != c.sperrToken {
		t.Error("Sperre nicht neu angelegt")
	}
	// fremde Sperre wird von freigeben nicht angerührt
	a.sperreFreigeben()
	if _, err := os.Stat(c.sperrPfad()); err != nil {
		t.Error("fremde Sperre gelöscht")
	}
}

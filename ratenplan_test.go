package main

import "testing"

func TestRatenplan(t *testing.T) {
	r := &Rechnung{Settings: Settings{Zahlungsziel: "2026-02-15"}, Result: Result{Gesamt: 300}}
	if faelligAm(r) != "2026-02-15" {
		t.Fatal("ohne Plan gilt das Zahlungsziel")
	}
	r.Ratenplan = []Rate{{"2026-03-01", 100}, {"2026-04-01", 100}, {"2026-05-01", 100}}
	if faelligAm(r) != "2026-03-01" {
		t.Fatalf("erste Rate erwartet, war %s", faelligAm(r))
	}
	b := 150.0
	r.BezahltAm, r.BezahltBetrag = "2026-03-02", &b
	if faelligAm(r) != "2026-04-01" {
		t.Fatalf("zweite Rate erwartet, war %s", faelligAm(r))
	}
	b = 300
	if faelligAm(r) != "2026-05-01" {
		t.Fatalf("letzte Rate erwartet, war %s", faelligAm(r))
	}
}

func TestRatenGeprueft(t *testing.T) {
	ok := []Rate{{"2026-03-01", 100.004}, {"2026-04-01", 99.996}}
	if _, err := ratenGeprueft(ok, 200); err != nil {
		t.Fatal(err)
	}
	for name, raten := range map[string][]Rate{
		"Summe":    {{"2026-03-01", 50}, {"2026-04-01", 100}},
		"Datum":    {{"2026-03-01", 100}, {"2026-02-01", 100}},
		"kaputt":   {{"gestern", 200}},
		"negativ":  {{"2026-03-01", 300}, {"2026-04-01", -100}},
		"zu viele": make([]Rate, 25),
	} {
		if _, err := ratenGeprueft(raten, 200); err == nil {
			t.Errorf("%s: Fehler erwartet", name)
		}
	}
	if out, err := ratenGeprueft(nil, 200); err != nil || len(out) != 0 {
		t.Error("leerer Plan löscht")
	}
}

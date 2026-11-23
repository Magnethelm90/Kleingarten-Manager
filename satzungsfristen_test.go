package main

import (
	"strings"
	"testing"
	"time"
)

func TestEinladungsschluss(t *testing.T) {
	if d, ok := einladungsschluss("2027-03-17", 14); !ok || d != "2027-03-03" {
		t.Fatalf("17.3. - 14 Tage = %s", d)
	}
	if d, _ := einladungsschluss("2028-03-14", 14); d != "2028-02-29" {
		t.Fatalf("Schaltjahr: %s", d)
	}
	if _, ok := einladungsschluss("kaputt", 14); ok {
		t.Fatal("ungültiges Datum angenommen")
	}
	if (Satzungsfristen{}).einladungTage() != 14 || (Satzungsfristen{EinladungTage: 500}).einladungTage() != 14 || (Satzungsfristen{EinladungTage: 21}).einladungTage() != 21 {
		t.Fatal("Standard/Grenzen falsch")
	}
}

func TestSatzungsfristenCockpit(t *testing.T) {
	a, h := newTestApp(t)
	heute := time.Date(2027, 2, 17, 10, 0, 0, 0, time.UTC)
	hinweise := func() []aufgabe {
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		return a.st.aufgabenLocked(heute)
	}
	finde := func(teil string) *aufgabe {
		for _, x := range hinweise() {
			if strings.Contains(x.Titel, teil) {
				return &x
			}
		}
		return nil
	}
	a.st.mu.Lock()
	a.st.d.Termine = []Termin{{ID: "v", Datum: "2027-03-17", Titel: "Mitgliederversammlung", Art: "Versammlung"}, {ID: "k", Datum: "2027-03-20", Titel: "Kaputt", Art: "Versammlung"}}
	a.st.d.Termine[1].Datum = "2027-02-30"
	a.st.mu.Unlock()
	// Schluss 3.3., heute 17.2. = genau 14 Tage vorher → Hinweis
	x := finde("Einladung zur Mitgliederversammlung")
	if x == nil || !strings.Contains(x.Titel, "03.03.2027") || x.Stufe != "warn" {
		t.Fatalf("Einladungshinweis fehlt: %+v", hinweise())
	}
	// 15 Tage vorher: noch kein Hinweis
	heute = time.Date(2027, 2, 16, 10, 0, 0, 0, time.UTC)
	if finde("Einladung zur") != nil {
		t.Fatal("Hinweis zu früh")
	}
	// Frist auf 21 Tage: Schluss 24.2.
	if rec := do(h, "PUT", "/api/admin/satzungsfristen", map[string]int{"einladungTage": 21}); rec.Code != 200 {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	if x := finde("Einladung zur"); x == nil || !strings.Contains(x.Titel, "24.02.2027") {
		t.Fatalf("21 Tage nicht berücksichtigt: %+v", x)
	}
	for _, falsch := range []int{0, -3, 91} {
		if rec := do(h, "PUT", "/api/admin/satzungsfristen", map[string]int{"einladungTage": falsch}); rec.Code != 400 {
			t.Errorf("%d Tage: Status %d", falsch, rec.Code)
		}
	}
	v := decode[satzungsfristenAnsicht](t, do(h, "GET", "/api/admin/satzungsfristen", nil))
	if v.EinladungTage != 21 || v.Kuendigung.Bis != "08-31" {
		t.Fatalf("Ansicht %+v", v)
	}
	// am Schlusstag selbst noch Hinweis, danach Vorbereitung erst in der letzten Woche
	heute = time.Date(2027, 2, 24, 8, 0, 0, 0, time.UTC)
	if finde("Einladung zur") == nil {
		t.Fatal("am Schlusstag kein Hinweis")
	}
	heute = time.Date(2027, 2, 27, 8, 0, 0, 0, time.UTC)
	if finde("ist abgelaufen") == nil {
		t.Fatal("kein Hinweis kurz nach Fristablauf")
	}
	heute = time.Date(2027, 3, 1, 8, 0, 0, 0, time.UTC)
	if finde("Einladung zur") != nil || finde("vorbereiten") != nil || finde("abgelaufen") != nil {
		t.Fatal("zwischen Schluss und letzter Woche sollte nichts stehen")
	}
	heute = time.Date(2027, 3, 12, 8, 0, 0, 0, time.UTC)
	if finde("Mitgliederversammlung am 17.03.2027 vorbereiten") == nil {
		t.Fatal("Vorbereitung fehlt")
	}

	// Kündigungsfrist: nur mit Zweitschreiben
	heute = time.Date(2027, 8, 10, 8, 0, 0, 0, time.UTC)
	if finde("Kündigungsfrist") != nil {
		t.Fatal("Kündigungsfrist ohne Anlass")
	}
	a.st.mu.Lock()
	a.st.d.Maengel = []Mangel{{ID: "m", Gartennr: "7", Datum: "2027-05-01", Art: "Hecke", Stufen: map[string]string{"zweit": "2027-07-01"}}}
	a.st.mu.Unlock()
	x = finde("Kündigungsfrist")
	if x == nil || !strings.Contains(x.Titel, "31.08.2027") || !strings.Contains(x.Text, "30.11.2027") || !strings.Contains(x.Text, "Garten 7") {
		t.Fatalf("Kündigungsfrist: %+v", x)
	}
	heute = time.Date(2027, 9, 1, 8, 0, 0, 0, time.UTC)
	if finde("Kündigungsfrist") != nil {
		t.Fatal("nach dem Stichtag noch Hinweis")
	}
}

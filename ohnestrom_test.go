package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGartenOhneStrom(t *testing.T) {
	s := defaultSettings()
	f := func(x float64) *float64 { return &x }
	p := Paechter{Name: "Erika", Gartengroesse: 300, OhneStrom: true}
	r := calculate(s, p, Ablesung{WasserVJ: f(10), WasserAkt: f(20), Stunden: f(12)})
	if !r.Vollstaendig || r.KostenEnergie != 0 || r.EnergieVerbrauch != nil {
		t.Fatalf("ohne Strom: %+v", r)
	}
	// alte Stromstände zählen nicht mehr mit
	r2 := calculate(s, p, Ablesung{WasserVJ: f(10), WasserAkt: f(20), StromVJ: f(5), StromAkt: f(1), Stunden: f(12)})
	if r2.Status != StatusOK || r2.KostenEnergie != 0 || r2.Gesamt != r.Gesamt {
		t.Fatalf("mit alten Stromständen: %+v", r2)
	}
	p.OhneStrom = false
	if calculate(s, p, Ablesung{WasserVJ: f(10), WasserAkt: f(20), Stunden: f(12)}).Vollstaendig {
		t.Fatal("mit Strom ohne Stromstände darf nicht vollständig sein")
	}

	// über die Oberfläche: anlegen, ablesen, ausstellen, Pächterwechsel
	a, h := newTestApp(t)
	alt := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "gartennr": "7", "name": "Erika Muster", "gartengroesse": 300, "ohneStrom": true}))
	if !alt.OhneStrom {
		t.Fatal("Schalter nicht gespeichert")
	}
	do(h, "PUT", "/api/ablesung/"+alt.ID, map[string]any{"wasserVJ": 10, "wasserAkt": 20, "stunden": 12})
	a.st.mu.Lock()
	fehlt, _ := a.st.zaehlerFehlenLocked(a.st.d.Settings.Jahr)
	a.st.mu.Unlock()
	if len(fehlt.Fehlend) != 0 {
		t.Fatalf("Erinnerung meldet Strom: %+v", fehlt.Fehlend)
	}
	rec := do(h, "POST", "/api/invoices/issue", map[string]any{"ids": []string{alt.ID}})
	if rec.Code != 200 {
		t.Fatalf("ausstellen: %d %s", rec.Code, rec.Body.String())
	}
	a.st.mu.Lock()
	datei := filepath.Join(a.st.dir, a.st.d.Rechnungen[0].Datei)
	a.st.mu.Unlock()
	if _, err := exec.LookPath("pdftotext"); err == nil {
		out, _ := exec.Command("pdftotext", "-layout", datei, "-").Output()
		if !bytes.Contains(out, []byte("kein Stromanschluss")) {
			t.Errorf("PDF ohne Hinweis: %s", out)
		}
	} else if _, err := os.Stat(datei); err != nil {
		t.Fatal(err)
	}
	// Zettel, Ablesebogen und Handy-Datei laufen durch
	for _, u := range []string{"/api/admin/zaehlerzettel", "/api/ablesebogen", "/api/admin/ablesung-handy"} {
		if rec := do(h, "GET", u, nil); rec.Code != 200 && rec.Code != 400 {
			t.Errorf("%s: %d %s", u, rec.Code, rec.Body.String())
		}
	}
	// Pächterwechsel ohne Stromstand; der Nachfolger übernimmt den Schalter
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	alt = decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "gartennr": "8", "name": "Jonas", "gartengroesse": 250, "ohneStrom": true}))
	do(h, "PUT", "/api/ablesung/"+alt.ID, map[string]any{"wasserVJ": 5})
	rec = do(h, "POST", "/api/admin/paechterwechsel", map[string]any{"altId": alt.ID, "datum": "2026-06-01", "wasserStand": 9, "stunden": 3,
		"neu": map[string]any{"mitgliedsnr": "3", "name": "Nora Neu"}})
	if rec.Code != 200 {
		t.Fatalf("Wechsel: %d %s", rec.Code, rec.Body.String())
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for _, q := range a.st.d.Paechter {
		if q.Name == "Nora Neu" && !q.OhneStrom {
			t.Fatal("Nachfolger ohne Schalter")
		}
	}
}

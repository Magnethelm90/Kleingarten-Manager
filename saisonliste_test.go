package main

import "testing"

func TestSaisonliste(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "s1", Name: "Test", Gartennr: "3", WasserzaehlerNr: "W-3"})
	app.st.mu.Unlock()
	for _, art := range []string{"fruehjahr", "herbst"} {
		if rec := do(h, "GET", "/api/admin/saisonliste?art="+art, nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
			t.Fatalf("%s: %d", art, rec.Code)
		}
	}
	if rec := do(h, "GET", "/api/admin/saisonliste?art=sommer", nil); rec.Code < 400 {
		t.Fatal("unbekannte Art angenommen")
	}
}

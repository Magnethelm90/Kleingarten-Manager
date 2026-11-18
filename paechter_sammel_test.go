package main

import "testing"

func TestPaechterSammel(t *testing.T) {
	a, srv := newTestApp(t)
	a.st.mu.Lock()
	u := 12.5
	a.st.d.Paechter = []Paechter{
		{ID: "a", Mitgliedsnr: "1", Name: "Anna", Email: "anna@beispiel.de", Versand: "Postversand"},
		{ID: "b", Mitgliedsnr: "2", Name: "Bernd", Versand: "Postversand", UmlageAbweichend: &u},
		{ID: "c", Mitgliedsnr: "3", Name: "Carla", Versand: "Postversand"},
	}
	a.st.mu.Unlock()
	erg := decode[paechterSammelErgebnis](t, do(srv, "POST", "/api/admin/paechter-sammel", map[string]any{"ids": []string{"a", "b"}, "versand": "Emailsendung", "umlageAendern": true, "umlage": 30}))
	if erg.Geaendert != 2 || len(erg.Uebersprungen) != 1 || erg.Uebersprungen[0] != "Bernd" {
		t.Fatalf("Ergebnis %+v", erg)
	}
	a.st.mu.Lock()
	p := a.st.d.Paechter
	if p[0].Versand != "Emailsendung" || p[1].Versand != "Postversand" || *p[0].UmlageAbweichend != 30 || *p[1].UmlageAbweichend != 30 || p[2].UmlageAbweichend != nil {
		t.Fatalf("falsch geändert: %+v", p)
	}
	a.st.mu.Unlock()
	// zurück auf Standard
	do(srv, "POST", "/api/admin/paechter-sammel", map[string]any{"ids": []string{"a", "b"}, "umlageAendern": true})
	a.st.mu.Lock()
	if a.st.d.Paechter[0].UmlageAbweichend != nil || a.st.d.Paechter[1].UmlageAbweichend != nil {
		t.Fatal("Umlage nicht auf Standard zurückgesetzt")
	}
	a.st.mu.Unlock()
	for _, falsch := range []map[string]any{
		{"ids": []string{}, "versand": "Postversand"},
		{"ids": []string{"a"}, "versand": "Brieftaube"},
		{"ids": []string{"a"}},
		{"ids": []string{"a"}, "umlageAendern": true, "umlage": -1},
	} {
		if rec := do(srv, "POST", "/api/admin/paechter-sammel", falsch); rec.Code != 400 {
			t.Errorf("%v: Status %d, erwartet 400", falsch, rec.Code)
		}
	}
}

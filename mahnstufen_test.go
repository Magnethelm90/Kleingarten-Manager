package main

import "testing"

func TestMahnstufen(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erster", "gartengroesse": 300}))
	abl := Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)}
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	id := st.Issued[p.ID].ID
	for i := 1; i <= 4; i++ {
		if c := do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{id}, "frist": "2099-01-01", "vermerken": true}).Code; c != 200 {
			t.Fatalf("Mahnung %d: %d", i, c)
		}
	}
	// nur ohne »vermerken« bleibt die Stufe stehen
	do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{id}})
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for _, r := range a.st.d.Rechnungen {
		if r.ID != id {
			continue
		}
		if len(r.Mahnungen) != 4 || r.Mahnungen[0].Stufe != 1 || r.Mahnungen[1].Stufe != 2 || r.Mahnungen[2].Stufe != 3 || r.Mahnungen[3].Stufe != 3 {
			t.Errorf("Stufen: %+v", r.Mahnungen)
		}
		if r.Mahnungen[0].Frist != "2099-01-01" {
			t.Errorf("Frist: %+v", r.Mahnungen[0])
		}
	}
}

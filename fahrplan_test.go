package main

import "testing"

func TestPostVermerken(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartengroesse": 300, "versand": "Postversand"}))
	q := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Jonas", "gartengroesse": 300, "versand": "Emailsendung", "email": "j@beispiel.de"}))
	for _, id := range []string{p.ID, q.ID} {
		do(h, "PUT", "/api/ablesung/"+id, map[string]any{"wasserVJ": 1, "wasserAkt": 5, "stromVJ": 1, "stromAkt": 9, "stunden": 10})
	}
	if rec := do(h, "POST", "/api/invoices/issue", map[string]any{"ids": []string{}}); rec.Code != 200 {
		t.Fatalf("ausstellen: %d %s", rec.Code, rec.Body.String())
	}
	r := decode[map[string]int](t, do(h, "POST", "/api/admin/rechnungen-druck/vermerken", nil))
	if r["vermerkt"] != 1 {
		t.Fatalf("vermerkt: %v", r)
	}
	if r := decode[map[string]int](t, do(h, "POST", "/api/admin/rechnungen-druck/vermerken", nil)); r["vermerkt"] != 0 {
		t.Fatalf("zweites Mal: %v", r)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	post := 0
	for _, e := range st.Archive {
		if e.PostAm != "" {
			post++
		}
	}
	if post != 1 {
		t.Fatalf("Archiv zeigt %d mit PostAm", post)
	}
	_ = a
}

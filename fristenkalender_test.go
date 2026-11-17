package main

import (
	"strings"
	"testing"
	"time"
)

func TestFristenKalender(t *testing.T) {
	app, h := newTestApp(t)
	frist := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	alt := time.Now().AddDate(0, -3, 0).Format("2006-01-02")
	app.st.mu.Lock()
	app.st.d.Maengel = append(app.st.d.Maengel,
		Mangel{ID: "m1", Gartennr: "4", Art: "Hecke", Beschreibung: "zu hoch", Frist: frist},
		Mangel{ID: "m2", Gartennr: "5", Art: "Zaun", Beschreibung: "erledigt", Frist: frist, ErledigtAm: alt},
		Mangel{ID: "m3", Gartennr: "6", Art: "Weg", Beschreibung: "alt", Frist: alt})
	app.st.mu.Unlock()
	rec := do(h, "GET", "/api/admin/fristen.ics", nil)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "UID:frist-mangel-m1@") || strings.Contains(body, "mangel-m2") || strings.Contains(body, "mangel-m3") {
		t.Fatalf("falsche Einträge:\n%s", body)
	}
	if !strings.Contains(body, "BEGIN:VALARM") || !strings.HasSuffix(body, "END:VCALENDAR\r\n") {
		t.Fatal("Erinnerung oder Abschluss fehlt")
	}
}

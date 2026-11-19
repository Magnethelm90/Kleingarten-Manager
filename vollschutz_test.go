package main

import (
	"net/http"
	"strings"
	"testing"
)

// »Alles nur mit Anmeldung«: ohne Sitzung darf nichts Personenbezogenes herausgehen.
func TestVollschutz(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika Geheim", "strasse": "Geheimweg 1", "gartennr": "7", "gartengroesse": 300})
	if rec := do(h, "PUT", "/api/admin/vollschutz", map[string]bool{"an": true}); rec.Code != 400 {
		t.Fatalf("ohne Passwort darf sich der Schutz nicht einschalten lassen: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "admingeheim1"})
	var cookie *http.Cookie
	for _, c := range do(h, "POST", "/api/admin/login", map[string]string{"password": "admingeheim1"}).Result().Cookies() {
		if c.Name == "ga_session" {
			cookie = c
		}
	}
	// vorher: offene Bereiche sind erreichbar
	if rec := do(h, "GET", "/api/state", nil); !strings.Contains(rec.Body.String(), "Erika Geheim") {
		t.Fatal("ohne Vollschutz ist der Zustand offen")
	}
	if rec := do(h, "PUT", "/api/admin/vollschutz", map[string]bool{"an": true}, cookie); rec.Code != 200 {
		t.Fatalf("einschalten: %d %s", rec.Code, rec.Body)
	}
	rec := do(h, "GET", "/api/state", nil)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "Geheim") {
		t.Fatalf("Zustand ohne Anmeldung verrät Daten: %s", rec.Body)
	}
	if st := decode[stateResp](t, rec); !st.Gesperrt || len(st.Paechter) != 0 || st.Settings.IBAN != "" || st.DataDir != "" {
		t.Fatalf("gekürzter Zustand erwartet: %+v", st)
	}
	// alle offenen Schnittstellen aus api.go sind jetzt gesperrt
	for _, x := range [][2]string{{"GET", "/api/garten-historie"}, {"GET", "/api/export"}, {"GET", "/api/export-payments"}, {"GET", "/api/ablesebogen"}, {"GET", "/api/archiv-alle"},
		{"GET", "/api/termine.ics"}, {"GET", "/api/termine-druck"}, {"GET", "/api/lageplan-druck"}, {"GET", "/api/lageplan-bild"}, {"GET", "/api/export-ics"}, {"GET", "/api/invoice/x"}, {"GET", "/api/archive/x"},
		{"PUT", "/api/ablesung/x"}, {"PUT", "/api/zaehler/x"}, {"PUT", "/api/payment/x"}, {"POST", "/api/invoices/issue"}, {"POST", "/api/open-folder"}, {"POST", "/api/tutorial"}} {
		if rec := do(h, x[0], x[1], map[string]any{}); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s ohne Anmeldung: %d", x[0], x[1], rec.Code)
		}
	}
	// angemeldet geht alles wie gewohnt
	if rec := do(h, "GET", "/api/state", nil, cookie); !strings.Contains(rec.Body.String(), "Erika Geheim") {
		t.Fatal("angemeldet muss der volle Zustand kommen")
	}
	if rec := do(h, "GET", "/api/garten-historie", nil, cookie); rec.Code != 200 {
		t.Fatalf("angemeldet: %d", rec.Code)
	}
	// Passwort entfernen hebt den Schutz auf
	do(h, "POST", "/api/admin/password", map[string]string{"old": "admingeheim1", "new": ""}, cookie)
	if rec := do(h, "GET", "/api/state", nil); !strings.Contains(rec.Body.String(), "Erika Geheim") {
		t.Fatal("ohne Passwort ist alles wieder offen")
	}
}

// Jede Route außerhalb von /api/admin muss entweder bewusst offen sein oder vom Vollschutz erfasst werden.
func TestVollschutzErfasstAlleOffenenRouten(t *testing.T) {
	for _, p := range []string{"/api/state", "/api/ping", "/api/logo", "/api/pruef/login", "/", "/app.js", "/api/admin/paechter"} {
		if !ohneAnmeldungErlaubt(p) {
			t.Errorf("%s muss ohne Anmeldung erreichbar bleiben", p)
		}
	}
	for _, p := range []string{"/api/export", "/api/invoice/1", "/api/irgendwas-neues"} {
		if ohneAnmeldungErlaubt(p) {
			t.Errorf("%s darf nicht offen sein", p)
		}
	}
}

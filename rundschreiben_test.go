package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRundschreiben(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	// zwei Pächter per E-Mail, der Rest per Post
	var mailIDs []string
	for _, p := range st.Paechter[:2] {
		p.Versand, p.Email = "Emailsendung", strings.ToLower(strings.Fields(p.Name)[0])+"@example.org"
		if rec := do(h, "PUT", "/api/admin/paechter/"+p.ID, p); rec.Code != 200 {
			t.Fatalf("Pächter: %d %s", rec.Code, rec.Body.String())
		}
		mailIDs = append(mailIDs, p.ID)
	}
	for _, p := range st.Paechter[2:] {
		if p.Versand == "Emailsendung" {
			p.Versand = "Postversand"
			do(h, "PUT", "/api/admin/paechter/"+p.ID, p)
		}
	}
	// ungültig: ohne Betreff, Auswahl ohne bekannte Empfänger
	if rec := do(h, "POST", "/api/admin/rundschreiben", map[string]any{"text": "x"}); rec.Code != 400 {
		t.Errorf("ohne Betreff: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/rundschreiben", map[string]any{"betreff": "x", "text": "y", "an": "auswahl", "auswahl": []string{"gibtsnicht"}}); rec.Code != 400 {
		t.Errorf("Auswahl ohne Empfänger: %d", rec.Code)
	}
	rec := do(h, "POST", "/api/admin/rundschreiben", map[string]any{"betreff": "Wasser wird angestellt", "text": "Liebe Gartenfreunde,\n\nam Samstag kommt das Wasser.\n\n" + strings.Repeat("Bitte Hähne schließen. ", 80), "an": "alle"})
	if rec.Code != 200 {
		t.Fatalf("anlegen: %d %s", rec.Code, rec.Body.String())
	}
	rs := decode[Rundschreiben](t, rec)
	liste := decode[[]rundschreibenAnsicht](t, do(h, "GET", "/api/admin/rundschreiben", nil))
	var meins rundschreibenAnsicht
	for _, x := range liste {
		if x.ID == rs.ID {
			meins = x
		}
	}
	if meins.PerMail != 2 || meins.PerPost != len(st.Paechter)-2 || meins.Verschickt != "" {
		t.Fatalf("Empfänger: %+v", meins)
	}
	// Briefe: eine Seite je Postempfänger, danach als verschickt markiert
	rec = do(h, "GET", "/api/admin/rundschreiben/"+rs.ID+"/briefe", nil)
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Briefe: %d %s", rec.Code, rec.Body.String())
	}
	if n := bytes.Count(rec.Body.Bytes(), []byte("/Type /Page\n")); n < meins.PerPost {
		t.Errorf("Briefe: %d Seiten für %d Empfänger", n, meins.PerPost)
	}
	if dir := os.Getenv("GA_PDF_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "rundschreiben.pdf"), rec.Body.Bytes(), 0o600)
	}
	if rec := do(h, "GET", "/api/admin/rundschreiben/"+rs.ID+"/aushang", nil); rec.Code != 200 {
		t.Errorf("Aushang: %d", rec.Code)
	}
	app.st.mu.Lock()
	verschickt := app.st.rundschreibenLocked(rs.ID).Verschickt
	app.st.mu.Unlock()
	if verschickt != time.Now().Format("2006-01-02") {
		t.Errorf("verschickt: %q", verschickt)
	}

	// E-Mail: langer Text passt nicht in den Link, Adressen im Bcc
	var urls []string
	alt := openMailFn
	openMailFn = func(u string) { urls = append(urls, u) }
	defer func() { openMailFn = alt }()
	m := decode[map[string]any](t, do(h, "POST", "/api/admin/rundschreiben/"+rs.ID+"/mail", nil))
	if m["mitText"] != false || m["pakete"] != 1.0 || len(urls) != 1 || !strings.Contains(urls[0], "bcc=") || strings.Contains(urls[0], "body=") {
		t.Fatalf("Mail: %v %v", m, urls)
	}
	if !strings.Contains(m["text"].(string), "Der Vorstand") {
		t.Errorf("Mailtext ohne Gruß: %v", m["text"])
	}

	// Auswahl: nur ein Empfänger; wird die Person gelöscht, bleibt das Rundschreiben ohne Empfänger (nicht »alle«)
	rec = do(h, "POST", "/api/admin/rundschreiben", map[string]any{"betreff": "Nur für dich", "text": "Hallo", "an": "auswahl", "auswahl": []string{mailIDs[0], mailIDs[0]}})
	rs2 := decode[Rundschreiben](t, rec)
	if len(rs2.Auswahl) != 1 {
		t.Fatalf("Auswahl doppelt: %+v", rs2)
	}
	app.st.mu.Lock()
	app.st.d.rundschreibenPersonEntfernen(mailIDs[0])
	post, mail := app.st.rundschreibenEmpfaengerLocked(*app.st.rundschreibenLocked(rs2.ID))
	app.st.mu.Unlock()
	if len(post)+len(mail) != 0 {
		t.Errorf("nach dem Entfernen: %d Empfänger", len(post)+len(mail))
	}
	if rec := do(h, "DELETE", "/api/admin/rundschreiben/"+rs2.ID, nil); rec.Code != 200 {
		t.Errorf("löschen: %d", rec.Code)
	}
}

func TestMailPakete(t *testing.T) {
	var adressen []string
	for i := 0; i < 150; i++ {
		adressen = append(adressen, fmt.Sprintf("gartenfreund.nummer%03d@example.org", i))
	}
	pakete, mitText := mailPakete("Betreff", "kurz", adressen)
	if !mitText || len(pakete) < 2 {
		t.Fatalf("%d Pakete, mitText %v", len(pakete), mitText)
	}
	gesamt := 0
	for _, p := range pakete {
		if len(p) > maxMailtoLaenge {
			t.Errorf("Paket zu lang: %d", len(p))
		}
		u, err := url.Parse(p)
		if err != nil {
			t.Fatal(err)
		}
		gesamt += len(strings.Split(u.Query().Get("bcc"), ","))
	}
	if gesamt != len(adressen) {
		t.Errorf("%d von %d Adressen in den Paketen", gesamt, len(adressen))
	}
}

func TestSchluesselUndGeraete(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	p := st.Paechter[0]

	rec := do(h, "POST", "/api/admin/schluessel", map[string]any{"art": "Tor", "nummer": "7", "pfand": 25})
	if rec.Code != 200 {
		t.Fatalf("Schlüssel anlegen: %d %s", rec.Code, rec.Body.String())
	}
	k := decode[Schluessel](t, rec)
	if rec := do(h, "POST", "/api/admin/schluessel", map[string]any{"art": "Tor", "nummer": "7"}); rec.Code != 400 {
		t.Errorf("doppelte Nummer: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/schluessel/"+k.ID+"/ausgabe", map[string]any{"paechterId": p.ID, "datum": "2025-04-01"}); rec.Code != 200 {
		t.Fatalf("Ausgabe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "POST", "/api/admin/schluessel/"+k.ID+"/ausgabe", map[string]any{"name": "Jemand"}); rec.Code != 400 {
		t.Errorf("zweite Ausgabe: %d", rec.Code)
	}
	// Bearbeiten ändert den Inhaber nicht
	do(h, "POST", "/api/admin/schluessel", map[string]any{"id": k.ID, "art": "Tor", "nummer": "7", "bezeichnung": "Nebentor", "paechterId": ""})
	liste := decode[struct {
		Schluessel []schluesselAnsicht `json:"schluessel"`
	}](t, do(h, "GET", "/api/admin/schluessel", nil)).Schluessel
	var mein schluesselAnsicht
	for _, x := range liste {
		if x.ID == k.ID {
			mein = x
		}
	}
	if mein.InhaberName != p.Name || mein.Bezeichnung != "Nebentor" || len(mein.Verlauf) != 1 || mein.Verlauf[0].Name != p.Name {
		t.Fatalf("Schlüssel: %+v", mein)
	}
	// Auskunft enthält den Schlüssel
	app.st.mu.Lock()
	ak, _ := app.st.auskunftLocked(p.ID)
	app.st.mu.Unlock()
	if len(ak.Inventar) == 0 || !ak.Inventar[0].Aktuell {
		t.Errorf("Auskunft: %+v", ak.Inventar)
	}
	// Person löschen: Schlüssel bleibt als »bei gelöschter Person«, Cockpit fordert ihn zurück
	app.st.mu.Lock()
	app.st.d.inventarPersonEntfernen(p.ID)
	var aufgaben []aufgabe
	app.st.inventarAufgabenLocked(time.Now(), func(stufe, titel, text, ziel, knopf string) {
		aufgaben = append(aufgaben, aufgabe{stufe, titel, text, ziel, knopf})
	})
	for _, x := range app.st.d.Schluessel {
		if x.ID == k.ID && (x.PaechterID != "" || x.Inhaber != geloeschtePerson || x.Verlauf[0].PaechterID != "") {
			t.Errorf("nach dem Löschen: %+v", x)
		}
	}
	app.st.mu.Unlock()
	if len(aufgaben) == 0 || !strings.Contains(aufgaben[0].Text, "Tor 7") {
		t.Errorf("Cockpit: %+v", aufgaben)
	}
	if rec := do(h, "POST", "/api/admin/schluessel/"+k.ID+"/rueckgabe", map[string]any{"datum": "2025-10-01"}); rec.Code != 200 {
		t.Fatalf("Rückgabe: %d", rec.Code)
	}

	// Geräte: verleihen, überfällig, zurück, Wartung
	rec = do(h, "POST", "/api/admin/geraete", map[string]any{"name": "Vertikutierer", "wartungMonate": 6, "letzteWartung": "2020-01-01"})
	g := decode[Geraet](t, rec)
	if g.naechsteWartung() != "2020-07-01" {
		t.Errorf("nächste Wartung: %s", g.naechsteWartung())
	}
	q := st.Paechter[1]
	if rec := do(h, "POST", "/api/admin/geraete/"+g.ID+"/verleihen", map[string]any{"paechterId": q.ID, "datum": "2020-05-01", "bis": "2020-04-01"}); rec.Code != 400 {
		t.Errorf("Rückgabe vor Ausleihe: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/geraete/"+g.ID+"/verleihen", map[string]any{"paechterId": q.ID, "datum": "2020-05-01", "bis": "2020-05-03"}); rec.Code != 200 {
		t.Fatalf("verleihen: %d %s", rec.Code, rec.Body.String())
	}
	app.st.mu.Lock()
	aufgaben = nil
	app.st.inventarAufgabenLocked(time.Now(), func(stufe, titel, text, ziel, knopf string) {
		aufgaben = append(aufgaben, aufgabe{stufe, titel, text, ziel, knopf})
	})
	app.st.mu.Unlock()
	texte := fmt.Sprint(aufgaben)
	if !strings.Contains(texte, "Vertikutierer ("+q.Name) || !strings.Contains(texte, "Wartung") {
		t.Errorf("Cockpit Geräte: %s", texte)
	}
	do(h, "POST", "/api/admin/geraete/"+g.ID+"/zurueck", map[string]any{})
	do(h, "POST", "/api/admin/geraete/"+g.ID+"/wartung", map[string]any{"notiz": "Messer geschliffen"})
	gl := decode[[]geraetAnsicht](t, do(h, "GET", "/api/admin/geraete", nil))
	for _, x := range gl {
		if x.ID == g.ID && (x.EntleiherName != "" || x.LetzteWartung != time.Now().Format("2006-01-02") || len(x.Verlauf) != 3) {
			t.Errorf("Gerät danach: %+v", x)
		}
	}
	if rec := do(h, "DELETE", "/api/admin/geraete/"+g.ID, nil); rec.Code != 200 {
		t.Errorf("Gerät löschen: %d", rec.Code)
	}
}

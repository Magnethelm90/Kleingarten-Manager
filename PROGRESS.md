# PROGRESS

Stand: 23.11.2026 · Branch `claude/github-improvements-pi59o5` · PR #15 offen (nicht mergen, bis der Nutzer es sagt) · Version 1.6

## Aktuelles Feature / Modul
- Vereinfachen und Feinschliff der Oberfläche für Vorstände (nur Frontend), Gesamtprüfung.

## Zuletzt erledigt
- Übersicht: Fahrplan oben, doppelte Karten/Zahlen entfernt, Begrüßung flacher.
- Zahlungen ohne Rechnung: nur Leer-Hinweis. Kasse: Versorger/Schwund/Verlauf eingeklappt.
- Zählerstände: Wasser-Vergleich erst rot, wenn alle Stände da sind; kürzere Texte.
- Design: einzeiliger Beispieldaten-Hinweis, grüne Links, 5 Kennzahlen pro Reihe, Symbole in Berichte, »Tabelle drucken« ab 6 Zeilen.
- Gesamtprüfung ohne Befund: vet, test, race, staticcheck, deadcode, govulncheck, ESLint, Zufallstests, Klick-/Formulartest, Handy-/Rechner-Durchlauf.
- Doku: `CLAUDE.md`, `prozess.md`, `PROGRESS.md`, `save.md`; PR-Beschreibung aktualisiert.
- Go-Dateien in dieser Etappe: keine erstellt oder geändert (nur `web/*.js`, `web/style.css`, `CLAUDE.md`).

## Go-Dateien der ganzen Session (gegen `main`, ohne `_test.go`)
- Neu (75): `abgleich.go`, `ablesung_handy.go`, `ablesung_handy_seite.go`, `abschluss.go`, `aemter.go`, `aufgaben.go`, `aufnahmeantrag.go`, `aushang.go`, `barkasse.go`, `bauten.go`, `befragung.go`, `beitragskonto.go`, `berichte_pdf.go`, `brieftexte.go`, `datencheck.go`, `ehrenamt.go`, `eichung.go`, `einsatz.go`, `einstellungsverlauf.go`, `freiervorschlag.go`, `fristenkalender.go`, `gartenzeitleiste.go`, `geburtstag.go`, `gutschrift.go`, `inventar.go`, `jahresabrechnung.go`, `kassenvorlagen.go`, `kontoauszug.go`, `kontolernen.go`, `kostenvergleich.go`, `kuendigung.go`, `lagekarte.go`, `maengel_foto.go`, `maengel_liste.go`, `module.go`, `neuverein.go`, `nutzung.go`, `pachtgrenze.go`, `pachtvertrag.go`, `paechter_sammel.go`, `papierkorb.go`, `planer_hinweise.go`, `pruefung.go`, `ratenplan.go`, `rundschreiben.go`, `saisonliste.go`, `satzungsfristen.go`, `schwund.go`, `selbsttest.go`, `sepa.go`, `sepa_pdf.go`, `sitzung.go`, `spenden.go`, `spendenpaket.go`, `sperre.go`, `statisch.go`, `statistik.go`, `stundenkonto.go`, `suche.go`, `terminserie.go`, `tresor.go`, `vereinsakte.go`, `vereinsjahr.go`, `vereinsmappe.go`, `versammlungspaket.go`, `verwaltung.go`, `vorauszahlung.go`, `wahl.go`, `warteliste_brief.go`, `wartung.go`, `wartungsplan.go`, `wiederherstellung.go`, `zaehlerwechsel.go`, `zaehlerzettel.go`, `zuwendung.go`
- Geändert (28): `ablesebogen.go`, `api.go`, `auswertung.go`, `calc.go`, `datenschutz.go`, `demo.go`, `druck.go`, `email.go`, `erinnerung.go`, `gartenakte.go`, `importexport.go`, `jahresbericht.go`, `jahresmeldung.go`, `lageplan.go`, `maengel.go`, `main.go`, `mitgliederliste.go`, `model.go`, `pdf.go`, `pdfhelfer.go`, `rechnungen.go`, `sicherung.go`, `store.go`, `termine.go`, `versammlung.go`, `warteliste.go`, `wechsel.go`, `xlsx.go`
- Neue Tests: 70 `*_test.go` (u. a. `apifuzz_test.go`, `zugriff_test.go`, `datenschutz_voll_test.go`, `neuverein_test.go`, `satzungsfristen_test.go`, `ohnestrom_test.go`, `ablesedatum_test.go`).

## Offene Bugs / Fehlgeschlagene Tests
- `go test ./...`, `go test -race`, `go vet`, staticcheck, deadcode, govulncheck: alle grün, keine Compiler-Fehler.
- Kein bekannter Programmfehler.
- Formulartest (Browser) brach einmal nach ca. 20 min auf »Ämter« ab (Browser geschlossen); einzeln sauber, wohl Testbrowser-Grenze.
- Lageplan am Handy breiter als der Bildschirm (gewollt).
- Cockpit-Warnung Wasserschwund öffnet die eingeklappte Karte `kb-schwund` nicht selbst.

## Nächster Schritt
- PR #15 mergen, sobald der Nutzer es sagt; danach GitHub Pages (`main`, `/docs`) und Release.
- Optional: Cockpit-Ziel »Schwund« öffnet `kb-schwund` (`zielOeffnen()`).

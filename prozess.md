# Projektstand: Kleingarten-Manager

Stand: 23.11.2026 · Branch `claude/github-improvements-pi59o5` · Sammel-PR [#15](https://github.com/Magnethelm90/Kleingarten-Manager/pull/15) (offen, **nicht mergen, bis der Nutzer es sagt**) · Version bleibt **1.5.1**

## Aktuelles Feature / Modul
Feinschliff für den Vereinsvorstand: **einfacher, ruhiger, nicht überladen**. Es gibt keine angefangene Funktion; alles ist gepusht und geprüft.

## Regeln vom Nutzer (gelten weiter)
- Antworten auf Deutsch, knapp. Programmtexte per **»du«** (nicht auf »Sie« umstellen), in einfachen Worten.
- Design professionell für Vorstände: Strichzeichnungen, gedeckte Farben, **kein Spielzeug, keine Emoji** in der Oberfläche.
- **Keine Funktion entfernen** (»das bleibt alles drin«). Vereinfachen = zusammenlegen, einklappen, Doppeltes streichen.
- Nach jeder Arbeit committen und pushen. Keine PR neu anlegen, nicht mergen.
- Das Programm geht nur für die Update-Prüfung ins Internet. Kein Luftbild/WMS, nichts von außen laden (CSP `default-src 'self'; img-src 'self' data:`). Gartenschild/Mitgliedsausweis und Tastenkürzel wurden abgelehnt.
- CI-Prüfungen nicht abschwächen (z. B. kein `continue-on-error`) ohne ausdrückliche Zustimmung.
- Neue Personendaten: in `auskunftLocked` aufnehmen, in `entfernePerson` löschen, `DATENSCHUTZ.md` ergänzen.
- Nach jedem Paket `CLAUDE.md` und die Liste `NEUERUNGEN` (`web/start.js`) ergänzen.

## Zuletzt erledigt

### Dieser Abschnitt der Arbeit (Vereinfachen und Prüfen)
- **Übersicht:** Jahres-Fahrplan ganz oben (5 Stationen: Zählerstände, Rechnungen, Verschicken, Zahlungen, Jahresabschluss). Startkarte »Was möchtest du tun?«, Kennzahl »Unvollständige Pächter« und Termin-Fakt entfernt (standen doppelt), Begrüßung flacher.
- **Zahlungen ohne Rechnung:** nur noch der Leer-Hinweis, keine leeren Kennzahlen, Filter und Knöpfe.
- **Kasse:** Versorger-Vergleich, Schwund und Jahresverlauf sind eingeklappt.
- **Zählerstände:** Wasser-Vergleich ist erst rot, wenn alle Stände da sind (vorher gelb, »vorläufig«); kürzere Erklärtexte.
- **Design-Feinschliff:** einzeiliger Beispieldaten-Hinweis, Links in Vereinsgrün, 5 Kennzahlen in einer Reihe, passende Symbole in Berichte, »Tabelle drucken« erst ab 6 Zeilen (nicht bei den Pächtern, dort gibt es »Pächterliste drucken«).
- **Gesamtprüfung:** `staticcheck`, `deadcode`, `govulncheck`, `go test -race`, Zufallstests der Schnittstelle (3 Läufe), Klick-Zufallstest, Formulartest im Browser, Handy-Durchlauf (390 px) und Rechner-Durchlauf (1366 px) – keine Programmfehler gefunden.

### Große Pakete davor (alle in PR #15)
- **Bedienung:** Menü auf 12 Einträge gebündelt (Reiter), Einstellungen mit einklappbaren »Weiteren Einstellungen«, Cockpit zeigt 5 Hinweise (Rest eingeklappt, bei vielen nach Bereich sortiert), Hilfe-»?« an jeder Seite, Anleitung mit Suche, Wörterbuch, »Probier es aus«, »Garten abgeben …« als ein Eintrag, Sammeländerung mehrerer Pächter, »Speichern und nächste« in Dialogen, Rückgängig-Meldungen statt Nachfragen, tolerante Suche, Tabellen sortierbar und druckbar.
- **Neuer Verein:** Einrichtungs-Assistent ohne Musterwerte, Startjahr änderbar, Warnung vor Rechnungen mit Musterwerten, Bereiche ein-/ausblenden.
- **Abrechnung:** Garten ohne Strom, Ablesedatum, Gutschrift, Ratenplan, Mahnwesen mit Vorschlag, Kontoauszug einlesen (lernt dazu), Vorauszahlungen, SEPA-Lastschrift (ausgeblendet bis eingeschaltet), Spenden mit Jahrespaket, Zählerstand-Zettel, Ablesung per Handy-Datei, Wasserschwund-Kontrolle.
- **Vorstand:** Satzungsfristen mit Cockpit-Erinnerung, Kündigung mit Fahrplan, Pachtvertrag, Beschlussbuch, Verträge, Ämter und Wahlen-Assistent, Versammlungsprotokoll und -paket, Haushaltsplan mit Ampel, Barkasse, Kassenprüfung, Mängel mit Fahrplan und Fotos, Laubenkataster, Schlüssel/Geräte/Pflegeplan, Warteliste mit Briefen, Rundschreiben, eigene Brieftexte mit Probebrief, Jahreszahlen (Bericht, Meldung, Statistik, Kostenvergleich), Gesamt-PDF der Jahresabrechnung, Fristenkalender.
- **Sicherheit und Datenschutz:** verschlüsselte Sicherung, Wiederherstellungsprobe (alle 30 Tage automatisch), Papierkorb, Selbsttest nach Updates, »Alles nur mit Anmeldung«, Abgleich zweier Rechner per USB-Stick, Datenschutz-Tests (endgültig löschen entfernt Name und ID überall).
- **Erscheinungsbild:** 3D-Lageplan (three.js lokal), Strichzeichnungen, Willkommensgeschichte, dezente Animationen (mit »Bewegung reduzieren«), README und Projektseite `docs/index.html` (GitHub Pages erst nach Merge).

### Betroffene Dateien (Auswahl)
- Frontend (`web/`): `app.js`, `uebersicht.js`, `zaehlerstaende.js`, `lageplan.js`, `zahlungen.js`, `verwaltung.js`, `admin.js`, `paechter.js`, `termine.js`, `anleitung.js`, `start.js`, `tabellen.js`, `style.css`, `index.html`
- Zuletzt geändert: `web/zahlungen.js`, `web/admin.js`, `web/tabellen.js`, `web/uebersicht.js`, `web/zaehlerstaende.js`, `web/termine.js`, `web/style.css`, `web/start.js`, `CLAUDE.md`
- Backend (Go): `api.go`, `model.go`, `aufgaben.go`, `satzungsfristen.go`, `neuverein.go`, `paechter_sammel.go`, `druck.go`, `ablesung_handy.go`, `aemter.go`, `wahl.go` und weitere je Funktion (Übersicht in `CLAUDE.md`)

## Offene Bugs / Fehlgeschlagene Tests
- Keine bekannten Fehler. Letzter Stand: `go vet`, `go test ./...`, `go test -race`, `staticcheck`, `deadcode`, `govulncheck`, ESLint sauber.
- Kein Fehler, nur gemerkt: Der Formulartest im Browser brach einmal nach ca. 20 Minuten Dauerlauf auf der Seite »Ämter« ab (Browserfenster geschlossen). Einzeln lief die Seite sauber; vermutlich Grenze des Testbrowsers.
- Am Handy ist nur der Lageplan breiter als der Bildschirm (gewollt, er lässt sich verschieben). Der Klick-Zufallstest klickt in Handy-Breite nicht, weil er das eingeklappte Menü nicht öffnet; dafür gibt es den Handy-Durchlauf.
- Kleinigkeit: Die Cockpit-Warnung zum Wasserschwund führt zur Kasse, öffnet die eingeklappte Karte »Schwund über die Jahre« aber nicht selbst.

## Nächster Schritt
- **PR #15 mergen**, sobald der Nutzer es sagt. Danach GitHub Pages einschalten (Settings → Pages → `main`, `/docs`) und die Version/Release anlegen.
- Optional: Cockpit-Ziel »Wasserschwund« öffnet automatisch die Karte `kb-schwund` (Ziel-Format `admin:<tab>[:<unterseite>]`, siehe `zielOeffnen()`).
- Optional: Klick-Zufallstest so erweitern, dass er das Handy-Menü öffnet.
- Weitere Vereinfachung nur auf Wunsch des Nutzers; neue Seiten als Reiter in bestehende Menüeinträge hängen, nicht das Menü verlängern.

## Prüfen vor dem Push
```
go vet ./... && go test ./...                      # ca. 35 s
staticcheck -checks "all,-ST1005,-ST1000,-ST1003,-ST1021" ./...
deadcode .                                         # externalURLAllowed ist gewollt unbenutzt
ESLint auf die zusammengehängten Dateien in web/   # app.js … tabellen.js
KM_FUZZ=1 KM_FUZZ_N=60 KM_FUZZ_SEED=n go test -run TestApiZufall
```
Browser-Tests (Playwright) immer mit frischem Datenordner (`mktemp -d`).

# Notizen für Claude (Kurzfassung, Token sparen)

## Arbeitsweise mit dem Nutzer
- Sprache: Deutsch. Nutzer will knappe Antworten.
- Branch: `claude/github-improvements-pi59o5`. Sammel-PR: Magnethelm90/Kleingarten-Manager#15 (MCP-Reponame `gartenabrechnung`).
- Nächste Version: **1.5.1** (Versionen fortlaufend weiterzählen).
- **Nicht mergen**, bis der Nutzer es sagt („wir bauen noch weiter und pullen nicht jede kleine Änderung“).
- Nach jeder Arbeit committen und pushen.
- Design: hochwertig, aber **professionell** – für den Vereinsvorstand, kein Spiel.
- App geht nur für den Update-Check ins Internet. Luftbild/WMS wurde bewusst komplett entfernt – nicht wieder einbauen.
- Idee #7 (Gartenschild/Mitgliedsausweis) wurde abgelehnt.

## Technik
- Go `net/http`, JSON-Store `Data` (model.go), Mutex `a.st.mu`, `saveLocked()`, `speichernUndAntworten(w,v)` (Aufrufer hält Mutex).
- Wrapper `admin()`/`adminOrPruef()`; Fehler `bad()`/`notFound()`; `writeJSON`/`writeErr`.
- PDF mit fpdf: `neuesPDF`, `zeichneKopf`, `briefAnfang`, `briefText`, `mappe*`-Helfer.
- Datenschutz-Muster für jede neue Personendaten-Art: in `auskunftLocked` aufnehmen und in `entfernePerson` per `d.xxxPersonEntfernen(id)` löschen; DATENSCHUTZ.md ergänzen.
- Cockpit: `aufgabenLocked` (aufgaben.go) ruft `inventar/schwund/kuendigung/vertragAufgabenLocked`; Ziel-Format `admin:<tab>[:<unterseite>]`.
- Demo-Daten in demo.go, `datenbestandLeer` mitpflegen.
- Frontend: Vanilla JS in `web/app.js` (`h()`, `svgEl`, `modal`, `breiterDialog`, `api`, `leer`, `confirmBox`), Navigation über `NAV`-Array, `viewAdmin` dispatcht. Assets per `//go:embed web`. CSP: `default-src 'self'; img-src 'self' data:` – nichts von außen laden.
- 3D-Lageplan: `web/lageplan3d.js` (three.js 0.170.0 lokal in `web/vendor/three/`, OrbitControls-Import auf relativen Pfad gepatcht).
- Animationen: `prefers-reduced-motion` beachten; `animation-fill-mode: backwards` (nicht `both`, sonst verrutschen fixed-Elemente wie das Mehr-Menü).
- Illustrationen: Blender-Skript `docs/blender/szene.py` (bpy 4.5.15 in Python-3.11-venv, Cycles CPU, AgX) → WebP in `web/bilder/`.
- Projektseite: `docs/index.html` (GitHub Pages: Settings → Pages → main, /docs – erst nach Merge).

## Prüfen vor Push
- `go vet ./... && go test ./...` (dauert ~3 min).
- UI mit Playwright prüfen (Chromium vorinstalliert; für WebGL swiftshader-Args). Frisches Datenverzeichnis nutzen (Mehrplatz-Sperre alter Prozesse).

## Stand (Nov 2026) – in PR #15 enthalten
- Lageplan: eigenes Hintergrundbild, Parzellen-Umrisse, Leitungen/Einrichtungen, 3D-Ansicht (Plan/3D-Umschalter).
- Rundschreiben (Brief/Aushang-PDF, Mail-Pakete), Schlüssel & Geräte (Ausgabe/Rückgabe, Wartung).
- Wasserschwund-Kontrolle (Kassenbericht + Cockpit-Warnung), Kündigung mit Fahrplan, Pachtvertrag (+Vertragsbestimmungen), Beschlussbuch, Verträge & Versicherungen, Haushaltsplan.
- Blender-Illustrationen, dezente Animationen, Erfolgsmeldung, Willkommensgeschichte (5 Bilder), neues README, Projektseite.
- Eichfristen (eichung.go, Dialog in Zählerstände), Kassenprüfung (pruefung.go, Karte im Kassenbericht, auch für
  Prüfer-Zugang), Ämter mit Amtswechsel-Checkliste (aemter.go, Unterseite in »Vereinsverwaltung«), Begehungsmodus
  (nur Frontend, Mängel-Seite), Pachtobergrenze § 5 BKleingG (pachtgrenze.go, Karte in Einstellungen),
  Cockpit-Hinweis für Ausgaben ohne Beleg.
- Barkasse (barkasse.go, Karte im Kassenbericht, Belegnummern über Jahr.BarLetzteNr nie doppelt; Kassenprüfung
  vergleicht gezähltes Bargeld mit dem Kassenbuch), Laubenkataster (bauten.go, Unterseite »Mängel & Lauben«),
  Versammlungsprotokoll (sitzung.go, Knopf »Protokoll« bei Terminen; Abstimmungen ↔ Beschlussbuch synchron,
  Nummer über beschlussNummerLocked = höchste + 1). modal() ignoriert null-Knöpfe.
- Gesamtprüfung Nov 2026: staticcheck, govulncheck, deadcode, ESLint (recommended), go test -race, Fuzz-Tests,
  Playwright-Durchlauf aller Seiten (prüft auch »null«/»NaN« im Text) – alles sauber.
- Bereiche ein-/ausblenden (module.go, Data.AusModule = Liste der AUSgeschalteten; leer = alles an, damit alte
  Bestände alles zeigen). Neue Installation: Grundpaket (standardAusModule), Beispieldaten: alles an. Frontend:
  NAV-Eintrag `modul:`, `modulAn()`, Karte »Bereiche« in Preise & Einstellungen; Cockpit-Hinweise per `s.modulAn()`.
  Neue Zusatzfunktionen dort einhängen statt das Menü zu verlängern.
- Einrichtungs-Assistent (app.js `einrichtungsAssistent`, 4 Schritte; startet nach der Willkommensgeschichte, wenn
  Bestand leer; auch über Karte »Bereiche«). Vereinsjahr (vereinsjahr.go, `vereinsjahrVorlagen` mit `erkannt`-Funktionen,
  Haken in Data.Vereinsjahr; Karte auf der Übersicht; Name »Jahresplan« ist schon der Termin-Aushang!). Hilfe-»?«:
  `hilfe(text)` in app.js, Texte für Einstellungen in `HILFE`. Ziele öffnet `zielOeffnen()`.
- Prüfung Nov 2026 (2): zugriff_test.go prüft ALLE /api/admin-Routen automatisch (401 ohne Login, Prüfer nur
  adminOrPruef); TestZielFormat prüft alle Cockpit-Ziele (Format `admin:`/`tab:`). Vereinsjahr richtet Rechnungen,
  Zahlungen und Jahreswechsel nach Rechnungsdatum/Zahlungsziel (`rhythmusLocked`, versatz 0/1).
- Offen: nichts Konkretes; auf Nutzer warten (weiterbauen oder testen/mergen).

## Prüfwerkzeuge (bei größeren Änderungen)
- `staticcheck -checks "all,-ST1005,-ST1000,-ST1003,-ST1021" ./...` (deutsche Fehlermeldungen sind gewollt)
- `deadcode .` (externalURLAllowed ist nur unter Build-Tag genutzt, kein Fehler)
- ESLint 9 mit `@eslint/js` recommended + `globals.browser` auf web/app.js und web/lageplan3d.js
- PDF-Schrift kann nur Zeichen bis U+FFFF: `trim()` und `texteBereinigen` (Laden/Speichern) entfernen Emoji, `pdfBytes` fängt Abstürze ab. Neue PDFs immer mit `pdfBytes(pdf)` beenden.
- Testdaten-Ordner immer mit `mktemp -d` anlegen – alte Ordner im Scratchpad enthalten veraltete Daten.

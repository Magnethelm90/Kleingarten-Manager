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
- Illustrationen: sachliche Strichzeichnungen als SVG in `web/bilder/` (garten, laube, baum, zaehler, rechnung, liste; Kopien in `docs/bilder/`). Der Nutzer wollte die bunten Blender-Bilder NICHT (»kein Spielzeug«) – keine Knet-/3D-Optik, keine Emoji in der Oberfläche. 3D-Lageplan im Stil eines Planungsmodells (hell, gedeckt; Farbe nur für Status/Leitungen).
- Projektseite: `docs/index.html` (GitHub Pages: Settings → Pages → main, /docs – erst nach Merge).

## Prüfen vor Push
- `go vet ./... && go test ./...` (dauert ~3 min).
- UI mit Playwright prüfen (Chromium vorinstalliert; für WebGL swiftshader-Args). Frisches Datenverzeichnis nutzen (Mehrplatz-Sperre alter Prozesse).

## Stand (Nov 2026) – in PR #15 enthalten
- Lageplan: eigenes Hintergrundbild, Parzellen-Umrisse, Leitungen/Einrichtungen, 3D-Ansicht (Plan/3D-Umschalter).
- Rundschreiben (Brief/Aushang-PDF, Mail-Pakete), Schlüssel & Geräte (Ausgabe/Rückgabe, Wartung).
- Wasserschwund-Kontrolle (Kassenbericht + Cockpit-Warnung), Kündigung mit Fahrplan, Pachtvertrag (+Vertragsbestimmungen), Beschlussbuch, Verträge & Versicherungen, Haushaltsplan.
- Dezente Animationen, Erfolgsmeldung, Willkommensgeschichte (5 Bilder), neues README, Projektseite.
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
- »Alles nur mit Anmeldung« (Data.AllesGeschuetzt, Middleware `vollschutz` + `ohneAnmeldungErlaubt` in api.go, gekürzter
  /api/state mit `gesperrt`, Karte auf der Passwort-Seite). Neue offene /api-Routen sind dadurch automatisch gesperrt.
- Übersicht gestrafft: Karte `anstehendKarte` mit Reitern »Was steht an?« / »Vereinsjahr« (S.ueberSeite), Schnellzugriff
  entfernt. Abschlussprüfung (abschluss.go, Ampel ok/warn/err auf der Jahreswechsel-Seite, blockiert nichts). Sicherung
  ansehen (sicherung.go `handleSicherungAnsehen`, nur lesen, Vergleich mit heute).
- Prüfung Nov 2026 (3): Tests `TestAlleLeseRoutenOhneFehler` (jede Bereichs-Konfiguration), `TestKeinDatenverlust`,
  `TestLoeschungInAllenBereichen`; 3D-Lageplan zeichnet nur bei Bewegung; Toolchain in go.mod aktuell halten
  (govulncheck in CI); MIT-Lizenz (LICENSE), Haftungsausschluss und »Rechtliche Hinweise« im README.
- Verschlüsselte Sicherung (tresor.go: AES-256-GCM + PBKDF2, Ordner »Verschlüsselte Sicherungen«, Öffnen legt eine
  gewöhnliche Sicherung »entschluesselt« ab; `entferneTresore` beim endgültigen Löschen). Dialoge: »cancel« nur
  behandeln, wenn `e.target === dlg` (Datei-Felder im Dialog feuern auch »cancel«).
- Ideen-Paket Nov 2026: wiederherstellung.go (Probe, Data.Probe, Cockpit 90 Tage), versammlungspaket.go (ZIP, Knopf »Paket«
  bei Versammlungen), maengel.go Fahrplan (Mangel.Stufen erst/erinnerung/zweit, `maengel-schreiben`, Cockpit
  `maengelFahrplanLocked`), stundenkonto.go (Karte in Berichte, bucht über `bucheStunden`), warteliste_brief.go (3 Briefe +
  Nachrücker), ablesung_handy.go + ablesung_handy_seite.go (eigenständige HTML-Datei, Ergebnis-JSON; JS darf kein »NaN«
  im Text haben, sonst schlägt TestAlleLeseRouten an), jahresbericht.go (+Mängel/Einsätze/Beschlüsse), vereinsakte.go (+Ämter,
  Verträge, Beschlüsse, Checkliste).
- Ideen-Paket 2 (Nov 2026): kostenvergleich.go (GET /api/admin/kostenvergleich, Kachel in Berichte; auffällig = +50 % und über
  minAbweichung), papierkorb.go (Data.Papierkorb, 30 Tage, Mängel/Termine/Beschlüsse/Verträge; Warteliste bewusst NICHT; `papierkorbLegeLocked`
  in den Löschhandlern, `papierkorbPersonEntfernen` in entfernePerson), maengel_foto.go (Mangel.Foto, `FotoBytes json:"-"` nur fürs PDF;
  Upload wird neu kodiert = EXIF/GPS weg, Orphan-Aufräumen `fotosAufraeumenLocked` nach Foto-/Papierkorb-/Personenlöschung),
  Wechsel-Assistent (app.js `wechselAssistent`, nur Frontend über den Kündigungs-Fahrplan; Pächter-»Mehr«-Menü).
  Falle: `replaceChildren(..., null)` schreibt den Text »null« – statt null ein leeres Element übergeben.
- Ideen-Paket 3 (Nov 2026): Mahnwesen (Rechnung.Mahnungen, Stufen 1–3, `buildMahnung(..., stufe, frist)`, Body `vermerken`, Cockpit
  `mahnFristText`), Bankabgleich lernt (kontolernen.go, Data.KontoGelernt = Hash→Pächter-ID, kein Klartext; Mitgliedsnr im Zweck,
  Namens-Tokens), Gartenakte-Zeitleiste (gartenzeitleiste.go), Terminserien (terminserie.go, legt Einzeltermine an, keine Regel),
  Gesamtsuche (suche.go, GET /api/admin/suche, im Kopf-Suchfeld nur für Admin), Wartungs-Check (wartung.go), gemeinsamer Papierkorb
  (Dialog zeigt auch Pächter-Papierkorb).
- Verbesserungs-Paket (Nov 2026): Barrierefreiheit (Sprunglink, aria-labelledby/Fokus zurück in modal(), aria-live bei Toasts),
  web/tabellen.js (nachträglich auf allen Tabellen: Spalten sortieren, Merkliste in localStorage `km-sortierung`, `td[data-label]` +
  Klasse `karten` für die Handy-Kartenansicht; überspringt Tabellen mit Eingabefeldern/colspan), Ungespeichert-Warnung in modal()
  (nur Dialoge mit »primary«-Knopf, nur Esc), Cockpit zurückstellen (aufgabe.Schluessel = Hash ohne Ziffern, Data.Zurueckgestellt,
  GET /api/admin/aufgaben?alle=1, POST …/zurueckstellen), `FEHLER_HILFE` in app.js (Toast mit Knopf zum Ziel), statisch.go
  (ETag + gzip im Speicher für .js/.css/.html/.svg), @media print in style.css (Menü/Knöpfe aus, offene Dialoge drucken sich selbst).
- Ideen-Paket 4 (Nov 2026): beitragskonto.go (Beitragsübersicht je Pächter, Mehr-Menü), zaehlerwechsel.go (Wechsel-Protokoll-PDF im Eichfristen-Dialog),
  fristenkalender.go (GET /api/admin/fristen.ics; Link im Cockpit), wahl.go (Wahlen-Assistent: Stimmzettel, Auszählung, Beschlussbuch + Amtswechsel,
  Knopf auf der Ämter-Seite), freiervorschlag.go (Vorschlag aus der Warteliste + Angebotsschreiben im Dialog »Garten vergeben«),
  saisonliste.go (Frühjahr/Herbst-Vordruck, Kachel in Berichte), Tabellen: Knopf »Sortierung zurücksetzen« (tabellen.js), Dialoge merken Art
  (`merkeLesen/merkeSchreiben`, localStorage `km-zuletzt-*`), Mängelliste mit Mehrfachauswahl »als erledigt markieren«.
- Ideen-Paket 5 (Nov 2026): Übergabeprotokoll mit offenen Mängeln am Stichtag (`maengelAmTag`), Handy-Import meldet Plausi-Hinweise,
  spendenpaket.go (GET /api/admin/spenden/jahr + /jahrespaket als ZIP, vermerkt NICHTS – erst »Als ausgestellt vermerken« ruft
  bestaetigen je Spender), aushang.go (POST-PDF; `pdfDrucken({..., body})` schickt dann POST), maengel_liste.go (Auswahl drucken),
  Löschen liefert `papierkorbId` → `geloeschtToast` mit »Rückgängig«, Entwürfe in modal() (Map `entwuerfe`, nur Speicher, Schlüssel =
  Titel + Anfangswerte; ohne Passwort-/Datei-/hidden-Felder; Zurückholen löst input+change aus, weil viele Dialoge über
  oninput speichern; jeder Knopf mit `action` zählt als gespeichert), pdf_zeichen_test.go (alle GET-Routen inkl. {id} mit schwierigen Zeichen), `pdfListe` skaliert zu breite
  Spalten, `icsUID` für alle UID-Zeilen. Sperre speichert `Port`: gleicher Rechner+Benutzer und dort antwortet kein Programm →
  sofort frei (`programmLaeuft` = alreadyRunning, im Test austauschbar).
- Ideen-Paket 6 (Nov 2026): gutschrift.go (Rechnung.Gutschriften, Nummer `GS-<Rechnungsnr>-<n>`, `zuZahlen(r)` = Betrag nach Gutschriften;
  openAmount/paidAmount/handlePayment rechnen damit; bei voll bezahlter Rechnung wird BezahltBetrag beim Ausstellen festgeschrieben →
  Erstattung sichtbar; Neuausstellung übernimmt Gutschriften; Freitext »Grund« wird bei Bereinigung/Löschung geleert; Dialog in app.js
  `gutschriftDialog`/`gutschriftListe`, Knopf im Zahlungs-Dialog), ehrenamt.go (Nachweis-PDF je Pächter/alle, Ämter per Namensvergleich,
  Knöpfe im Stundenkonto-Dialog), Kassenbericht.Wechsel (`wechselLocked`: ausgeschiedene Pächter mit fehlender Schlussrechnung/offenem
  Posten, Hinweis `wechselHinweis`), berichte_pdf.go (PDF für Jahresmeldung, Kostenvergleich, Arbeitsstunden, Wartungs-Check; `pdfKnopf`
  in Berichte). Design-Prüfung: `.cards` ist jetzt ein Grid (gleich breite Kennzahlen), Berichte-Kacheln mit Knöpfen unten, Handy-Kopfzeile
  kompakter (2 Zeilen), Kennzahlen auf dem Handy zweispaltig. Kein Kautions-Feld im Programm (bewusst nicht ergänzt).
- Ideen-Paket 7 (Nov 2026): aufnahmeantrag.go (Vordruck mit Einwilligungen, über `GET …/warteliste/{id}/brief?art=aufnahmeantrag`, Knopf im Dialog »Garten vergeben« und im Briefe-Menü der Warteliste), befragung.go (Data.Befragungen, Fragearten janein/skala/wahl/text, nur anonyme Strichlisten-Summen; PDFs fragebogen/erfassung/auswertung über `GET /api/admin/befragungen/{id}/pdf?art=`; Auswertung kommt ins Versammlungspaket; Dialog `befragungenDialog` als Kachel in Berichte). fpdf-Schrift »lib« hat KEINEN Kursivschnitt (nur "" und "B").
- Verständlichkeit (Nov 2026): Berichte in 4 Gruppen mit Überschrift (neue Kachel in die passende Gruppe), Einstellungen mit Sprungleiste (Preise zuerst, Farbe/Logo zuletzt), Hinweisbalken »Rechnungen offen« nur bei Rechnungen/Zählerständen, Passwort-Hinweis nur in Einstellungen/Daten/Datenschutz + Cockpit-Aufgabe.
- Gesamtdurchsicht (Nov 2026): Mehrzahl über `anz(n, eins, viele)` (app.js) bzw. `mehrzahl()` (Go) – nie »Rechnung(en)«; Termine: `terminAktionen` (eine Hauptaktion vor/nach dem Termin + Bearbeiten + Mehr-Menü); Handy-Karten: Zellen mit mehreren Teilen als Grid untereinander, `td.wrap`-Breitengrenze dort aufgehoben; Cockpit am Handy: Knöpfe unter dem Text; Menü auf Laptops dichter; Checkboxen in Dialogen nie volle Breite.
- Versandart-Vorgabe: neue Pächter (Formular + Import) ohne E-Mail-Adresse → Postversand, mit E-Mail → Emailsendung (`versandNachEmail`); im Formular folgt die Auswahl der E-Mail, bis sie von Hand geändert wird.
- Menü gestrafft (Nov 2026, 26 → 18 Einträge): NAV-Einträge bündeln Seiten über `reiter: [...]` (erster Reiter = Seite selbst):
  Zahlungen | Vorauszahlungen | Lastschrift; Pächter | Kündigungen | Jubiläen; Kasse = Kassenbericht | Haushaltsplan | Spenden;
  Sicherheit & Datenschutz = Passwort | Datenschutz | Änderungsprotokoll. adminTab-IDs/Cockpit-Ziele unverändert. `reiterKopf()` unter der
  Überschrift, `reiterLeiste()` (Klasse `.reiter`, Unterstrich) für ALLE Seitenteile (auch Rechnungen/Archiv, Mängel/Lauben, Schlüssel/Geräte,
  Vereinsverwaltung); runde `.subtabs` nur noch für Filter/Ansichten. Seitenleiste 244 px.
- Klicks gespart: Zahlungen »Alle überfälligen auswählen« (Mahnung), `ausgabeDialog` ist Schleife um `ausgabeDialogEinmal` (Knopf »Speichern und nächste«, Datum/Kategorie bleiben), `issue()` zeigt im Abschluss »Postversand drucken« / »E-Mails vorbereiten«.
- Menü weiter gestrafft (14 Einträge): Pächter (+Warteliste), Anlage = Mängel & Lauben | Schlüssel & Geräte, Berichte & Jahresabschluss = Berichte & Meldungen | Jahreswechsel, Einstellungen = Preise & Einstellungen | Import, Export, Sicherung. Neue Seiten als Reiter in bestehende NAV-Einträge hängen.
- Verbesserungen (Nov 2026): Spenden-/Kassenbuch-Dialog mit »Speichern und nächste« (Muster `xxxDialog` = Schleife um `xxxDialogEinmal`, liefert true wenn etwas gespeichert), Übersicht/Schnelleingabe springen zum ersten/nächsten unvollständigen Pächter, Suchtreffer mit Chips Rechnung/Zahlung/Akte (`.gs-zeile`), letzte Seite in localStorage `km-seite` (`seiteMerken/seiteWiederherstellen`), Leer-Hinweise für Vorauszahlungen/Jubiläen. `navLabel` nur für Label »Mängel & Lauben«.
- Verbesserungs-Paket 2 (Nov 2026): Löschen von Beschluss/Vertrag/Mangel/Termin ohne Nachfrage, dafür `geloeschtToast` mit »Rückgängig« (Mängel »als erledigt« ebenso); Teilbetrag-Feld direkt in der Zahlungszeile (`input.teilbetrag`); Pächter-Formular mit einklappbaren »Weitere Angaben«; `feldFehler(el, msg)` markiert fehlerhafte Felder (Pächter, Ausgabe, Spende, Barbuchung); `SEITEN_HILFE`/`seitenHilfe(key)` = Hilfe-»?« an jeder Seitenüberschrift; Termin-Dialog merkt Ort (`km-zuletzt-termin-ort`) und schlägt Titel passend zur Art vor; `probierDialog` (Knopf »Probier es aus« im Beispieldaten-Banner, drei Aufgaben mit Sprung zur Stelle).
- Ideen-Paket 8 (Nov 2026): jahresabrechnung.go (Gesamt-PDF: Übersicht + alle Rechnungen, Kachel in Berichte), `verbrauchToast` (auffälliger Verbrauch beim Eintippen, Toast-Art `warn`), datencheck.go (»Daten aufräumen«, nur lesend, Knopf öffnet Pächter), kassenvorlagen.go (Data.KassenVorlagen, Chips im Ausgabe-Dialog, »Als Vorlage merken«), abgleich.go (Abgleich-Datei per USB-Stick: nur NEUE Einträge nach ID, Vorschau vor `?anwenden=1`; ohne Belege/Fotos/Barkasse), statistik.go (+`statistikDialog`, Säulen mit Ein-Satz-Erklärung). Einfach bedienbar: `startKarte` (»Was möchtest du tun?«, große Knöpfe), `hilfeMenue` hinter dem »?« (Rundgang / Wörter erklärt / Probier es aus), `WOERTER` = Wörterbuch in einfachen Sätzen. Neue Texte immer in einfachen Worten für Kinder-Niveau schreiben.
- Ideen-Paket 9 (Nov 2026): ratenplan.go (Rechnung.Ratenplan = []Rate{Faellig,Betrag}, PUT /api/admin/rechnungen/{id}/ratenplan, leere Liste löscht; `faelligAm(r)` = erste noch nicht gedeckte Rate, genutzt in Cockpit-Überfällig und archiveEntry.Faellig; Gutschrift löscht den Plan; gezahlt wird weiter über den Zahlungs-Dialog mit Gesamtbetrag; Dialog `ratenDialog`, Knopf »Ratenplan« im Zahlungs-Dialog), planer_hinweise.go (Cockpit-Hinweise aus dem Vereinsjahr, 14 Tage Vorlauf, max. 4, nur mit Pächtern), `PROTOKOLL_BAUSTEINE` im Protokoll-Dialog (Textbausteine/Beschlussformeln, nur Frontend). Prüfung: staticcheck/deadcode/govulncheck sauber (per `go install` nachinstallierbar, ~/go/bin).
- Ideen-Paket 10 (Nov 2026): Rechnungs-Dialog mit Reitern Zahlung/Ratenplan/Gutschrift (`paymentDialog(e,start)` baut `zahlungsPanel/ratenPanel/gutschriftPanel`, Panels liefern `{el, knopf, save}`; save gibt false = offen lassen, 'neu' = Seite schon neu geladen), `savePayment(e, patch, {stumm})` zeigt Toast mit »Rückgängig«, `ratenHinweis` (Rate x von y in der Zahlungstabelle), Mahnbrief nennt fällige Rate, Kontoauszug erkennt Raten (`ratePasst/betragPasst` in ratenplan.go), Platzhalter {Name}/{Garten}/{Mitgliedsnr}/{Offen} in Rundschreiben (`platzhalterLocked`, Aushang/Sammelmail neutral), `vordruckeDialog` (Berichte: ersetzt 6 Kacheln), `paechterMappe` (Mehr-Menü »Mappe …«: Beiträge + Dokumente), Vereinsjahr-Karte: »+ Termin im Monat«, Abschluss-Karte: »Nächster Schritt«, Termine: »Vergangene aufräumen« (`sammelLoeschen`), Beschlüsse/Verträge: Mehrfachauswahl + »Auswahl löschen« (`auswahlHelfer`). Tagessicherung läuft schon automatisch beim Speichern – kein »Beim Beenden sichern« nötig.
- Ideen-Paket 11 (Nov 2026): wartungsplan.go (Data.Wartungsplan, Pflegeplan als dritter Reiter in »Schlüssel & Geräte«, Ziel `admin:inventar:wartung`, Cockpit 14 Tage Vorlauf), geburtstag.go (Data.Geburtstage nur MIT Einwilligung, Reiter »Angaben« in `paechterMappe`, Cockpit 7 Tage, auskunft/entfernePerson/DATENSCHUTZ.md), `haushaltAufgabenLocked` (Posten ≥ 90 % info, > 100 % warn, Jahr = Settings.Jahr; Ampel `.fast` im Haushaltsplan), `mahnVorschlag` (Zahlungen; schlägt nur vor, erzeugt über `mahnungErzeugen`; laufende Frist = nicht vorgeschlagen), `schwundBanner` über den Zählerständen (S.schwundEingabe, wird in reload() geleert), `NEUERUNGEN`/`neuerungenPruefen` (localStorage `km-version`; bei neuer Version Eintrag in NEUERUNGEN ergänzen!), Probe läuft automatisch (`probeAutomatisch` in main.go, alle 30 Tage, Test `probeAutomatischLocked`), Kontoauszug-Toast »Zu den Spenden«.
- Ideen-Paket 12 (Nov 2026): tolerante Suche (suche.go `einFehler`/`tippfehlerTreffer` + app.js `sucheTrifft`: Umlaute, 1 Tippfehler ab 5 Buchstaben), `tabelleDrucken` in tabellen.js (Knopf »Tabelle drucken« über jeder Tabelle ohne Eingabefelder, Druckfläche `#druckflaeche`, Klasse `druckt-tabelle`), Cockpit bei > 6 Hinweisen nach Bereich (`COCKPIT_BEREICHE`/`cockpitBereich(ziel)` – neue Ziele dort einordnen), einstellungsverlauf.go (Data.EinstellungsVerlauf, 10 Stände, Zurücknehmen lässt Jahr + zweite Sicherung stehen, nur Stände des laufenden Jahres; Dialog `einstellungsVerlaufDialog`), selbsttest.go (Data.LetzteVersion/Selbsttest, läuft einmal je appVersion beim Start, Ergebnis in /api/state `selbsttest` nur für Admin, im Neuerungen-Dialog). Bei neuer Version: NEUERUNGEN ergänzen.
- Ideen-Paket 13 (Nov 2026): `web/app.js` aufgeteilt (klassische Skripte, ein gemeinsamer globaler Scope; Reihenfolge in index.html): app.js (Hilfsfunktionen, Laden, Grundgerüst), uebersicht, zaehlerstaende, lageplan, zahlungen, verwaltung, admin, paechter, termine, anleitung, start, tabellen. Neue Seiten in die passende Datei, neue Dateien in index.html eintragen. ESLint über die zusammengehängten Dateien laufen lassen. anleitung.js: `ANLEITUNGEN` (14 Schritt-für-Schritt-Texte, Suche über `sucheTrifft`, bei <= 2 Treffern automatisch offen), Zugang: Hilfe-»?« → »Anleitung«; neue häufige Aufgabe = neuer Eintrag. Lasttest `lasttest_test.go` (nur mit `KM_LAST=1`, ~3 min): 150 Pächter x 10 Jahre = 1500 Rechnungen, 4,9 MB Datei, alle Abfragen < 10 ms, Ablesung speichern ~120 ms, Probe ~180 ms. Falle: Jahre vor Settings.Jahr gibt es nicht; Testläufe starten im Startjahr.
- Offen: nichts Konkretes; auf Nutzer warten (weiterbauen oder testen/mergen).

## Prüfwerkzeuge (bei größeren Änderungen)
- `staticcheck -checks "all,-ST1005,-ST1000,-ST1003,-ST1021" ./...` (deutsche Fehlermeldungen sind gewollt)
- `deadcode .` (externalURLAllowed ist nur unter Build-Tag genutzt, kein Fehler)
- ESLint 9 mit `@eslint/js` recommended + `globals.browser` auf web/app.js, web/tabellen.js und web/lageplan3d.js
- PDF-Schrift kann nur Zeichen bis U+FFFF: `trim()` und `texteBereinigen` (Laden/Speichern) entfernen Emoji, `pdfBytes` fängt Abstürze ab. Neue PDFs immer mit `pdfBytes(pdf)` beenden.
- Testdaten-Ordner immer mit `mktemp -d` anlegen – alte Ordner im Scratchpad enthalten veraltete Daten.

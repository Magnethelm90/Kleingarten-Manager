# Funktionen des Kleingarten-Managers

Vollständige Liste, damit beim Umbauen **nichts vergessen oder weggelassen** wird. Vor jedem Entfernen oder Zusammenlegen hier nachsehen; neue Funktionen sofort eintragen (Bereich, Kurzbeschreibung, Fundstelle). Ausführliche Beschreibung für Anwender: `README.md`. Stand: Version 1.6.

Schreibweise: **Menü › Reiter** = Fundstelle in der Oberfläche; `datei.go` = Hauptdatei im Backend; `web/…` = Frontend. Ausblendbare Bereiche (Einstellungen › Bereiche) sind mit *[Bereich]* markiert.

## Menü und Seiten (NAV in `web/app.js`)
| Gruppe | Menüpunkt | Reiter |
|---|---|---|
| Abrechnung | Übersicht | – |
| | Zählerstände | – |
| | Rechnungen | – |
| | Zahlungen | Zahlungen · Vorauszahlungen *[vorauszahlung]* · Lastschrift (nur wenn eingeschaltet) |
| Verein | Lageplan | – |
| | Termine & Post | Termine · Rundschreiben *[rundschreiben]* |
| | Pächter | Alle Pächter · Warteliste *[warteliste]* · Kündigungen · Jubiläen *[jubilaeen]* |
| | Anlage | Mängel & Lauben · Schlüssel & Geräte *[inventar]* |
| | Vereinsverwaltung | – |
| Kasse & Berichte | Kasse | Kassenbericht · Haushaltsplan *[haushalt]* · Spenden *[spenden]* |
| | Berichte & Jahresabschluss | Berichte & Meldungen · Jahreswechsel |
| Einstellungen | Einstellungen | Preise & Einstellungen · Import, Export, Sicherung · Passwort & Sicherheit · Datenschutz · Änderungsprotokoll |

## Bedienung und Oberfläche
- **Kopfzeile:** Zurück-Knopf (vorherige Seite, erscheint erst nach dem ersten Seitenwechsel), Klick auf Logo/Vereinsname = Übersicht, Gesamtsuche, Jahr-Auswahl, Hilfe »?«, Beenden (`web/app.js`, `suche.go`).
- **Übersicht:** Jahres-Fahrplan (Zählerstände, Rechnungen, Verschicken, Zahlungen, Jahresabschluss), Kennzahlen, Cockpit »Was steht an?« (fünf Hinweise offen, Rest nach Bereich eingeklappt, Hinweise 7/30 Tage zurückstellbar, Knopf springt zur richtigen Stelle), Vereinsjahr-Reiter, Erfolgsmeldung bei vollständig bezahlten Rechnungen (`web/uebersicht.js`, `aufgaben.go`, `vereinsjahr.go`).
- **Erster Start:** Willkommens-Bildergeschichte, Rundgang, Einrichtungs-Assistent (Verein, Preise, Bereiche, Pächter-Import), Beispieldaten zum Ausprobieren (entfernbar), »Probier es aus«-Aufgaben. Nach dem Speichern der Einrichtung verschwindet die Startkarte (`EinrichtungFertig`) (`web/start.js`, `web/admin.js`, `neuverein.go`, `demo.go`).
- **Hilfe:** »?« an jeder Seite, Anleitung mit Suche, Wörterbuch in einfachen Worten, Erklärungen zum Aufklappen an Preisen und Beiträgen, Neuerungen nach einem Update (`NEUERUNGEN` in `web/start.js`, `web/anleitung.js`).
- **Komfort:** tolerante Suche (Tippfehler), Tabellen sortierbar und druckbar (ab 6 Zeilen), »Speichern und nächste« in Dialogen, Rückgängig-Meldungen statt Nachfragen, Papierkorb, Ungespeichert-Warnung, Dialog-Entwürfe bis zum Programmende, letzte Seite wird gemerkt, Sammeländerung mehrerer Pächter (`paechter_sammel.go`), Bereiche ein-/ausblenden (`module.go`).
- **Darstellung:** Vereinsfarbe und Logo, Modus »Gut lesbar«, Schriftgröße, Handy-Layout (390 px), Barrierefreiheit (Sprunglink, Dialogtitel, Fokus), dezente Animationen (aus bei »Bewegung reduzieren«), Strichzeichnungen (`web/bilder/`), Projektseite `docs/index.html`.

## Abrechnung
- **Zählerstände:** Schnelleingabe, Tabellenansicht, Ablesedatum je Garten, Ablesebogen (PDF), Zählerstand-Zettel zum Einwerfen, Zählerstand-Erinnerung mit Aushang, Zählerwechsel-Protokoll, Garten ohne Strom, Handy-Ablesung als eigenständige Datei (Import mit Auffälligkeits-Hinweis; Seite `ablesung_handy_seite.go`), Wasser-Vergleich mit Hauptzähler (`ablesebogen.go`, `zaehlerzettel.go`, `zaehlerwechsel.go`, `ablesung_handy.go`, `erinnerung.go`, `schwund.go`).
- **Rechnungen:** Prüfen, Vorschau, Ausstellen einzeln/alle, PDF mit GiroCode (QR; Druckordner `druck.go`, gemeinsame PDF-Helfer `pdfhelfer.go`), Postversand-Sammeldatei, E-Mail-Vorbereitung (eigenes Mailprogramm), Warnung bei Musterwerten, Gesamt-PDF der Jahresabrechnung, Gutschrift, Ratenplan (`rechnungen.go`, `pdf.go`, `email.go`, `jahresabrechnung.go`, `gutschrift.go`, `ratenplan.go`).
- **Zahlungen:** Haken »bezahlt«, Teilbeträge in der Zeile, Offene Posten, Offene Vorjahre, Exporte, Mahnwesen in 3 Stufen mit Frist und Mahn-Vorschlag, Kontoauszug einlesen (CSV, lernender Bankabgleich, auch für Spenden/Vorauszahlungen) (`kontoauszug.go`, `kontolernen.go`, `abgleich.go`).
- **Vorauszahlungen** per Überweisung *[vorauszahlung]*: Monatsbetrag je Pächter, Monatsübersicht, Erinnerung (PDF) (`vorauszahlung.go`).
- **SEPA-Lastschrift** (einschaltbar): Gläubiger-ID, Mandate, Vorabankündigung, Lauf mit Vorschau, Datei pain.008, Rücklastschrift (`sepa.go`, `sepa_pdf.go`).
- **Jahreswechsel:** Checkliste, Ampel-Prüfung vor dem Wechsel, Übernahme von Zählerständen und Kassenbestand, Sicherung vorher, abgeschlossene Jahre gesperrt (`abschluss.go`).
- **Beitragsübersicht** je Pächter, Pachtobergrenze nach § 5 BKleingG *[pachtgrenze]* (`beitragskonto.go`, `pachtgrenze.go`).

## Verein und Gärten
- **Lageplan:** frei anordnen, 3D-Ansicht (three.js, lokal), Hintergrundbild, Umrisse, Leitungen, Wege, Einrichtungen, Druck mit Legende und Maßstab (`lageplan.go`, `lagekarte.go`, `web/lageplan.js`, `web/lageplan3d.js`).
- **Pächter:** Anlegen, Import aus Excel/CSV mit Vorschau (`importexport.go`, `xlsx.go`), Kontakt, Zähler, Umlage/Versandart, Garten abgeben (mit Kündigung oder sofort), Pächterwechsel-Assistent mit Schlussrechnung, Wertermittlung, Gartenakte (Dokumente, Zeitleiste), Pächter-Mappe, Willkommensmappe, Exposé freier Gärten, freier-Garten-Vorschlag, Pachtvertrag (PDF) (`gartenakte.go`, `gartenzeitleiste.go`, `wechsel.go`, `pachtvertrag.go`, `vereinsmappe.go`, `freiervorschlag.go`).
- **Warteliste** *[warteliste]*: Reihenfolge, Besichtigungstermine, Briefe (Angebot, Nachfrage, Bestätigung), Nachrücker-Vorschlag, Aufnahmeantrag (`warteliste.go`, `warteliste_brief.go`, `aufnahmeantrag.go`).
- **Kündigungen:** Fristprüfung nach Vereinsregel, Bestätigungsbrief, Fahrplan bis zur Übergabe (`kuendigung.go`).
- **Jubiläen** *[jubilaeen]*, Geburtstage (nur mit Einverständnis) (`geburtstag.go`).
- **Mängel & Begehung:** Begehungsliste, geführte Begehung, Mängel mit Foto, Fahrplan (Aufforderung, Erinnerung, Zweitschreiben), Drittelregelung, Mehrfachauswahl, Auswahl drucken (`maengel*.go`, `nutzung.go`).
- **Laubenkataster** *[bauten]*: Bauten je Garten, Warnung über 24 m² (`bauten.go`).
- **Schlüssel & Geräte** *[inventar]*: Ausgabe/Pfand/Verlauf, Gerätebuch, Wartung, Pflegeplan für Gemeinschaftsflächen (`inventar.go`, `wartungsplan.go`).
- **Eichfristen** *[eichung]* der Zähler (`eichung.go`).
- **Termine:** Jahresplaner mit Hinweisen im Cockpit (`planer_hinweise.go`), Terminserien, Aushang, Kalenderdatei, Arbeitseinsätze mit Stundenliste und Abhaken, Gemeinschaftsstunden-Konto, Ehrenamts-Nachweis, Saison-Checkliste (`termine.go`, `terminserie.go`, `einsatz.go`, `stundenkonto.go`, `ehrenamt.go`, `saisonliste.go`, `fristenkalender.go`).
- **Rundschreiben** *[rundschreiben]*: E-Mail an alle/Auswahl (Bcc, Pakete), Serienbrief, Aushang, Vorlagen, Platzhalter (`rundschreiben.go`, `aushang.go`).

## Vorstandsarbeit (Vereinsverwaltung)
- Satzungsfristen (Einladungsfrist, Kündigungsregel) mit Cockpit-Erinnerung (`satzungsfristen.go`).
- Ämter und Wahlen-Assistent (Stimmzettel, Auszählung, Amtswechsel) (`aemter.go`, `wahl.go`).
- Beschlussbuch (durchsuchbar, PDF), Verträge und Versicherungen mit Kündigungsterminen (`verwaltung.go`).
- Versammlung: Einladung, Anwesenheitsliste, Protokoll mit Abstimmungen, Textbausteine, Versammlungspaket als ZIP, Mitgliederbefragung (`versammlung.go`, `sitzung.go`, `versammlungspaket.go`, `befragung.go`).
- Eigene Brieftexte (Anrede, Grußformel, Mahnstufen, Mängel, Warteliste, Kündigung) mit Probebrief (`brieftexte.go`).
- Amtsübergabe: Vereinsakte als ZIP mit »Lies mich zuerst« (`vereinsakte.go`).

## Kasse und Berichte
- **Kassenbericht:** Anfangs-/Endbestand, Vorjahresvergleich, Ausgaben mit Belegen und Prüfhaken, Verlauf über die Jahre, Versorger-Vergleich, Wasserschwund (eingeklappt), Wechsel-Hinweis ausgeschiedener Pächter (`auswertung.go`, `schwund.go`).
- **Haushaltsplan** *[haushalt]* mit Ampel; **Barkasse** *[barkasse]* mit Belegnummern; **Kassenprüfung** und Kassenprüfer-Zugang; Kassenbuch-Vorlagen (`barkasse.go`, `pruefung.go`, `kassenvorlagen.go`).
- **Spenden** *[spenden]*: Zuwendungsbestätigungen nach amtlichem Muster, Sammelbestätigung, Jahrespaket (ZIP) (`spenden.go`, `zuwendung.go`, `spendenpaket.go`).
- **Berichte & Meldungen:** Berichte als PDF (`berichte_pdf.go`), Jahresbericht, Jahresmeldung an den Verband, Statistik, Kostenvergleich über die Jahre, Pächterliste (`mitgliederliste.go`), Wartungs-Check, Datenbereinigung (`jahresbericht.go`, `jahresmeldung.go`, `statistik.go`, `kostenvergleich.go`, `wartung.go`, `datencheck.go`).

## Sicherheit und Datenschutz
- Automatische Sicherungen (bei Änderung, vor heiklen Schritten, vor dem Jahreswechsel), zweiter Sicherungsordner, verschlüsselte Sicherung (AES-256-GCM), Sicherung ansehen/vergleichen, Wiederherstellungs-Probe (alle 30 Tage automatisch, Cockpit-Erinnerung nach 90 Tagen), Selbsttest nach Updates (`sicherung.go`, `tresor.go`, `wiederherstellung.go`, `selbsttest.go`).
- Passwort, »Alles nur mit Anmeldung«, Mehrplatz-Sperre ohne Cloud, Abgleich zweier Rechner per USB-Stick, Änderungsprotokoll, Einstellungs-Verlauf (zurücknehmbar) (`sperre.go`, `einstellungsverlauf.go`).
- Datenschutz: Auskunft nach Art. 15, endgültiges Löschen entfernt Name und ID überall, Bereinigung nach Fristen, Papierkorb 30 Tage (`datenschutz.go`, `papierkorb.go`).
- Updates: nur der Update-Check geht ins Internet (`api.go`).

## Nicht bauen (abgelehnt)
Luftbild/WMS, Gartenschild/Mitgliedsausweis, Tastenkürzel, Kautions-Feld.

## Bekannte Lücken / Ideen (nicht verbindlich)
- Cockpit-Hinweis »Wasserschwund« öffnet die eingeklappte Karte `kb-schwund` nicht selbst.
- Klick-Zufallstest klickt in Handy-Breite nicht (Menü eingeklappt).

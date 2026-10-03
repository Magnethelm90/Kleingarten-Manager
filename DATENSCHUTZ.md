# Datenschutz (DSGVO) – Hinweise für den Verein

> **Wichtig:** Ein Programm kann nicht „DSGVO-konform“ sein, das ist immer die Art, wie ein Verein es einsetzt.
> Der Kleingarten-Manager unterstützt dabei technisch (Auskunft, Löschung, Fristen, lokale Speicherung).
> Dieses Dokument ist eine Arbeitshilfe, **keine Rechtsberatung**. Vorlagen bitte an euren Verein anpassen und im
> Zweifel mit dem Landes- oder Kreisverband, einem Datenschutzbeauftragten oder einer Anwältin abstimmen.

## 1. Überblick

- **Verantwortlich** im Sinne der DSGVO ist der Verein (Vorstand), nicht der Programmautor.
- Alle Daten liegen **nur lokal** im Datenordner (`kleingarten-manager-daten.json`, `Rechnungen/`, `Belege/`,
  `Sicherungen/`). Es gibt keine Cloud, keine Konten, keine Telemetrie, keinen Auftragsverarbeiter.
- **Einzige Verbindung nach außen:** der Knopf „Nach Updates suchen“. Er fragt nur auf Klick die öffentliche
  GitHub-Release-Seite ab; dabei sieht GitHub die IP-Adresse. Es werden keine Vereinsdaten übertragen.
- **Achtung bei Cloud-Ordnern:** Liegt der Datenordner oder der „zweite Sicherungsordner“ in OneDrive, iCloud,
  Dropbox o. Ä., gibt der Verein Mitgliederdaten an diesen Anbieter weiter. Dann ist ein Auftragsverarbeitungsvertrag
  nötig. Besser: lokale Platte oder verschlüsselter USB-Stick.

## 2. Was das Programm für die Betroffenenrechte tut

| Recht | Wo im Programm |
| --- | --- |
| Auskunft (Art. 15) und Datenkopie (Art. 20) | Admin → Pächter → **Auskunft** (auch im Papierkorb und unter Admin → Datenschutz): alle Daten einer Person als maschinenlesbare JSON-Datei |
| Berichtigung (Art. 16) | Admin → Pächter → Bearbeiten |
| Löschung (Art. 17) | Admin → Pächter → Löschen (Papierkorb) → Papierkorb → **Endgültig löschen** |
| Einschränkung (Art. 18) | Pächter im Papierkorb lassen: er verschwindet aus allen Ansichten und Berechnungen, bleibt aber erhalten |
| Speicherbegrenzung (Art. 5) | Admin → Datenschutz → **Abgelaufene Unterlagen bereinigen** nach Ablauf der Aufbewahrungsfrist |

Eine Anfrage ist innerhalb **eines Monats** zu beantworten (Art. 12). Die Identität der anfragenden Person
vorher prüfen.

## 3. Löschkonzept

**Endgültig löschen** entfernt sofort:

- Stammdaten, interne Notiz, Zählernummern, offene (nicht abgeschlossene) Zählerstände,
- den Namen und die Mitgliedsnummer in der Garten-Historie und im Änderungsprotokoll,
- Hinweis- und Notizfelder in aufbewahrten Rechnungen und Jahresunterlagen,
- **alles Genannte auch in allen vorhandenen Sicherungen** (inkl. zweitem Sicherungsordner, sofern erreichbar).
  Dafür wird bewusst keine Sicherung „vor dem Löschen“ angelegt.

**Bleibt zunächst bestehen:** ausgestellte Rechnungen (Archiv und PDF-Dateien) und die eingefrorenen Pächterlisten
abgeschlossener Jahre. Sie sind Buchungsunterlagen und unterliegen der steuerlichen Aufbewahrungspflicht
(Art. 17 Abs. 3 lit. b DSGVO erlaubt diese Ausnahme).

**Frist im Programm:** 10 Jahre, gerechnet ab Ende des Kalenderjahres der Ausstellung (konservativ: Bücher und
Jahresabschlüsse 10 Jahre; für reine Buchungsbelege gilt seit 2025 meist 8 Jahre). Welche Frist für euch gilt, klärt
ihr mit Steuerberatung oder Kassenprüfern. Nach Ablauf entfernt **Admin → Datenschutz → Abgelaufene Unterlagen
bereinigen** Name, Anschrift und Zählernummern aus Rechnungen und Jahresunterlagen, löscht die PDF-Dateien (auch aus
den Sicherungen) und sperrt den Nachdruck. Die Beträge bleiben für die Statistik.

Das Programm erinnert nicht von selbst: einmal im Jahr (z. B. beim Jahreswechsel) unter Admin → Datenschutz
nachsehen.

## 4. Technische und organisatorische Maßnahmen (Art. 32)

**Im Programm eingebaut**

- Server lauscht nur auf `127.0.0.1`, Schutz gegen fremde Webseiten (Host-/Origin-Prüfung, Pflicht-Header,
  Content-Security-Policy ohne Inline-Skripte)
- Admin-Passwort optional, nur als PBKDF2-Hash gespeichert, Sperre nach Fehlversuchen
- getrennter Kassenprüfer-Zugang mit Lesezugriff
- Änderungsprotokoll, automatische Sicherungen, Papierkorb statt sofortigem Löschen
- Dateien werden nur für den Benutzer lesbar angelegt (Rechte 0600/0700; wirksam unter macOS und Linux, unter Windows
  schützt das Benutzerkonto bzw. die Festplattenverschlüsselung)

**Muss der Verein selbst tun**

- Admin-Passwort setzen; Rechner mit Windows-/macOS-Benutzerkonto schützen, Bildschirmsperre
- **Festplatte verschlüsseln** (BitLocker, FileVault): die Datendatei ist nicht verschlüsselt
- Sicherungsmedien (USB-Stick) verschlüsseln und sicher verwahren
- Wenige, namentlich bekannte Personen mit Zugang; Weitergabe des Passworts dokumentieren
- Rechner nicht an Dritte weitergeben, ohne den Datenordner vorher sicher zu löschen

## 5. Vorlage: Verzeichnis der Verarbeitungstätigkeiten (Art. 30)

| Feld | Eintrag |
| --- | --- |
| Verantwortlicher | [Vereinsname, Anschrift, Vorstand, Kontakt] |
| Datenschutzbeauftragter | [Name/Kontakt oder „nicht benannt, da weniger als 20 Personen ständig mit der Verarbeitung beschäftigt sind (§ 38 BDSG)“] |
| Zweck | Verwaltung der Pachtverhältnisse, Abrechnung von Wasser, Strom, Pacht und Beiträgen, Zahlungsüberwachung, Kassenführung des Vereins |
| Rechtsgrundlage | Art. 6 Abs. 1 lit. b DSGVO (Pacht-/Mitgliedschaftsvertrag), lit. c (steuerliche Aufbewahrungspflichten), lit. f (Vereinsverwaltung) |
| Betroffene | Pächterinnen und Pächter, Mitglieder |
| Datenkategorien | Name, Anrede, Anschrift, Mitgliedsnummer, Gartennummer und -größe, Zählernummern und -stände, Arbeitsstunden, Rechnungs- und Zahlungsdaten, interne Notiz |
| Empfänger | Vorstand/Kassenwart; Kassenprüfer (Lesezugriff auf den Kassenbericht); Steuerberatung, Bank bei Zahlungsvorgängen; keine Auftragsverarbeiter |
| Drittlandübermittlung | keine |
| Löschfristen | Stammdaten: nach Ende der Mitgliedschaft bzw. des Pachtverhältnisses; Rechnungen und Jahresunterlagen: [10] Jahre ab Ende des Ausstellungsjahres |
| Technisch-organisatorische Maßnahmen | siehe Abschnitt 4 |

## 6. Vorlage: Information für Mitglieder (Art. 13)

> **Datenschutzhinweis**
> Der [Vereinsname], [Anschrift], verarbeitet eure Daten (Name, Anschrift, Mitglieds- und Gartennummer,
> Zählerstände, Arbeitsstunden, Rechnungs- und Zahlungsdaten), um das Pachtverhältnis zu verwalten und
> abzurechnen (Art. 6 Abs. 1 lit. b DSGVO) sowie um steuerliche Aufbewahrungspflichten zu erfüllen
> (lit. c). Die Daten werden ausschließlich auf einem Vereinsrechner gespeichert und nicht an Dritte
> weitergegeben, außer an Steuerberatung und Bank, soweit dafür nötig. Kassenprüfer erhalten Einsicht in die
> Abrechnung.
> Rechnungsunterlagen speichern wir [10] Jahre ab Ende des Jahres der Ausstellung, alle übrigen Daten bis zum
> Ende der Mitgliedschaft.
> Ihr habt das Recht auf Auskunft, Berichtigung, Löschung, Einschränkung der Verarbeitung, Datenübertragbarkeit und
> Widerspruch sowie auf Beschwerde bei der zuständigen Datenschutzaufsichtsbehörde. Ansprechpartner: [Kontakt].

## 7. Datenpanne (Art. 33, 34)

Gehen Rechner, USB-Stick oder Sicherung mit Mitgliederdaten verloren, oder wird darauf unbefugt zugegriffen:
Vorfall dokumentieren und prüfen, ob ein Risiko für die Betroffenen besteht. Wenn ja, ist er **innerhalb von
72 Stunden** der Datenschutzaufsichtsbehörde zu melden. Ist die Festplatte bzw. der Stick verschlüsselt, ist das
Risiko meist gering. Auch deshalb: Verschlüsselung (Abschnitt 4).

## 8. Bekannte Grenzen

- Die Datendatei und die Sicherungen sind **nicht verschlüsselt** (Festplattenverschlüsselung ist Aufgabe des
  Betriebssystems).
- Im Änderungsprotokoll werden Namen anhand des **aktuell gespeicherten** Namens entfernt; ältere Schreibweisen
  nach einer Namensänderung erkennt das Programm nicht. Das Protokoll enthält ohnehin nur Kurztexte.
- Die Rechnungsnummer enthält die Mitgliedsnummer, und PDF-Dateinamen enthalten den Namen; beides bleibt bis zum
  Ablauf der Aufbewahrungsfrist bestehen und wird danach mit bereinigt (Dateien gelöscht).
- Belege zu Vereinsausgaben (Fotos, PDFs) können Daten Dritter enthalten (Lieferanten); sie werden getrennt unter
  `Belege/` aufbewahrt und nicht automatisch bereinigt.
- Über die Rechnungen hinaus gibt es keine Löschfristen für Stammdaten aktiver Mitglieder; der Verein entscheidet,
  wann eine Person in den Papierkorb kommt.

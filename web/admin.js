'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Admin-Bereich und Einrichtungs-Assistent. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Tab: Admin

function viewAdmin() {
  const st = S.state;
  if (st.hasPassword && !st.loggedIn) return viewLogin();
  const t = S.adminTab;
  const body = t === 'paechter' ? adminPaechter() : t === 'warteliste' ? adminWarteliste() : t === 'maengel' ? adminBegehung() : t === 'spenden' ? adminSpenden() : t === 'sepa' ? adminSepa() : t === 'vorauszahlung' ? adminVorauszahlung() : t === 'rundschreiben' ? adminRundschreiben() : t === 'inventar' ? adminInventar() : t === 'kuendigungen' ? adminKuendigungen() : t === 'verwaltung' ? adminVerwaltung() : t === 'haushalt' ? adminHaushalt() : t === 'einstellungen' ? adminSettings()
    : t === 'jahreswechsel' ? adminYear() : t === 'kassenbericht' ? adminKassenbericht(false) : t === 'berichte' ? adminBerichte()
      : t === 'jubilaeen' ? adminJubilaeen() : t === 'daten' ? adminData() : t === 'protokoll' ? adminProtokoll() : t === 'datenschutz' ? adminDatenschutz() : adminSecurity();
  const eintrag = navEintrag('admin', t);
  return h('div', null,
    h('h2', null, eintrag ? navLabel(eintrag) : 'Verwaltung', ' ', seitenHilfe(t)),
    reiterKopf(),
    !st.hasPassword && ['einstellungen', 'daten', 'datenschutz'].includes(t) ? h('div', { class: 'banner warn' }, 'Für die Verwaltung ist noch kein Passwort gesetzt – jeder an diesem Rechner kann Pächter und Preise ändern. ',
      h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('sicherheit'); render(); } }, 'Passwort festlegen')) : null,
    body);
}

function viewLogin() {
  const st = S.state;
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/admin/login', { password: pw.value }); await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card login' }, h('img', { class: 'login-bild', src: '/bilder/garten.svg', alt: '', onerror: (e) => e.target.remove() }), h('h2', null, st.gesperrt ? 'Anmeldung' : 'Anmeldung zur Verwaltung'),
    h('p', { class: 'hint' }, st.gesperrt ? 'Das Programm ist durch ein Passwort geschützt.' : 'Dieser Bereich ist durch ein Passwort geschützt.'), pw,
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: go }, 'Anmelden')),
    st.hasPruefPassword ? h('p', { style: 'margin-top:18px' },
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = true; render(); } }, 'Als Kassenprüfer anmelden')) : null);
}

function viewPruefLogin() {
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/pruef/login', { password: pw.value }); S.pruefLoginMode = false; await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card' }, h('h2', null, 'Kassenprüfer-Anmeldung'),
    h('p', { class: 'hint' }, 'Lesezugriff auf den Kassenbericht, ohne die übrigen Admin-Rechte.'), pw,
    h('div', { class: 'actions' },
      h('button', { class: 'btn primary', onclick: go }, 'Anmelden'),
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = false; render(); } }, 'Zurück')));
}

// Eigenständige, schmale Oberfläche für den Kassenprüfer-Zugang: nur Lesezugriff
// auf den Kassenbericht (inkl. Geprüft-Haken), kein Zugriff auf die übrigen Tabs.
function viewPruefShell() {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.year;
  const app = h('div');
  const yearSel = h('select', { onchange: async (e) => { S.year = Number(e.target.value); S.kasse = null; S.kasseYear = S.year; await reload(); } },
    st.years.map((y) => h('option', { value: y, selected: y === st.year }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  app.append(
    h('header', { class: 'top' },
      h('div', { class: 'brand' }, h('b', null, 'Kleingarten-Manager'), h('span', null, st.settings.vereinName + ' · Kassenprüfer-Zugang')),
      h('div', { class: 'spacer' }),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/pruef/logout'); } catch (e) { /* egal */ } await reload(); } }, 'Abmelden')),
    h('main', null, h('h2', null, 'Kassenbericht'), adminKassenbericht(true)));
  return app;
}

// ---- Pächter verwalten

function paechterDialog(p, vorlage) {
  const isNew = !p;
  const cur = p || { mitgliedsnr: '', gartennr: '', anrede: '', name: '', strasse: '', plzOrt: /Musterstadt/.test(S.state.settings.ort) ? '' : S.state.settings.ort, versand: 'Postversand',
    email: '', telefon: '', gartengroesse: 0, umlageAbweichend: null, wasserzaehlerNr: '', stromzaehlerNr: '', notiz: '', ...(vorlage || {}) };
  const t = (val, ph) => h('input', { type: 'text', value: val || '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const f = {
    mitgliedsnr: t(cur.mitgliedsnr, 'z. B. 35-95'), gartennr: t(cur.gartennr, 'z. B. 35'),
    anrede: h('select', { value: cur.anrede }, ['', 'Herr', 'Frau', 'Familie', 'Herr und Frau'].map((a) => h('option', { value: a }, a || '(keine)'))),
    name: t(cur.name), strasse: t(cur.strasse), plzOrt: t(cur.plzOrt),
    versand: h('select', { value: cur.versand }, ['Emailsendung', 'Postversand'].map((a) => h('option', { value: a }, a))),
    email: h('input', { type: 'email', value: cur.email || '', placeholder: 'name@beispiel.de', maxlength: 100, autocomplete: 'off' }),
    telefon: h('input', { type: 'tel', value: cur.telefon || '', maxlength: 40, autocomplete: 'off' }),
    gartengroesse: t(numIn(cur.gartengroesse || null)),
    umlage: t(numIn(cur.umlageAbweichend), `leer = Standard (${eur(S.state.settings.umlageStandard)})`),
    wz: t(cur.wasserzaehlerNr), sz: t(cur.stromzaehlerNr),
    ohneStrom: h('input', { type: 'checkbox', checked: !!cur.ohneStrom }),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%' }, cur.notiz || ''),
  };
  // Neuer Pächter: Versandart folgt der E-Mail-Adresse (ohne Adresse = Post), bis sie von Hand gewählt wird.
  if (!p) {
    if (!(vorlage && vorlage.versand)) f.versand.value = f.email.value.trim() ? 'Emailsendung' : 'Postversand';
    let vonHand = false;
    f.versand.addEventListener('change', () => { vonHand = true; });
    f.email.addEventListener('input', () => { if (!vonHand) f.versand.value = f.email.value.trim() ? 'Emailsendung' : 'Postversand'; });
  }
  f.gartengroesse.setAttribute('inputmode', 'decimal');
  const stromSchalten = () => { f.sz.disabled = f.ohneStrom.checked; };
  f.ohneStrom.addEventListener('change', stromSchalten); stromSchalten();
  f.umlage.setAttribute('inputmode', 'decimal');
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  let weitere;
  const body = h('div', { class: 'form' },
    fld('Mitgliedsnummer *', f.mitgliedsnr), fld('Gartennummer', f.gartennr), fld('Anrede', f.anrede),
    fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('Versandart', f.versand, 'Steht oben rechts auf der Rechnung'),
    fld('E-Mail-Adresse', f.email, 'Empfänger beim Vorbereiten der Rechnungs-E-Mail'), fld('Telefon', f.telefon),
    fld('Gartengröße in m² *', f.gartengroesse),
    // selten gebrauchte Angaben eingeklappt; aufgeklappt, sobald dort schon etwas steht oder ein Fehler gemeldet wird
    weitere = h('details', { class: 'weitere field wide', open: !!(cur.umlageAbweichend != null || cur.wasserzaehlerNr || cur.stromzaehlerNr || cur.notiz || cur.ohneStrom) },
      h('summary', null, 'Weitere Angaben (Umlage, Zählernummern, Notiz)'),
      h('div', { class: 'form' },
        fld('Umlage abweichend in €', f.umlage, 'Nur ausfüllen, wenn dieser Pächter nicht die Standard-Umlage zahlt'),
        fld('Wasserzähler-Nr.', f.wz),
        h('div', { class: 'field' }, h('label', null, 'Stromzähler-Nr.'), f.sz,
          h('label', { class: 'inline', style: 'font-weight:400' }, f.ohneStrom, ' Garten hat keinen Strom')),
        fld('Notiz (nur intern, steht nicht auf der Rechnung – bitte keine Gesundheitsdaten oder sonstige sensible Angaben)', f.notiz, null, true))));
  return modal(isNew ? 'Pächter anlegen' : `Pächter bearbeiten – ${cur.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const gg = parseNum(f.gartengroesse.value), um = parseNum(f.umlage.value);
      if (!f.mitgliedsnr.value.trim()) return feldFehler(f.mitgliedsnr, 'Bitte die Mitgliedsnummer eintragen.');
      if (!f.name.value.trim()) return feldFehler(f.name, 'Bitte den Namen eintragen.');
      if (Number.isNaN(gg) || (gg !== null && gg < 0)) return feldFehler(f.gartengroesse, 'Die Gartengröße muss eine Zahl ab 0 sein, z. B. 300.');
      if (Number.isNaN(um) || (um !== null && um < 0)) { weitere.open = true; return feldFehler(f.umlage, 'Die Umlage muss eine Zahl ab 0 sein, z. B. 25,00.'); }
      const data = { mitgliedsnr: f.mitgliedsnr.value, gartennr: f.gartennr.value, anrede: f.anrede.value, name: f.name.value,
        strasse: f.strasse.value, plzOrt: f.plzOrt.value, versand: f.versand.value, email: f.email.value.trim(), telefon: f.telefon.value, gartengroesse: gg ?? 0, umlageAbweichend: um,
        wasserzaehlerNr: f.wz.value, stromzaehlerNr: f.ohneStrom.checked ? '' : f.sz.value, ohneStrom: f.ohneStrom.checked, notiz: f.notiz.value.trim() };
      if (data.versand === 'Emailsendung' && !data.email && !(await confirmBox('Versandart ist »Emailsendung«, aber es ist keine E-Mail-Adresse eingetragen. Trotzdem speichern?', 'Trotzdem speichern'))) return false;
      const gnr = data.gartennr.trim().toLowerCase();
      const gleich = gnr ? S.state.paechter.find((q) => q.id !== cur.id && (q.gartennr || '').trim().toLowerCase() === gnr) : null;
      if (gleich && !(await confirmBox(`Die Gartennummer ${data.gartennr.trim()} ist schon bei ${gleich.name} (${gleich.mitgliedsnr}) eingetragen. Trotzdem speichern?`, 'Trotzdem speichern'))) return false;
      try {
        if (isNew) await api('POST', '/api/admin/paechter', data);
        else await api('PUT', `/api/admin/paechter/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function adminPaechter() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  // Spalte »Umlage« nur, wenn jemand eine abweichende Umlage zahlt (spart Platz im Programmfenster)
  const mitUmlage = list.some((p) => p.umlageAbweichend != null);
  const tbody = h('tbody');
  // Auswahl für die Sammeländerung (Versandart/Umlage für mehrere auf einmal)
  const sel = (S.paechterSel = S.paechterSel || new Set());
  for (const id of [...sel]) if (!list.some((p) => p.id === id)) sel.delete(id);
  const sammelKnopf = h('button', { class: 'btn', title: 'Versandart oder Umlage für alle ausgewählten Pächter auf einmal ändern', onclick: () => paechterSammelDialog(list.filter((p) => sel.has(p.id))) });
  const alleKasten = h('input', { type: 'checkbox', 'aria-label': 'Alle angezeigten auswählen' });
  let shown = [];
  const auswahlZeigen = () => {
    sammelKnopf.hidden = !sel.size;
    sammelKnopf.textContent = `Auswahl ändern (${sel.size})`;
    alleKasten.checked = shown.length > 0 && shown.every((p) => sel.has(p.id));
  };
  alleKasten.addEventListener('change', () => { for (const p of shown) { if (alleKasten.checked) sel.add(p.id); else sel.delete(p.id); } fill(); });
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    shown = list.filter((p) => !q || [p.mitgliedsnr, p.name, p.gartennr, p.strasse, p.email, p.telefon].some((x) => (x || '').toLowerCase().includes(q)));
    for (const p of shown) {
      tbody.append(h('tr', null,
        h('td', { class: 'auswahl' }, h('input', { type: 'checkbox', 'aria-label': 'Auswählen: ' + p.name, checked: sel.has(p.id),
          onchange: (e) => { if (e.target.checked) sel.add(p.id); else sel.delete(p.id); auswahlZeigen(); } })),
        h('td', null, p.mitgliedsnr), h('td', null, p.gartennr),
        h('td', { class: 'name', title: p.name }, p.name, p.notiz ? h('span', { title: p.notiz, style: 'margin-left:4px;cursor:help', class: 'notiz-zeichen' }, icon('bericht', 14)) : null, schlussBadge(p)),
        h('td', { class: 'wrap' }, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
        h('td', null, [p.telefon, p.email].filter(Boolean).map((x, i) => [i ? h('br') : null, x])),
        h('td', { class: 'num' }, nfFlex.format(p.gartengroesse) + ' m²'),
        mitUmlage ? h('td', { class: 'num' }, p.umlageAbweichend == null ? 'Standard' : eur(p.umlageAbweichend)) : null,
        h('td', { class: 'zaehler-zelle' }, p.wasserzaehlerNr ? h('div', null, h('span', { class: 'hint' }, 'W '), p.wasserzaehlerNr) : null,
          p.ohneStrom ? h('div', { class: 'hint' }, 'kein Strom') : p.stromzaehlerNr ? h('div', null, h('span', { class: 'hint' }, 'S '), p.stromzaehlerNr) : null),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', onclick: async () => { if (await paechterDialog(p)) await reload(); } }, 'Bearbeiten'), ' ',
          p.gartennr ? [h('button', { class: 'btn small', title: `Gartenakte von Garten ${p.gartennr}: Dokumente, Wertermittlungen, Übergaben, Verlauf`, onclick: () => gartenakteDialog(p.gartennr.trim()) }, 'Akte'), ' '] : null,
          mehrMenue([
            { label: 'Mappe …', title: 'Beiträge, Pachtvertrag, Willkommensmappe, Ehrenamts-Nachweis, Gartenakte und DSGVO-Auskunft an einer Stelle', aktion: () => paechterMappe(p) },
            p.gartennr && !p.ausgeschiedenAm ? { label: 'Garten abgeben …', title: 'Kündigung, Nachfolger, Wertermittlung, Schlüssel und Übergabe – Schritt für Schritt', aktion: () => wechselAssistent(p) } : null,
            { label: 'In den Papierkorb', gefahr: true, aktion: async () => {
              const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) in den Papierkorb legen? Er verschwindet aus allen Ansichten, Zählerstände und Rechnungen bleiben aber erhalten und lassen sich im Papierkorb wiederherstellen.`, 'In den Papierkorb', true);
              if (!ok) return;
              try { await api('DELETE', `/api/admin/paechter/${p.id}`); toast('In den Papierkorb gelegt', 'ok'); S.papierkorb = null; await reload(); } catch (e) { handleErr(e); }
            } }]))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: mitUmlage ? 10 : 9, class: 'empty' }, list.length ? 'Keine Treffer.'
      : leer('personen', 'Noch keine Pächter', 'Lege den ersten mit »+ Pächter anlegen« an oder übernimm alle auf einmal aus einer Excel- oder CSV-Liste.', 'Liste importieren', () => { adminTabWaehlen('daten'); render(); }))));
    auswahlZeigen();
  };
  fill();
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier pflegst du die Stammdaten der Pächter. Sie ändern sich selten und müssen nur einmal angelegt werden.'),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: async () => { if (await paechterDialog(null)) await reload(); } }, '+ Pächter anlegen'),
      h('input', { type: 'search', placeholder: 'Suchen', value: S.filter, style: 'width:220px', oninput: (e) => { S.filter = e.target.value; fill(); } }),
      h('span', { class: 'stat' }, `${list.length} Pächter`),
      sammelKnopf,
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', title: 'Liste aller Pächter mit Anschrift als PDF zum Ausdrucken', onclick: paechterlisteDrucken }, 'Pächterliste drucken')),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, h('th', { class: 'auswahl' }, alleKasten), ['Mitgl.-Nr.', 'Garten', 'Name', 'Anschrift', 'Kontakt', 'Größe', mitUmlage ? 'Umlage' : null, 'Zähler', ''].filter((x) => x !== null).map((x) => h('th', null, x)))), tbody)),
    uebergabenCard(),
    papierkorbCard());
}

// Kleines Aufklappmenü »Mehr« für seltener gebrauchte Aktionen einer Tabellenzeile.
function mehrMenue(eintraege, beschriftung = 'Mehr ▾') {
  const liste = h('div', { class: 'mehr-liste', role: 'menu' });
  const knopf = h('button', { class: (beschriftung === 'Mehr ▾' ? 'btn small' : 'btn') + ' mehr-knopf', 'aria-haspopup': 'menu', 'aria-expanded': 'false', title: 'Weitere Aktionen' }, beschriftung);
  const box = h('div', { class: 'mehr' }, knopf, liste);
  const platzieren = () => {
    // fest am Fenster ausrichten (die Tabelle scrollt und würde das Menü sonst abschneiden);
    // nach oben aufklappen, wenn unten kein Platz ist
    const r = knopf.getBoundingClientRect(), hoehe = liste.offsetHeight, breite = liste.offsetWidth;
    liste.style.left = Math.max(8, r.right - breite) + 'px';
    liste.style.top = (r.bottom + 4 + hoehe > window.innerHeight ? r.top - 4 - hoehe : r.bottom + 4) + 'px';
  };
  const zu = () => {
    box.classList.remove('offen'); knopf.setAttribute('aria-expanded', 'false');
    document.removeEventListener('click', aussen, true); window.removeEventListener('scroll', platzieren, true);
  };
  const aussen = (e) => { if (!box.contains(e.target)) zu(); };
  knopf.addEventListener('click', () => {
    if (box.classList.contains('offen')) { zu(); return; }
    document.querySelectorAll('.mehr.offen').forEach((m) => m.classList.remove('offen'));
    box.classList.add('offen'); knopf.setAttribute('aria-expanded', 'true');
    platzieren();
    document.addEventListener('click', aussen, true);
    window.addEventListener('scroll', platzieren, true);
  });
  for (const x of eintraege.filter(Boolean)) {
    liste.append(h('button', { class: 'mehr-eintrag' + (x.gefahr ? ' gefahr' : ''), role: 'menuitem', title: x.title || null,
      onclick: () => { zu(); x.aktion(); } }, x.label));
  }
  return box;
}

// Pächterliste (Garten, Name, Anschrift, Größe, Versandart, Zähler) zum Ausdrucken.
const paechterlisteDrucken = () => pdfDrucken({
  url: '/api/admin/paechterliste', oeffnenUrl: '/api/admin/paechterliste/oeffnen',
  titel: 'Pächterliste', meldung: () => 'Die Pächterliste wurde im PDF-Programm geöffnet – dort drucken.',
});

function auskunftLink(p) {
  return h('a', { class: 'btn small', href: `/api/admin/paechter/${p.id}/auskunft`,
    title: 'Alle gespeicherten Daten dieser Person als Datei herunterladen (Auskunft nach Art. 15 DSGVO)' }, 'Auskunft');
}

// ---- Datenschutz

async function loadDatenschutz() {
  if (S.datenschutzLoading) return;
  S.datenschutzLoading = true;
  try { S.datenschutz = await api('GET', '/api/admin/datenschutz'); } catch (e) { handleErr(e); S.datenschutz = false; }
  S.datenschutzLoading = false;
  render();
}

function adminDatenschutz() {
  const st = S.state;
  if (S.datenschutz == null) { if (!S.datenschutzLoading) loadDatenschutz(); }
  const d = S.datenschutz || null;
  const list = [...st.paechter].sort(cmpNr);
  const sel = h('select', { style: 'min-width:260px' }, list.map((p) => h('option', { value: p.id }, `${p.mitgliedsnr}  ${p.name}`)));
  const bereinigen = async () => {
    const ok = await confirmBox(`Bei ${anz(d.faelligRechnungen, 'Rechnung', 'Rechnungen')} und ${anz(d.faelligJahre, 'Jahresunterlage', 'Jahresunterlagen')} ist die Aufbewahrungsfrist (${d.aufbewahrungJahre} Jahre) abgelaufen. Name und Anschrift werden dort entfernt, die PDF-Dateien gelöscht, auch in den Sicherungen. Die Beträge bleiben für die Statistik. Das kann nicht rückgängig gemacht werden.`, 'Jetzt bereinigen', true);
    if (!ok) return;
    try {
      const r = await api('POST', '/api/admin/datenschutz/bereinigen');
      toast(`Bereinigt: ${anz(r.rechnungen, 'Rechnung', 'Rechnungen')}, ${anz(r.jahre, 'Jahresunterlage', 'Jahresunterlagen')}, ${anz(r.dateien, 'PDF-Datei', 'PDF-Dateien')}, ${anz(r.sicherungen, 'Sicherung', 'Sicherungen')}`, 'ok');
      S.datenschutz = null; S.archivAlle = null; await reload();
    } catch (e) { handleErr(e); }
  };
  const li = (...kids) => h('li', null, ...kids);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hilfen für den Umgang mit personenbezogenen Daten nach der DSGVO. Für die Einhaltung ist der Verein verantwortlich; das Programm unterstützt dabei, ersetzt aber keine Rechtsberatung.'),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Was wird wo gespeichert?'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li(h('b', null, 'Alles liegt nur auf diesem Rechner'), ' (Datenordner: ', h('span', { class: 'mono' }, st.dataDir), '). Es gibt keine Cloud und keine Übertragung ins Internet.'),
        li('Pächter: Name, Anschrift, Mitgliedsnummer, Garten, Zählernummern, interne Notiz; dazu Zählerstände, Arbeitsstunden, Rechnungen, Zahlungen, Übergabeprotokolle.'),
        li('Warteliste: Name, Anschrift und Kontakt der Interessenten. Wer kein Interesse mehr hat, wird gelöscht (auch aus den Sicherungen).'),
        li('Kopien davon stecken in ausgestellten Rechnungen (Archiv und PDF-Dateien), abgeschlossenen Jahren, der Garten-Historie, dem Änderungsprotokoll und den automatischen Sicherungen.'),
        li('Einzige Verbindung nach außen: der Knopf »Nach Updates suchen« (nur auf Klick, überträgt dabei die IP-Adresse an GitHub).'))),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Auskunft und Datenkopie (Art. 15 und 20 DSGVO)'),
      h('p', { class: 'hint' }, 'Wer wissen möchte, was über sie oder ihn gespeichert ist, bekommt hier alle Daten als Datei (maschinenlesbar, JSON). Dieselbe Datei gibt es auch je Person in der Pächter-Liste (»Auskunft«) und im Papierkorb – am besten erstellst du sie, bevor du jemanden löschst.'),
      list.length
        ? h('div', { class: 'toolbar' }, sel, h('a', { class: 'btn primary', href: '#', onclick: (e) => { e.preventDefault(); location.href = `/api/admin/paechter/${encodeURIComponent(sel.value)}/auskunft`; } }, 'Auskunft herunterladen'))
        : h('p', { class: 'hint' }, 'Noch keine Pächter angelegt.')),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Löschen und Aufbewahrungsfristen (Art. 17 DSGVO)'),
      h('p', { class: 'hint' }, 'Ein Pächter, der nicht mehr dabei ist, kommt zuerst in den Papierkorb. Dort kannst du ihn »endgültig löschen«: Dabei werden Stammdaten, Notizen, offene Zählerstände und sein Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen entfernt. Rechnungen und Jahresunterlagen müssen aus steuerlichen Gründen aber noch eine Zeit lang bleiben. Diese Frist beträgt hier ' + (d ? d.aufbewahrungJahre : 10) + ' Jahre ab Ende des Ausstellungsjahres (die längere der üblichen Fristen; ob bei euch 8 oder 10 Jahre genügen, klärt ihr mit Steuerberatung oder Kassenprüfern). Danach entfernst du hier den Personenbezug.'),
      !d ? h('p', { class: 'hint' }, d === false ? 'Konnte nicht geladen werden.' : 'Wird geladen …') : h('div', null,
        h('ul', { style: 'margin:0 0 10px;padding-left:20px;line-height:1.6' },
          li(`Im Papierkorb wartend: ${d.papierkorb} Pächter`),
          d.wartelisteNachfragen ? li(h('b', null, `Warteliste: ${anz(d.wartelisteNachfragen, 'Eintrag', 'Einträge')} seit über 2 Jahren nicht bestätigt – nachfragen oder löschen (unter »Warteliste«).`)) : null,
          li(d.faelligRechnungen + d.faelligJahre > 0
            ? h('b', null, `Aufbewahrungsfrist abgelaufen: ${anz(d.faelligRechnungen, 'Rechnung', 'Rechnungen')}, ${anz(d.faelligJahre, 'Jahresunterlage', 'Jahresunterlagen')} (ausgestellt bis ${d.bereinigtBisJahr})`)
            : 'Keine Unterlagen mit abgelaufener Aufbewahrungsfrist.'),
          d.naechsteFrist ? li(`Die nächsten Rechnungen werden ${d.naechsteFrist} freigegeben.`) : null),
        h('button', { class: 'btn danger', disabled: d.faelligRechnungen + d.faelligJahre === 0, onclick: bereinigen }, 'Abgelaufene Unterlagen bereinigen'))),

    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Was der Verein selbst erledigen muss'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li('Verzeichnis der Verarbeitungstätigkeiten führen und die Mitglieder über die Verarbeitung informieren (Vorlagen: Datei ', h('span', { class: 'mono' }, 'DATENSCHUTZ.md'), ' im Projekt).'),
        li('Admin-Passwort setzen (Reiter »Passwort«) und die Festplatte verschlüsseln (z. B. BitLocker, FileVault); die Datendatei selbst ist nicht verschlüsselt.'),
        li('Sicherungen (auch den zweiten Sicherungsordner, USB-Stick) sicher aufbewahren; sie enthalten dieselben Daten.'),
        li('Nur nötige Angaben erfassen, keine Gesundheitsdaten oder sonstigen sensiblen Angaben in Notizfeldern.'))));
}

async function loadPapierkorb() {
  if (S.papierkorbLoading) return;
  S.papierkorbLoading = true;
  try { S.papierkorb = await api('GET', '/api/admin/paechter-papierkorb'); } catch (e) { handleErr(e); }
  S.papierkorbLoading = false;
  render();
}

function papierkorbCard() {
  if (!S.papierkorbOffen) {
    return h('div', { style: 'margin-top:14px' },
      h('button', { class: 'btn', onclick: () => { S.papierkorbOffen = true; S.papierkorb = null; render(); } }, 'Papierkorb'));
  }
  if (!S.papierkorb) {
    if (!S.papierkorbLoading) loadPapierkorb();
    return h('div', { class: 'card', style: 'margin-top:14px' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const rows = S.papierkorb.map((p) => h('tr', null,
    h('td', null, p.mitgliedsnr), h('td', null, p.name), h('td', null, deDate(p.geloeschtAm)),
    h('td', null,
      h('button', { class: 'btn small primary', onclick: async () => {
        try { await api('POST', `/api/admin/paechter/${p.id}/wiederherstellen`); toast('Wiederhergestellt', 'ok'); S.papierkorb = null; await reload(); }
        catch (e) { handleErr(e); }
      } }, 'Wiederherstellen'), ' ',
      auskunftLink(p), ' ',
      h('button', { class: 'btn small danger', onclick: async () => {
        const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) endgültig löschen? Das kann nicht rückgängig gemacht werden. Stammdaten, Notizen, offene Zählerstände sowie der Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen werden entfernt. Bereits ausgestellte Rechnungen müssen wegen der gesetzlichen Aufbewahrungspflicht noch 10 Jahre (ab Ende des Ausstellungsjahres) bleiben und verlieren ihren Namen erst danach unter Admin → Datenschutz.`, 'Endgültig löschen', true);
        if (!ok) return;
        try { await api('DELETE', `/api/admin/paechter/${p.id}/endgueltig`); toast('Endgültig gelöscht', 'ok'); S.papierkorb = null; render(); await loadPapierkorb(); }
        catch (e) { handleErr(e); }
      } }, 'Endgültig löschen'))));
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'toolbar' }, h('b', null, 'Papierkorb'), h('div', { class: 'spacer' }),
      h('button', { class: 'btn small', onclick: () => { S.papierkorbOffen = false; render(); } }, 'Schließen')),
    S.papierkorb.length
      ? h('div', { class: 'tablewrap', style: 'margin-top:10px' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Gelöscht am', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows)))
      : h('p', { class: 'hint', style: 'margin:10px 0 0' }, 'Papierkorb ist leer.'));
}

// ---- Einstellungen

const HILFE = {
  einladungTage: 'Steht in eurer Satzung, oft »zwei Wochen vorher«. Das Cockpit erinnert zwei Wochen vor dem letzten Tag, an dem die Einladung verschickt sein muss.',
  kuendigungBis: 'Bis zu diesem Tag muss eine Kündigung eingegangen sein, damit sie zum Pachtende im selben Jahr gilt. Ist bei einem Garten schon das Zweitschreiben verschickt, erinnert das Cockpit einen Monat vorher.',
  kuendigungZum: 'Zu diesem Tag endet die Pacht nach einer fristgerechten Kündigung. Das Bundeskleingartengesetz sieht für Kündigungen durch den Verein den 30. November vor.',
  rechnungsdatum: 'Steht auf allen Rechnungen dieses Jahres. Das Zahlungsziel ist der Tag, bis zu dem bezahlt sein soll – danach gilt eine Rechnung als überfällig.',
  einspruchTage: 'So viele Tage haben Pächter Zeit, sich bei Fragen oder Unstimmigkeiten zur Abrechnung zu melden. Der Satz steht auf Rechnung und E-Mail; 0 = kein Hinweis.',
  wasserGrundpreis: 'Fester Betrag je Garten und Jahr, unabhängig vom Verbrauch (z. B. für Zähler und Leitungsnetz). Dazu kommt der Verbrauch mal Preis je m³.',
  energieGrundpreis: 'Fester Betrag je Garten und Jahr, unabhängig vom Verbrauch. Dazu kommt der Verbrauch mal Preis je kWh. Hat ein Garten keinen Strom, trage bei ihm als Stand Vorjahr und Stand aktuell jeweils 0 ein – dann zahlt er nur den Grundpreis.',
  pflichtstunden: 'So viele Gemeinschaftsstunden soll jeder Pächter im Jahr leisten. Für jede fehlende Stunde wird die Nachzahlung berechnet.',
  stundenObergrenze: 'Mehrstunden werden nur bis zu dieser Gesamtzahl vergütet. Beispiel: Pflicht 10, Grenze 15 – vergütet werden höchstens 5 Stunden.',
  pachtJeQm: 'Pacht je m² Gartenfläche, wie sie der Verpächter dem Verein berechnet. Die Gartengröße steht beim Pächter. Die Pacht darf nach § 5 BKleingG höchstens das Vierfache der ortsüblichen Pacht im Obst- und Gemüseanbau betragen.',
  vereinsflaecheQm: 'Anteil an Wegen, Vereinsheim und Gemeinschaftsflächen, den jeder Pächter mitpachtet – in m². Er wird mit der Pacht je m² berechnet.',
  freieGaertenQm: 'Leer stehende Gärten muss der Verein trotzdem pachten. Die Fläche wird auf alle Pächter umgelegt – Anteil je Pächter in m², berechnet mit der Pacht je m².',
  territorialverband: 'Beitrag, den der Verein je Mitglied an den Stadt-, Kreis- oder Territorialverband abführt. Steht auf der Rechnung zusammen mit dem Vereinsbeitrag.',
  umlageStandard: 'Betrag für besondere Ausgaben, z. B. eine neue Wasserleitung – beschlossen von der Mitgliederversammlung. Beim einzelnen Pächter lässt sich ein abweichender Betrag eintragen.',
};

const SETTINGS_FORM = [
  ['Verein und Rechnung', [
    ['vereinName', 'Vereinsname', 'text', '', true], ['ort', 'PLZ und Ort', 'text'], ['absenderzeile', 'Absenderzeile (kleine Zeile über der Anschrift)', 'text', '', true],
    ['rechnungsdatum', 'Rechnungsdatum', 'date'], ['zahlungsziel', 'Zahlbar bis', 'date'],
    ['rechnungsnrPraefix', 'Rechnungsnummer-Vorsatz', 'text', 'Rechnungsnr. = Vorsatz + Mitgliedsnr., z. B. 100-35-95'],
    ['einspruchTage', 'Einspruchsfrist', 'num', 'Tage']]],
  ['Bankverbindung', [['bankName', 'Bank', 'text'], ['iban', 'IBAN', 'text'], ['bic', 'BIC', 'text']]],
  ['Wasser', [['wasserGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['wasserPreis', 'Preis je m³', 'num', '€']]],
  ['Strom / Energie', [['energieGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['energiePreis', 'Preis je kWh', 'num', '€']]],
  ['Arbeitsstunden', [
    ['pflichtstunden', 'Pflichtstunden', 'num', 'Stunden'], ['stundenObergrenze', 'Vergütung bis zu dieser Stundenzahl', 'num', 'Stunden'],
    ['verguetungJeStd', 'Vergütung je Stunde über der Pflicht', 'num', '€'], ['nachzahlungJeStd', 'Nachzahlung je fehlende Stunde', 'num', '€']]],
  ['Pacht und Beiträge (für das Folgejahr)', [
    ['pachtJeQm', 'Pacht je m²', 'num', '€'], ['vereinsflaecheQm', 'Anteil Vereinsfläche', 'num', 'm²'], ['freieGaertenQm', 'Pachtanteil freie Gärten', 'num', 'm²'],
    ['vereinsbeitrag', 'Vereinsbeitrag', 'num', '€'], ['territorialverband', 'Territorialverband', 'num', '€'], ['umlageStandard', 'Umlage (Standard für alle)', 'num', '€']]],
];

function adminSettings() {
  const st = S.state;
  const inputs = {};
  const form = h('div', { class: 'form' });
  for (const [title, fields] of SETTINGS_FORM) {
    form.append(h('div', { class: 'section-title' }, title));
    for (const [key, label, type, hint, wide] of fields) {
      const val = st.settings[key];
      const input = type === 'date' ? h('input', { type: 'date', value: val })
        : type === 'num' ? h('input', { type: 'text', inputmode: 'decimal', value: numIn(val), autocomplete: 'off' })
          : h('input', { type: 'text', value: val, maxlength: 250, autocomplete: 'off' });
      inputs[key] = { input, type };
      const unit = type === 'num' ? hint : null;
      form.append(h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label, HILFE[key] ? hilfe(HILFE[key]) : null),
        unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input,
        type !== 'num' && hint ? h('span', { class: 'fhint' }, hint) : null));
    }
  }
  const save = async () => {
    // alle übrigen Einstellungen (zweiter Sicherungsordner, Logo, Farbe) unverändert mitschicken, sonst würden sie geleert
    const data = { ...st.settings, jahr: st.settings.jahr };
    for (const [key, { input, type }] of Object.entries(inputs)) {
      if (type === 'num') {
        const v = parseNum(input.value);
        if (v === null || Number.isNaN(v) || v < 0) { input.classList.add('err'); toast('Bitte in allen Zahlenfeldern eine Zahl ab 0 eintragen.', 'err'); return; }
        input.classList.remove('err');
        data[key] = v;
      } else data[key] = input.value;
    }
    data.einspruchTage = Math.round(data.einspruchTage);
    try { await api('PUT', '/api/admin/settings', data); toast('Einstellungen gespeichert', 'ok'); await reload(); } catch (e) { handleErr(e); }
  };
  // Preise sind das Wichtigste und stehen offen da; alles Seltene ist eingeklappt (eine Zeile mit kurzer Zusammenfassung).
  const fr = S.satzungsfristen, bt = S.brieftexte || [];
  const zusatzAn = BEREICHE.filter(([k]) => modulAn(k)).length;
  return h('div', null,
    h('div', { class: 'card', id: 'es-preise' },
      h('p', { class: 'hint', style: 'margin-top:0' }, `Diese Werte gelten für das Abrechnungsjahr ${st.settings.jahr} und für alle Rechnungen. Nach einer Änderung rechnen alle Beträge automatisch neu.`),
      startjahrZeile(st),
      form, h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern'),
        h('button', { class: 'btn', title: 'Die letzten Änderungen ansehen und zurücknehmen', onclick: einstellungsVerlaufDialog }, 'Letzte Änderungen'))),
    h('h3', { style: 'margin:22px 0 8px' }, 'Weitere Einstellungen'),
    klappKarte('es-fristen', 'Fristen aus der Satzung', fr ? `Einladung ${fr.einladungTage} Tage vorher · Kündigung bis ${tagMonatAnzeige(fr.kuendigung.bis)}` : '', fristenKarte()),
    klappKarte('es-briefe', 'Brieftexte', bt.some((b) => b.angepasst) ? anz(bt.filter((b) => b.angepasst).length, 'eigener Text', 'eigene Texte') : 'Standardtexte', brieftexteKarte()),
    klappKarte('es-pacht', 'Pachtobergrenze', '§ 5 Bundeskleingartengesetz', pachtgrenzeKarte()),
    klappKarte('es-bereiche', 'Bereiche ein- und ausblenden', `${zusatzAn} von ${BEREICHE.length} Zusatzbereichen an`, bereicheKarte(st)),
    klappKarte('es-aussehen', 'Farbe und Logo', st.settings.logo ? 'mit Logo' : 'ohne Logo', aussehenKarte(st)));
}

// Einklappbare Zeile in den Einstellungen; merkt sich, was offen ist (auch nach dem Neuzeichnen).
function klappKarte(id, titel, kurz, inhalt) {
  if (!inhalt) return null;
  S.esOffen = S.esOffen || new Set();
  const d = h('details', { class: 'klapp', id, open: S.esOffen.has(id) },
    h('summary', null, h('b', null, titel), kurz ? h('span', { class: 'hint' }, kurz) : null), inhalt);
  d.addEventListener('toggle', () => { if (d.open) S.esOffen.add(id); else S.esOffen.delete(id); });
  return d;
}

// Änderungsverlauf: die letzten gespeicherten Stände der Preise, einzeln zurücknehmbar (Jahr und Sicherungsordner bleiben).
async function einstellungsVerlaufDialog() {
  let l;
  try { l = await api('GET', '/api/admin/einstellungen/verlauf'); } catch (e) { handleErr(e); return; }
  const box = h('div');
  const zeichne = () => box.replaceChildren(l.length ? h('div', null, l.map((z) => h('div', { class: 'vordruck' },
    h('div', null, h('b', null, z.datum.replace(/^(\d{4})-(\d\d)-(\d\d)/, '$3.$2.$1')), h('ul', { style: 'margin:4px 0 0;padding-left:18px' }, z.aenderungen.map((t) => h('li', null, t)))),
    h('div', { class: 'row' }, h('button', { class: 'btn', onclick: async () => {
      try { await api('POST', `/api/admin/einstellungen/verlauf/${z.index}/zurueck`); toast('Der alte Stand ist wieder da', 'ok'); box.closest('dialog').close(); S.brieftexte = null; await reload(); } catch (e) { handleErr(e); }
    } }, 'Zurücknehmen')))))
    : h('p', { class: 'hint' }, 'Noch keine Änderungen gespeichert. Sobald du Preise oder Einstellungen änderst, steht der alte Stand hier.'));
  zeichne();
  await modal('Letzte Änderungen', h('div', null, h('p', { class: 'hint' }, 'Hier siehst du, was sich geändert hat. »Zurücknehmen« stellt den alten Stand wieder her – das Abrechnungsjahr und der zweite Sicherungsordner bleiben, wie sie sind.'), box),
    [{ label: 'Schließen', cls: 'primary', value: true }]);
}

// ------------------------------------------------------------------ Einrichtungs-Assistent
// Vier Schritte: Verein, Preise, Bereiche, Pächter. Gespeichert wird nach Schritt 3 in einem Rutsch;
// alles lässt sich später unter »Preise & Einstellungen« ändern.
async function einrichtungsAssistent() {
  if (document.querySelector('dialog.assistent')) return;
  const st = S.state, set = st.settings;
  // Frische Installation: Die Musterwerte des Programms nicht vorschlagen – sonst landen sie unbemerkt auf der Rechnung.
  const muster = !st.demo && (/Mustername/.test(set.vereinName || '') || (set.iban || '').replace(/\s/g, '') === 'DE00000000000000000000');
  const text = (key, ph) => h('input', { type: 'text', value: muster ? '' : set[key] || '', placeholder: ph || '', maxlength: 250, autocomplete: 'off' });
  const zahl = (key, ph) => h('input', { type: 'text', inputmode: 'decimal', value: muster ? '' : numIn(set[key]), placeholder: ph || '0', autocomplete: 'off' });
  const f = {
    vereinName: text('vereinName', 'z. B. KGV Sonnenschein e. V.'), ort: text('ort', 'z. B. 12345 Gartenstadt'),
    absenderzeile: text('absenderzeile', 'z. B. KGV Sonnenschein e. V. · Gartenweg 1 · 12345 Gartenstadt'),
    bankName: text('bankName'), iban: text('iban', 'DE…'), bic: text('bic'),
    wasserGrundpreis: zahl('wasserGrundpreis'), wasserPreis: zahl('wasserPreis', 'z. B. 2,50'), energieGrundpreis: zahl('energieGrundpreis'), energiePreis: zahl('energiePreis', 'z. B. 0,40'),
    pachtJeQm: zahl('pachtJeQm', 'z. B. 0,10'), vereinsflaecheQm: zahl('vereinsflaecheQm'), freieGaertenQm: zahl('freieGaertenQm'),
    vereinsbeitrag: zahl('vereinsbeitrag'), territorialverband: zahl('territorialverband'), umlageStandard: zahl('umlageStandard'),
    pflichtstunden: zahl('pflichtstunden'), nachzahlungJeStd: zahl('nachzahlungJeStd'), verguetungJeStd: zahl('verguetungJeStd'), stundenObergrenze: zahl('stundenObergrenze'),
  };
  const zahlFelder = ['wasserGrundpreis', 'wasserPreis', 'energieGrundpreis', 'energiePreis', 'pachtJeQm', 'vereinsflaecheQm', 'freieGaertenQm', 'vereinsbeitrag', 'territorialverband', 'umlageStandard', 'pflichtstunden', 'nachzahlungJeStd', 'verguetungJeStd', 'stundenObergrenze'];
  // Absender folgt Name und Ort, bis er von Hand geändert wird
  let absenderVonHand = !muster && !!set.absenderzeile;
  const absenderVorschlag = () => { if (!absenderVonHand) f.absenderzeile.value = [f.vereinName.value.trim(), f.ort.value.trim()].filter(Boolean).join(' · '); };
  f.absenderzeile.addEventListener('input', () => { absenderVonHand = true; });
  f.vereinName.addEventListener('input', absenderVorschlag); f.ort.addEventListener('input', absenderVorschlag);
  // Erstes Abrechnungsjahr: solange noch nichts gebucht ist
  const jetzt = new Date().getFullYear();
  const jahrWahl = st.startjahrAenderbar ? h('select', null, [jetzt - 1, jetzt].concat(set.jahr < jetzt - 1 || set.jahr > jetzt ? [set.jahr] : [])
    .map((j) => h('option', { value: j, selected: j === set.jahr }, String(j)))) : null;
  const feld = (label, input, einheit, key, breit) => h('div', { class: 'field' + (breit ? ' wide' : '') }, h('label', null, label, key && HILFE[key] ? hilfe(HILFE[key]) : null),
    einheit ? h('div', { class: 'inputunit' }, input, h('span', null, einheit)) : input);
  const bereiche = BEREICHE.map(([k, name, t]) => ({ k, cb: h('input', { type: 'checkbox', checked: modulAn(k) }), name, t }));
  const sepaCb = h('input', { type: 'checkbox', checked: !!st.sepaAktiv });
  const schritte = [
    { titel: 'Euer Verein', text: 'Name, Absender und Bankverbindung stehen auf jeder Rechnung.', inhalt: h('div', { class: 'form' },
      feld('Vereinsname', f.vereinName, null, null, true), feld('PLZ und Ort', f.ort),
      jahrWahl ? h('div', { class: 'field' }, h('label', null, 'Erstes Abrechnungsjahr', hilfe('Das Jahr, dessen Wasser- und Stromverbrauch auf den ersten Rechnungen steht. Die Rechnungen dafür schreibst du meist Anfang des nächsten Jahres.')), jahrWahl) : null,
      feld('Absender (steht klein über der Anschrift)', f.absenderzeile, null, null, true), feld('Bank', f.bankName), feld('IBAN', f.iban), feld('BIC', f.bic)) },
    { titel: 'Preise und Beiträge', text: 'Die Werte aus eurer letzten Abrechnung oder dem Beschluss der Mitgliederversammlung. Alles hier steht auf der Rechnung. Ein leeres Feld zählt als 0 – also: gibt es bei euch nicht.', inhalt: h('div', { class: 'form' },
      h('div', { class: 'section-title' }, 'Wasser'), feld('Grundpreis pro Jahr', f.wasserGrundpreis, '€', 'wasserGrundpreis'), feld('Preis je m³', f.wasserPreis, '€'),
      h('div', { class: 'section-title' }, 'Strom'), feld('Grundpreis pro Jahr', f.energieGrundpreis, '€', 'energieGrundpreis'), feld('Preis je kWh', f.energiePreis, '€'),
      h('div', { class: 'section-title' }, 'Pacht'), feld('Pacht je m²', f.pachtJeQm, '€', 'pachtJeQm'), feld('Anteil Vereinsfläche je Pächter', f.vereinsflaecheQm, 'm²', 'vereinsflaecheQm'),
      feld('Pachtanteil freie Gärten je Pächter', f.freieGaertenQm, 'm²', 'freieGaertenQm'),
      h('div', { class: 'section-title' }, 'Beiträge'), feld('Vereinsbeitrag', f.vereinsbeitrag, '€'), feld('Beitrag an den Verband', f.territorialverband, '€', 'territorialverband'), feld('Umlage für alle', f.umlageStandard, '€', 'umlageStandard'),
      h('div', { class: 'section-title' }, 'Gemeinschaftsstunden'), feld('Pflichtstunden', f.pflichtstunden, 'Stunden', 'pflichtstunden'), feld('Nachzahlung je fehlende Stunde', f.nachzahlungJeStd, '€'),
      feld('Vergütung je Mehrstunde', f.verguetungJeStd, '€', 'verguetungJeStd'), feld('Mehrstunden vergütet bis insgesamt', f.stundenObergrenze, 'Stunden', 'stundenObergrenze')) },
    { titel: 'Was braucht euer Verein?', text: 'Abrechnung, Pächter, Termine, Mängel und Kassenbericht sind immer dabei. Zusatzbereiche nur ankreuzen, wenn ihr sie nutzt – das lässt sich jederzeit ändern.', inhalt: h('div', { class: 'bereiche' },
      bereiche.map((b) => h('label', { class: 'bereich' }, b.cb, h('span', null, h('b', null, b.name), h('small', null, b.t)))),
      h('label', { class: 'bereich' }, sepaCb, h('span', null, h('b', null, 'Lastschrift'), h('small', null, 'SEPA-Mandate und Datei für das Online-Banking')))) },
    { titel: 'Pächter eintragen', text: 'Gespeichert! Jetzt fehlen nur noch die Pächter. Habt ihr eine Liste in Excel, übernimmt das Programm sie mit Gartennummer, Anschrift, Größe und Zählernummern.', inhalt: h('div', { class: 'assi-wahl' },
      h('button', { class: 'assi-option', onclick: () => ende(() => adminTabWaehlen('daten')) }, icon('daten', 26), h('b', null, 'Aus einer Excel-Liste übernehmen'), h('span', null, 'Unter »Import, Export, Sicherung« die Datei wählen – vor dem Übernehmen zeigt das Programm eine Vorschau.')),
      h('button', { class: 'assi-option', onclick: () => ende(() => adminTabWaehlen('paechter')) }, icon('personen', 26), h('b', null, 'Von Hand anlegen'), h('span', null, 'Für kleine Vereine oder zum Ergänzen einzelner Pächter.'))) },
  ];
  let i = 0;
  const dlg = h('dialog', { class: 'assistent', 'aria-label': 'Einrichtung' });
  const kopf = h('div', { class: 'assi-kopf' }), koerper = h('div', { class: 'assi-koerper' });
  const zurueck = h('button', { class: 'btn' }, 'Zurück'), weiter = h('button', { class: 'btn primary' });
  let beendet = false, fragt = false;
  const ende = (danach) => { if (beendet) return; beendet = true; if (dlg.open) dlg.close(); dlg.remove(); if (danach) danach(); S.aufgaben = null; reload(); };
  const zeigen = () => {
    const sch = schritte[i];
    kopf.replaceChildren(h('div', { class: 'assi-schritte' }, schritte.map((x, k) => h('span', { class: k === i ? 'an' : k < i ? 'fertig' : '' }, `${k + 1}. ${x.titel}`))),
      h('h2', null, sch.titel), h('p', { class: 'hint' }, sch.text));
    koerper.replaceChildren(sch.inhalt);
    zurueck.style.visibility = i > 0 && i < 3 ? 'visible' : 'hidden';
    weiter.textContent = i === 2 ? 'Speichern' : i === 3 ? 'Später' : 'Weiter';
    const erstes = koerper.querySelector('input');
    if (erstes) erstes.focus();
  };
  const speichern = async () => {
    const data = { ...set };
    for (const k of ['vereinName', 'ort', 'absenderzeile', 'bankName', 'iban', 'bic']) data[k] = f[k].value.trim();
    for (const k of zahlFelder) {
      const v = parseNum(f[k].value);
      if (Number.isNaN(v) || (v !== null && v < 0)) { i = 1; zeigen(); f[k].classList.add('err'); f[k].focus(); toast('Bitte eine Zahl ab 0 eintragen, z. B. 2,50 – oder das Feld leer lassen.', 'err'); return false; }
      f[k].classList.remove('err');
      data[k] = v ?? 0;
    }
    if (!data.vereinName) { i = 0; zeigen(); f.vereinName.classList.add('err'); toast('Bitte den Vereinsnamen eintragen.', 'err'); return false; }
    try {
      await api('PUT', '/api/admin/settings', data);
      // nach den Einstellungen: setzt auch Rechnungsdatum und Zahlungsziel passend zum Jahr
      if (jahrWahl) { await api('POST', '/api/admin/startjahr', { jahr: Number(jahrWahl.value) }); S.year = null; }
    } catch (e) { i = 0; zeigen(); handleErr(e); return false; }
    try {
      await api('PUT', '/api/admin/module', Object.fromEntries(bereiche.map((b) => [b.k, b.cb.checked])));
      if (sepaCb.checked !== !!st.sepaAktiv) {
        const d = await api('GET', '/api/admin/sepa');
        await api('PUT', '/api/admin/sepa/einstellungen', { ...d.einstellungen, aktiv: sepaCb.checked });
      }
    } catch (e) { handleErr(e); return false; }
    toast('Einrichtung gespeichert', 'ok');
    return true;
  };
  zurueck.addEventListener('click', () => { if (i > 0) { i--; zeigen(); } });
  weiter.addEventListener('click', async () => {
    if (i === 0 && !f.vereinName.value.trim()) { f.vereinName.classList.add('err'); f.vereinName.focus(); toast('Bitte den Vereinsnamen eintragen.', 'err'); return; }
    if (i === 2) { weiter.disabled = true; const ok = await speichern(); weiter.disabled = false; if (!ok) return; }
    if (i === 3) { ende(); return; }
    i++; zeigen();
  });
  dlg.addEventListener('cancel', async (e) => {
    if (e.target !== dlg) return;
    e.preventDefault();
    if (fragt) return;
    fragt = true;
    try {
      if (i === 3 || await confirmBox('Einrichtung abbrechen? Bisher Eingetragenes wird nicht gespeichert.', 'Einrichtung beenden')) ende();
      else if (!beendet && !dlg.open) dlg.showModal(); // der Browser hat ihn beim zweiten Esc schon geschlossen: wieder öffnen
    } finally { fragt = false; }
  });
  // Zweimal Esc hintereinander schließt den Dialog am Programm vorbei (Browser-Verhalten). Läuft gerade die Rückfrage,
  // entscheidet deren Antwort; sonst sauber aufräumen.
  dlg.addEventListener('close', () => { if (!fragt) ende(); });
  dlg.append(h('button', { class: 'gs-zu', title: 'Schließen (Esc)', onclick: () => dlg.dispatchEvent(new Event('cancel', { cancelable: true })) }, 'Schließen ✕'),
    kopf, koerper, h('div', { class: 'assi-fuss' }, zurueck, weiter));
  document.body.append(dlg);
  dlg.showModal();
  zeigen();
}

// Bereiche ein- und ausblenden. Daten ausgeblendeter Bereiche bleiben erhalten.
const BEREICHE = [
  ['warteliste', 'Warteliste', 'Interessenten für freie Gärten, Besichtigungen'],
  ['rundschreiben', 'Rundschreiben', 'Briefe und Aushänge an alle Pächter'],
  ['inventar', 'Schlüssel & Geräte', 'Schlüsselausgabe, Gerätebuch, Wartung'],
  ['bauten', 'Laubenkataster', 'Lauben und Bauten je Garten, 24-m²-Grenze'],
  ['eichung', 'Eichfristen', 'Wann Wasser- und Stromzähler getauscht werden müssen'],
  ['barkasse', 'Barkasse', 'Kassenbuch für Bargeld mit Belegnummern'],
  ['haushalt', 'Haushaltsplan', 'Plan für das Jahr, Vergleich mit dem Ist'],
  ['spenden', 'Spenden', 'Zuwendungsbestätigungen, Gemeinnützigkeit'],
  ['vorauszahlung', 'Vorauszahlungen', 'Monatliche Abschläge per Überweisung'],
  ['pachtgrenze', 'Pachtobergrenze', 'Vergleich mit der Höchstpacht nach § 5 BKleingG'],
  ['jubilaeen', 'Jubiläen', 'Runde Mitgliedsjahre, Ehrungen'],
];

function bereicheKarte(st) {
  const kaesten = BEREICHE.map(([k, name, text]) => {
    const cb = h('input', { type: 'checkbox', checked: st.module ? st.module[k] !== false : true });
    cb.addEventListener('change', async () => {
      try {
        await api('PUT', '/api/admin/module', { [k]: cb.checked });
        S.aufgaben = null;
        await reload();
      } catch (e) { handleErr(e); cb.checked = !cb.checked; }
    });
    return h('label', { class: 'bereich' }, cb, h('span', null, h('b', null, name), h('small', null, text)));
  });
  // Lastschrift hat einen eigenen Schalter; mit vorhandenen Mandaten bleibt der Bereich sichtbar.
  const sepaCb = h('input', { type: 'checkbox', checked: !!st.sepaAktiv });
  sepaCb.addEventListener('change', async () => {
    try {
      const d = await api('GET', '/api/admin/sepa');
      await api('PUT', '/api/admin/sepa/einstellungen', { ...d.einstellungen, aktiv: sepaCb.checked });
      await reload();
      if (!sepaCb.checked && S.state.sepaAktiv) toast('Es gibt noch Lastschrift-Mandate – der Bereich bleibt deshalb sichtbar.');
    } catch (e) { handleErr(e); sepaCb.checked = !sepaCb.checked; }
  });
  const sepa = h('label', { class: 'bereich' }, sepaCb, h('span', null, h('b', null, 'Lastschrift'), h('small', null, 'SEPA-Mandate und Datei für das Online-Banking')));
  return h('div', { class: 'card' },
    h('div', { class: 'fs-kopf' }, h('h3', { style: 'margin:0' }, 'Bereiche'), h('button', { class: 'btn small', onclick: einrichtungsAssistent }, 'Einrichtungs-Assistent')),
    h('p', { class: 'hint' }, 'Was euer Verein nicht braucht, lässt sich ausblenden – Menü und Aufgaben bleiben dann übersichtlich. Eingetragene Daten bleiben erhalten und sind nach dem Einschalten wieder da.'),
    h('div', { class: 'bereiche' }, kaesten, sepa));
}

// Pachtobergrenze nach § 5 BKleingG: höchstens das Vierfache der ortsüblichen Pacht im Erwerbsobstbau.
function pachtgrenzeKarte() {
  if (!modulAn('pachtgrenze')) return null;
  if (S.pachtgrenze == null) {
    if (!S.pachtgrenzeLaedt) {
      S.pachtgrenzeLaedt = true;
      api('GET', '/api/admin/pachtgrenze').then((d) => { S.pachtgrenze = d; }).catch(() => { S.pachtgrenze = false; }).finally(() => { S.pachtgrenzeLaedt = false; render(); });
    }
    return null;
  }
  const d = S.pachtgrenze;
  if (!d) return null;
  const wert = h('input', { type: 'text', inputmode: 'decimal', value: d.erwerbsobstbauJeQm ? numIn(d.erwerbsobstbauJeQm) : '', placeholder: 'z. B. 0,03', autocomplete: 'off' });
  const quelle = h('input', { type: 'text', maxlength: 120, value: d.quelle || '', placeholder: 'z. B. Gutachterausschuss 2025' });
  const speichern = async () => {
    const v = wert.value.trim() === '' ? 0 : parseNum(wert.value);
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte eine Zahl ab 0 eintragen.', 'err'); return; }
    try { S.pachtgrenze = await api('PUT', '/api/admin/pachtgrenze', { erwerbsobstbauJeQm: v, quelle: quelle.value }); S.aufgaben = null; toast('Gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };
  const fmt4 = (x) => new Intl.NumberFormat('de-DE', { maximumFractionDigits: 4 }).format(x);
  let ergebnis = h('p', { class: 'hint' }, 'Trag die ortsübliche Pacht ein – das Programm rechnet die Höchstgrenze aus und vergleicht sie mit eurem Pachtsatz.');
  if (d.hoechst != null) {
    const pct = Math.min(100, d.auslastung);
    ergebnis = h('div', { class: 'pg-ergebnis' },
      h('div', { class: 'pg-zahlen' }, h('span', null, 'Euer Pachtsatz ', h('b', null, fmt4(d.pachtJeQm) + ' €/m²')), h('span', null, 'Höchstpacht ', h('b', null, fmt4(d.hoechst) + ' €/m²'))),
      h('div', { class: 'balken' + (d.ueber ? ' err' : '') }, h('div', { style: `width:${pct}%` })),
      h('p', { class: d.ueber ? 'warn-text' : 'hint' }, d.ueber
        ? `Der Pachtsatz liegt ${nfFlex.format(Math.round((d.auslastung - 100) * 10) / 10)} % über der gesetzlichen Höchstgrenze. Bitte mit dem Verband klären.`
        : `Ihr nutzt ${nfFlex.format(d.auslastung)} % der zulässigen Pacht – bis zur Grenze sind noch ${fmt4(Math.round((d.hoechst - d.pachtJeQm) * 10000) / 10000)} €/m² Luft.`));
  }
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'section-title', style: 'margin-top:0' }, 'Pachtobergrenze (§ 5 Bundeskleingartengesetz)', hilfe('Die Pacht für Kleingärten darf höchstens das Vierfache der ortsüblichen Pacht im erwerbsmäßigen Obst- und Gemüseanbau betragen. Den Wert erfahrt ihr beim Gutachterausschuss, der Gemeinde oder eurem Verband.')),
    h('p', { class: 'hint' }, 'Die Pacht darf höchstens das Vierfache der ortsüblichen Pacht im erwerbsmäßigen Obst- und Gemüseanbau betragen. Den Wert nennt der Gutachterausschuss der Gemeinde oder der Verband. Gemeinschaftsflächen dürfen anteilig mit umgelegt werden.'),
    h('div', { class: 'form' },
      h('div', { class: 'field' }, h('label', null, 'Ortsübliche Pacht Erwerbsobstbau'), h('div', { class: 'inputunit' }, wert, h('span', null, '€/m²'))),
      h('div', { class: 'field' }, h('label', null, 'Quelle'), quelle)),
    h('div', { class: 'actions' }, h('button', { class: 'btn', onclick: speichern }, 'Übernehmen')),
    ergebnis);
}

// ---- Aussehen: Vereinsfarbe und Logo (Programm und Rechnung)

const FARBVORSCHLAEGE = [['#2f6f3e', 'Gartengrün'], ['#1e5aa8', 'Blau'], ['#0f6e6e', 'Petrol'], ['#8c2f39', 'Weinrot'], ['#7a4b1f', 'Erdbraun'], ['#3d4752', 'Anthrazit']];

// Erstes Abrechnungsjahr wechseln – nur solange noch keine Rechnung und keine Kassenbuchung existiert.
function startjahrZeile(st) {
  if (!st.startjahrAenderbar) return null;
  const jetzt = new Date().getFullYear();
  const jahre = [...new Set([jetzt - 1, jetzt, st.currentYear])].sort();
  const wahl = h('select', { style: 'width:auto' }, jahre.map((j) => h('option', { value: j, selected: j === st.currentYear }, String(j))));
  return h('div', { class: 'toolbar', style: 'margin:0 0 12px' },
    h('span', null, 'Erstes Abrechnungsjahr', hilfe('Das Jahr, dessen Wasser- und Stromverbrauch auf euren ersten Rechnungen steht. Wechseln geht nur, solange noch keine Rechnung ausgestellt ist. Eingetragene Zählerstände wandern mit.')),
    wahl, h('button', { class: 'btn small', onclick: async () => {
      try {
        const r = await api('POST', '/api/admin/startjahr', { jahr: Number(wahl.value) });
        S.year = null; toast(`Abrechnungsjahr ${r.jahr} – Rechnungsdatum ${r.rechnungsdatum.split('-').reverse().join('.')}`, 'ok'); await reload();
      } catch (e) { handleErr(e); }
    } }, 'Übernehmen'));
}

// ---- Fristen aus der Satzung: Einladungsfrist (Cockpit rechnet den letzten Tag aus) und Kündigungsfrist
// (dieselbe Regel wie auf der Seite Kündigungen). Tag.Monat wird als MM-TT gespeichert.
const tagMonatAnzeige = (mmtt) => (/^\d\d-\d\d$/.test(mmtt || '') ? `${mmtt.slice(3)}.${mmtt.slice(0, 2)}.` : '');
function tagMonatLesen(text) {
  const m = String(text || '').trim().match(/^(\d{1,2})\.(\d{1,2})\.?$/);
  return m ? `${m[2].padStart(2, '0')}-${m[1].padStart(2, '0')}` : null;
}
function fristenKarte() {
  if (S.satzungsfristen == null) {
    if (!S.satzungsfristenLaedt) {
      S.satzungsfristenLaedt = true;
      api('GET', '/api/admin/satzungsfristen').then((d) => { S.satzungsfristen = d; }).catch((e) => { handleErr(e); S.satzungsfristen = false; })
        .finally(() => { S.satzungsfristenLaedt = false; render(); });
    }
    return null;
  }
  if (!S.satzungsfristen) return null;
  const f = S.satzungsfristen;
  const tage = h('input', { type: 'number', min: 1, max: 90, value: f.einladungTage, style: 'width:6em' });
  const bis = h('input', { type: 'text', value: tagMonatAnzeige(f.kuendigung.bis), placeholder: 'z. B. 31.08.', style: 'width:7em' });
  const zum = h('input', { type: 'text', value: tagMonatAnzeige(f.kuendigung.zum), placeholder: 'z. B. 30.11.', style: 'width:7em' });
  const speichern = async () => {
    const t = Math.round(Number(tage.value));
    if (!(t >= 1 && t <= 90)) return feldFehler(tage, 'Die Einladungsfrist muss zwischen 1 und 90 Tagen liegen.');
    const b = tagMonatLesen(bis.value), z = tagMonatLesen(zum.value);
    if (!b) return feldFehler(bis, 'Bitte Tag und Monat schreiben, z. B. 31.08.');
    if (!z) return feldFehler(zum, 'Bitte Tag und Monat schreiben, z. B. 30.11.');
    try {
      await api('PUT', '/api/admin/satzungsfristen', { einladungTage: t });
      await api('PUT', '/api/admin/kuendigungen/regel', { zum: z, bis: b });
      toast('Fristen gespeichert', 'ok');
      S.satzungsfristen = null; S.kuend = null; S.aufgaben = null; render();
    } catch (e) { handleErr(e); }
  };
  const feld = (label, input, nach, key) => h('div', { class: 'field' }, h('label', null, label, hilfe(HILFE[key])), h('div', { class: 'toolbar', style: 'margin:0' }, input, h('span', { class: 'hint' }, nach)));
  return h('div', { class: 'card' },
    h('h3', null, 'Fristen aus der Satzung'),
    h('p', { class: 'hint' }, 'Trage hier die Fristen aus eurer Satzung und dem Pachtvertrag ein. Das Cockpit rechnet daraus aus, bis wann etwas raus muss – zum Beispiel »Einladung bis 3. März verschicken«.'),
    h('div', { class: 'form' },
      feld('Einladung zur Mitgliederversammlung', tage, 'Tage vorher verschicken', 'einladungTage'),
      feld('Kündigung muss eingegangen sein bis', bis, 'jedes Jahr', 'kuendigungBis'),
      feld('Pachtende bei Kündigung', zum, 'jedes Jahr', 'kuendigungZum')),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: speichern }, 'Fristen speichern')));
}

// ---- Brieftexte: eigene Fassungen der festen Sätze in Briefen (leer = Standard des Programms)
function brieftexteKarte() {
  if (S.brieftexte == null) {
    if (!S.brieftexteLaedt) {
      S.brieftexteLaedt = true;
      api('GET', '/api/admin/brieftexte').then((d) => { S.brieftexte = d; }).catch((e) => { handleErr(e); S.brieftexte = []; })
        .finally(() => { S.brieftexteLaedt = false; render(); });
    }
    return null;
  }
  const gruppen = [...new Set(S.brieftexte.map((b) => b.gruppe))];
  const kurz = (t) => { const x = (t || '').replace(/\s+/g, ' ').trim(); return x ? (x.length > 110 ? x.slice(0, 110) + ' …' : x) : '(kein Zusatz)'; };
  return h('div', { class: 'card' },
    h('h3', null, 'Brieftexte'),
    h('p', { class: 'hint' }, 'Hier schreibst du die festen Sätze der Briefe so, wie euer Verein spricht. Namen, Daten und Beträge setzt das Programm selbst ein. Ein geänderter Text gilt ab sofort für alle neuen Briefe; schon ausgestellte Rechnungen bleiben, wie sie sind.'),
    gruppen.map((g) => h('div', { class: 'brieftext-gruppe' },
      h('h4', null, g),
      S.brieftexte.filter((b) => b.gruppe === g).map((b) => h('div', { class: 'brieftext-zeile' },
        h('div', null, h('b', null, b.titel), b.angepasst ? h('span', { class: 'badge ok', style: 'margin-left:8px' }, 'eigener Text') : null,
          h('div', { class: 'hint' }, kurz(b.text))),
        h('button', { class: 'btn small', onclick: () => brieftextDialog(b) }, 'Bearbeiten'))))));
}

async function brieftextDialog(b) {
  const lang = b.max > 400;
  const feld = h('textarea', { rows: lang ? 10 : 3, maxlength: b.max, style: 'width:100%' });
  feld.value = b.text;
  const zaehler = h('span', { class: 'hint' });
  const zaehle = () => { zaehler.textContent = `${feld.value.length} von höchstens ${b.max} Zeichen`; };
  feld.addEventListener('input', zaehle);
  zaehle();
  const einsetzen = (p) => {
    const a = feld.selectionStart ?? feld.value.length, e = feld.selectionEnd ?? a;
    feld.value = feld.value.slice(0, a) + `{${p}}` + feld.value.slice(e);
    feld.focus(); feld.selectionStart = feld.selectionEnd = a + p.length + 2;
    zaehle();
  };
  const ok = await modal(`Brieftext: ${b.titel}`, h('div', { class: 'form' },
    b.hilfe ? h('p', { class: 'hint' }, b.hilfe) : null,
    b.platzhalter.length ? h('div', { class: 'chips' }, h('span', { class: 'hint' }, 'Einsetzen:'),
      b.platzhalter.map((p) => h('button', { class: 'btn small', type: 'button', title: 'Wird im Brief durch die passende Angabe ersetzt', onclick: () => einsetzen(p) }, `{${p}}`))) : null,
    feld, zaehler,
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn small', type: 'button', title: 'Zeigt den Brief mit diesem Text und erfundenen Beispieldaten – gespeichert wird dabei nichts', onclick: () => pdfDrucken({
        url: `/api/admin/brieftexte/${encodeURIComponent(b.key)}/probe`, oeffnenUrl: `/api/admin/brieftexte/${encodeURIComponent(b.key)}/probe/oeffnen`,
        titel: 'Probebrief', meldung: () => 'Der Probebrief wurde im PDF-Programm geöffnet.', body: { text: feld.value } }) }, 'Probebrief ansehen'),
      h('button', { class: 'btn small', type: 'button', onclick: () => { feld.value = b.standard; zaehle(); feld.dispatchEvent(new Event('input')); } }, 'Standardtext einsetzen'))),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('PUT', `/api/admin/brieftexte/${encodeURIComponent(b.key)}`, { text: feld.value }); } catch (e) { handleErr(e); return false; }
  } }]);
  if (ok) { S.brieftexte = null; toast(feld.value.trim() === '' || feld.value.trim() === b.standard.trim() ? 'Standardtext gilt wieder' : 'Brieftext gespeichert', 'ok'); render(); }
}

function aussehenKarte(st) {
  const aktuell = st.settings.farbe || '#2f6f3e';
  const speichereFarbe = async (hex) => {
    try { await api('PUT', '/api/admin/settings', { ...st.settings, farbe: hex === '#2f6f3e' ? '' : hex }); toast('Vereinsfarbe gespeichert', 'ok'); await reload(); } catch (e) { handleErr(e); }
  };
  const waehler = h('input', { type: 'color', value: aktuell, title: 'Eigene Farbe wählen',
    oninput: (e) => farbeAnwenden(e.target.value), onchange: (e) => speichereFarbe(e.target.value) });
  const datei = h('input', { type: 'file', accept: '.png,.jpg,.jpeg', style: 'display:none', onchange: async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    try { await api('POST', '/api/admin/logo', fd, true); toast('Logo gespeichert – es erscheint auf allen neuen Rechnungen', 'ok'); await reload(); } catch (e) { handleErr(e); }
  } });
  return h('div', { class: 'card', style: 'margin-bottom:16px' },
    h('h3', null, 'Aussehen: Vereinsfarbe und Logo'),
    h('p', { class: 'hint' }, 'Farbe und Logo erscheinen im Programm und auf allen neu erzeugten Rechnungen und Mahnungen. Bereits ausgestellte Rechnungen bleiben, wie sie sind. Zu helle Farben werden automatisch so abgedunkelt, dass alles gut lesbar bleibt.'),
    h('div', { class: 'aussehen' },
      h('div', null, h('div', { class: 'form' }, h('label', null, 'Vereinsfarbe')),
        h('div', { class: 'farbwahl', style: 'margin-top:6px' },
          FARBVORSCHLAEGE.map(([hex, name]) => h('button', { class: 'swatch' + (hex === aktuell ? ' an' : ''), title: name, 'aria-label': name,
            style: `background:${hex}`, onclick: () => speichereFarbe(hex) })),
          waehler, h('span', { class: 'hint' }, 'eigene Farbe'))),
      h('div', null, h('div', { class: 'form' }, h('label', null, 'Vereinslogo')),
        h('div', { class: 'toolbar', style: 'margin-top:6px' },
          h('div', { class: 'logovorschau' }, st.settings.logo ? h('img', { src: `/api/logo?v=${encodeURIComponent(st.settings.logo)}`, alt: 'Vereinslogo' }) : h('span', { class: 'hint' }, 'kein Logo')),
          h('div', null, datei,
            h('button', { class: 'btn', onclick: () => datei.click() }, st.settings.logo ? 'Anderes Logo wählen' : 'Logo hochladen'), ' ',
            st.settings.logo ? h('button', { class: 'btn danger', onclick: async () => {
              try { await api('DELETE', '/api/admin/logo'); toast('Logo entfernt', 'ok'); await reload(); } catch (e) { handleErr(e); }
            } }, 'Entfernen') : null,
            h('div', { class: 'hint', style: 'margin-top:6px' }, 'PNG oder JPG, am besten mit hellem oder durchsichtigem Hintergrund.'))))));
}

// ---- Jahreswechsel

// Ampel vor dem Jahreswechsel (abschluss.go): Rot sollte vorher erledigt sein, Gelb bitte ansehen. Blockiert nichts.
function abschlussPruefung() {
  if (S.abschluss == null) {
    if (!S.abschlussLaedt) {
      S.abschlussLaedt = true;
      api('GET', '/api/admin/abschlusspruefung').then((d) => { S.abschluss = d; }).catch(() => { S.abschluss = false; }).finally(() => { S.abschlussLaedt = false; render(); });
    }
    return h('div', { class: 'card narrow', style: 'max-width:760px;margin-bottom:16px' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geprüft …'));
  }
  const d = S.abschluss;
  if (!d) return null;
  const gesamt = d.rot ? ['err', `${d.rot} ${d.rot === 1 ? 'Punkt sollte' : 'Punkte sollten'} vor dem Abschluss erledigt sein`]
    : d.gelb ? ['warn', `Abschluss möglich – ${d.gelb} ${d.gelb === 1 ? 'Hinweis' : 'Hinweise'} bitte ansehen`] : ['ok', 'Alles in Ordnung – das Jahr kann abgeschlossen werden'];
  return h('div', { class: 'card narrow abschluss', style: 'max-width:760px;margin-bottom:16px' },
    h('div', { class: 'fs-kopf' }, h('h3', { style: 'margin:0' }, `Prüfung vor dem Abschluss ${d.jahr}`),
      h('button', { class: 'btn small', onclick: () => { S.abschluss = null; render(); } }, 'Neu prüfen')),
    h('div', { class: 'ampel-gesamt ' + gesamt[0] }, h('i', null), gesamt[1]),
    (() => {
      const n = d.punkte.find((x) => x.stufe === 'err' && x.ziel) || d.punkte.find((x) => x.stufe === 'warn' && x.ziel);
      return n ? h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => zielOeffnen(n.ziel) }, `Nächster Schritt: ${n.titel} →`)) : null;
    })(),
    h('ul', { class: 'ampel-liste' }, d.punkte.map((x) => h('li', null,
      h('i', { class: 'ampel ' + x.stufe, title: x.stufe === 'err' ? 'Vorher erledigen' : x.stufe === 'warn' ? 'Bitte ansehen' : 'In Ordnung' }),
      h('div', { class: 'ampel-text' }, h('b', null, x.titel), x.text ? h('div', { class: 'hint' }, x.text) : null),
      x.ziel ? h('button', { class: 'btn small', onclick: () => zielOeffnen(x.ziel) }, x.knopf + ' →') : null))),
    h('p', { class: 'hint', style: 'margin-bottom:0' }, 'Die Prüfung ist ein Hinweis: Der Jahreswechsel lässt sich trotzdem durchführen, offene Punkte bleiben danach bearbeitbar.'));
}

function adminYear() {
  const st = S.state;
  return h('div', null,
    abschlussPruefung(),
    h('div', { class: 'card narrow', style: 'max-width:760px' },
      h('h3', { style: 'margin-top:0' }, `Abrechnungsjahr ${st.currentYear} abschließen`),
      h('p', null, 'Am Ende der Abrechnung schließt du das Jahr ab. Dabei passiert Folgendes:'),
      h('ul', { class: 'plain' },
        h('li', null, `Das Jahr ${st.currentYear} wird eingefroren. Preise und Pächterdaten bleiben so gespeichert, dass du alte Rechnungen jederzeit unverändert neu drucken kannst.`),
        h('li', null, `Es startet das Jahr ${st.currentYear + 1}: Die aktuellen Zählerstände werden zu den Vorjahresständen. Stunden, Abschlagszahlungen und weitere Angaben sind wieder leer.`),
        h('li', null, 'Rechnungsdatum und Zahlungsziel rücken um ein Jahr weiter. Prüfe sie danach unter »Preise & Einstellungen«.'),
        h('li', null, 'Vorher wird automatisch eine Sicherung angelegt.')),
      h('p', { class: 'hint', style: 'margin-top:12px' }, 'Tipp: Vor dem Abschluss den ',
        druckKnopf({ url: `/api/admin/jahresbericht?year=${st.currentYear}`, oeffnenUrl: `/api/admin/jahresbericht/oeffnen?year=${st.currentYear}`, titel: `Jahresbericht ${st.currentYear}` }, { label: `Jahresbericht ${st.currentYear}`, klein: true }),
        ' für die Mitgliederversammlung drucken (später geht das auch noch im Kassenbericht).'),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        let offen = [];
        try { S.abschluss = await api('GET', '/api/admin/abschlusspruefung'); offen = S.abschluss.punkte.filter((x) => x.stufe === 'err').map((x) => x.titel); } catch (e) { /* Prüfung ist nur ein Hinweis */ }
        const warnung = offen.length ? ` Achtung, noch offen: ${offen.join('; ')}.` : '';
        if (!(await confirmBox(`Jahr ${st.currentYear} jetzt abschließen und ${st.currentYear + 1} beginnen?${warnung} Das kann nicht rückgängig gemacht werden (die Sicherung vorher bleibt aber erhalten).`, 'Jahr abschließen', true))) return;
        try { await api('POST', '/api/admin/jahreswechsel'); toast('Neues Jahr gestartet', 'ok'); S.year = null; S.dashKasse = null; S.kasse = null; S.kasseVerlauf = null; S.abschluss = null; S.vj = null; await reload(); } catch (e) { handleErr(e); }
      } }, `Jahr ${st.currentYear} abschließen`))));
}

// ---- Kassenbericht

async function loadKasse(year) {
  if (S.kasseLoading) return;
  S.kasseLoading = true;
  try { S.kasse = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { handleErr(e); }
  S.kasseLoading = false;
  render();
}

async function refreshKasse() { S.kasse = null; S.kasseVerlauf = null; await loadKasse(S.kasseYear); }

async function loadKasseVerlauf() {
  if (S.kasseVerlaufLoading) return;
  S.kasseVerlaufLoading = true;
  try { S.kasseVerlauf = await api('GET', '/api/admin/kassenbericht-verlauf'); } catch (e) { handleErr(e); }
  S.kasseVerlaufLoading = false;
  render();
}

const AUSGABEN_KATEGORIEN = ['Instandhaltung', 'Anschaffung', 'Verwaltung', 'Versicherung & Gebühren', 'Sonstiges'];

// Neue Ausgaben nacheinander erfassen: »Speichern und nächste« öffnet den Dialog erneut, Datum und Kategorie bleiben stehen.
// Nachbuchen in einem abgeschlossenen Jahr: einmal nachfragen (je Jahr bis zum Neuladen). Späte Zahlungen und
// nachgereichte Rechnungen sind normal; der Anfangsbestand des Folgejahres passt sich automatisch an.
async function altesJahrFreigabe(jahr) {
  if (!S.state || jahr >= S.state.currentYear || S.altesJahrFrei === jahr) return true;
  const ok = await confirmBox(`Das Jahr ${jahr} ist schon abgeschlossen. Trotzdem hier nachbuchen? Der Kassenbestand Ende ${jahr} ändert sich dadurch. Der Anfangsbestand ${jahr + 1} passt sich automatisch an, wenn ihn niemand von Hand geändert hat.`, 'Nachbuchen');
  if (ok) S.altesJahrFrei = jahr;
  return ok;
}

async function ausgabeDialog(a) {
  if (!(await altesJahrFreigabe(S.kasseYear))) return false;
  let vorlage = null, r;
  if (!a) { try { ausgabeDialogEinmal.vorlagen = await api('GET', '/api/admin/kassenvorlagen'); } catch (e) { ausgabeDialogEinmal.vorlagen = []; } }
  do {
    r = await ausgabeDialogEinmal(a, vorlage);
    vorlage = r === 'weiter' ? ausgabeDialogEinmal.letzte : null;
    if (r === 'weiter') await refreshKasse();
  } while (r === 'weiter');
  return r;
}

function ausgabeDialogEinmal(a, vorlage) {
  const isNew = !a;
  const datum = h('input', { type: 'date', value: (a && a.datum) || (vorlage && vorlage.datum) || todayIso() });
  const besch = h('input', { type: 'text', maxlength: 120, value: (a && a.beschreibung) || '', autocomplete: 'off' });
  const kategorie = h('input', { type: 'text', list: 'kategorien-liste', maxlength: 40, value: (a && a.kategorie) || (vorlage && vorlage.kategorie) || '', placeholder: AUSGABEN_KATEGORIEN[AUSGABEN_KATEGORIEN.length - 1], autocomplete: 'off' });
  const katList = h('datalist', { id: 'kategorien-liste' }, AUSGABEN_KATEGORIEN.map((x) => h('option', { value: x })));
  const betrag = h('input', { type: 'text', inputmode: 'decimal', value: a ? numIn(a.betrag) : '', autocomplete: 'off' });
  const wiederkehrend = h('input', { type: 'checkbox', checked: !!(a && a.wiederkehrend) });
  const alsVorlage = h('input', { type: 'checkbox' });
  const fileInput = h('input', { type: 'file', accept: '.jpg,.jpeg,.png,.webp,.pdf', multiple: true });
  // Vorlagen für wiederkehrende Ausgaben: ein Klick füllt Beschreibung, Kategorie und Betrag.
  let vorlagenListe = isNew ? (ausgabeDialogEinmal.vorlagen || []) : [];
  const vorlagenBox = h('div', { class: 'vorlagen' });
  const zeichneVorlagen = () => {
    vorlagenBox.replaceChildren(...(vorlagenListe.length ? [h('span', { class: 'hint' }, 'Vorlage einsetzen: '), ...vorlagenListe.map((v) => h('span', { class: 'chip vorlage' },
      h('button', { type: 'button', class: 'btn small', title: `${v.kategorie || 'Sonstiges'} · ${eur(v.betrag)}`, onclick: () => {
        besch.value = v.beschreibung; kategorie.value = v.kategorie || ''; betrag.value = v.betrag ? numIn(v.betrag) : '';
        for (const el of [besch, kategorie, betrag]) el.dispatchEvent(new Event('input', { bubbles: true }));
        (v.betrag ? datum : betrag).focus();
      } }, `${v.beschreibung}${v.betrag ? ' · ' + eur(v.betrag) : ''}`),
      h('button', { type: 'button', class: 'btn small danger', title: 'Vorlage löschen', 'aria-label': `Vorlage ${v.beschreibung} löschen`, onclick: async () => {
        try {
          vorlagenListe = await api('PUT', '/api/admin/kassenvorlagen', vorlagenListe.filter((x) => x.id !== v.id));
          ausgabeDialogEinmal.vorlagen = vorlagenListe; zeichneVorlagen();
        } catch (e) { handleErr(e); }
      } }, '×')))] : []));
  };
  zeichneVorlagen();
  const fld = (label, input) => h('div', { class: 'field' }, h('label', null, label), input);

  // Vorhandene Belege: Liste mit eigenem Entfernen-Knopf je Beleg, aktualisiert
  // sich selbst ohne den Dialog zu schließen.
  const belegeBox = h('div', { style: 'display:flex;flex-wrap:wrap;gap:8px;margin:4px 0' });
  let belegeAktuell = (a && a.belege) || [];
  const zeichneBelege = () => {
    belegeBox.replaceChildren();
    if (!belegeAktuell.length) { belegeBox.append(h('span', { class: 'hint' }, 'Noch keine Belege hinterlegt.')); return; }
    belegeAktuell.forEach((_, i) => {
      belegeBox.append(h('span', { class: 'chip' },
        h('a', { href: `/api/admin/beleg/${a.id}?year=${S.kasseYear}&idx=${i}`, target: '_blank' }, `Beleg ${i + 1}`), ' ',
        h('button', { class: 'btn small danger', type: 'button', title: 'Beleg entfernen', onclick: async () => {
          const ok = await confirmBox(`Beleg ${i + 1} wirklich entfernen?`, 'Entfernen', true);
          if (!ok) return;
          try {
            await api('DELETE', `/api/admin/ausgaben/${a.id}/beleg?year=${S.kasseYear}&idx=${i}`);
            belegeAktuell = belegeAktuell.filter((_, j) => j !== i);
            zeichneBelege();
            // Tabelle hinter dem Dialog sofort aktualisieren, damit sie nicht
            // veraltet bleibt, falls danach "Abbrechen" statt "Speichern" kommt
            // (das Entfernen ist bereits endgültig passiert, nicht Teil des
            // Speichern-Schritts).
            await refreshKasse();
          } catch (e) { handleErr(e); }
        } }, '×')));
    });
  };
  if (!isNew) zeichneBelege();

  const body = h('div', { class: 'form' },
    isNew && vorlagenListe.length ? h('div', { class: 'field wide', style: 'border:none' }, vorlagenBox) : null,
    fld('Datum', datum), fld('Beschreibung (z. B. Rasenmäher, Kontoführungsgebühren)', besch),
    fld('Kategorie', h('div', null, kategorie, katList)),
    h('div', { class: 'field' }, h('label', null, 'Betrag'), h('div', { class: 'inputunit' }, betrag, h('span', null, '€'))),
    isNew ? h('div', { class: 'field wide', style: 'border:none' },
      h('label', { class: 'check' }, alsVorlage, ' Als Vorlage merken'),
      h('span', { class: 'fhint' }, 'Beim nächsten Mal genügt ein Klick auf die Vorlage – Datum und Beleg gibst du dann nur noch neu an.')) : null,
    h('div', { class: 'field wide', style: 'border:none' },
      h('label', { class: 'check' }, wiederkehrend, ' Jährlich wiederkehrend'),
      h('span', { class: 'fhint' }, 'Wird beim Jahreswechsel automatisch als Vorschlag fürs neue Jahr angelegt (gleiche Beschreibung, Kategorie und Betrag, ungeprüft, ohne Beleg) – z. B. für Kontoführungsgebühren. Lässt sich dort weiterhin anpassen oder löschen.')),
    h('div', { class: 'field wide' }, h('label', null, 'Belege (Fotos oder PDF, z. B. Vorder- und Rückseite, optional)'),
      isNew ? null : belegeBox,
      fileInput,
      isNew ? h('span', { class: 'fhint' }, 'Werden nach dem Anlegen hochgeladen.') : null));
  const speichern = async () => {
      const b = parseNum(betrag.value);
      if (!datum.value) return feldFehler(datum, 'Bitte ein Datum angeben.');
      if (!besch.value.trim()) return feldFehler(besch, 'Bitte eine Beschreibung eintragen, z. B. Rasenmäher.');
      if (b === null || Number.isNaN(b) || b <= 0) return feldFehler(betrag, 'Bitte einen Betrag größer 0 eintragen, z. B. 49,90.');
      const data = { datum: datum.value, beschreibung: besch.value.trim(), kategorie: kategorie.value.trim(), betrag: b, wiederkehrend: wiederkehrend.checked };
      try {
        const saved = isNew ? await api('POST', `/api/admin/ausgaben?year=${S.kasseYear}`, data) : await api('PUT', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`, data);
        for (const file of fileInput.files) {
          const fd = new FormData();
          fd.append('file', file);
          await api('POST', `/api/admin/ausgaben/${saved.id}/beleg?year=${S.kasseYear}`, fd, true);
        }
      } catch (e) { handleErr(e); return false; }
      if (isNew && alsVorlage.checked) {
        try {
          const neu = { beschreibung: data.beschreibung, kategorie: data.kategorie, betrag: data.betrag };
          const rest = (ausgabeDialogEinmal.vorlagen || []).filter((x) => x.beschreibung !== neu.beschreibung);
          ausgabeDialogEinmal.vorlagen = await api('PUT', '/api/admin/kassenvorlagen', [...rest, neu]);
        } catch (e) { handleErr(e); }
      }
      ausgabeDialogEinmal.letzte = { datum: datum.value, kategorie: kategorie.value.trim() };
  };
  return modal(isNew ? 'Ausgabe erfassen' : `Ausgabe bearbeiten – ${a.beschreibung}`, body, [
    { label: 'Abbrechen', value: false },
    isNew ? null : { label: 'Löschen', cls: 'danger', value: 'del', action: async () => {
      const ok = await confirmBox(`Ausgabe „${a.beschreibung}“ (${eur(a.betrag)}) wirklich löschen?`, 'Löschen', true);
      if (!ok) return false;
      try { await api('DELETE', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`); } catch (e) { handleErr(e); return false; }
    } },
    isNew ? { label: 'Speichern und nächste', value: 'weiter', action: speichern } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: speichern },
  ].filter(Boolean));
}

// Hinweis, wenn sich der Kassenbestand des Vorjahres nach dem Jahreswechsel noch geändert hat (z. B. eine
// Vorjahresrechnung wurde erst jetzt bezahlt): sonst fehlt dieses Geld im Kassenbestand des laufenden Jahres.
function anfangsbestandHinweis(k, pruefMode) {
  const vj = k.vorjahrEndbestand;
  if (vj == null || Math.abs(vj - k.anfangsbestand) < 0.005) return null;
  const text = `Der Kassenbestand Ende ${k.jahr - 1} beträgt inzwischen ${eur(vj)}, der Anfangsbestand ${k.jahr} ist ${eur(k.anfangsbestand)} (Unterschied ${eur(vj - k.anfangsbestand)}). Das passiert zum Beispiel, wenn eine Vorjahresrechnung erst nach dem Jahreswechsel bezahlt oder eine Ausgabe nachgetragen wurde.`;
  if (pruefMode || vj < 0) return h('div', { class: 'banner warn' }, text);
  return h('div', { class: 'banner warn' }, text, ' ',
    h('button', { class: 'btn small', onclick: async () => {
      try { S.kasse = await api('PUT', `/api/admin/anfangsbestand?year=${k.jahr}`, { betrag: vj, automatisch: true }); S.kasseVerlauf = null; S.dashKasse = null; toast(`Anfangsbestand auf ${eur(vj)} gesetzt`, 'ok'); render(); } catch (e) { handleErr(e); }
    } }, `Anfangsbestand auf ${eur(vj)} setzen`));
}

// ---- Barkasse: Kassenbuch für Bargeld mit fortlaufenden Belegnummern

async function barbuchungDialog(b) {
  if (!(await altesJahrFreigabe(S.kasseYear))) return false;
  let vorlage = null, r, gespeichert = false;
  do {
    r = await barbuchungDialogEinmal(b, vorlage);
    if (r) gespeichert = true;
    vorlage = r === 'weiter' ? barbuchungDialogEinmal.letzt : null;
  } while (r === 'weiter');
  return gespeichert;
}

function barbuchungDialogEinmal(b, vorlage) {
  const jahr = S.kasseYear;
  const ausgabe = b ? b.betrag < 0 : !(vorlage && vorlage.art === 'ein');
  const f = {
    datum: h('input', { type: 'date', value: b ? b.datum : (vorlage && vorlage.datum) || todayIso() }),
    art: h('select', null, h('option', { value: 'aus', selected: ausgabe }, 'Ausgabe'), h('option', { value: 'ein', selected: !ausgabe }, 'Einnahme')),
    betrag: h('input', { type: 'text', inputmode: 'decimal', value: b ? numIn(Math.abs(b.betrag)) : '', style: 'width:8em' }),
    text: h('input', { type: 'text', maxlength: 200, value: b ? b.text : '', placeholder: 'z. B. Sommerfest: Getränkeverkauf' }),
    kat: h('input', { type: 'text', maxlength: 60, value: b ? b.kategorie || '' : (vorlage && vorlage.kat) || '', list: 'bar-kategorien', placeholder: 'z. B. Feste' }),
  };
  const fld = (label, el, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el);
  const speichern = async () => {
    const v = parseNum(f.betrag.value);
    if (v == null || Number.isNaN(v) || v <= 0) return feldFehler(f.betrag, 'Bitte einen Betrag größer 0 eintragen, z. B. 12,50.');
    if (!f.text.value.trim()) return feldFehler(f.text, 'Bitte einen Buchungstext eintragen.');
    try {
      S.barkasse = await api('POST', `/api/admin/barkasse?year=${jahr}`, { id: b ? b.id : '', datum: f.datum.value, text: f.text.value, kategorie: f.kat.value, betrag: f.art.value === 'aus' ? -v : v });
      barbuchungDialogEinmal.letzt = { datum: f.datum.value, art: f.art.value, kat: f.kat.value };
    } catch (e) { handleErr(e); return false; }
  };
  return modal(b ? `Beleg ${b.belegNr} bearbeiten` : 'Barkasse: Buchung', h('div', { class: 'form' },
    h('datalist', { id: 'bar-kategorien' }, ['Feste', 'Instandhaltung', 'Anschaffung', 'Verwaltung', 'Pfand', 'Umbuchung'].map((k) => h('option', { value: k }))),
    fld('Datum', f.datum), fld('Art', f.art), fld('Betrag in €', h('div', { class: 'inputunit' }, f.betrag, h('span', null, '€'))), fld('Kategorie', f.kat),
    fld('Buchungstext *', f.text, true)),
  [{ label: 'Abbrechen', value: false },
    b ? null : { label: 'Speichern und nächste', value: 'weiter', action: speichern },
    { label: 'Speichern', cls: 'primary', value: true, action: speichern }].filter(Boolean));
}

function barkasseKarte(pruefMode) {
  if (!modulAn('barkasse')) return null;
  const jahr = S.kasseYear;
  if (!S.barkasse || S.barkasse.jahr !== jahr) {
    if (!S.barkasseLaedt) {
      S.barkasseLaedt = true;
      api('GET', `/api/admin/barkasse?year=${jahr}`).then((d) => { S.barkasse = d; }).catch(() => { S.barkasse = { jahr, fehler: true }; })
        .finally(() => { S.barkasseLaedt = false; render(); });
    }
    return null;
  }
  const d = S.barkasse;
  if (d.fehler) return null;
  const nachher = async (p) => { if (await p) { S.pruefung = null; S.aufgaben = null; render(); } };
  const anfang = async () => {
    const inp = h('input', { type: 'text', inputmode: 'decimal', value: d.anfangVorjahr ? '' : numIn(d.anfang), placeholder: d.anfangVorjahr ? `aus dem Vorjahr: ${numIn(d.anfang)}` : '' });
    const ok = await modal('Anfangsbestand der Barkasse', h('div', null, h('p', { class: 'hint' }, 'Bargeld am 1. Januar. Leer lassen, um den Endbestand des Vorjahres zu übernehmen.'), h('div', { class: 'inputunit' }, inp, h('span', null, '€'))),
      [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
        const v = inp.value.trim() === '' ? null : parseNum(inp.value);
        if (Number.isNaN(v) || (v != null && v < 0)) { toast('Bitte eine Zahl ab 0 eintragen.', 'err'); return false; }
        try { S.barkasse = await api('PUT', `/api/admin/barkasse/anfang?year=${jahr}`, { betrag: v }); } catch (e) { handleErr(e); return false; }
      } }]);
    if (ok) nachher(Promise.resolve(true));
  };
  const loeschen = (b) => ({ label: 'Löschen', gefahr: true, aktion: async () => {
    if (!(await altesJahrFreigabe(jahr))) return;
    if (!(await confirmBox(`Beleg ${b.belegNr} »${b.text}« löschen? Die Belegnummer wird nicht neu vergeben.`, 'Löschen', true))) return;
    try { S.barkasse = await api('DELETE', `/api/admin/barkasse/${b.id}?year=${jahr}`); nachher(Promise.resolve(true)); } catch (e) { handleErr(e); }
  } });
  const leerBuch = !d.buchungen.length;
  return h('details', { class: 'card barkasse', open: !leerBuch },
    h('summary', null, h('b', null, 'Barkasse'), h('span', { class: 'hint' }, leerBuch ? ' · noch nicht geführt' : ` · Bestand ${eur(d.bestand)}`),
      d.bestand < 0 ? h('span', { class: 'badge err', style: 'margin-left:8px' }, 'im Minus') : null),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Kassenbuch für Bargeld, z. B. Sommerfest, Getränke oder kleine Einkäufe. Jede Buchung bekommt eine Belegnummer – unter dieser Nummer den Papierbeleg abheften. Bei der Kassenprüfung wird das gezählte Bargeld mit dem Bestand verglichen.'),
    h('div', { class: 'toolbar' },
      pruefMode ? null : h('button', { class: 'btn primary', onclick: () => nachher(barbuchungDialog(null)) }, '+ Buchung'),
      pruefMode ? null : h('button', { class: 'btn', onclick: anfang }, `Anfangsbestand ${eur(d.anfang)}${d.anfangVorjahr ? ' (Vorjahr)' : ''}`),
      h('div', { class: 'spacer' }),
      leerBuch ? null : h('span', { class: 'stat' }, `Einnahmen ${eur(d.einnahmen)} · Ausgaben ${eur(d.ausgaben)}`),
      leerBuch ? null : druckKnopf({ url: `/api/admin/barkasse/kassenbuch?year=${jahr}`, oeffnenUrl: `/api/admin/barkasse/kassenbuch/oeffnen?year=${jahr}`, titel: 'Kassenbuch' }, { label: 'Kassenbuch drucken' })),
    leerBuch ? null : h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Beleg', 'Datum', 'Text', 'Kategorie', 'Betrag', 'Bestand', ''].map((x) => h('th', null, x)))),
      h('tbody', null, d.buchungen.map((b, i) => h('tr', null, h('td', null, b.belegNr), h('td', null, deDate(b.datum)), h('td', { class: 'wrap' }, b.text), h('td', null, b.kategorie || ''),
        h('td', { class: 'r' + (b.betrag < 0 ? ' neg' : '') }, eur(b.betrag)), h('td', { class: 'r' }, eur(d.laufend[i])),
        h('td', { class: 'aktionen' }, pruefMode ? null : [h('button', { class: 'btn small', onclick: () => nachher(barbuchungDialog(b)) }, 'Bearbeiten'), ' ', mehrMenue([loeschen(b)])])))))));
}

// ---- Kassenprüfung: Status-Karte im Kassenbericht und geführter Dialog

function pruefungKarte() {
  const jahr = S.kasseYear;
  if (!S.pruefung || S.pruefung.jahr !== jahr) {
    if (!S.pruefungLaedt) {
      S.pruefungLaedt = true;
      api('GET', `/api/admin/kassenpruefung?year=${jahr}`).then((d) => { S.pruefung = d; }).catch(() => { S.pruefung = { jahr, fehler: true }; })
        .finally(() => { S.pruefungLaedt = false; render(); });
    }
    return null;
  }
  const d = S.pruefung;
  if (d.fehler) return null;
  const erledigt = d.punkte.filter((p) => d.pruefung.punkte[p.key]).length;
  const status = d.pruefung.ergebnis === 'ohne' ? ['ok', 'ohne Beanstandungen abgeschlossen'] : d.pruefung.ergebnis === 'mit' ? ['warn', 'mit Beanstandungen abgeschlossen']
    : erledigt ? ['neu', `${erledigt} von ${d.punkte.length} Punkten geprüft`] : ['', 'noch nicht begonnen'];
  return h('div', { class: 'card pruef-karte' },
    h('div', null, h('b', null, `Kassenprüfung ${jahr}`), hilfe('Die von der Mitgliederversammlung gewählten Kassenprüfer sehen Kassenbuch, Belege und Bestände durch und berichten der Versammlung. Ohne Beanstandungen empfehlen sie, den Vorstand zu entlasten.'), ' ', h('span', { class: 'badge ' + status[0] }, status[1]),
      h('div', { class: 'hint' }, 'Geführte Prüfung: Kontostand abgleichen, Belege und Stichprobe prüfen – am Ende steht der Bericht für die Mitgliederversammlung.')),
    h('div', { class: 'spacer' }),
    h('button', { class: 'btn primary', onclick: pruefungDialog }, erledigt || d.pruefung.ergebnis ? 'Prüfung fortsetzen' : 'Prüfung beginnen'),
    druckKnopf({ url: `/api/admin/kassenpruefung/bericht?year=${jahr}`, oeffnenUrl: `/api/admin/kassenpruefung/bericht/oeffnen?year=${jahr}`, titel: 'Prüfbericht' }, { label: 'Prüfbericht drucken' }));
}

async function pruefungDialog() {
  let d = S.pruefung;
  const p = d.pruefung;
  const f = {
    datum: h('input', { type: 'date', value: p.datum || todayIso() }),
    pruefer: h('input', { type: 'text', maxlength: 200, value: p.pruefer || '', placeholder: 'z. B. Uwe Brandt, Karin Vogel' }),
    konto: h('input', { type: 'text', inputmode: 'decimal', value: p.kontostand != null ? numIn(p.kontostand) : '', placeholder: 'laut Kontoauszug' }),
    bar: h('input', { type: 'text', inputmode: 'decimal', value: p.barkasse != null ? numIn(p.barkasse) : '', placeholder: 'falls vorhanden' }),
    fest: h('textarea', { rows: 3, maxlength: 3000, class: 'rs-eingabe', placeholder: 'z. B. Beleg für die Reparatur vom 12.05. fehlt, wird nachgereicht.' }),
    ergebnis: h('select', null, [['', 'Prüfung noch offen'], ['ohne', 'Ohne Beanstandungen'], ['mit', 'Mit Beanstandungen']].map(([v, t]) => h('option', { value: v, selected: (p.ergebnis || '') === v }, t))),
    entl: h('input', { type: 'checkbox', checked: !!p.entlastung }),
  };
  f.fest.value = p.feststellungen || '';
  const haken = Object.fromEntries(d.punkte.map((x) => [x.key, h('input', { type: 'checkbox', checked: !!p.punkte[x.key] })]));
  const punkte = h('ul', { class: 'haken-liste pruef-punkte' });
  const stich = h('div');
  const zeichne = () => {
    punkte.replaceChildren(...d.punkte.map((x) => h('li', null, h('label', null, haken[x.key], ' ', h('b', null, x.text)),
      h('div', { class: 'hint' + (x.warnung ? ' warn-text' : '') }, x.hinweis))));
    stich.replaceChildren(
      h('div', { class: 'toolbar' }, h('b', null, 'Stichprobe'), h('div', { class: 'spacer' }),
        h('button', { class: 'btn small', disabled: !d.ausgaben, onclick: async () => {
          try { d = await api('POST', `/api/admin/kassenpruefung/stichprobe?year=${d.jahr}`); S.pruefung = d; zeichne(); } catch (e) { handleErr(e); }
        } }, d.stichprobe.length ? 'Neu ziehen' : 'Stichprobe ziehen')),
      d.stichprobe.length ? h('table', { class: 'data' }, h('tbody', null, d.stichprobe.map((x) => h('tr', null, h('td', null, deDate(x.datum)), h('td', null, x.beschreibung), h('td', { class: 'r' }, eur(x.betrag)),
        h('td', null, x.belege && x.belege.length ? h('a', { href: `/api/admin/beleg/${x.id}?year=${d.jahr}&idx=0`, target: '_blank' }, 'Beleg ansehen') : h('span', { class: 'badge warn' }, 'kein Beleg')),
        h('td', null, h('label', null, h('input', { type: 'checkbox', checked: x.geprueft, onchange: async (e) => {
          try { await api('PUT', `/api/admin/ausgaben/${x.id}/geprueft?year=${d.jahr}`, { geprueft: e.target.checked }); d = await api('GET', `/api/admin/kassenpruefung?year=${d.jahr}`); S.pruefung = d; S.kasse = null; zeichne(); }
          catch (err) { handleErr(err); }
        } }), ' geprüft')))))) : h('p', { class: 'hint' }, 'Das Programm wählt die drei größten Ausgaben und zufällig weitere aus (etwa jede zehnte, mindestens fünf). Diese vergleicht ihr mit den Belegen.'));
  };
  zeichne();
  const fld = (label, el) => h('div', { class: 'field' }, h('label', null, label), el);
  const diffZeile = h('p', { class: 'hint' });
  const diffRechnen = () => {
    const k = parseNum(f.konto.value), bar = parseNum(f.bar.value);
    // mit Kassenbuch zählt das Bargeld gegen dessen Bestand, sonst gegen den Kassenbestand
    const b = bar == null || Number.isNaN(bar) ? 0 : d.barSoll != null ? bar - d.barSoll : bar;
    const diff = k == null || Number.isNaN(k) ? null : Math.round((k + b - d.kassenbestand) * 100) / 100;
    diffZeile.className = diff != null && Math.abs(diff) >= 0.005 ? 'warn-text' : 'hint';
    diffZeile.textContent = `Kassenbestand laut Programm: ${eur(d.kassenbestand)}` + (d.barSoll != null ? `, Barkasse laut Kassenbuch: ${eur(d.barSoll)}` : '') + (diff == null ? '.' : Math.abs(diff) < 0.005 ? ' – stimmt mit Konto und Barkasse überein.' : ` – Differenz ${eur(diff)}.`);
  };
  f.konto.addEventListener('input', diffRechnen); f.bar.addEventListener('input', diffRechnen);
  diffRechnen();
  const speichern = async () => {
    const zahl = (el) => (el.value.trim() === '' ? null : parseNum(el.value));
    const konto = zahl(f.konto); const bar = zahl(f.bar);
    if (Number.isNaN(konto) || Number.isNaN(bar)) { toast('Bitte Kontostand und Barkasse als Zahl eintragen.', 'err'); return false; }
    try {
      S.pruefung = await api('PUT', `/api/admin/kassenpruefung?year=${d.jahr}`, { datum: f.datum.value, pruefer: f.pruefer.value, kontostand: konto, barkasse: bar,
        punkte: Object.fromEntries(Object.entries(haken).map(([k, el]) => [k, el.checked])), feststellungen: f.fest.value, ergebnis: f.ergebnis.value, entlastung: f.entl.checked });
    } catch (e) { handleErr(e); return false; }
    return true;
  };
  const ok = await breiterDialog(`Kassenprüfung ${d.jahr}`, h('div', null,
    h('div', { class: 'form' }, fld('Tag der Prüfung', f.datum), fld('Kassenprüfer', f.pruefer), fld('Kontostand 31.12. in €', f.konto), fld('Barkasse in €', f.bar)),
    diffZeile,
    h('h4', null, 'Prüfpunkte'), punkte, stich,
    h('h4', null, 'Ergebnis'),
    h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Feststellungen'), f.fest), fld('Ergebnis', f.ergebnis),
      h('div', { class: 'field' }, h('label', null, ' '), h('label', null, f.entl, ' Entlastung des Vorstands empfehlen')))),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: speichern }]);
  if (ok) { S.aufgaben = null; toast('Kassenprüfung gespeichert', 'ok'); }
  render();
}

function adminKassenbericht(pruefMode) {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.currentYear;
  const yearSel = h('select', { onchange: (e) => { S.kasseYear = Number(e.target.value); S.kasse = null; render(); } },
    st.years.map((y) => h('option', { value: y, selected: y === S.kasseYear }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  if (!S.kasse || S.kasse.jahr !== S.kasseYear) {
    if (!S.kasseLoading) loadKasse(S.kasseYear);
    return h('div', null, h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel), h('p', { class: 'hint' }, 'Wird geladen …'));
  }
  const k = S.kasse;
  if (!S.kasseVerlauf && !S.kasseVerlaufLoading) loadKasseVerlauf();
  const vorjahr = (S.kasseVerlauf || []).find((j) => j.jahr === S.kasseYear - 1);
  const vjDelta = (cur, prev) => {
    if (prev == null) return '';
    const diff = cur - prev;
    if (Math.abs(diff) < 0.005) return ` · = Vorjahr (${S.kasseYear - 1})`;
    const pct = prev !== 0 ? Math.round((diff / Math.abs(prev)) * 1000) / 10 : null;
    const pctTxt = pct == null ? '' : ` (${pct > 0 ? '+' : ''}${nfFlex.format(pct)} %)`;
    return ` · ${diff > 0 ? '▲' : '▼'} ${eur(Math.abs(diff))}${pctTxt} ggü. ${S.kasseYear - 1}`;
  };
  const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
  const cards = h('div', { class: 'cards' },
    card('Kassenbestand', eur(k.kassenbestand), `Anfangsbestand ${eur(k.anfangsbestand)} + Zahlungen${k.spenden ? ' + Spenden' : ''} − Ausgaben${vorjahr ? vjDelta(k.kassenbestand, vorjahr.kassenbestand) : ''}`, k.kassenbestand >= 0 ? 'ok' : 'err'),
    card('Von Pächtern eingegangen', `${eur(k.einnahmenBezahlt)}`, `tatsächlich gezahlt, nach heutigem Stand${vorjahr ? vjDelta(k.einnahmenBezahlt, vorjahr.einnahmenBezahlt) : ''}`, 'ok'),
    k.guthabenAusgezahlt ? card('An Pächter ausgezahlt', eur(k.guthabenAusgezahlt), 'Guthaben') : null,
    k.vorauszahlungen ? card('Vorauszahlungen', eur(k.vorauszahlungen), 'monatliche Raten der Pächter', 'ok') : null,
    k.spenden ? card('Spenden', eur(k.spenden), `an den Verein${vorjahr ? vjDelta(k.spenden, vorjahr.spenden || 0) : ''}`, 'ok') : null,
    card('Sonstige Ausgaben', eur(k.ausgabenSumme), `${anz(k.ausgaben.length, 'Buchung', 'Buchungen')}${vorjahr ? vjDelta(k.ausgabenSumme, vorjahr.ausgabenSumme) : ''}`, k.ausgabenSumme ? 'warn' : 'ok'));

  const anfInput = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.anfangsbestand), style: 'width:140px', autocomplete: 'off' });
  const saveAnfang = async () => {
    const v = parseNum(anfInput.value);
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte eine Zahl ab 0 eingeben.', 'err'); return; }
    try { S.kasse = await api('PUT', `/api/admin/anfangsbestand?year=${S.kasseYear}`, { betrag: v }); S.kasseVerlauf = null; toast('Gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };

  const ohneBeleg = k.ausgaben.filter((x) => !(x.belege && x.belege.length)).length;
  const belegFilter = h('input', { type: 'checkbox', checked: S.kasseNurOhneBeleg,
    onchange: (e) => { S.kasseNurOhneBeleg = e.target.checked; render(); } });
  const gezeigt = S.kasseNurOhneBeleg ? k.ausgaben.filter((x) => !(x.belege && x.belege.length)) : k.ausgaben;
  const toggleGeprueft = async (x, checked) => {
    try { await api('PUT', `/api/admin/ausgaben/${x.id}/geprueft?year=${S.kasseYear}`, { geprueft: checked }); await refreshKasse(); }
    catch (e) { handleErr(e); }
  };
  const tbody = h('tbody');
  for (const x of gezeigt) {
    tbody.append(h('tr', null, h('td', null, deDate(x.datum)),
      h('td', null, x.beschreibung, x.wiederkehrend ? h('span', { title: 'Jährlich wiederkehrend', style: 'margin-left:4px;cursor:help', class: 'notiz-zeichen' }, '↻') : null),
      h('td', null, x.kategorie),
      h('td', { class: 'r' }, eur(x.betrag)),
      h('td', null, x.belege && x.belege.length ? h('a', { href: `/api/admin/beleg/${x.id}?year=${S.kasseYear}&idx=0`, target: '_blank', title: `${anz(x.belege.length, 'Beleg', 'Belege')} ansehen` }, x.belege.length > 1 ? `${x.belege.length} Belege` : 'Beleg') : null),
      h('td', { title: x.geprueft ? `Geprüft am ${deDate(x.geprueftAm)}` : 'Von der Kassenprüfung abhaken' },
        h('input', { type: 'checkbox', checked: x.geprueft, onchange: (e) => toggleGeprueft(x, e.target.checked) })),
      h('td', null, pruefMode ? null : h('button', { class: 'btn small', onclick: async () => { const r = await ausgabeDialog(x); if (r !== false && r !== undefined) await refreshKasse(); } }, 'Bearbeiten'))));
  }
  if (!gezeigt.length) tbody.append(h('tr', null, h('td', { colspan: 7, class: 'empty' },
    k.ausgaben.length ? 'Alle Ausgaben haben einen Beleg.' : leer('kasse', 'Noch keine Ausgaben', 'Hier stehen Vereinskosten neben der Pächterabrechnung, z. B. Kontoführung oder Reparaturen – jede mit Beleg-Foto, wenn du möchtest.'))));

  const katTable = (k.ausgabenKategorie || []).length > 1
    ? h('div', { class: 'tablewrap', style: 'max-width:360px;margin-top:10px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Kategorie', 'Summe'].map((t) => h('th', null, t)))),
        h('tbody', null, k.ausgabenKategorie.map((kat) => h('tr', null, h('td', null, kat.kategorie), h('td', { class: 'r' }, eur(kat.summe)))))))
    : null;

  const vW3 = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserM3), autocomplete: 'off', disabled: !!pruefMode });
  const vWE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserEUR), autocomplete: 'off', disabled: !!pruefMode });
  const vSK = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromKWh), autocomplete: 'off', disabled: !!pruefMode });
  const vSE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromEUR), autocomplete: 'off', disabled: !!pruefMode });
  const diffRow = (label, total, vInput, unit) => {
    const v = parseNum(vInput.value);
    const diff = v == null || Number.isNaN(v) ? null : Math.round((v - total) * 100) / 100;
    return h('tr', null, h('td', null, label), h('td', { class: 'r' }, unit === '€' ? eur(total) : nfFlex.format(total) + ' ' + unit),
      h('td', null, vInput), h('td', { class: 'r' + (diff != null && Math.abs(diff) > 0.004 ? ' diffwarn' : '') },
        diff == null ? '–' : (unit === '€' ? eur(diff) : nfFlex.format(diff) + ' ' + unit)));
  };
  const versorgerTable = h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['', 'Pächter gesamt', 'Versorger / Hauptzähler', 'Differenz'].map((t) => h('th', null, t)))),
    h('tbody', null,
      diffRow('Wasser', k.wasserVerbrauch, vW3, 'm³'), diffRow('Wasser (€)', k.summen.kostenWasser, vWE, '€'),
      diffRow('Strom', k.stromVerbrauch, vSK, 'kWh'), diffRow('Strom (€)', k.summen.kostenEnergie, vSE, '€')));
  const saveVersorger = async () => {
    const vals = [vW3, vWE, vSK, vSE].map((i) => parseNum(i.value));
    if (vals.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Zahlen ab 0 eingeben.', 'err'); return; }
    try {
      S.kasse = await api('PUT', `/api/admin/versorger?year=${S.kasseYear}`, { wasserM3: vals[0], wasserEUR: vals[1], stromKWh: vals[2], stromEUR: vals[3] });
      S.schwund = null;
      toast('Gespeichert', 'ok');
      render();
    } catch (e) { handleErr(e); }
  };

  return h('div', null,
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel),
    h('p', { class: 'hint' }, `Übersicht für ${S.kasseYear}: ${k.paechter} Pächter (${k.ausgestellt} aus ausgestellten Rechnungen, ${k.berechnet} berechnet, aber noch nicht ausgestellt${k.unvollstaendig.length ? `, ${k.unvollstaendig.length} unvollständig` : ''}). Die Beträge »sonstige Ausgaben« sind Vereinskosten neben der Pächterabrechnung, z. B. Kontoführungsgebühren, Anwaltskosten oder Anschaffungen.`),
    k.unvollstaendig.length ? h('div', { class: 'banner warn' }, `Nicht enthalten (Angaben fehlen): ${k.unvollstaendig.join(', ')}`) : null,
    anfangsbestandHinweis(k, pruefMode),
    wechselHinweis(k, pruefMode),
    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('div', { class: 'toolbar' },
        h('label', null, 'Anfangsbestand der Kasse'),
        pruefMode ? h('b', null, eur(k.anfangsbestand)) : h('div', { class: 'inputunit' }, anfInput, h('span', null, '€')),
        pruefMode ? null : h('button', { class: 'btn small primary', onclick: saveAnfang }, 'Speichern'),
        pruefMode ? null : h('span', { class: 'hint' }, k.anfangAutomatisch ? 'kommt automatisch aus dem Kassenbestand Ende des Vorjahres und passt sich an, wenn dort noch etwas nachgebucht wird. Wer hier von Hand etwas einträgt, schaltet das ab.' : 'von Hand eingetragen – passt sich nicht mehr automatisch an das Vorjahr an'))),
    cards,
    pruefungKarte(),
    barkasseKarte(pruefMode),
    h('h3', null, 'Sonstige Ausgaben der Vereinskasse'),
    h('div', { class: 'card' },
      h('div', { class: 'toolbar' },
        pruefMode ? null : h('button', { class: 'btn primary', onclick: async () => { const r = await ausgabeDialog(null); if (r !== false && r !== undefined) await refreshKasse(); } }, '+ Ausgabe erfassen'),
        h('div', { class: 'spacer' }),
        h('label', { class: 'hint' }, belegFilter, ` nur ohne Beleg${ohneBeleg ? ` (${ohneBeleg})` : ''}`)),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Datum', 'Beschreibung', 'Kategorie', 'Betrag', 'Beleg', 'Geprüft', ''].map((t) => h('th', null, t)))), tbody)),
      katTable),
    h('h3', null, 'Vergleich mit dem Versorger'),
    h('div', { class: 'card' },
      h('p', { class: 'hint' }, 'Trage hier die Werte der Hauptzähler bzw. der Versorgerrechnung ein, um sie mit der Summe der Pächterabrechnung zu vergleichen. Eine größere Abweichung kann auf einen Zählerfehler, Schwund oder eine falsche Ablesung hindeuten.'),
      h('div', { class: 'tablewrap' }, versorgerTable),
      pruefMode ? null : h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: saveVersorger }, 'Werte speichern'))),
    h('h3', null, 'Schwund über die Jahre'),
    schwundKarte(),
    h('div', { class: 'actions', style: 'margin-top:16px;margin-bottom:22px' },
      druckKnopf({ url: `/api/admin/jahresbericht?year=${S.kasseYear}`, oeffnenUrl: `/api/admin/jahresbericht/oeffnen?year=${S.kasseYear}`, titel: `Jahresbericht ${S.kasseYear}` }, { label: `Jahresbericht ${S.kasseYear}`, primary: true }),
      h('a', { class: 'btn', href: `/api/admin/export-kassenbericht?year=${S.kasseYear}` }, `Kassenbericht ${S.kasseYear} als Excel`)),
    h('h3', null, 'Verlauf über die Jahre'),
    kassenberichtVerlaufCard());
}

// Nach einem Pächterwechsel: ausgeschiedene Pächter, bei denen noch eine Schlussrechnung, eine Zahlung oder ein Guthaben offen ist.
function wechselHinweis(k, pruefMode) {
  const l = k.wechsel || [];
  if (!l.length) return null;
  return h('div', { class: 'banner warn wechselhinweis' },
    h('b', null, l.length === 1 ? 'Ausgeschiedener Pächter mit offenem Posten' : `${l.length} ausgeschiedene Pächter mit offenen Posten`),
    h('ul', null, l.map((w) => h('li', null,
      `Garten ${w.gartennr}, ${w.name} (ausgeschieden ${deDate(w.ausgeschieden)}): `,
      [w.ohneRechnung ? 'Schlussrechnung fehlt' : null, w.offen > 0 ? `${eur(w.offen)} noch zu zahlen` : null, w.guthaben > 0 ? `${eur(w.guthaben)} Guthaben auszuzahlen` : null].filter(Boolean).join(', ')))),
    pruefMode ? null : h('div', { class: 'actions' },
      h('button', { class: 'btn small', onclick: () => { S.tab = 'rechnungen'; render(); } }, 'Zu den Rechnungen'),
      h('button', { class: 'btn small', onclick: () => { S.tab = 'zahlungen'; render(); } }, 'Zu den Zahlungen')));
}

// Wasserschwund: Hauptzähler gegen die Summe der Gartenzähler, Jahr für Jahr, und Gärten mit auffälligem Verbrauch.
function schwundKarte() {
  if (!S.schwund || S.schwund.jahr !== S.kasseYear) {
    if (!S.schwundLoading) {
      S.schwundLoading = true;
      api('GET', `/api/admin/schwund?year=${S.kasseYear}`).then((d) => { S.schwund = d; }).catch((e) => { handleErr(e); S.schwund = { jahr: S.kasseYear, jahre: [], auffaellig: [] }; })
        .finally(() => { S.schwundLoading = false; render(); });
    }
    return h('div', { class: 'card' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const d = S.schwund;
  const mitWert = d.jahre.filter((j) => j.wasser.prozent != null || j.strom.prozent != null);
  const zahl = (x, einheit) => (x == null ? '–' : nfFlex.format(x) + (einheit ? ' ' + einheit : ''));
  const prozent = (w, warn) => (w.prozent == null ? '–' : h('span', { class: 'badge ' + (w.prozent >= warn ? 'err' : w.prozent < 0 ? 'warn' : 'ok') }, nfFlex.format(w.prozent) + ' %'));
  const zeile = (j) => h('tr', null, h('td', null, j.jahr, j.unvollstaendig ? h('div', { class: 'hint' }, `${j.unvollstaendig} Gärten fehlen`) : null),
    h('td', { class: 'r' }, zahl(j.wasser.haupt, 'm³')), h('td', { class: 'r' }, zahl(j.wasser.gaerten, 'm³')), h('td', { class: 'r' }, zahl(j.wasser.menge, 'm³')),
    h('td', { class: 'r' }, prozent(j.wasser, d.warnAb)), h('td', { class: 'r' }, j.wasser.kosten == null ? '–' : eur(j.wasser.kosten)),
    h('td', { class: 'r' }, zahl(j.strom.menge, 'kWh')), h('td', { class: 'r' }, j.strom.prozent == null ? '–' : nfFlex.format(j.strom.prozent) + ' %'));
  const chart = mitWert.filter((j) => j.wasser.prozent != null).map((j) => ({ label: String(j.jahr), value: Math.max(0, j.wasser.prozent) }));
  return h('div', { class: 'card' },
    h('p', { class: 'hint' }, `Schwund ist Wasser (oder Strom), das der Hauptzähler zählt, das aber in keinem Garten ankommt – durch undichte Leitungen, stehende oder falsch abgelesene Zähler. Bezahlt wird es vom Verein. Ab ${nfFlex.format(d.warnAb)} % oder bei einem deutlichen Anstieg meldet sich das Cockpit. Grundlage sind die Hauptzählerwerte oben, je Jahr eingetragen.`),
    mitWert.length ? [
      chart.length > 1 ? [h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Wasserschwund in Prozent:'), barChart(chart, { valueFmt: (v) => nfFlex.format(v) + ' %' })] : null,
      h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Jahr', 'Hauptzähler', 'Gärten', 'Schwund', 'Anteil', 'Kosten', 'Strom-Schwund', 'Anteil'].map((t) => h('th', null, t)))),
        h('tbody', null, [...mitWert].reverse().map(zeile))))]
      : h('p', { class: 'hint', style: 'margin:0' }, 'Noch keine Hauptzählerwerte eingetragen. Trage oben den Wert des Hauptzählers bzw. der Versorgerrechnung ein – dann erscheint hier der Schwund, auch im Vergleich der Jahre.'),
    h('h4', { style: 'margin:16px 0 6px' }, `Auffällige Gärten ${d.jahr}`),
    d.auffaellig.length ? h('ul', { class: 'schwund-liste' }, d.auffaellig.map((x) => h('li', null, h('b', null, `Garten ${x.gartennr || '–'} – ${x.name}: `), x.hinweise.join('; '))))
      : h('p', { class: 'hint', style: 'margin:0' }, 'Kein Garten fällt auf (Verbrauch im üblichen Rahmen, kein Zähler steht still).'));
}

function kassenberichtVerlaufCard() {
  if (!S.kasseVerlauf) {
    if (!S.kasseVerlaufLoading) loadKasseVerlauf();
    return h('div', { class: 'card' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const v = S.kasseVerlauf;
  if (!v.length) return h('div', { class: 'card empty' }, 'Noch keine Jahre vorhanden.');
  const rows = v.map((j) => h('tr', null,
    h('td', null, j.jahr),
    h('td', { class: 'r' }, eur(j.rechnungssumme)),
    h('td', { class: 'r' }, eur(j.einnahmenBezahlt)),
    h('td', { class: 'r' }, eur(j.ausgabenSumme)),
    h('td', { class: 'r' }, eur(j.kassenbestand))));
  const chartData = [...v].reverse().map((j) => ({ label: String(j.jahr), value: j.kassenbestand }));
  return h('div', { class: 'card' },
    h('p', { class: 'hint' }, 'Summen des Vereins insgesamt (nicht je Pächter), zum Vergleich über die Jahre – z. B. für die Mitgliederversammlung.'),
    h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Kassenbestand am Jahresende:'),
    barChart(chartData, { valueFmt: (v) => eur(v) }),
    h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Jahr', 'Rechnungen gestellt', 'Zahlungen eingegangen', 'Sonstige Ausgaben', 'Kassenbestand'].map((t) => h('th', null, t)))),
      h('tbody', null, rows))));
}

// ---- Import / Export / Sicherung

// Verschlüsselte Sicherung für den USB-Stick: eine Datei, die sich nur mit dem Passwort öffnen lässt.
async function tresorErstellen() {
  const pw = h('input', { type: 'password', autocomplete: 'new-password', style: 'width:100%' });
  const pw2 = h('input', { type: 'password', autocomplete: 'new-password', style: 'width:100%' });
  const body = h('div', {},
    h('p', { class: 'hint', style: 'margin-top:0' }, 'Der ganze Datenbestand wird in eine Datei geschrieben, die sich nur mit diesem Passwort öffnen lässt. Sie ist für den USB-Stick oder einen anderen Ort außer Haus gedacht.'),
    h('label', {}, 'Passwort (mindestens 10 Zeichen)', pw),
    h('label', { style: 'margin-top:8px;display:block' }, 'Passwort wiederholen', pw2),
    h('p', { class: 'banner warn', style: 'margin-top:12px' }, 'Ohne das Passwort ist die Datei unbrauchbar, niemand kann es zurücksetzen. Bitte aufschreiben und getrennt vom Stick aufbewahren, zum Beispiel beim Vorsitzenden.'));
  const ok = await modal('Verschlüsselte Sicherung erstellen', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Erstellen', cls: 'primary', value: true, action: () => {
      if (pw.value.length < 10) { toast('Das Passwort braucht mindestens 10 Zeichen.', 'err'); pw.focus(); return false; }
      if (pw.value !== pw2.value) { toast('Die beiden Passwörter stimmen nicht überein.', 'err'); pw2.focus(); return false; }
      return true;
    } },
  ]);
  if (!ok) return;
  try {
    const r = await api('POST', '/api/admin/sicherung-verschluesselt', { passwort: pw.value });
    toast(`Verschlüsselte Sicherung erstellt: ${r.datei} (${groesseText(r.groesse)}). Der Ordner wurde geöffnet – Datei auf den USB-Stick kopieren.`, 'ok');
  } catch (e) { handleErr(e); }
}

async function tresorOeffnen() {
  const datei = h('input', { type: 'file', accept: '.kgsicherung' });
  const pw = h('input', { type: 'password', autocomplete: 'off', style: 'width:100%' });
  const body = h('div', {},
    h('p', { class: 'hint', style: 'margin-top:0' }, 'Die Datei wird entschlüsselt und in die Liste der Sicherungen gelegt. Am heutigen Stand ändert das nichts: Ansehen und Wiederherstellen entscheidet ihr danach in der Liste.'),
    h('label', {}, 'Datei (.kgsicherung)', datei),
    h('label', { style: 'margin-top:8px;display:block' }, 'Passwort', pw));
  let antwort = null;
  const ok = await modal('Verschlüsselte Sicherung öffnen', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Öffnen', cls: 'primary', value: true, action: async () => {
      if (!datei.files.length) { toast('Bitte eine Datei auswählen.', 'err'); return false; }
      const fd = new FormData();
      fd.append('file', datei.files[0]);
      fd.append('passwort', pw.value);
      try { antwort = await api('POST', '/api/admin/sicherung-entschluesseln', fd, true); } catch (e) { handleErr(e); return false; }
      return true;
    } },
  ]);
  if (!ok || !antwort) return;
  toast(`Sicherung geöffnet (${antwort.paechter} Pächter, ${antwort.rechnungen} Rechnungen). Sie steht jetzt unten in der Liste der Sicherungen.`, 'ok');
  await reload();
}

async function vereinsakteErstellen() {
  try {
    const r = await api('POST', '/api/admin/vereinsakte');
    toast(`Vereinsakte erstellt: ${r.datei} (${r.dateien} Dateien, ${groesseText(r.groesse)}). Der Ordner wurde geöffnet – Datei auf den USB-Stick kopieren.`, 'ok');
  } catch (e) { handleErr(e); }
}

function adminData() {
  const st = S.state;
  const wrap = h('div');
  const file = h('input', { type: 'file', accept: '.xlsx,.csv,.txt' });
  const previewBox = h('div');

  const drawPreview = () => {
    previewBox.replaceChildren();
    const pv = S.importPreview;
    if (!pv) return;
    const checks = [];
    const tb = h('tbody');
    for (const r of pv.rows) {
      const c = h('input', { type: 'checkbox', checked: true });
      checks.push([c, r]);
      tb.append(h('tr', null, h('td', null, c), h('td', null, h('span', { class: 'badge ' + (r.aktion === 'neu' ? 'neu' : 'warn') }, r.aktion === 'neu' ? 'neu' : 'aktualisieren')),
        h('td', null, r.paechter.mitgliedsnr), h('td', null, r.paechter.name), h('td', null, [r.paechter.strasse, r.paechter.plzOrt].filter(Boolean).join(', ')),
        h('td', { class: 'num' }, r.paechter.gartengroesse ? nfFlex.format(r.paechter.gartengroesse) + ' m²' : ''),
        h('td', null, r.ablesung ? '✓ Zählerstände' : ''), h('td', null, (r.warnungen || []).join('; '))));
    }
    previewBox.append(h('div', null,
      h('h3', null, `Vorschau: ${pv.rows.length} Zeilen gefunden`),
      (pv.warnings || []).length ? h('div', { class: 'banner warn' }, h('ul', { class: 'plain', style: 'margin:0' }, pv.warnings.map((w) => h('li', null, w)))) : null,
      h('p', { class: 'hint' }, 'Erkannte Spalten: ', (pv.rows[0].felder || []).join(', ') || 'nur Mitgliedsnr. und Name',
        '. Bereits vorhandene Pächter (gleiche Mitgliedsnummer) werden aktualisiert, alle anderen neu angelegt. Angaben, die in der Datei fehlen, bleiben unverändert.'),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', null,
        h('thead', null, h('tr', null, ['', 'Aktion', 'Mitgl.-Nr.', 'Name', 'Anschrift', 'Größe', 'Zählerstände', 'Hinweis'].map((x) => h('th', null, x)))), tb)),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        const rows = checks.filter(([c]) => c.checked).map(([, r]) => r);
        if (!rows.length) { toast('Keine Zeile ausgewählt.', 'err'); return; }
        try {
          const r = await api('POST', '/api/admin/import/apply', { rows });
          toast(`Import fertig: ${r.neu} neu, ${r.aktualisiert} aktualisiert`, 'ok');
          S.importPreview = null;
          await reload();
        } catch (e) { handleErr(e); }
      } }, 'Ausgewählte Zeilen importieren'), h('button', { class: 'btn', onclick: () => { S.importPreview = null; drawPreview(); file.value = ''; } }, 'Abbrechen'))));
  };
  file.addEventListener('change', async () => {
    if (!file.files.length) return;
    const fd = new FormData();
    fd.append('file', file.files[0]);
    try { S.importPreview = await api('POST', '/api/admin/import/preview', fd, true); drawPreview(); } catch (e) { S.importPreview = null; drawPreview(); handleErr(e); }
  });
  drawPreview();

  wrap.append(
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Pächter aus Excel oder CSV importieren'),
      h('p', { class: 'hint' }, 'Die Datei braucht eine Kopfzeile mit mindestens »Mitgliedsnr.« und »Name«. Weitere Spalten werden automatisch erkannt: Gartennr., Anrede, Straße, PLZ Ort, Versandart, E-Mail, Telefon, Gartengröße, Umlage abweichend, Wasserzähler-Nr., Stromzähler-Nr., Wasser/Energie Stand Vorjahr und aktuell, Arbeitsstunden, Versicherung, Grundsteuer, Auslagen, Hinweis, Abschlag. Eine passend aufgebaute Excel-Vorlage (Blatt »Mitglieder«) funktioniert direkt. Bei bekannten Mitgliedsnummern werden die Angaben aktualisiert; leere Zellen lassen die gespeicherten Werte unverändert.'),
      file, previewBox),
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Export'),
      h('p', { class: 'hint' }, `Alle Pächter des Jahres ${st.year} mit Zählerständen und Beträgen als Excel-Datei, zum Beispiel für die Kassenprüfung.`),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, `Jahresübersicht ${st.year} als Excel`)),
    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Datensicherung'),
      h('p', { class: 'hint' }, 'Das Programm legt bei Änderungen automatisch eine Tagessicherung an (die letzten 60 Tage) und vor Löschen und Import zusätzlich eine eigene. Die Sicherung vor jedem Jahreswechsel bleibt 12 Jahre erhalten, so lässt sich der Stand jedes Abrechnungsjahres über die ganze Aufbewahrungsfrist zurückholen. Du kannst außerdem den gesamten Datenbestand herunterladen. Eine Sicherung spielst du direkt hier unten wieder ein.'),
      h('div', { class: 'actions', style: 'margin-top:8px' },
        h('button', { class: 'btn', title: 'Sofort eine Sicherung anlegen und in den zweiten Sicherungsordner kopieren', onclick: sicherungJetzt }, 'Jetzt sichern'),
        h('a', { class: 'btn', href: '/api/admin/backup' }, 'Gesamten Datenbestand herunterladen'),
        h('button', { class: 'btn', title: 'Liest die jüngsten Sicherungen probeweise ein – ohne etwas zu verändern', onclick: wiederherstellungsProbe }, 'Sicherungen prüfen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'sicherungen' }).catch(handleErr) }, 'Sicherungsordner öffnen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'daten' }).catch(handleErr) }, 'Datenordner öffnen')),
      h('h4', { style: 'margin:16px 0 4px' }, 'Verschlüsselt für den USB-Stick', hilfe('Die gewöhnlichen Sicherungen sind nicht verschlüsselt: Wer den Stick findet, kann sie lesen. Die verschlüsselte Sicherung ist eine einzelne Datei, die sich nur mit eurem Passwort öffnen lässt. Sie enthält den Datenbestand, aber keine Rechnungs-PDFs und Belege.')),
      h('div', { class: 'actions' },
        h('button', { class: 'btn', onclick: tresorErstellen }, 'Verschlüsselte Sicherung erstellen'),
        h('button', { class: 'btn', onclick: tresorOeffnen }, 'Verschlüsselte Sicherung öffnen')),
      zweiteSicherungField()),
    h('div', { class: 'card', style: 'margin-top:16px' },
      h('h3', { style: 'margin-top:0' }, 'Amtsübergabe'),
      h('p', { class: 'hint' }, 'Neuer Vorstand oder Kassenwart? Die Vereinsakte enthält den kompletten Datenordner (Daten, Rechnungen, Gartenakten, Belege) und ein PDF »Lies mich zuerst« mit Zahlen, Preisen, Terminen, Gemeinnützigkeit und dem, was gerade ansteht. Auf einen USB-Stick kopieren und persönlich übergeben – das Passwort steht nicht darin. Es wird immer nur die neueste Akte aufbewahrt.'),
      h('button', { class: 'btn primary', onclick: vereinsakteErstellen }, 'Vereinsakte erstellen')),
    sicherungenCard());
  return wrap;
}

async function loadSicherungen() {
  if (S.sicherungenLoading) return;
  S.sicherungenLoading = true;
  try { S.sicherungen = await api('GET', '/api/admin/sicherungen'); } catch (e) { handleErr(e); S.sicherungen = null; S.sicherungenZeigen = false; }
  S.sicherungenLoading = false;
  render();
}

// Sicherung wiederherstellen: Liste der vorhandenen Sicherungen, Auswahl mit deutlicher Rückfrage.
function sicherungenCard() {
  const kopf = h('h3', { style: 'margin-top:0' }, 'Sicherung ansehen oder wiederherstellen');
  if (!S.sicherungenZeigen) {
    return h('div', { class: 'card', style: 'margin-top:14px' }, kopf,
      h('p', { class: 'hint' }, 'Hat sich etwas versehentlich verändert oder ist die Datendatei beschädigt? Hier siehst du alle vorhandenen Sicherungen und kannst eine davon einspielen. Der aktuelle Stand wird vorher selbst gesichert, auch das lässt sich also rückgängig machen.'),
      h('button', { class: 'btn', onclick: () => { S.sicherungenZeigen = true; S.sicherungen = null; render(); } }, 'Sicherungen anzeigen'));
  }
  if (!S.sicherungen) {
    if (!S.sicherungenLoading) loadSicherungen();
    return h('div', { class: 'card', style: 'margin-top:14px' }, kopf, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const wiederherstellen = async (x) => {
    const ok = await confirmBox(`Den Stand vom ${fmtWhen(x.zeit)} (${x.art}; ${x.paechter} Pächter, ${x.rechnungen} Rechnungen) wiederherstellen? Alles, was seitdem geändert wurde, verschwindet aus dem Programm, auch Einstellungen und Passwörter gehen auf den damaligen Stand zurück. Der heutige Stand bleibt als Sicherung »vor-Wiederherstellung« erhalten.`, 'Wiederherstellen', true);
    if (!ok) return;
    try {
      await api('POST', '/api/admin/sicherungen/wiederherstellen', { name: x.name });
      toast('Sicherung wiederhergestellt', 'ok');
      location.reload();
    } catch (e) { handleErr(e); }
  };
  const ansehen = async (x) => {
    let d;
    try { d = await api('POST', '/api/admin/sicherungen/ansehen', { name: x.name }); } catch (e) { handleErr(e); return; }
    const anders = (i) => d.zahlen[i][1] !== d.heute[i][1];
    await breiterDialog(`Sicherung vom ${fmtWhen(x.zeit)}`, h('div', null,
      h('p', { class: 'hint' }, 'Nur zum Ansehen – am heutigen Stand ändert sich nichts. So siehst du, ob die Sicherung vollständig ist, bevor du sie brauchst.'),
      h('p', null, h('b', null, d.verein || '–'), d.jahre.length ? ` · Jahre ${d.jahre[0]}–${d.jahre[d.jahre.length - 1]}` : '',
        d.letzteAenderung ? h('span', { class: 'hint' }, ` · letzte Änderung ${fmtWhen(d.letzteAenderung)}: ${d.letzteAktion}`) : null),
      h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, h('th', null, ''), h('th', { class: 'num' }, 'In der Sicherung'), h('th', { class: 'num' }, 'Heute'))),
        h('tbody', null, d.zahlen.map((z, i) => h('tr', null, h('td', null, z[0]), h('td', { class: 'num' }, h('b', null, z[1])),
          h('td', { class: 'num' + (anders(i) ? ' warn-text' : '') }, d.heute[i][1])))))),
      h('details', { style: 'margin-top:12px' }, h('summary', null, `Pächter in dieser Sicherung (${d.paechter.length})`),
        h('div', { class: 'sich-paechter' }, d.paechter.map((p) => h('span', null, h('b', null, p[0] || '–'), ' ', p[1]))))),
    [{ label: 'Schließen', cls: 'primary', value: true }]);
  };
  const rows = S.sicherungen.map((x) => h('tr', null,
    h('td', null, fmtWhen(x.zeit)), h('td', null, x.art),
    h('td', { class: 'num' }, x.lesbar ? x.paechter : '–'), h('td', { class: 'num' }, x.lesbar ? x.rechnungen : '–'),
    h('td', null, x.lesbar
      ? [h('button', { class: 'btn small', onclick: () => ansehen(x) }, 'Ansehen'), ' ', h('button', { class: 'btn small danger', onclick: () => wiederherstellen(x) }, 'Wiederherstellen')]
      : h('span', { class: 'hint' }, x.hinweis || 'nicht lesbar'))));
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'toolbar' }, kopf, h('div', { class: 'spacer' }),
      h('button', { class: 'btn small', onclick: () => { S.sicherungenZeigen = false; render(); } }, 'Schließen')),
    S.sicherungen.length
      ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Stand vom', 'Art', 'Pächter', 'Rechnungen', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows)))
      : h('p', { class: 'hint', style: 'margin:0' }, 'Noch keine Sicherungen vorhanden.'),
    S.sicherungen.length >= 40 ? h('p', { class: 'hint', style: 'margin-bottom:0' }, `Es werden die neuesten ${S.sicherungen.length} Sicherungen angezeigt. Ältere liegen im Sicherungsordner.`) : null);
}

function zweiteSicherungField() {
  const st = S.state;
  const inp = h('input', { type: 'text', value: st.settings.zweiteSicherung || '', style: 'width:100%', autocomplete: 'off',
    placeholder: 'z. B. D:\\Sicherung-Kleingarten-Manager oder ein Netzlaufwerk-Pfad' });
  const save = async () => {
    try {
      await api('PUT', '/api/admin/settings', { ...st.settings, zweiteSicherung: inp.value.trim() });
      toast('Gespeichert', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { style: 'margin-top:16px;border-top:1px solid var(--line,#ddd);padding-top:12px' },
    h('div', { class: 'field wide' }, h('label', null, 'Zweiter Sicherungsordner (optional, z. B. USB-Stick oder Netzlaufwerk)'), inp,
      h('span', { class: 'fhint' }, 'Leer lassen = deaktiviert. Ist der Pfad gerade nicht erreichbar (USB-Stick nicht eingesteckt), läuft das Programm trotzdem normal weiter, es wird nur dort nicht zusätzlich gesichert.')),
    h('div', { class: 'actions' }, h('button', { class: 'btn', onclick: save }, 'Speichern')));
}

// ---- Jubiläen

async function loadJubilaeen() {
  if (S.jubilaeenLoading) return;
  S.jubilaeenLoading = true;
  try { S.jubilaeen = await api('GET', '/api/garten-historie'); } catch (e) { handleErr(e); S.jubilaeen = []; }
  S.jubilaeenLoading = false;
  render();
}

// Zeigt, wie lange ein Pächter schon dabei ist – aus dem frühesten Eintrag der
// Gartenhistorie je Pächter, nützlich z. B. für die Ehrung langjähriger
// Mitglieder bei der Mitgliederversammlung. Steht erst ab Einführung der
// Gartenhistorie zur Verfügung; ältere Pächter ohne Eintrag landen gesondert.
function adminJubilaeen() {
  const st = S.state;
  if (!S.jubilaeen) {
    if (!S.jubilaeenLoading) loadJubilaeen();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const seitByPaechter = {};
  for (const e of S.jubilaeen) {
    if (!seitByPaechter[e.paechterId] || e.seit < seitByPaechter[e.paechterId]) seitByPaechter[e.paechterId] = e.seit;
  }
  const list = st.paechter.filter((p) => !p.ausgeschiedenAm).sort(cmpNr);
  const rows = [];
  const ohneDatum = [];
  for (const p of list) {
    const seit = seitByPaechter[p.id];
    if (!seit) { ohneDatum.push(p); continue; }
    rows.push({ p, seit, jahre: jahreSeit(seit) });
  }
  rows.sort((a, b) => a.seit.localeCompare(b.seit));
  const tbody = h('tbody', null, rows.map((r) => h('tr', null,
    h('td', null, r.p.mitgliedsnr), h('td', null, r.p.name), h('td', null, r.p.gartennr),
    h('td', null, deDate(r.seit)),
    h('td', { class: 'r' }, r.jahre > 0 && r.jahre % 5 === 0
      ? h('b', { title: 'Jubiläum', style: 'color:var(--ok)' }, `${r.jahre} Jahre`)
      : `${r.jahre} Jahre`))));
  return h('div', null,
    h('p', { class: 'hint' }, 'Mitglied-seit-Datum aus der Gartenhistorie (frühester Eintrag je Pächter). Nützlich für die Mitgliederversammlung, z. B. um langjährige Mitglieder zu ehren. Steht erst ab Einführung der Gartenhistorie-Funktion zur Verfügung – bei Pächtern von davor fehlt das Datum, sie stehen unten in einer eigenen Liste.'),
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Garten', 'Mitglied seit', 'Dabei seit'].map((t) => h('th', null, t)))),
      rows.length ? tbody : h('tbody', null, h('tr', null, h('td', { colspan: 5, class: 'empty' }, leer('stern', 'Noch keine Mitgliedsjahre', st.paechter.length ? 'Das Eintrittsdatum kommt aus der Gartenhistorie. Bei Pächtern ohne Eintrag erscheint es, sobald ein Garten zugewiesen ist.' : 'Lege zuerst die Pächter an. Danach siehst du hier, wer wie lange dabei ist.', st.paechter.length ? null : 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); })))))),
    ohneDatum.length ? h('div', { class: 'card', style: 'margin-top:12px' },
      h('h3', { style: 'margin-top:0' }, 'Ohne bekanntes Eintrittsdatum'),
      h('p', { class: 'hint' }, ohneDatum.map((p) => `${p.mitgliedsnr} ${p.name}`).join(', '))) : null);
}

// ---- Änderungsprotokoll

async function loadProtokoll() {
  if (S.protokollLoading) return;
  S.protokollLoading = true;
  try { S.protokoll = await api('GET', '/api/admin/audit-log'); } catch (e) { handleErr(e); S.protokoll = []; }
  S.protokollLoading = false;
  render();
}

function adminProtokoll() {
  if (!S.protokoll) {
    if (!S.protokollLoading) loadProtokoll();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const fmtZeit = (iso) => { const d = new Date(iso); return isNaN(d) ? iso : d.toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }).replace(',', ''); };
  const rows = S.protokoll.map((e) => h('tr', null, h('td', { class: 'mono', style: 'white-space:nowrap' }, fmtZeit(e.zeit)), h('td', null, e.aktion)));
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer wann welchen Pächter oder welche Preise geändert hat. Da es nur ein gemeinsames Admin-Passwort gibt, wird nicht festgehalten, welche Person es war – nur Zeitpunkt und Art der Änderung. Die letzten 1000 Einträge bleiben erhalten.'),
    h('div', { class: 'card tablewrap', style: 'max-height:520px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, h('th', null, 'Zeitpunkt'), h('th', null, 'Änderung'))),
      h('tbody', null, rows.length ? rows : h('tr', null, h('td', { colspan: 2, class: 'empty' }, 'Noch keine Einträge.'))))));
}

// ---- Passwort

// »Alles nur mit Anmeldung«: für Rechner, an denen mehrere Personen arbeiten.
function vollschutzCard(st) {
  if (!st.hasPassword) return null;
  const cb = h('input', { type: 'checkbox', checked: !!st.allesGeschuetzt });
  cb.addEventListener('change', async () => {
    try { await api('PUT', '/api/admin/vollschutz', { an: cb.checked }); toast(cb.checked ? 'Ohne Anmeldung ist jetzt nichts mehr sichtbar' : 'Abrechnung ist wieder ohne Anmeldung offen', 'ok'); await reload(); } catch (e) { handleErr(e); cb.checked = !cb.checked; }
  });
  return h('div', { class: 'card narrow' },
    h('h3', { style: 'margin-top:0' }, 'Alles nur mit Anmeldung'),
    h('p', { class: 'hint' }, 'Normalerweise sind Übersicht, Zählerstände, Rechnungen, Zahlungen, Lageplan und Termine ohne Passwort erreichbar. Nutzen mehrere Personen diesen Rechner, lässt sich das abstellen: Dann zeigt das Programm ohne Anmeldung nur noch die Anmeldeseite – keine Namen, keine Anschriften, keine Zahlen.'),
    h('label', { class: 'check' }, cb, ' Ohne Anmeldung nichts anzeigen'));
}

function adminSecurity() {
  const st = S.state;
  const old = h('input', { type: 'password', autocomplete: 'current-password' });
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden neuen Passwörter sind nicht gleich.', 'err'); return; }
    if (!st.hasPassword && !n1.value) { toast('Bitte ein Passwort eingeben.', 'err'); return; }
    try {
      await api('POST', '/api/admin/password', { old: old.value, new: n1.value });
      toast(n1.value ? 'Passwort gespeichert' : 'Passwortschutz entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('div', { class: 'card narrow' },
      h('h3', { style: 'margin-top:0' }, st.hasPassword ? 'Admin-Passwort ändern oder entfernen' : 'Admin-Passwort festlegen'),
      h('p', { class: 'hint' }, 'Das Passwort schützt die Verwaltung: Pächter, Preise, Kasse, Jahreswechsel und Import. Zählerstände, Rechnungen und Zahlungen bleiben offen, damit beim Ablesen jemand helfen kann – außer ihr schaltet unten »Alles nur mit Anmeldung« ein.'),
      h('div', { class: 'form', style: 'grid-template-columns:1fr' },
        st.hasPassword ? fld('Bisheriges Passwort', old) : null,
        fld('Neues Passwort', n1, st.hasPassword ? 'Leer lassen, um den Passwortschutz zu entfernen' : 'Mindestens 8 Zeichen'),
        fld('Neues Passwort wiederholen', n2)),
      h('div', { class: 'actions' },
        h('button', { class: 'btn primary', onclick: save }, 'Speichern'),
        st.hasPassword ? h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/admin/logout'); await reload(); } catch (e) { handleErr(e); } } }, 'Abmelden') : null),
      h('p', { class: 'hint', style: 'margin-top:16px' }, 'Passwort vergessen? Das Programm mit dem Zusatz "--reset-admin" starten (in der Eingabeaufforderung: Kleingarten-Manager.exe --reset-admin). Dann ist der Passwortschutz entfernt und du kannst ein neues Passwort festlegen.')),
    vollschutzCard(st),
    pruefPasswortCard(),
    updateCheckCard());
}

// ---- Kassenprüfer-Zugang verwalten (nur der Admin kann ihn einrichten)

function pruefPasswortCard() {
  const st = S.state;
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden Passwörter sind nicht gleich.', 'err'); return; }
    try {
      await api('POST', '/api/admin/pruef-password', { new: n1.value });
      toast(n1.value ? 'Kassenprüfer-Zugang gespeichert' : 'Kassenprüfer-Zugang entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Kassenprüfer-Zugang'),
    h('p', { class: 'hint' }, `Eigenes, optionales Passwort für den Kassenprüfer: Lesezugriff auf den Kassenbericht (inkl. Geprüft-Haken für die sonstigen Ausgaben), aber ohne die übrigen Admin-Rechte. ${st.hasPruefPassword ? 'Zugang ist eingerichtet.' : 'Zugang ist nicht eingerichtet.'}`),
    h('div', { class: 'form', style: 'grid-template-columns:1fr' },
      fld('Passwort', n1, st.hasPruefPassword ? 'Leer lassen, um den Zugang zu entfernen' : 'Mindestens 8 Zeichen'),
      fld('Passwort wiederholen', n2)),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern')));
}

// ---- Update-Prüfung (nur auf Knopfdruck, nie automatisch)

async function checkUpdate() {
  if (S.updateChecking) return;
  S.updateChecking = true;
  try { S.updateCheck = await api('GET', '/api/admin/check-update'); } catch (e) { S.updateCheck = { hinweis: e.message }; }
  S.updateChecking = false;
  render();
}

function updateCheckCard() {
  const st = S.state;
  const r = S.updateCheck;
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Nach Updates suchen'),
    h('p', { class: 'hint' }, `Installierte Version: ${st.version}. Die Prüfung fragt nur auf Knopfdruck einmalig die öffentliche GitHub-Release-Seite dieses Projekts ab, es läuft sonst keine Internetverbindung im Hintergrund.`),
    h('div', { class: 'actions' },
      h('button', { class: 'btn', disabled: S.updateChecking, onclick: checkUpdate }, S.updateChecking ? 'Prüfe …' : 'Nach Updates suchen')),
    r ? (r.available
      ? h('div', { class: 'banner info', style: 'margin-top:10px' }, `Version ${r.version} ist verfügbar. `,
          h('a', { href: r.url, target: '_blank', rel: 'noopener noreferrer', onclick: (e) => {
            // Im eigenen Programmfenster über den Systembrowser öffnen statt im Fenster selbst.
            if (typeof window.kgmOpenExternal !== 'function') return;
            e.preventDefault();
            window.kgmOpenExternal(r.url).catch(() => toast('Die Release-Seite konnte nicht geöffnet werden.', 'err'));
          } }, 'Release-Seite öffnen'))
      : r.hinweis
        ? h('p', { class: 'hint', style: 'margin-top:10px' }, r.hinweis)
        : h('p', { class: 'hint', style: 'margin-top:10px' }, 'Du hast die aktuelle Version.')) : null);
}

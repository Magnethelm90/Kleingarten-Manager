'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Rundschreiben, Schlüssel, Kündigungen, Beschlüsse, Haushalt, Vorauszahlungen, Lastschrift, Kontoauszug. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Rundschreiben

const RS_VORLAGEN = [
  ['Wasser wird angestellt', 'Wasser wird angestellt – bitte Leitungen prüfen', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … stellen wir das Wasser in der Anlage wieder an. Bitte schließt vorher die Entleerungshähne in euren Gärten und prüft die Leitungen auf Frostschäden.\n\nWer beim Anstellen helfen möchte, ist um … Uhr am Haupttor herzlich willkommen.'],
  ['Wasser wird abgestellt', 'Wasser wird abgestellt', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … wird das Wasser in der Anlage abgestellt. Bitte entleert danach eure Leitungen und lasst die Hähne geöffnet, damit über den Winter nichts einfriert.\n\nBitte denkt auch daran, uns bis … euren Zählerstand zu melden.'],
  ['Arbeitseinsatz', 'Einladung zum Arbeitseinsatz', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … ab … Uhr findet unser nächster Arbeitseinsatz statt. Treffpunkt ist das Vereinsheim. Geplant sind: …\n\nBitte bringt, wenn vorhanden, eigenes Werkzeug mit. Die geleisteten Stunden werden auf die Gemeinschaftsarbeit angerechnet.'],
  ['Allgemeine Information', '', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\n'],
];

async function loadRundschreiben() {
  if (S.rsLoading) return;
  S.rsLoading = true;
  try { S.rundschreiben = await api('GET', '/api/admin/rundschreiben'); } catch (e) { handleErr(e); S.rundschreiben = []; }
  S.rsLoading = false;
  render();
}

function adminRundschreiben() {
  if (S.rundschreiben == null) { loadRundschreiben(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const l = S.rundschreiben;
  const neu = async (vorlage) => { if (await rundschreibenDialog(vorlage || null)) { S.rundschreiben = null; render(); } };
  const loeschen = async (x) => {
    if (!(await confirmBox(`Rundschreiben »${x.betreff}« löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/rundschreiben/${x.id}`); S.rundschreiben = null; render(); } catch (e) { handleErr(e); }
  };
  const briefe = (x, alle) => pdfDrucken({ url: `/api/admin/rundschreiben/${x.id}/briefe${alle ? '?alle=1' : ''}`, oeffnenUrl: `/api/admin/rundschreiben/${x.id}/briefe/oeffnen${alle ? '?alle=1' : ''}`,
    titel: 'Rundschreiben', meldung: () => 'Die Briefe wurden im PDF-Programm geöffnet – dort drucken.' }).then(() => { S.rundschreiben = null; render(); });
  const aushang = (x) => pdfDrucken({ url: `/api/admin/rundschreiben/${x.id}/aushang`, oeffnenUrl: `/api/admin/rundschreiben/${x.id}/aushang/oeffnen`,
    titel: 'Aushang', meldung: () => 'Der Aushang wurde im PDF-Programm geöffnet – dort drucken.' });
  return h('div', null,
    h('p', { class: 'hint' }, 'Ein Brief des Vorstands an alle Pächter oder an eine Auswahl. Wer »Versand per E-Mail« und eine Adresse hat, bekommt ihn über dein E-Mail-Programm (alle Adressen unsichtbar im Bcc), alle anderen als Brief zum Ausdrucken – mit Anschrift für den Fensterumschlag. Dazu gibt es den Text als Aushang für den Schaukasten.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => neu() }, '+ Neues Rundschreiben')),
    l.length ? h('div', { class: 'rs-liste' }, l.map((x) => h('div', { class: 'card rs-karte' },
      h('div', { class: 'rs-kopf' },
        h('div', null, h('b', null, x.betreff), h('div', { class: 'hint' }, `${deDate(x.datum)} · ${x.an === 'auswahl' ? 'an ausgewählte Pächter' : 'an alle Pächter'} (${x.empfaenger}): ${x.perMail} per E-Mail, ${x.perPost} per Post`)),
        x.verschickt ? h('span', { class: 'badge ok' }, `verschickt ${deDate(x.verschickt)}`) : h('span', { class: 'badge warn' }, 'Entwurf')),
      h('p', { class: 'rs-text' }, x.text.length > 260 ? x.text.slice(0, 260) + ' …' : x.text),
      h('div', { class: 'rs-knoepfe' },
        h('button', { class: 'btn small primary', disabled: !x.perMail, onclick: () => rundschreibenMail(x) }, `E-Mail an ${x.perMail}`), ' ',
        h('button', { class: 'btn small', disabled: !x.perPost, onclick: () => briefe(x, false) }, `Briefe drucken (${x.perPost})`), ' ',
        h('button', { class: 'btn small', onclick: () => aushang(x) }, 'Aushang'), ' ',
        h('button', { class: 'btn small', onclick: async () => { if (await rundschreibenDialog(x)) { S.rundschreiben = null; render(); } } }, 'Bearbeiten'), ' ',
        mehrMenue([
          { label: `Briefe an alle drucken (${x.empfaenger})`, aktion: () => briefe(x, true) },
          { label: 'Als Vorlage für ein neues', aktion: () => neu({ betreff: x.betreff, text: x.text, an: x.an, auswahl: x.auswahl }) },
          { label: 'Löschen', gefahr: true, aktion: () => loeschen(x) }])))))
      : h('div', { class: 'card' }, leer('brief', 'Noch keine Rundschreiben', 'Zum Beispiel: Wasser wird angestellt, Einladung zum Arbeitseinsatz, neue Gartenordnung. Vorlagen helfen beim Start.', '+ Neues Rundschreiben', () => neu())));
}

// Dialog zum Schreiben. x: gespeichertes Rundschreiben (mit id) oder Vorlage (ohne id) oder null.
async function rundschreibenDialog(x) {
  const st = S.state;
  const aktiv = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) => (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));
  const f = {
    datum: h('input', { type: 'date', value: (x && x.id && x.datum) || todayIso() }),
    betreff: h('input', { type: 'text', maxlength: 150, value: (x && x.betreff) || '' }),
    text: h('textarea', { rows: 13, maxlength: 12000, class: 'rs-eingabe' }),
  };
  f.text.value = (x && x.text) || 'Liebe Gartenfreundinnen und Gartenfreunde,\n\n';
  let an = (x && x.an) || 'alle';
  const gewaehlt = new Set((x && x.auswahl) || []);
  const zaehler = h('span', { class: 'hint' });
  const zaehlen = () => { zaehler.textContent = `${gewaehlt.size} ausgewählt`; };
  const kaesten = aktiv.map((p) => {
    const cb = h('input', { type: 'checkbox', checked: gewaehlt.has(p.id), onchange: (e) => { if (e.target.checked) gewaehlt.add(p.id); else gewaehlt.delete(p.id); zaehlen(); } });
    const mail = p.versand === 'Emailsendung' && p.email;
    return { p, el: h('label', { class: 'rs-wahl' }, cb, ` ${p.gartennr ? 'Garten ' + p.gartennr + ' – ' : ''}${p.name} `, h('span', { class: 'hint' }, mail ? 'E-Mail' : 'Post')), cb };
  });
  const filter = h('input', { type: 'search', placeholder: 'Name oder Garten suchen', oninput: () => {
    const q = filter.value.trim().toLowerCase();
    for (const k of kaesten) k.el.style.display = !q || `${k.p.name} ${k.p.gartennr}`.toLowerCase().includes(q) ? '' : 'none';
  } });
  const alleSetzen = (wert, nur) => () => { for (const k of kaesten) if (k.el.style.display !== 'none' && (!nur || nur(k.p))) { k.cb.checked = wert; if (wert) gewaehlt.add(k.p.id); else gewaehlt.delete(k.p.id); } zaehlen(); };
  const auswahlBox = h('div', { class: 'rs-auswahl', style: an === 'auswahl' ? '' : 'display:none' },
    h('div', { class: 'toolbar' }, filter, h('button', { class: 'btn small', type: 'button', onclick: alleSetzen(true) }, 'Alle'), h('button', { class: 'btn small', type: 'button', onclick: alleSetzen(false) }, 'Keine'), zaehler),
    h('div', { class: 'rs-wahlliste' }, kaesten.map((k) => k.el)));
  zaehlen();
  const radio = (wert, label) => h('label', { class: 'rs-an' }, h('input', { type: 'radio', name: 'rs-an', checked: an === wert, onchange: () => { an = wert; auswahlBox.style.display = wert === 'auswahl' ? '' : 'none'; } }), ' ', label);
  const vorlagen = h('select', { onchange: () => {
    const v = RS_VORLAGEN[Number(vorlagen.value)];
    if (!v) return;
    if (v[1]) f.betreff.value = v[1];
    f.text.value = v[2];
    vorlagen.value = '';
  } }, h('option', { value: '' }, 'Vorlage einsetzen …'), RS_VORLAGEN.map((v, i) => h('option', { value: i }, v[0])));
  const fld = (label, el, hint) => h('div', { class: 'field wide' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return breiterDialog(x && x.id ? 'Rundschreiben bearbeiten' : 'Neues Rundschreiben', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Datum'), f.datum),
    h('div', { class: 'field' }, h('label', null, 'Vorlagen'), vorlagen),
    fld('Betreff *', f.betreff),
    fld('Text *', f.text, 'Anrede und Text schreiben; Gruß und »Der Vorstand« setzt das Programm darunter. Leerzeilen trennen Absätze.'),
    h('div', { class: 'field wide' }, h('label', null, 'Persönlich einsetzen'), h('div', { class: 'toolbar' },
      ...[['{Name}', 'Name'], ['{Garten}', 'Garten'], ['{Mitgliedsnr}', 'Mitgliedsnr.'], ['{Offen}', 'offener Betrag']].map(([t, l]) => h('button', { class: 'btn small', type: 'button', title: `Setzt ${t} an die Cursorstelle`, onclick: () => {
        const a = f.text.selectionStart ?? f.text.value.length, b = f.text.selectionEnd ?? a;
        f.text.setRangeText(t, a, b, 'end'); f.text.focus(); f.text.dispatchEvent(new Event('input', { bubbles: true }));
      } }, l)),
      h('span', { class: 'hint' }, 'In Briefen steht dort der Wert des Pächters, in Aushang und Sammel-E-Mail ein neutrales Wort.'))),
    h('div', { class: 'field wide' }, h('label', null, 'Empfänger'), h('div', null, radio('alle', 'alle Pächter'), radio('auswahl', 'nur ausgewählte'))),
    h('div', { class: 'field wide' }, auswahlBox)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    if (an === 'auswahl' && !gewaehlt.size) { toast('Bitte mindestens einen Empfänger auswählen.', 'err'); return false; }
    try {
      await api('POST', '/api/admin/rundschreiben', { id: x && x.id ? x.id : '', datum: f.datum.value, betreff: f.betreff.value, text: f.text.value, an, auswahl: an === 'auswahl' ? [...gewaehlt] : [] });
      toast('Rundschreiben gespeichert', 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

// E-Mail: öffnet das E-Mail-Programm, bei vielen Empfängern in mehreren Paketen. Passt der Text nicht in den
// Link, liegt er in der Zwischenablage und wird im E-Mail-Programm eingefügt.
async function rundschreibenMail(x) {
  for (let n = 1; ; n++) {
    let r;
    try { r = await api('POST', `/api/admin/rundschreiben/${x.id}/mail?paket=${n}`); } catch (e) { handleErr(e); break; }
    let kopiert = r.mitText;
    if (!r.mitText) { try { await navigator.clipboard.writeText(r.text); kopiert = true; } catch { kopiert = false; } }
    const textFeld = h('textarea', { rows: 8, readonly: true, class: 'rs-eingabe' }); textFeld.value = r.text;
    const weiter = n < r.pakete;
    const ok = await modal(r.pakete > 1 ? `E-Mail ${n} von ${r.pakete} geöffnet` : 'E-Mail geöffnet', h('div', null,
      h('p', null, `Dein E-Mail-Programm hat eine neue Nachricht an ${r.pakete > 1 ? 'einen Teil der' : 'alle'} ${r.empfaenger} Empfänger geöffnet (Adressen im Bcc). Bitte prüfen und absenden.`),
      r.mitText ? null : h('div', null,
        h('p', null, kopiert ? 'Der Text ist zu lang für den direkten Weg und liegt in der Zwischenablage: im E-Mail-Programm in die Nachricht klicken und mit Strg+V einfügen.' : 'Der Text ist zu lang für den direkten Weg. Bitte hier markieren, kopieren und im E-Mail-Programm einfügen:'),
        kopiert ? null : textFeld),
      weiter ? h('p', { class: 'hint' }, 'Viele Empfänger passen nicht in eine Nachricht. Nach dem Absenden geht es mit der nächsten weiter.') : null),
    weiter ? [{ label: 'Abbrechen', value: false }, { label: 'Nächste E-Mail öffnen', cls: 'primary', value: true }] : [{ label: 'Fertig', cls: 'primary', value: false }]);
    if (!ok || !weiter) break;
  }
  S.rundschreiben = null; render();
}

// ------------------------------------------------------------------ Schlüssel und Geräte

async function loadInventar() {
  if (S.invLoading) return;
  S.invLoading = true;
  try {
    const [k, g, w] = await Promise.all([api('GET', '/api/admin/schluessel'), api('GET', '/api/admin/geraete'), api('GET', '/api/admin/wartungsplan')]);
    S.inventar = { schluessel: k.schluessel, arten: k.arten, geraete: g, wartung: w };
  } catch (e) { handleErr(e); S.inventar = { schluessel: [], arten: [], geraete: [], wartung: [] }; }
  S.invLoading = false;
  render();
}

const invNeuLaden = () => { S.inventar = null; render(); };

function adminInventar() {
  if (S.inventar == null) { loadInventar(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const seite = S.invSeite === 'geraete' || S.invSeite === 'wartung' ? S.invSeite : 'schluessel';
  const sub = reiterLeiste([
    { label: `Schlüssel (${S.inventar.schluessel.length})`, aktiv: seite === 'schluessel', geh: () => { S.invSeite = 'schluessel'; render(); } },
    { label: `Geräte (${S.inventar.geraete.length})`, aktiv: seite === 'geraete', geh: () => { S.invSeite = 'geraete'; render(); } },
    { label: `Pflegeplan (${(S.inventar.wartung || []).length})`, aktiv: seite === 'wartung', geh: () => { S.invSeite = 'wartung'; render(); } }]);
  return h('div', null, sub, seite === 'geraete' ? inventarGeraete() : seite === 'wartung' ? inventarWartung() : inventarSchluessel());
}

// Pflegeplan für Gemeinschaftsflächen: Hecke, Wege, Spielplatz … kehrt in festen Abständen wieder.
const PFLEGE_VORLAGEN = [['Hecke schneiden', 6], ['Wege kehren', 1], ['Spielplatz prüfen', 3], ['Zaun kontrollieren', 12], ['Bäume kontrollieren', 12],
  ['Gemeinschaftshaus reinigen', 1], ['Müllplatz leeren', 1], ['Wasserleitungen winterfest machen', 12]];

async function wartungDialog(w) {
  const titel = h('input', { type: 'text', maxlength: 120, value: w ? w.titel : '', placeholder: 'z. B. Hecke schneiden' });
  const ort = h('input', { type: 'text', maxlength: 120, value: w ? w.ort || '' : '', placeholder: 'z. B. Vereinsheim, Hauptweg' });
  const monate = h('input', { type: 'number', min: 1, max: 60, value: w ? w.monate : 6, style: 'width:90px' });
  const faellig = h('input', { type: 'date', value: w ? w.faellig : todayIso() });
  const chips = w ? null : h('div', { class: 'chips' }, PFLEGE_VORLAGEN.map(([t, m]) => h('button', { type: 'button', class: 'chip', onclick: () => { titel.value = t; monate.value = m; titel.dispatchEvent(new Event('input')); } }, t)));
  return modal(w ? 'Pflegearbeit bearbeiten' : 'Neue Pflegearbeit', h('div', { class: 'form' },
    chips ? h('div', { class: 'field wide' }, h('label', null, 'Vorschläge'), chips) : null,
    h('div', { class: 'field wide' }, h('label', null, 'Was ist zu tun?'), titel),
    h('div', { class: 'field wide' }, h('label', null, 'Wo? (freiwillig)'), ort),
    h('div', { class: 'field' }, h('label', null, 'Alle wie viele Monate?'), monate),
    h('div', { class: 'field' }, h('label', null, 'Das nächste Mal am'), faellig)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    if (!titel.value.trim()) { feldFehler(titel, 'Bitte sag, was zu tun ist.'); return false; }
    try { await api('POST', '/api/admin/wartungsplan', { id: w ? w.id : '', letzte: w ? w.letzte || '' : '', titel: titel.value, ort: ort.value, monate: Number(monate.value), faellig: faellig.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function inventarWartung() {
  const l = S.inventar.wartung || [];
  const heute = todayIso(), in14 = isoInTagen(14);
  const nachher = async (p) => { if (await p) invNeuLaden(); };
  return h('div', null,
    h('p', { class: 'hint' }, 'Arbeiten an den Gemeinschaftsflächen, die immer wiederkommen. Das Programm merkt sich, wann sie dran sind, und erinnert im Cockpit zwei Wochen vorher. Mit »Erledigt« rechnet es den nächsten Termin selbst aus.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(wartungDialog(null)) }, '+ Pflegearbeit')),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Arbeit', 'Wo', 'Abstand', 'Nächstes Mal', 'Zuletzt', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((w) => h('tr', null,
        h('td', { class: 'wrap' }, h('b', null, w.titel)), h('td', null, w.ort || ''), h('td', null, w.monate === 1 ? 'jeden Monat' : `alle ${w.monate} Monate`),
        h('td', null, deDate(w.faellig), ' ', w.faellig < heute ? h('span', { class: 'badge err' }, 'überfällig') : w.faellig <= in14 ? h('span', { class: 'badge warn' }, 'bald') : null),
        h('td', null, w.letzte ? deDate(w.letzte) : h('span', { class: 'hint' }, '–')),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', onclick: () => nachher(datumDialog(`${w.titel} erledigt`, 'Wann wurde es gemacht? Der nächste Termin wird danach selbst ausgerechnet.', `/api/admin/wartungsplan/${w.id}/erledigt`)) }, 'Erledigt'), ' ',
          mehrMenue([
            { label: 'Bearbeiten', aktion: () => nachher(wartungDialog(w)) },
            { label: 'Löschen', gefahr: true, aktion: async () => {
              if (!(await confirmBox(`»${w.titel}« aus dem Pflegeplan löschen?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/wartungsplan/${w.id}`); invNeuLaden(); } catch (e) { handleErr(e); }
            } }])))))))
      : leer('Noch keine Pflegearbeiten', 'Lege an, was regelmäßig getan werden muss – zum Beispiel die Hecke schneiden. Das Cockpit erinnert dich dann rechtzeitig.'));
}

function inventarVerlaufDialog(titel, verlauf) {
  const l = [...(verlauf || [])].reverse();
  return modal(titel, l.length ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['Datum', 'Vorgang', 'Wer', 'Notiz'].map((t) => h('th', null, t)))),
    h('tbody', null, l.map((v) => h('tr', null, h('td', null, deDate(v.datum)), h('td', null, v.aktion), h('td', null, v.name || ''), h('td', { class: 'wrap' }, v.notiz || ''))))))
    : h('p', { class: 'hint' }, 'Noch keine Vorgänge.'), [{ label: 'Schließen', value: true }]);
}

// Wer bekommt den Schlüssel / das Gerät: ein Pächter oder jemand anderes (z. B. Vorstand).
async function empfaengerDialog(titel, url, mitBis) {
  const st = S.state;
  const aktiv = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) => (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));
  const wer = h('select', null, h('option', { value: '' }, 'Jemand anderes (Name eintragen) …'), aktiv.map((p) => h('option', { value: p.id }, `${p.gartennr ? 'Garten ' + p.gartennr + ' – ' : ''}${p.name}`)));
  const name = h('input', { type: 'text', maxlength: 80, placeholder: 'z. B. Vorstand (Vorsitz), Hausmeister' });
  const nameFeld = h('div', { class: 'field wide' }, h('label', null, 'Name'), name);
  wer.addEventListener('change', () => { nameFeld.style.display = wer.value ? 'none' : ''; });
  if (aktiv.length) { wer.value = aktiv[0].id; nameFeld.style.display = 'none'; }
  const datum = h('input', { type: 'date', value: todayIso() });
  const bis = h('input', { type: 'date' });
  const notiz = h('input', { type: 'text', maxlength: 200, placeholder: mitBis ? '' : 'z. B. 20 € Pfand bar erhalten' });
  return modal(titel, h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'An'), wer), nameFeld,
    h('div', { class: 'field' }, h('label', null, 'Datum'), datum),
    mitBis ? h('div', { class: 'field' }, h('label', null, 'Zurück bis'), bis) : null,
    h('div', { class: 'field wide' }, h('label', null, 'Notiz'), notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('POST', url, { paechterId: wer.value, name: wer.value ? '' : name.value, datum: datum.value, bis: mitBis ? bis.value : '', notiz: notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

async function datumDialog(titel, text, url) {
  const datum = h('input', { type: 'date', value: todayIso() });
  const notiz = h('input', { type: 'text', maxlength: 200 });
  return modal(titel, h('div', { class: 'form' }, text ? h('p', { class: 'field wide' }, text) : null,
    h('div', { class: 'field' }, h('label', null, 'Datum'), datum), h('div', { class: 'field wide' }, h('label', null, 'Notiz'), notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('POST', url, { datum: datum.value, notiz: notiz.value }); } catch (e) { handleErr(e); return false; }
  } }]);
}

async function schluesselDialog(k) {
  const arten = S.inventar.arten;
  const f = {
    art: h('select', null, arten.map((a) => h('option', { value: a, selected: k ? k.art === a : a === 'Tor' }, a))),
    nummer: h('input', { type: 'text', maxlength: 30, value: k ? k.nummer : '' }),
    bez: h('input', { type: 'text', maxlength: 100, value: k ? k.bezeichnung || '' : '', placeholder: 'z. B. Nebentor Parkplatz' }),
    pfand: h('input', { type: 'text', inputmode: 'decimal', value: k && k.pfand ? numIn(k.pfand) : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: k ? k.notiz || '' : '' }),
  };
  const fld = (label, el, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el);
  return modal(k ? `Schlüssel ${k.art} ${k.nummer}` : 'Neuer Schlüssel', h('div', { class: 'form' },
    fld('Art', f.art), fld('Nummer *', f.nummer), fld('Bezeichnung', f.bez, true), fld('Pfand in €', f.pfand), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const pfand = f.pfand.value.trim() ? parseNum(f.pfand.value) : 0;
    if (Number.isNaN(pfand)) { toast('Bitte das Pfand als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/schluessel', { id: k ? k.id : '', art: f.art.value, nummer: f.nummer.value, bezeichnung: f.bez.value, pfand, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function inventarSchluessel() {
  const l = S.inventar.schluessel;
  const aus = l.filter((k) => k.inhaberName);
  const pfand = aus.reduce((a, k) => a + (k.pfand || 0), 0);
  const nachher = async (p) => { if (await p) invNeuLaden(); };
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer hat welchen Schlüssel? Bei Ausgabe und Rückgabe wird der Verlauf festgehalten. Hat ein ausgeschiedener Pächter noch einen Schlüssel, meldet sich das Cockpit auf der Übersicht.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(schluesselDialog(null)) }, '+ Schlüssel'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Schlüssel · ${aus.length} ausgegeben${pfand ? ' · Pfand ' + eur(pfand) : ''}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Art', 'Nr.', 'Bezeichnung', 'Bei', 'Seit', 'Pfand', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((k) => h('tr', null,
        h('td', null, k.art), h('td', null, h('b', null, k.nummer)), h('td', { class: 'wrap' }, k.bezeichnung || ''),
        h('td', { class: 'wrap' }, k.inhaberName ? [k.inhaberName, k.gartennr ? h('span', { class: 'hint' }, ` (Garten ${k.gartennr})`) : null,
          k.ausgeschieden || k.inhaber === '(gelöschte Person)' ? h('span', { class: 'badge err', style: 'margin-left:6px' }, 'zurückfordern') : null] : h('span', { class: 'hint' }, 'im Schrank')),
        h('td', null, deDate(k.seit)), h('td', { class: 'num' }, k.pfand ? eur(k.pfand) : ''),
        h('td', { class: 'aktionen' },
          k.inhaberName
            ? h('button', { class: 'btn small', onclick: () => nachher(datumDialog(`Rückgabe ${k.art} ${k.nummer}`, `Zurück von ${k.inhaberName}.${k.pfand ? ' Pfand ' + eur(k.pfand) + ' zurückzahlen.' : ''}`, `/api/admin/schluessel/${k.id}/rueckgabe`)) }, 'Rückgabe')
            : h('button', { class: 'btn small', onclick: () => nachher(empfaengerDialog(`${k.art}schlüssel ${k.nummer} ausgeben`, `/api/admin/schluessel/${k.id}/ausgabe`, false)) }, 'Ausgeben'), ' ',
          mehrMenue([{ label: 'Bearbeiten', aktion: () => nachher(schluesselDialog(k)) }, { label: 'Verlauf', aktion: () => inventarVerlaufDialog(`Verlauf ${k.art} ${k.nummer}`, k.verlauf) },
            { label: 'Löschen', gefahr: true, aktion: async () => {
              if (!(await confirmBox(`Schlüssel ${k.art} ${k.nummer} löschen (z. B. verloren oder Schloss getauscht)?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/schluessel/${k.id}`); invNeuLaden(); } catch (e) { handleErr(e); }
            } }])))))))
      : h('div', { class: 'card' }, leer('schluessel', 'Noch keine Schlüssel', 'Trage Tor-, Vereinsheim- und Schachtschlüssel ein – dann weißt du jederzeit, wer welchen hat.', '+ Schlüssel', () => nachher(schluesselDialog(null)))));
}

async function geraetDialog(g) {
  const f = {
    name: h('input', { type: 'text', maxlength: 80, value: g ? g.name : '', placeholder: 'z. B. Gartenhäcksler' }),
    inv: h('input', { type: 'text', maxlength: 30, value: g ? g.inventarnr || '' : '' }),
    ort: h('input', { type: 'text', maxlength: 80, value: g ? g.standort || '' : '', placeholder: 'z. B. Geräteschuppen' }),
    seit: h('input', { type: 'date', value: g ? g.angeschafft || '' : '' }),
    wert: h('input', { type: 'text', inputmode: 'decimal', value: g && g.wert ? numIn(g.wert) : '' }),
    monate: h('input', { type: 'text', inputmode: 'numeric', value: g && g.wartungMonate ? String(g.wartungMonate) : '', placeholder: 'z. B. 12' }),
    letzte: h('input', { type: 'date', value: g ? g.letzteWartung || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: g ? g.notiz || '' : '' }),
  };
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(g ? g.name : 'Neues Gerät', h('div', { class: 'form' },
    fld('Name *', f.name, true), fld('Inventar-Nr.', f.inv), fld('Standort', f.ort), fld('Angeschafft am', f.seit), fld('Wert in €', f.wert),
    fld('Wartung alle … Monate', f.monate, false, 'leer = keine regelmäßige Wartung'), fld('Letzte Wartung', f.letzte), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const wert = f.wert.value.trim() ? parseNum(f.wert.value) : 0, monate = f.monate.value.trim() ? Number(f.monate.value) : 0;
    if (Number.isNaN(wert) || !Number.isInteger(monate)) { toast('Bitte Wert und Wartungsabstand als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/geraete', { id: g ? g.id : '', name: f.name.value, inventarnr: f.inv.value, standort: f.ort.value, angeschafft: f.seit.value, wert, wartungMonate: monate, letzteWartung: f.letzte.value, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function inventarGeraete() {
  const l = S.inventar.geraete;
  const heute = todayIso();
  const in30 = isoInTagen(30);
  const nachher = async (p) => { if (await p) invNeuLaden(); };
  const wert = l.reduce((a, g) => a + (g.wert || 0), 0);
  return h('div', null,
    h('p', { class: 'hint' }, 'Vereinsgeräte mit Standort, Wert und Wartung. Verleihst du ein Gerät, steht es mit Rückgabedatum hier; ist es überfällig oder steht eine Wartung an, erinnert das Cockpit daran.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(geraetDialog(null)) }, '+ Gerät'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Geräte${wert ? ' · Wert ' + eur(wert) : ''}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Gerät', 'Standort', 'Status', 'Nächste Wartung', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((g) => {
        const verliehen = !!g.entleiherName;
        const ueberfaellig = verliehen && g.bis && g.bis < heute;
        return h('tr', null,
          h('td', { class: 'wrap' }, h('b', null, g.name), g.inventarnr ? h('div', { class: 'hint' }, g.inventarnr) : null),
          h('td', null, g.standort || ''),
          h('td', { class: 'wrap' }, verliehen ? [`verliehen an ${g.entleiherName}`, h('div', { class: 'hint' }, `seit ${deDate(g.seit)}${g.bis ? ', zurück bis ' + deDate(g.bis) : ''}`),
            ueberfaellig ? h('span', { class: 'badge err' }, 'überfällig') : null] : h('span', { class: 'badge ok' }, 'da')),
          h('td', null, g.wartungFaellig ? [deDate(g.wartungFaellig), g.wartungFaellig <= in30 ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'fällig') : null] : h('span', { class: 'hint' }, '–')),
          h('td', { class: 'aktionen' },
            verliehen
              ? h('button', { class: 'btn small', onclick: () => nachher(datumDialog(`${g.name} zurück`, `Zurück von ${g.entleiherName}.`, `/api/admin/geraete/${g.id}/zurueck`)) }, 'Zurück')
              : h('button', { class: 'btn small', onclick: () => nachher(empfaengerDialog(`${g.name} verleihen`, `/api/admin/geraete/${g.id}/verleihen`, true)) }, 'Verleihen'), ' ',
            mehrMenue([
              { label: 'Wartung erledigt', aktion: () => nachher(datumDialog(`Wartung ${g.name}`, '', `/api/admin/geraete/${g.id}/wartung`)) },
              { label: 'Bearbeiten', aktion: () => nachher(geraetDialog(g)) },
              { label: 'Verlauf', aktion: () => inventarVerlaufDialog(`Verlauf ${g.name}`, g.verlauf) },
              { label: 'Löschen', gefahr: true, aktion: async () => {
                if (!(await confirmBox(`Gerät »${g.name}« löschen?`, 'Löschen', true))) return;
                try { await api('DELETE', `/api/admin/geraete/${g.id}`); invNeuLaden(); } catch (e) { handleErr(e); }
              } }])));
      }))))
      : h('div', { class: 'card' }, leer('werkzeug', 'Noch keine Geräte', 'Häcksler, Rasenmäher, Leitern, Schubkarren … – mit Standort, Wartung und wer gerade was ausgeliehen hat.', '+ Gerät', () => nachher(geraetDialog(null)))));
}

// ------------------------------------------------------------------ Kündigungen

async function loadKuendigungen() {
  if (S.kuendLoading) return;
  S.kuendLoading = true;
  try { S.kuend = await api('GET', '/api/admin/kuendigungen'); } catch (e) { handleErr(e); S.kuend = { regel: { zum: '11-30', bis: '08-31' }, kuendigungen: [] }; }
  S.kuendLoading = false;
  render();
}

const mtText = (mt) => { const [m, d] = (mt || '').split('-').map(Number); return m && d ? `${d}.${m}.` : mt; };

// frühestes Pachtende nach der Regel (wie KuendigungsRegel.fristgerecht in kuendigung.go)
function fristgerecht(eingang, regel) {
  if (!eingang) return '';
  let y = Number(eingang.slice(0, 4));
  if (eingang > `${y}-${regel.bis}`) y++;
  let ende = `${y}-${regel.zum}`;
  if (ende < eingang) ende = `${y + 1}-${regel.zum}`;
  return ende;
}

async function kuendigungDialog(p, k) {
  let regel = (S.kuend && S.kuend.regel) || null;
  if (!regel) { try { regel = (await api('GET', '/api/admin/kuendigungen')).regel; } catch (e) { handleErr(e); return false; } }
  const f = {
    eingang: h('input', { type: 'date', value: k ? k.eingang : todayIso() }),
    zum: h('input', { type: 'date', value: k ? k.zum : fristgerecht(todayIso(), regel) }),
    von: h('select', null, [['paechter', 'vom Pächter'], ['verein', 'vom Verein'], ['einvernehmlich', 'im Einvernehmen']].map(([v, l]) => h('option', { value: v, selected: k ? k.von === v : v === 'paechter' }, l))),
    grund: h('input', { type: 'text', maxlength: 200, value: k ? k.grund || '' : '', placeholder: 'z. B. Umzug, Alter' }),
    notiz: h('input', { type: 'text', maxlength: 1000, value: k ? k.notiz || '' : '' }),
  };
  const frist = h('span', { class: 'fhint' });
  const pruefe = () => {
    const fg = fristgerecht(f.eingang.value, regel);
    const zuFrueh = f.von.value === 'paechter' && f.zum.value && f.zum.value < fg;
    frist.textContent = fg ? `Fristgerecht frühestens zum ${deDate(fg)} (Regel: Eingang bis ${mtText(regel.bis)}, Ende zum ${mtText(regel.zum)}).${zuFrueh ? ' Das gewählte Pachtende ist früher – geht nur im Einvernehmen.' : ''}` : '';
    frist.classList.toggle('warn-text', !!zuFrueh);
  };
  f.eingang.addEventListener('change', () => { if (!k) f.zum.value = fristgerecht(f.eingang.value, regel); pruefe(); });
  f.zum.addEventListener('change', pruefe); f.von.addEventListener('change', pruefe);
  pruefe();
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint || null);
  return modal(k ? `Kündigung Garten ${k.gartennr}` : `Kündigung: ${p.name} (Garten ${p.gartennr})`, h('div', { class: 'form' },
    fld('Eingang der Kündigung', f.eingang), fld('Kündigung', f.von), fld('Pachtende', f.zum, true, frist), fld('Grund', f.grund, true), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try {
      await api('POST', '/api/admin/kuendigungen', { id: k ? k.id : '', paechterId: k ? k.paechterId : p.id, eingang: f.eingang.value, zum: f.zum.value, von: f.von.value, grund: f.grund.value, notiz: f.notiz.value });
      toast('Kündigung gespeichert – den Fahrplan findest du unter »Pächter« im Reiter »Kündigungen«', 'ok'); S.kuend = null;
    } catch (e) { handleErr(e); return false; }
  } }]);
}

function adminKuendigungen() {
  if (S.kuend == null) { loadKuendigungen(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.kuend;
  const laufend = d.kuendigungen.filter((k) => !k.abgeschlossen), fertig = d.kuendigungen.filter((k) => k.abgeschlossen);
  const neu = async () => {
    const aktiv = S.state.paechter.filter((p) => !p.ausgeschiedenAm && p.gartennr && !laufend.some((k) => k.paechterId === p.id)).sort(cmpNr);
    if (!aktiv.length) { toast('Es gibt keinen Pächter ohne laufende Kündigung.', 'err'); return; }
    const wer = h('select', null, aktiv.map((p) => h('option', { value: p.id }, `Garten ${p.gartennr} – ${p.name}`)));
    if (!(await modal('Wer kündigt?', h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Pächter'), wer)),
      [{ label: 'Abbrechen', value: false }, { label: 'Weiter', cls: 'primary', value: true }]))) return;
    if (await kuendigungDialog(aktiv.find((p) => p.id === wer.value))) { S.kuend = null; render(); }
  };
  const schritt = async (k, key, an) => {
    try { await api('POST', `/api/admin/kuendigungen/${k.id}/schritt`, { schritt: key, erledigt: an }); S.kuend = null; render(); } catch (e) { handleErr(e); }
  };
  const bestaetigung = (k) => pdfDrucken({ url: `/api/admin/kuendigungen/${k.id}/bestaetigung`, oeffnenUrl: `/api/admin/kuendigungen/${k.id}/bestaetigung/oeffnen`,
    titel: 'Kündigungsbestätigung', meldung: () => 'Die Bestätigung wurde im PDF-Programm geöffnet – dort drucken.' }).then(() => { S.kuend = null; render(); });
  const karte = (k) => h('div', { class: 'card kd-karte' + (k.abgeschlossen ? ' kd-fertig' : '') },
    h('div', { class: 'rs-kopf' },
      h('div', null, h('b', null, `Garten ${k.gartennr} – ${k.name}`),
        h('div', { class: 'hint' }, `Kündigung ${k.von === 'verein' ? 'durch den Verein' : k.von === 'einvernehmlich' ? 'im Einvernehmen' : 'durch den Pächter'} vom ${deDate(k.eingang)} zum ${deDate(k.zum)}${k.grund ? ' · ' + k.grund : ''}`),
        k.zuFrueh ? h('div', { class: 'warn-text' }, `Fristgerecht wäre erst der ${deDate(k.fristgemaess)}.`) : null),
      k.abgeschlossen ? h('span', { class: 'badge ok' }, `abgeschlossen ${deDate(k.abgeschlossen)}`) : h('span', { class: 'badge ' + (k.offen ? 'warn' : 'ok') }, k.offen ? `${k.offen} offen` : 'alles erledigt')),
    k.abgeschlossen ? null : h('ul', { class: 'kd-schritte' }, k.schritte.map((x) => h('li', { class: x.erledigt ? 'erledigt' : '' },
      h('label', { class: 'check' }, h('input', { type: 'checkbox', checked: !!x.erledigt, disabled: x.auto, onchange: (e) => schritt(k, x.key, e.target.checked) }),
        h('span', null, x.titel, x.erledigt ? h('span', { class: 'hint' }, x.auto ? ' – vom Programm erkannt' : ` – ${deDate(x.erledigt)}`) : null)),
      x.erledigt ? null : h('div', { class: 'hint kd-hilfe' }, x.hilfe)))),
    h('div', { class: 'rs-knoepfe' },
      k.abgeschlossen ? null : h('button', { class: 'btn small primary', onclick: () => bestaetigung(k) }, 'Bestätigung drucken'), ' ',
      k.abgeschlossen ? null : h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Warteliste'), ' ',
      k.abgeschlossen || !k.gartennr ? null : h('button', { class: 'btn small', onclick: () => gartenakteDialog(k.gartennr) }, 'Gartenakte / Wertermittlung'), ' ',
      mehrMenue([
        k.abgeschlossen ? null : { label: 'Bearbeiten', aktion: async () => { if (await kuendigungDialog(null, k)) { S.kuend = null; render(); } } },
        { label: k.abgeschlossen ? 'Wieder öffnen' : 'Abschließen', aktion: () => schritt(k, 'abgeschlossen', !k.abgeschlossen) },
        { label: 'Löschen (zurückgezogen)', gefahr: true, aktion: async () => {
          if (!(await confirmBox(`Kündigung für Garten ${k.gartennr} löschen, z. B. weil sie zurückgezogen wurde?`, 'Löschen', true))) return;
          try { await api('DELETE', `/api/admin/kuendigungen/${k.id}`); S.kuend = null; render(); } catch (e) { handleErr(e); }
        } }])));
  const zum = h('input', { type: 'text', value: d.regel.zum, maxlength: 5, style: 'width:5em', placeholder: 'MM-TT' });
  const bis = h('input', { type: 'text', value: d.regel.bis, maxlength: 5, style: 'width:5em', placeholder: 'MM-TT' });
  return h('div', null,
    h('p', { class: 'hint' }, 'Eine Kündigung erfassen, die Frist prüfen und Schritt für Schritt bis zur Übergabe abhaken. Wertermittlung, Schlüsselrückgabe und Pächterwechsel erkennt das Programm selbst; das Cockpit erinnert an alles, was noch offen ist.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: neu }, '+ Kündigung erfassen')),
    laufend.length ? h('div', { class: 'rs-liste' }, laufend.map(karte))
      : h('div', { class: 'card' }, leer('kuendigung', 'Keine laufenden Kündigungen', 'Kündigt jemand, erfasse es hier oder beim Pächter unter »Mehr«. Dann gibt es Bestätigungsbrief und Fahrplan bis zur Übergabe.', '+ Kündigung erfassen', neu)),
    fertig.length ? [h('h3', null, 'Abgeschlossen'), h('div', { class: 'rs-liste' }, fertig.map(karte))] : null,
    h('h3', null, 'Kündigungsregel des Vereins'),
    h('div', { class: 'card' }, h('div', { class: 'toolbar' },
      h('span', null, 'Kündigung muss eingehen bis'), bis, h('span', null, 'zum Pachtende am'), zum, h('span', { class: 'hint' }, '(Monat-Tag, z. B. 08-31 und 11-30)'),
      h('button', { class: 'btn small', onclick: async () => {
        try { await api('PUT', '/api/admin/kuendigungen/regel', { zum: zum.value.trim(), bis: bis.value.trim() }); toast('Regel gespeichert', 'ok'); S.kuend = null; render(); } catch (e) { handleErr(e); }
      } }, 'Speichern')),
    h('p', { class: 'hint', style: 'margin:6px 0 0' }, 'Steht in eurem Pachtvertrag. Das Bundeskleingartengesetz sieht für Kündigungen durch den Verein den 30. November vor.')));
}

// ------------------------------------------------------------------ Pachtvertrag

async function pachtvertragDialog(p) {
  const beginn = h('input', { type: 'date' });
  const text = h('textarea', { rows: 14, class: 'rs-eingabe' });
  try { text.value = (await api('GET', '/api/admin/vertragsbestimmungen')).text; } catch (e) { handleErr(e); return; }
  const q = () => `?paechter=${encodeURIComponent(p.id)}${beginn.value ? '&beginn=' + beginn.value : ''}`;
  const speichereText = async () => { await api('PUT', '/api/admin/vertragsbestimmungen', { text: text.value }); };
  const w = await breiterDialog(`Pachtvertrag Garten ${p.gartennr} – ${p.name}`, h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Pachtbeginn'), beginn, h('span', { class: 'fhint' }, 'leer = aus der Gartenhistorie')),
    h('div', { class: 'field wide' }, h('label', null, 'Bestimmungen des Vereins'), text,
      h('span', { class: 'fhint' }, 'Gilt für alle Verträge. Zeilen mit »§« werden Überschriften. Pächter, Garten, Pacht, Beiträge, Preise und Kündigungsfrist setzt das Programm selbst ein; der Lageplan kommt als Anlage dazu.'))),
  [{ label: 'Abbrechen', value: '' }, { label: 'In die Gartenakte legen', value: 'akte' }, { label: 'Drucken', cls: 'primary', value: 'druck' }]);
  if (!w) return;
  try { await speichereText(); } catch (e) { handleErr(e); return; }
  if (w === 'druck') await pdfDrucken({ url: `/api/admin/pachtvertrag${q()}`, oeffnenUrl: `/api/admin/pachtvertrag/oeffnen${q()}`, titel: 'Pachtvertrag', meldung: () => 'Der Pachtvertrag wurde im PDF-Programm geöffnet – dort zweimal drucken.' });
  if (w === 'akte') { try { await api('POST', `/api/admin/pachtvertrag/ablegen${q()}`); toast(`Pachtvertrag in der Gartenakte von Garten ${p.gartennr} abgelegt`, 'ok'); } catch (e) { handleErr(e); } }
}

// ------------------------------------------------------------------ Beschlüsse und Verträge

async function loadVerwaltung() {
  if (S.verwLoading) return;
  S.verwLoading = true;
  try {
    const [b, v, am] = await Promise.all([api('GET', '/api/admin/beschluesse'), api('GET', '/api/admin/vertraege'), api('GET', '/api/admin/aemter')]);
    S.verw = { beschluesse: b, vertraege: v.vertraege, arten: v.arten, jaehrlich: v.jaehrlich, aemter: am };
  } catch (e) { handleErr(e); S.verw = { beschluesse: [], vertraege: [], arten: [], jaehrlich: 0, aemter: { aemter: [], punkte: [], vorschlaege: [] } }; }
  S.verwLoading = false;
  render();
}

function adminVerwaltung() {
  if (S.verw == null) { loadVerwaltung(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const seite = ['vertraege', 'aemter'].includes(S.verwSeite) ? S.verwSeite : 'beschluesse';
  const knopf = (id, label) => ({ label, aktiv: seite === id, geh: () => { S.verwSeite = id; render(); } });
  return h('div', null, reiterLeiste([
    knopf('beschluesse', `Beschlussbuch (${S.verw.beschluesse.length})`),
    knopf('vertraege', `Verträge & Versicherungen (${S.verw.vertraege.length})`),
    knopf('aemter', `Ämter (${S.verw.aemter.aemter.filter((x) => !x.ehemalig).length})`)]),
  seite === 'vertraege' ? verwaltungVertraege() : seite === 'aemter' ? verwaltungAemter() : verwaltungBeschluesse());
}

// ---- Wahlen-Assistent

async function wahlDialog() {
  const d = S.verw.aemter;
  const aktiv = d.aemter.filter((x) => !x.ehemalig);
  const amtWahl = h('select', null, h('option', { value: '' }, 'Amt neu besetzen / anderes Amt …'),
    aktiv.map((x) => h('option', { value: x.id }, `${x.amt} (bisher ${x.name})`)));
  const amt = h('input', { type: 'text', maxlength: 60, list: 'amt-vorschlaege-w', placeholder: 'z. B. Kassenwart/in' });
  const liste = h('datalist', { id: 'amt-vorschlaege-w' }, d.vorschlaege.map((v) => h('option', { value: v })));
  amtWahl.addEventListener('change', () => { const x = aktiv.find((y) => y.id === amtWahl.value); if (x) amt.value = x.amt; });
  const datum = h('input', { type: 'date', value: todayIso() });
  const anzahl = h('input', { type: 'number', min: 1, max: 400, value: 60, style: 'width:80px' });
  const kandBox = h('div', { class: 'wahl-kandidaten' });
  const zeilen = [];
  const neueZeile = (name = '') => {
    const n = h('input', { type: 'text', maxlength: 100, placeholder: 'Name der Kandidatin / des Kandidaten', value: name });
    const st = h('input', { type: 'number', min: 0, value: 0, style: 'width:90px', 'aria-label': 'Stimmen' });
    const z = { n, st, el: h('div', { class: 'wahl-zeile' }, n, st, h('button', { type: 'button', class: 'btn small', title: 'Entfernen', 'aria-label': 'Kandidat entfernen', onclick: () => { if (zeilen.length > 1) { zeilen.splice(zeilen.indexOf(z), 1); z.el.remove(); rechne(); } } }, '✕')) };
    st.addEventListener('input', rechne); n.addEventListener('input', rechne);
    zeilen.push(z); kandBox.append(z.el);
  };
  const nein = h('input', { type: 'number', min: 0, value: 0, style: 'width:90px' });
  const enth = h('input', { type: 'number', min: 0, value: 0, style: 'width:90px' });
  const ung = h('input', { type: 'number', min: 0, value: 0, style: 'width:90px' });
  const absolut = h('input', { type: 'checkbox' });
  const zeit = h('input', { type: 'number', min: 0, max: 10, value: 3, style: 'width:70px' });
  const uebernehmen = h('input', { type: 'checkbox', checked: true });
  const live = h('div', { class: 'wahl-live', role: 'status' });
  const neinFeld = h('div', { class: 'field' }, h('label', null, 'Nein-Stimmen'), nein, h('span', { class: 'fhint' }, 'bei nur einem Kandidaten'));
  const zahl = (e) => Math.max(0, Math.floor(Number(e.value) || 0));
  const kand = () => zeilen.map((z) => ({ name: z.n.value.trim() || '…', stimmen: zahl(z.st) }));
  function rechne() {
    neinFeld.style.display = zeilen.length === 1 ? '' : 'none';
    const k = kand(); const n = zeilen.length === 1 ? zahl(nein) : 0;
    const summe = k.reduce((a, x) => a + x.stimmen, 0) + n + zahl(enth);
    const besten = [...k].sort((a, b) => b.stimmen - a.stimmen);
    let text;
    if (!summe) text = 'Trage die Stimmen ein – das Ergebnis erscheint hier.';
    else if (k.length === 1) text = k[0].stimmen > n && (!absolut.checked || k[0].stimmen * 2 > summe) ? `${k[0].name} wäre gewählt (${k[0].stimmen} Ja, ${n} Nein).` : 'Keine Wahl: nicht genug Ja-Stimmen.';
    else if (besten[0].stimmen === besten[1].stimmen) text = 'Stimmengleichheit – Stichwahl nötig.';
    else text = absolut.checked && besten[0].stimmen * 2 <= summe ? 'Absolute Mehrheit verfehlt – Stichwahl der beiden Bestplatzierten.' : `${besten[0].name} wäre gewählt (${besten[0].stimmen} von ${summe} gültigen Stimmen).`;
    live.replaceChildren(text);
  }
  neueZeile(); neueZeile();
  [nein, enth, ung].forEach((e) => e.addEventListener('input', rechne)); absolut.addEventListener('change', rechne);
  rechne();
  const kandidatenListe = () => zeilen.map((z) => z.n.value.trim()).filter(Boolean);
  const stimmzettel = () => {
    const k = kandidatenListe();
    if (!amt.value.trim() || !k.length) { toast('Bitte Amt und mindestens einen Kandidaten eintragen.', 'err'); return false; }
    const q = new URLSearchParams(); q.set('amt', amt.value.trim()); k.forEach((x) => q.append('kandidat', x)); q.set('anzahl', String(zahl(anzahl) || 1));
    return pdfDrucken({ url: `/api/admin/wahl/stimmzettel?${q}`, oeffnenUrl: `/api/admin/wahl/stimmzettel/oeffnen?${q}`, titel: 'Stimmzettel', meldung: () => 'Die Stimmzettel wurden im PDF-Programm geöffnet.' });
  };
  const fld = (label, el, hint) => h('div', { class: 'field' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  let ergebnis = null;
  const ok = await modal('Wahl durchführen', h('div', { class: 'form' }, liste,
    h('p', { class: 'hint' }, 'Erst Stimmzettel drucken, nach der Wahl die Stimmen eintragen. Das Ergebnis kommt ins Beschlussbuch, auf Wunsch auch in die Ämter, und der Protokollabsatz steht danach zum Kopieren bereit.'),
    fld('Amt *', amtWahl), fld('Bezeichnung *', amt), fld('Tag der Wahl *', datum),
    h('div', { class: 'field' }, h('label', null, 'Kandidaten und Stimmen *'), kandBox,
      h('button', { type: 'button', class: 'btn small', onclick: () => { neueZeile(); } }, '+ Kandidat')),
    neinFeld, fld('Enthaltungen', enth), fld('Ungültige Stimmzettel', ung),
    h('label', { class: 'inline' }, absolut, ' Absolute Mehrheit nötig (laut Satzung)'),
    live,
    h('div', { class: 'field' }, h('label', null, 'Stimmzettel'), h('div', null, anzahl, ' Stück ', h('button', { type: 'button', class: 'btn small', onclick: stimmzettel }, 'Stimmzettel drucken'))),
    h('label', { class: 'inline' }, uebernehmen, ' Bei erfolgreicher Wahl das Amt eintragen (bei Wechsel mit Übergabe-Checkliste, bei Wiederwahl nur die neue Amtszeit)'),
    fld('Amtszeit in Jahren', zeit, '0 = unbefristet')),
  [{ label: 'Abbrechen', value: false }, { label: 'Auszählen und eintragen', cls: 'primary', value: true, action: async () => {
    try {
      ergebnis = await api('POST', '/api/admin/wahl', { amt: amt.value, amtId: amtWahl.value, datum: datum.value, kandidaten: zeilen.map((z) => ({ name: z.n.value, stimmen: zahl(z.st) })),
        nein: zeilen.length === 1 ? zahl(nein) : 0, enthaltung: zahl(enth), ungueltig: zahl(ung), absolut: absolut.checked, uebernehmen: uebernehmen.checked, amtszeit: zahl(zeit) });
    } catch (e) { handleErr(e); return false; }
  } }]);
  if (!ok || !ergebnis) return false;
  S.verw = null; S.aufgaben = null;
  const kopie = h('textarea', { readonly: true, rows: 6, style: 'width:100%' }, ergebnis.protokoll);
  await modal(ergebnis.ok ? `Gewählt: ${ergebnis.gewaehlt}` : 'Keine Wahl zustande gekommen', h('div', null,
    ergebnis.hinweis ? h('p', { class: 'hint' }, ergebnis.hinweis) : null,
    h('p', null, ergebnis.beschluss ? `Eingetragen im Beschlussbuch als ${ergebnis.beschluss}.` : 'Es wurde nichts gespeichert.',
      ergebnis.amtEingetragen ? ' Das Amt ist in den Ämtern eingetragen – die Übergabe-Checkliste steht dort bereit.' : ''),
    h('p', { class: 'hint' }, 'Protokollabsatz zum Kopieren:'), kopie),
  [{ label: 'Kopieren', value: false, action: async () => { try { await navigator.clipboard.writeText(ergebnis.protokoll); toast('Kopiert', 'ok'); } catch (e) { kopie.select(); toast('Bitte mit Strg+C kopieren', 'ok'); } return false; } },
    { label: 'Schließen', cls: 'primary', value: true }]);
  return true;
}

// ---- Ämter und Amtswechsel

async function amtDialog(x, wechsel) {
  const d = S.verw.aemter;
  const liste = h('datalist', { id: 'amt-vorschlaege' }, d.vorschlaege.map((v) => h('option', { value: v })));
  const f = {
    amt: h('input', { type: 'text', maxlength: 60, list: 'amt-vorschlaege', value: x && !wechsel ? x.amt : (x ? x.amt : ''), disabled: !!wechsel, placeholder: 'z. B. Kassenwart/in' }),
    name: h('input', { type: 'text', maxlength: 100, value: x && !wechsel ? x.name : '' }),
    seit: h('input', { type: 'date', value: x && !wechsel ? x.seit || '' : wechsel ? todayIso() : '' }),
    bis: h('input', { type: 'date', value: x && !wechsel ? x.bis || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: x && !wechsel ? x.notiz || '' : '' }),
  };
  const fld = (label, el, hint) => h('div', { class: 'field' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  const titel = wechsel ? `Amtswechsel: ${x.amt}` : x ? `${x.amt} bearbeiten` : 'Amt eintragen';
  return modal(titel, h('div', null,
    wechsel ? h('p', { class: 'hint' }, `${x.name} gibt das Amt ab. Die bisherige Amtszeit endet am Tag der Wahl, danach führt eine Checkliste durch die Übergabe.`) : null,
    h('div', { class: 'form' }, liste, fld('Amt *', f.amt), fld(wechsel ? 'Neu gewählt *' : 'Name *', f.name),
      fld(wechsel ? 'Tag der Wahl *' : 'Im Amt seit', f.seit), fld('Amtszeit bis', f.bis, 'leer = unbefristet'), fld('Notiz', f.notiz))),
  [{ label: 'Abbrechen', value: false }, { label: wechsel ? 'Wechsel eintragen' : 'Speichern', cls: 'primary', value: true, action: async () => {
    const body = { id: x && !wechsel ? x.id : '', amt: f.amt.value, name: f.name.value, seit: f.seit.value, bis: f.bis.value, notiz: f.notiz.value };
    try { S.verw.aemter = await api('POST', wechsel ? `/api/admin/aemter/${x.id}/wechsel` : '/api/admin/aemter', body); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function verwaltungAemter() {
  const d = S.verw.aemter;
  const heute = todayIso();
  const aktiv = d.aemter.filter((x) => !x.ehemalig);
  const frueher = d.aemter.filter((x) => x.ehemalig);
  const nachher = async (p) => { if (await p) render(); };
  const bald = (bis) => { if (!bis) return null; const t = Math.round((new Date(bis) - new Date(heute)) / 86400000);
    return t < 0 ? h('span', { class: 'badge err' }, 'abgelaufen') : t <= 120 ? h('span', { class: 'badge warn' }, 'Wahl vorbereiten') : null; };
  const checkliste = (x) => {
    if (!x.checkliste) return null;
    const offen = d.punkte.filter((p) => !x.checkliste[p.key]).length;
    return h('details', { class: 'amt-check', open: offen > 0 }, h('summary', null, offen ? `Übergabe: ${offen} von ${d.punkte.length} Punkten offen` : 'Übergabe abgeschlossen ✓'),
      h('ul', { class: 'haken-liste' }, d.punkte.map((p) => h('li', null, h('label', null,
        h('input', { type: 'checkbox', checked: !!x.checkliste[p.key], onchange: async (e) => {
          try { S.verw.aemter = await api('PUT', `/api/admin/aemter/${x.id}/checkliste`, { key: p.key, erledigt: e.target.checked }); render(); }
          catch (err) { handleErr(err); e.target.checked = !e.target.checked; }
        } }), ' ', p.text, x.checkliste[p.key] ? h('span', { class: 'hint' }, ` · ${deDate(x.checkliste[p.key])}`) : null)))));
  };
  const loeschen = (x) => ({ label: 'Löschen', gefahr: true, aktion: async () => {
    if (!(await confirmBox(`Eintrag »${x.amt}: ${x.name}« löschen?`, 'Löschen', true))) return;
    try { S.verw.aemter = await api('DELETE', `/api/admin/aemter/${x.id}`); render(); } catch (e) { handleErr(e); }
  } });
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer hat welches Amt bis wann? Rund vier Monate vor Ende einer Amtszeit erinnert das Cockpit an die Wahl. Beim Amtswechsel hilft eine Checkliste, nichts zu vergessen – von der Bankvollmacht bis zum Vereinsregister.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(amtDialog(null)) }, '+ Amt'),
      h('button', { class: 'btn', title: 'Stimmzettel drucken, Stimmen auszählen, Ergebnis ins Beschlussbuch und Amt eintragen', onclick: () => nachher(wahlDialog()) }, 'Wahl durchführen …')),
    aktiv.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Amt', 'Name', 'Seit', 'Amtszeit bis', ''].map((t) => h('th', null, t)))),
      h('tbody', null, aktiv.map((x) => h('tr', null,
        h('td', { class: 'wrap' }, h('b', null, x.amt), x.notiz ? h('div', { class: 'hint' }, x.notiz) : null),
        h('td', { class: 'wrap' }, x.name, checkliste(x)),
        h('td', null, x.seit ? deDate(x.seit) : ''),
        h('td', null, x.bis ? deDate(x.bis) : h('span', { class: 'hint' }, 'unbefristet'), ' ', bald(x.bis)),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(amtDialog(x, true)) }, 'Amtswechsel'), ' ',
          mehrMenue([{ label: 'Bearbeiten', aktion: () => nachher(amtDialog(x)) }, loeschen(x)])))))))
      : h('div', { class: 'card' }, leer('personen', 'Noch keine Ämter eingetragen', 'Vorsitz, Kasse, Schriftführung, Kassenprüfung … mit Amtszeit – dann denkt das Programm an die nächste Wahl.', '+ Amt', () => nachher(amtDialog(null)))),
    frueher.length ? h('details', { class: 'card', style: 'margin-top:12px' }, h('summary', null, `Frühere Amtsinhaber (${frueher.length})`),
      h('table', { class: 'data' }, h('tbody', null, frueher.map((x) => h('tr', null, h('td', null, x.amt), h('td', null, x.name),
        h('td', null, `${x.seit ? deDate(x.seit) : '?'} – ${x.bis ? deDate(x.bis) : '?'}`), h('td', { class: 'aktionen' }, mehrMenue([loeschen(x)]))))))) : null);
}

async function beschlussDialog(b) {
  const f = {
    datum: h('input', { type: 'date', value: b ? b.datum : todayIso() }),
    gremium: h('select', null, ['Mitgliederversammlung', 'Vorstand'].map((g) => h('option', { value: g, selected: b ? b.gremium === g : false }, g))),
    nummer: h('input', { type: 'text', maxlength: 30, value: b ? b.nummer : '', placeholder: 'leer = automatisch' }),
    titel: h('input', { type: 'text', maxlength: 200, value: b ? b.titel : '', placeholder: 'z. B. Wasserpreis ab 2027: 2,10 € je m³' }),
    text: h('textarea', { rows: 4, maxlength: 5000, class: 'rs-eingabe' }),
    ja: h('input', { type: 'number', min: 0, value: b && b.ja ? b.ja : '' }), nein: h('input', { type: 'number', min: 0, value: b && b.nein ? b.nein : '' }),
    enth: h('input', { type: 'number', min: 0, value: b && b.enthaltung ? b.enthaltung : '' }),
    ergebnis: h('select', null, ['angenommen', 'abgelehnt', 'zur Kenntnis'].map((g) => h('option', { value: g, selected: b ? b.ergebnis === g : false }, g))),
    stich: h('input', { type: 'text', maxlength: 200, value: b ? b.stichworte || '' : '', placeholder: 'z. B. Wasser, Preise' }),
  };
  f.text.value = b ? b.text || '' : '';
  const fld = (label, el, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el);
  return modal(b ? `Beschluss ${b.nummer}` : 'Neuer Beschluss', h('div', { class: 'form' },
    fld('Datum', f.datum), fld('Gremium', f.gremium), fld('Beschluss *', f.titel, true), fld('Wortlaut / Erläuterung', f.text, true),
    fld('Ja', f.ja), fld('Nein', f.nein), fld('Enthaltung', f.enth), fld('Ergebnis', f.ergebnis), fld('Nummer', f.nummer), fld('Stichworte', f.stich)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const n = (el) => (el.value === '' ? 0 : Number(el.value));
    try { await api('POST', '/api/admin/beschluesse', { id: b ? b.id : '', datum: f.datum.value, gremium: f.gremium.value, nummer: f.nummer.value, titel: f.titel.value, text: f.text.value,
      ja: n(f.ja), nein: n(f.nein), enthaltung: n(f.enth), ergebnis: f.ergebnis.value, stichworte: f.stich.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

// Mehrfachauswahl für Listen: Haken je Zeile, Kopf-Haken für alle, Knopf »Auswahl löschen« (Papierkorb mit Rückgängig).
function auswahlHelfer(art, liste, pfad, was) {
  S.verwSel = S.verwSel || {};
  const sel = (S.verwSel[art] = S.verwSel[art] || new Set());
  for (const id of [...sel]) if (!liste.some((x) => x.id === id)) sel.delete(id);
  const neu = () => { S.verw = null; render(); };
  const kopf = h('input', { type: 'checkbox', 'aria-label': 'Alle auswählen', checked: liste.length > 0 && liste.every((x) => sel.has(x.id)),
    onchange: (e) => { for (const x of liste) { if (e.target.checked) sel.add(x.id); else sel.delete(x.id); } render(); } });
  const zelle = (x) => h('input', { type: 'checkbox', 'aria-label': 'Auswählen', checked: sel.has(x.id), onchange: (e) => { if (e.target.checked) sel.add(x.id); else sel.delete(x.id); render(); } });
  const knopf = sel.size ? h('button', { class: 'btn danger', onclick: async () => {
    if (!(await confirmBox(`${anz(sel.size, was[0], was[1])} in den Papierkorb legen?`, 'In den Papierkorb', true))) return;
    const ids = [...sel]; sel.clear();
    await sammelLoeschen(ids.map((id) => `${pfad}/${id}`), anz(ids.length, was[0], was[1]), async () => neu());
  } }, `Auswahl löschen (${sel.size})`) : null;
  return { kopf, zelle, knopf };
}

function verwaltungBeschluesse() {
  const alle = S.verw.beschluesse;
  const q = (S.beschlussSuche || '').trim().toLowerCase();
  const l = alle.filter((b) => !q || [b.titel, b.text, b.stichworte, b.nummer, b.datum.slice(0, 4)].some((x) => (x || '').toLowerCase().includes(q)));
  const nachher = async (p) => { if (await p) { S.verw = null; render(); } };
  const suche = h('input', { type: 'search', placeholder: 'Suchen, z. B. Wasser oder 2024', value: S.beschlussSuche || '', style: 'width:260px',
    oninput: (e) => { S.beschlussSuche = e.target.value; clearTimeout(S.beschlussTimer); S.beschlussTimer = setTimeout(() => { render(); const el = document.querySelector('.beschluss-suche'); if (el) { el.focus(); el.setSelectionRange(el.value.length, el.value.length); } }, 250); } });
  suche.classList.add('beschluss-suche');
  const aw = auswahlHelfer('beschluesse', l, '/api/admin/beschluesse', ['Beschluss', 'Beschlüsse']);
  return h('div', null,
    h('p', { class: 'hint' }, 'Was Vorstand und Mitgliederversammlung wann beschlossen haben – mit Abstimmungsergebnis und Stichworten zum Wiederfinden. Die Nummer vergibt das Programm fortlaufend je Jahr (z. B. MV 2025/3).'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(beschlussDialog(null)) }, '+ Beschluss'), suche, aw.knopf,
      h('div', { class: 'spacer' }),
      alle.length ? h('button', { class: 'btn', onclick: () => pdfDrucken({ url: '/api/admin/beschlussbuch', oeffnenUrl: '/api/admin/beschlussbuch/oeffnen', titel: 'Beschlussbuch', meldung: () => 'Das Beschlussbuch wurde im PDF-Programm geöffnet.' }) }, 'Beschlussbuch drucken') : null),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, h('th', null, aw.kopf), ...['Nr.', 'Datum', 'Beschluss', 'Ergebnis', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((b) => h('tr', null, h('td', null, aw.zelle(b)), h('td', null, b.nummer), h('td', null, deDate(b.datum), h('div', { class: 'hint' }, b.gremium)),
        h('td', { class: 'wrap' }, h('b', null, b.titel), b.text ? h('div', { class: 'hint' }, b.text) : null, b.stichworte ? h('div', { class: 'hint' }, 'Stichworte: ' + b.stichworte) : null),
        h('td', null, h('span', { class: 'badge ' + (b.ergebnis === 'angenommen' ? 'ok' : b.ergebnis === 'abgelehnt' ? 'err' : 'neu') }, b.ergebnis),
          b.ja + b.nein + b.enthaltung ? h('div', { class: 'hint' }, `${b.ja} : ${b.nein} : ${b.enthaltung}`) : null),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(beschlussDialog(b)) }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: async () => {
            try { const r = await api('DELETE', `/api/admin/beschluesse/${b.id}`); S.verw = null; render(); geloeschtToast(r, 'Beschluss', () => { S.verw = null; render(); }); } catch (e) { handleErr(e); }
          } }])))))))
      : h('div', { class: 'card' }, alle.length ? h('p', { class: 'hint' }, 'Keine Treffer.') : leer('akte', 'Noch keine Beschlüsse', 'Trage Beschlüsse nach jeder Versammlung ein – dann ist die Frage »Wann haben wir das beschlossen?« schnell beantwortet.', '+ Beschluss', () => nachher(beschlussDialog(null)))));
}

async function vertragDialog(v) {
  const f = {
    art: h('select', null, S.verw.arten.map((a) => h('option', { value: a, selected: v ? v.art === a : false }, a))),
    bez: h('input', { type: 'text', maxlength: 120, value: v ? v.bezeichnung : '', placeholder: 'z. B. Vereinshaftpflicht' }),
    partner: h('input', { type: 'text', maxlength: 120, value: v ? v.partner || '' : '' }),
    nummer: h('input', { type: 'text', maxlength: 60, value: v ? v.nummer || '' : '' }),
    beginn: h('input', { type: 'date', value: v ? v.beginn || '' : '' }),
    ende: h('input', { type: 'date', value: v ? v.ende || '' : '' }),
    frist: h('input', { type: 'number', min: 0, max: 60, value: v && v.fristMonate ? v.fristMonate : '' }),
    verl: h('input', { type: 'number', min: 0, max: 120, value: v && v.verlaengert ? v.verlaengert : '' }),
    kosten: h('input', { type: 'text', inputmode: 'decimal', value: v && v.kosten ? numIn(v.kosten) : '' }),
    turnus: h('select', null, ['jährlich', 'vierteljährlich', 'monatlich', 'einmalig'].map((t) => h('option', { value: t, selected: v ? v.turnus === t : false }, t))),
    gekuendigt: h('input', { type: 'date', value: v ? v.gekuendigt || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 1000, value: v ? v.notiz || '' : '' }),
  };
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(v ? v.bezeichnung : 'Neuer Vertrag', h('div', { class: 'form' },
    fld('Art', f.art), fld('Bezeichnung *', f.bez), fld('Vertragspartner', f.partner), fld('Vertrags-/Versicherungsnr.', f.nummer),
    fld('Beginn', f.beginn), fld('Läuft bis', f.ende, false, 'Ende der aktuellen Laufzeit, leer = unbefristet'),
    fld('Kündigungsfrist (Monate)', f.frist), fld('Verlängert sich um (Monate)', f.verl, false, '0 = endet von selbst'),
    fld('Kosten in €', f.kosten), fld('Zahlweise', f.turnus), fld('Gekündigt am', f.gekuendigt), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const kosten = f.kosten.value.trim() ? parseNum(f.kosten.value) : 0;
    if (Number.isNaN(kosten)) { toast('Bitte die Kosten als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/vertraege', { id: v ? v.id : '', art: f.art.value, bezeichnung: f.bez.value, partner: f.partner.value, nummer: f.nummer.value, beginn: f.beginn.value, ende: f.ende.value,
      fristMonate: Number(f.frist.value || 0), verlaengert: Number(f.verl.value || 0), kosten, turnus: f.turnus.value, gekuendigt: f.gekuendigt.value, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function verwaltungVertraege() {
  const l = S.verw.vertraege;
  const heute = todayIso();
  const in60 = isoInTagen(60);
  const nachher = async (p) => { if (await p) { S.verw = null; render(); } };
  const aw = auswahlHelfer('vertraege', l, '/api/admin/vertraege', ['Vertrag', 'Verträge']);
  return h('div', null,
    h('p', { class: 'hint' }, 'Versicherungen, Versorger, Zwischenpacht und andere Verträge mit Laufzeit und Kündigungsfrist. Rund zwei Monate vor dem letzten Kündigungstag erinnert das Cockpit – Zeit genug, Angebote zu vergleichen.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(vertragDialog(null)) }, '+ Vertrag'), aw.knopf,
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Verträge · ${eur(S.verw.jaehrlich)} im Jahr`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, h('th', null, aw.kopf), ...['Vertrag', 'Partner', 'Läuft bis', 'Kündigen bis', 'Kosten / Jahr', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((v) => h('tr', null, h('td', null, aw.zelle(v)),
        h('td', { class: 'wrap' }, h('b', null, v.bezeichnung), h('div', { class: 'hint' }, v.art + (v.nummer ? ' · ' + v.nummer : ''))),
        h('td', { class: 'wrap' }, v.partner || ''),
        h('td', null, v.laeuftBis ? deDate(v.laeuftBis) : h('span', { class: 'hint' }, 'unbefristet'), v.verlaengert && !v.gekuendigt ? h('div', { class: 'hint' }, `verlängert sich um ${v.verlaengert} Mon.`) : null),
        h('td', null, v.gekuendigt ? h('span', { class: 'badge neu' }, `gekündigt ${deDate(v.gekuendigt)}`) : v.kuendigenBis ? [deDate(v.kuendigenBis),
          v.kuendigenBis >= heute && v.kuendigenBis <= in60 ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'bald') : null] : '–'),
        h('td', { class: 'num' }, v.jaehrlich ? eur(v.jaehrlich) : ''),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(vertragDialog(v)) }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: async () => {
            try { const r = await api('DELETE', `/api/admin/vertraege/${v.id}`); S.verw = null; render(); geloeschtToast(r, 'Vertrag', () => { S.verw = null; render(); }); } catch (e) { handleErr(e); }
          } }])))))))
      : h('div', { class: 'card' }, leer('akte', 'Noch keine Verträge', 'Haftpflicht, Strom, Wasser, Zwischenpacht … – mit Laufzeit und Kündigungsfrist, damit kein Kündigungstermin mehr verpasst wird.', '+ Vertrag', () => nachher(vertragDialog(null)))));
}

// ------------------------------------------------------------------ Haushaltsplan

function adminHaushalt() {
  const st = S.state;
  const jahr = S.haushaltJahr || st.currentYear;
  if (!S.haushalt || S.haushalt.jahr !== jahr) {
    if (!S.haushaltLoading) {
      S.haushaltLoading = true;
      api('GET', `/api/admin/haushalt?year=${jahr}`).then((d) => { S.haushalt = d; }).catch((e) => { handleErr(e); S.haushalt = { jahr, einnahmen: [], ausgaben: [] }; })
        .finally(() => { S.haushaltLoading = false; render(); });
    }
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const d = S.haushalt;
  const eingaben = { einnahmen: new Map(), ausgaben: new Map() };
  const notiz = h('textarea', { rows: 3, class: 'rs-eingabe', maxlength: 2000, placeholder: 'z. B. geplante Anschaffungen, Rücklagen' }); notiz.value = d.notiz || '';
  const jahrSel = h('select', { onchange: (e) => { S.haushaltJahr = Number(e.target.value); render(); } },
    [...new Set([...st.years, st.currentYear + 1])].sort((a, b) => b - a).map((y) => h('option', { value: y, selected: y === jahr }, String(y))));
  const tabelle = (art, titel, zeilen) => {
    let soll = 0, ist = 0, abwSumme = 0;
    const rows = zeilen.map((z) => {
      const inp = h('input', { type: 'text', inputmode: 'decimal', value: z.soll != null ? numIn(z.soll) : '', class: 'hh-in', placeholder: '–' });
      eingaben[art].set(z.posten, inp);
      if (z.soll != null) soll += z.soll;
      ist += z.ist;
      const abw = z.soll != null ? z.ist - z.soll : null;
      if (abw != null) abwSumme += abw;
      const anteil = z.soll ? Math.min(150, Math.round(z.ist / z.soll * 100)) : null;
      const schlecht = abw != null && (art === 'ausgaben' ? abw > 0.004 : abw < -0.004);
      const fast = art === 'ausgaben' && !schlecht && anteil != null && anteil >= 90;
      return h('tr', null, h('td', null, z.posten), h('td', { class: 'r' }, inp), h('td', { class: 'r' }, eur(z.ist)),
        h('td', { class: 'r' + (schlecht ? ' diffwarn' : '') }, abw == null ? '–' : eur(abw)),
        h('td', null, anteil == null ? null : h('div', { class: 'hh-erreicht' }, h('div', { class: 'hh-balken' }, h('span', { class: schlecht ? 'zu-viel' : fast ? 'fast' : '', style: `width:${Math.min(100, anteil)}%` })), h('i', null, (anteil >= 150 ? '≥ ' : '') + anteil + ' %'))));
    });
    return h('div', { class: 'card tablewrap', style: 'margin-bottom:14px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, [titel, 'Plan', 'Ist', 'Abweichung', 'erreicht'].map((t) => h('th', null, t)))),
      h('tbody', null, rows, h('tr', { class: 'summe' }, h('td', null, h('b', null, 'Summe')), h('td', { class: 'r' }, h('b', null, eur(soll))), h('td', { class: 'r' }, h('b', null, eur(ist))), h('td', { class: 'r', title: 'nur Posten mit Plan' }, eur(abwSumme)), h('td')))));
  };
  const sammeln = () => {
    const out = { einnahmen: {}, ausgaben: {} };
    for (const art of ['einnahmen', 'ausgaben']) for (const [p, inp] of eingaben[art]) {
      if (!inp.value.trim()) continue;
      const v = parseNum(inp.value);
      if (Number.isNaN(v) || v < 0) throw new Error(`Bitte bei »${p}« einen Betrag ab 0 eintragen.`);
      out[art][p] = v;
    }
    return out;
  };
  const speichern = async () => {
    let werte;
    try { werte = sammeln(); } catch (e) { toast(e.message, 'err'); return; }
    try { S.haushalt = await api('PUT', `/api/admin/haushalt?year=${jahr}`, { ...werte, notiz: notiz.value }); toast('Haushaltsplan gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };
  const ausVorjahr = async () => {
    try {
      const vj = await api('GET', `/api/admin/haushalt?year=${jahr - 1}`);
      for (const art of ['einnahmen', 'ausgaben']) for (const z of vj[art]) { const inp = eingaben[art].get(z.posten); if (inp && z.ist) inp.value = numIn(Math.round(z.ist)); }
      toast('Ist-Werte des Vorjahres eingesetzt (gerundet) – anpassen und speichern', 'ok');
    } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('p', { class: 'hint' }, 'Plane Einnahmen und Ausgaben des Jahres. Das Ist kommt laufend aus dem Kassenbericht (Rechnungen, Spenden, sonstige Ausgaben nach Kategorie) – so sieht man jederzeit, wo der Verein steht. Praktisch für Mitgliederversammlung und Kassenprüfer.'),
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel,
      d.vorjahr ? h('button', { class: 'btn', onclick: ausVorjahr }, `Plan aus dem Ist ${jahr - 1} vorschlagen`) : null,
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', onclick: () => pdfDrucken({ url: `/api/admin/haushalt/pdf?year=${jahr}`, oeffnenUrl: `/api/admin/haushalt/pdf/oeffnen?year=${jahr}`, titel: 'Haushaltsplan', meldung: () => 'Der Haushaltsplan wurde im PDF-Programm geöffnet.' }) }, 'Als PDF')),
    tabelle('einnahmen', 'Einnahmen', d.einnahmen),
    tabelle('ausgaben', 'Ausgaben', d.ausgaben),
    h('div', { class: 'card' }, h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Erläuterungen'), notiz)),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: speichern }, 'Plan speichern'))));
}

// ------------------------------------------------------------------ Vorauszahlungen per Überweisung

async function loadVz(jahr) {
  if (S.vzLoading) return;
  S.vzLoading = true;
  try { S.vz = await api('GET', `/api/admin/vorauszahlungen?jahr=${jahr}`); } catch (e) { handleErr(e); S.vz = { jahr, zeilen: [] }; }
  S.vzLoading = false;
  render();
}

function vzZahlungDialog(z, jahr) {
  const f = { datum: h('input', { type: 'date', value: todayIso() }), betrag: h('input', { type: 'text', inputmode: 'decimal', value: z.soll ? numIn(z.soll) : '' }),
    jahr: h('select', null, [jahr - 1, jahr, jahr + 1].map((j) => h('option', { value: j, selected: j === jahr }, String(j)))),
    notiz: h('input', { type: 'text', maxlength: 200, placeholder: 'z. B. bar beim Arbeitseinsatz' }) };
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Vorauszahlung: ${z.name}`, h('div', { class: 'form' },
    fld('Eingegangen am', f.datum), fld('Betrag (€)', f.betrag, z.soll ? `Monatsbetrag ${eur(z.soll)} – mehrere Monate auf einmal gehen auch` : null),
    fld('Für das Abrechnungsjahr', f.jahr), fld('Notiz', f.notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const b = parseNum(f.betrag.value);
    if (!b || Number.isNaN(b) || b <= 0) { toast('Bitte einen Betrag eintragen.', 'err'); return false; }
    try {
      const v = await api('POST', '/api/admin/vorauszahlungen', { paechterId: z.paechterId, datum: f.datum.value, betrag: b, jahr: Number(f.jahr.value), notiz: f.notiz.value });
      toast(v.gebucht ? 'Gespeichert' : `Gespeichert – vorgemerkt, wird beim Jahreswechsel auf ${v.jahr} gebucht`, 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

async function vzZahlungenDialog(z) {
  await modal(`Zahlungen ${z.name}`, z.zahlungen.length ? h('div', { class: 'tablewrap' }, h('table', null,
    h('thead', null, h('tr', null, ['Datum', 'Betrag', 'Herkunft', ''].map((x) => h('th', null, x)))),
    h('tbody', null, z.zahlungen.map((v) => h('tr', null, h('td', null, deDate(v.datum)), h('td', { class: 'num' }, eur(v.betrag)),
      h('td', { class: 'klein' }, (v.quelle === 'kontoauszug' ? 'Kontoauszug' : 'von Hand') + (v.gebucht ? '' : ' · vorgemerkt') + (v.notiz ? ' · ' + v.notiz : '')),
      h('td', null, h('button', { class: 'btn small danger', onclick: async () => {
        if (!(await confirmBox(`Zahlung vom ${deDate(v.datum)} über ${eur(v.betrag)} löschen? Sie wird auch vom Abschlag der Jahresrechnung abgezogen.`, 'Löschen', true))) return;
        try { await api('DELETE', `/api/admin/vorauszahlungen/${v.id}`); toast('Gelöscht', 'ok'); S.vz = null; document.querySelector('dialog[open] .btn.primary')?.click(); await reload(); } catch (e) { handleErr(e); }
      } }, 'Löschen')))))))
    : h('p', { class: 'hint' }, 'Noch keine Zahlungen in diesem Jahr.'),
  [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function adminVorauszahlung() {
  const jahrHeute = S.state.currentYear; // Abrechnungsjahr: darauf werden die Vorauszahlungen gebucht
  const jahr = S.vzJahr || jahrHeute;
  if (S.vz == null || S.vz.jahr !== jahr) { if (!S.vzLoading) loadVz(jahr); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.vz;
  const jahrSel = h('select', { onchange: (e) => { S.vzJahr = Number(e.target.value); S.vz = null; render(); } },
    [jahrHeute - 1, jahrHeute, jahrHeute + 1].map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  const sollSpeichern = async (z, feld) => {
    const b = parseNum(feld.value) ?? 0;
    if (Number.isNaN(b) || b < 0) { toast('Bitte einen gültigen Betrag eintragen.', 'err'); feld.value = z.soll ? numIn(z.soll) : ''; return; }
    if (b === z.soll) return;
    try { await api('PUT', '/api/admin/vorauszahlungen/soll', { paechterId: z.paechterId, betrag: b }); toast('Monatsbetrag gespeichert', 'ok'); S.vz = null; render(); } catch (e) { handleErr(e); }
  };
  const zelle = (z, m) => {
    const wert = z.monate[m];
    const faellig = m < d.faelligeMonate;
    const cls = !z.soll ? 'vz-leer' : wert >= 0.999 ? 'vz-ok' : wert > 0 ? 'vz-teil' : faellig ? 'vz-offen' : 'vz-kommt';
    return h('td', { class: 'vz-zelle' }, h('span', { class: 'vz-punkt ' + cls, title: `${MONATE[m]}: ${!z.soll ? 'kein Monatsbetrag' : wert >= 0.999 ? 'bezahlt' : wert > 0 ? 'teilweise' : faellig ? 'offen' : 'noch nicht fällig'}` }));
  };
  const erinnerung = (z) => pdfDrucken({ url: `/api/admin/vorauszahlungen/erinnerung?paechter=${z.paechterId}&jahr=${jahr}`, oeffnenUrl: `/api/admin/vorauszahlungen/erinnerung/oeffnen?paechter=${z.paechterId}&jahr=${jahr}`,
    titel: `Erinnerung ${z.name}`, meldung: () => 'Die Erinnerung wurde im PDF-Programm geöffnet – dort drucken oder verschicken.' });
  const mitSoll = d.zeilen.filter((z) => z.soll > 0).length;
  const sepaSchalter = h('input', { type: 'checkbox', checked: !!S.state.sepaAktiv });
  sepaSchalter.addEventListener('change', async () => {
    try {
      const sepa = await api('GET', '/api/admin/sepa');
      await api('PUT', '/api/admin/sepa/einstellungen', { ...sepa.einstellungen, aktiv: sepaSchalter.checked });
      await reload();
    } catch (e) { handleErr(e); sepaSchalter.checked = !sepaSchalter.checked; }
  });
  if (!d.zeilen.length) {
    return h('div', null, leer('euro', 'Noch keine Pächter für Vorauszahlungen', 'Vorauszahlungen werden je Pächter mit einem Monatsbetrag geplant. Lege zuerst die Pächter an.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }));
  }
  return h('div', null,
    h('p', { class: 'hint' }, 'Monatliche Vorauszahlungen, die Pächter selbst überweisen (z. B. per Dauerauftrag). Monatsbetrag je Pächter eintragen; eingegangene Zahlungen erkennt der Kontoauszug (Name und Betrag), sonst »+ Zahlung«. Alles wird als Abschlag auf die Jahresrechnung gebucht.'),
    d.vorgemerkt ? h('div', { class: 'banner info' }, `${anz(d.vorgemerkt, 'Zahlung', 'Zahlungen')} für ${jahr} ${d.vorgemerkt === 1 ? 'ist' : 'sind'} vorgemerkt – das Abrechnungsjahr ist im Programm noch nicht angelegt. Sie werden beim Jahreswechsel automatisch gebucht.`) : null,
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel,
      h('button', { class: 'btn', onclick: () => { S.tab = 'zahlungen'; render(); }, title: 'Unter »Zahlungen« → »Kontoauszug einlesen«' }, 'Kontoauszug einlesen →'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${mitSoll} mit Monatsbetrag · eingegangen ${eur(d.summe)}${d.rueckstand ? ' · Rückstand ' + eur(d.rueckstand) : ''}`)),
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data vz-tab' },
      h('thead', null, h('tr', null, h('th', null, 'Garten'), h('th', null, 'Pächter'), h('th', null, 'je Monat'), ...MONATE.map((m) => h('th', { class: 'vz-zelle' }, m.slice(0, 3))),
        h('th', null, 'Eingang'), h('th', null, 'Rückstand'), h('th', null, ''))),
      h('tbody', null, d.zeilen.map((z) => {
        const feld = h('input', { type: 'text', inputmode: 'decimal', class: 'std-in', value: z.soll ? numIn(z.soll) : '', placeholder: '–', 'aria-label': `Monatsbetrag ${z.name}` });
        feld.addEventListener('change', () => sollSpeichern(z, feld));
        return h('tr', null, h('td', null, z.gartennr), h('td', null, z.name), h('td', null, feld), ...z.monate.map((_, m) => zelle(z, m)),
          h('td', { class: 'num' }, z.summe ? eur(z.summe) : '–'), h('td', { class: 'num' }, z.rueckstand ? h('b', { class: 'warn-text' }, eur(z.rueckstand)) : '–'),
          h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: async () => { if (await vzZahlungDialog(z, jahr)) { S.vz = null; await reload(); } } }, '+ Zahlung'), ' ',
            mehrMenue([
              { label: `Zahlungen ansehen (${z.zahlungen.length})`, aktion: () => vzZahlungenDialog(z) },
              z.rueckstand ? { label: 'Erinnerung schreiben', aktion: () => erinnerung(z) } : null])));
      })))),
    h('div', { class: 'vz-legende hint' }, h('span', { class: 'vz-punkt vz-ok' }), ' bezahlt ', h('span', { class: 'vz-punkt vz-teil' }), ' teilweise ',
      h('span', { class: 'vz-punkt vz-offen' }), ' offen ', h('span', { class: 'vz-punkt vz-kommt' }), ' noch nicht fällig (ab dem 10. eines Monats fällig)'),
    h('details', { class: 'card', style: 'margin-top:16px' }, h('summary', null, 'Zahlt euer Verein (auch) per Lastschrift?'),
      h('p', { class: 'hint' }, 'Dann lässt sich unter »Zahlungen« der Reiter »Lastschrift« einschalten: Mandate, Datei für das Online-Banking (SEPA), Vorabankündigung.'),
      h('label', { class: 'check' }, sepaSchalter, ' Reiter »Lastschrift« unter »Zahlungen« anzeigen')));
}

// ------------------------------------------------------------------ SEPA-Lastschrift

async function loadSepa() {
  if (S.sepaLoading) return;
  S.sepaLoading = true;
  try { S.sepa = await api('GET', '/api/admin/sepa'); } catch (e) { handleErr(e); S.sepa = { mandate: [], laeufe: [], einstellungen: {} }; }
  S.sepaLoading = false;
  render();
}
const isoTag = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ibanKurz = (iban) => (iban && iban.length > 8 ? `${iban.slice(0, 4)} … ${iban.slice(-4)}` : iban || '');

// Vorschlag fürs Fälligkeitsdatum: Vorauszahlung zum Fälligkeitstag des nächsten möglichen Monats, Rechnungen in einer Woche
function sepaFaelligVorschlag(art, tag) {
  const werktag = (d) => { while (d.getDay() === 0 || d.getDay() === 6) d.setDate(d.getDate() + 1); return d; };
  const frueh = new Date(); frueh.setDate(frueh.getDate() + 3);
  if (art === 'rechnungen') { const d = new Date(); d.setDate(d.getDate() + 7); return isoTag(werktag(d)); }
  let d = werktag(new Date(frueh.getFullYear(), frueh.getMonth(), tag || 1));
  if (d < frueh) d = werktag(new Date(frueh.getFullYear(), frueh.getMonth() + 1, tag || 1));
  return isoTag(d);
}

function mandatDialog(m) {
  const st = S.state;
  const isNew = !m || !m.id;
  const cur = { paechterId: '', kontoinhaber: '', iban: '', bic: '', unterschrieben: todayIso(), monatsbetrag: 0, rechnungen: true, referenz: '', widerrufen: '', ...(m || {}) };
  const genutzt = !!cur.zuletztGenutzt;
  const f = {
    paechter: h('select', { disabled: !isNew }, [h('option', { value: '' }, '– Pächter wählen –'),
      ...st.paechter.filter((p) => !p.geloescht && !p.ausgeschiedenAm).map((p) => h('option', { value: p.id, selected: p.id === cur.paechterId }, `${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name}`))]),
    inhaber: h('input', { type: 'text', value: cur.kontoinhaber, maxlength: 70, placeholder: 'leer = Name des Pächters' }),
    iban: h('input', { type: 'text', value: cur.iban, maxlength: 42, placeholder: 'DE12 3456 …', autocomplete: 'off' }),
    bic: h('input', { type: 'text', value: cur.bic || '', maxlength: 11, placeholder: 'bei deutschen Konten nicht nötig' }),
    datum: h('input', { type: 'date', value: cur.unterschrieben, disabled: genutzt }),
    betrag: h('input', { type: 'text', inputmode: 'decimal', value: cur.monatsbetrag ? numIn(cur.monatsbetrag) : '', placeholder: '0,00' }),
    rechnungen: h('input', { type: 'checkbox', checked: !!cur.rechnungen }),
    referenz: h('input', { type: 'text', value: cur.referenz, maxlength: 35, disabled: genutzt, placeholder: 'wird automatisch vergeben' }),
    widerrufen: h('input', { type: 'date', value: cur.widerrufen || '' }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Pächter *', f.paechter, null, true), fld('Kontoinhaber', f.inhaber), fld('IBAN *', f.iban), fld('BIC', f.bic),
    fld('Unterschrieben am *', f.datum), fld('Monatliche Vorauszahlung (€)', f.betrag, '0 = keine Vorauszahlung'),
    h('div', { class: 'field wide' }, h('label', { class: 'check' }, f.rechnungen, ' Auch den Restbetrag der Jahresrechnung einziehen')),
    fld('Mandatsreferenz', f.referenz, genutzt ? 'schon genutzt – nicht mehr änderbar' : null),
    isNew ? null : fld('Widerrufen am', f.widerrufen, 'leer = gültig'));
  return modal(isNew ? 'Lastschriftmandat anlegen' : 'Lastschriftmandat bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const betrag = parseNum(f.betrag.value) ?? 0;
      if (Number.isNaN(betrag) || betrag < 0) { toast('Bitte einen gültigen Monatsbetrag eintragen.', 'err'); return false; }
      const daten = { ...cur, paechterId: f.paechter.value, kontoinhaber: f.inhaber.value, iban: f.iban.value, bic: f.bic.value, unterschrieben: f.datum.value,
        monatsbetrag: betrag, rechnungen: f.rechnungen.checked, referenz: f.referenz.value, widerrufen: f.widerrufen.value };
      try {
        if (isNew) await api('POST', '/api/admin/sepa/mandate', daten); else await api('PUT', `/api/admin/sepa/mandate/${cur.id}`, daten);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

const mandatsformularDrucken = (pid) => pdfDrucken({
  url: `/api/admin/sepa/formular?paechter=${encodeURIComponent(pid)}`, oeffnenUrl: `/api/admin/sepa/formular/oeffnen?paechter=${encodeURIComponent(pid)}`,
  titel: 'SEPA-Lastschriftmandat', meldung: () => 'Das Mandat wurde im PDF-Programm geöffnet – drucken und unterschreiben lassen.',
});
const vorabDrucken = (jahr) => pdfDrucken({
  url: `/api/admin/sepa/vorab?jahr=${jahr}`, oeffnenUrl: `/api/admin/sepa/vorab/oeffnen?jahr=${jahr}`,
  titel: `Vorabankündigung ${jahr}`, meldung: () => 'Die Vorabankündigungen wurden im PDF-Programm geöffnet – je Pächter eine Seite.',
});

async function laufDatei(l) {
  if (inNativeWindow()) {
    try { const r = await api('POST', `/api/admin/sepa/lauf/${l.id}/datei`); toast(`${r.datei} liegt im Ordner »Lastschriften« – im Online-Banking als SEPA-Sammellastschrift hochladen.`, 'ok'); } catch (e) { handleErr(e); }
  } else location.href = `/api/admin/sepa/lauf/${encodeURIComponent(l.id)}/xml`;
}

async function laufDetails(l) {
  const zeilen = l.posten.map((x) => h('tr', { class: x.zurueck ? 'vorbei' : '' },
    h('td', null, x.name), h('td', { class: 'klein' }, x.referenz), h('td', { class: 'num' }, eur(x.betrag)),
    h('td', null, x.zurueck ? h('span', { class: 'badge warn' }, `zurück ${deDate(x.zurueck)}`) : h('button', { class: 'btn small', onclick: async () => {
      if (!(await confirmBox(`Lastschrift von ${x.name} über ${eur(x.betrag)} ist zurückgegangen? Die Buchung wird zurückgenommen${l.art === 'rechnungen' ? ' (Rechnung wieder offen)' : ' (Vorauszahlung nicht verrechnet)'}.`, 'Rücklastschrift buchen', true))) return;
      try { await api('POST', `/api/admin/sepa/lauf/${l.id}/rueck`, { endToEnd: x.endToEnd }); toast('Rücklastschrift gebucht', 'ok'); S.sepa = null; await reload(); } catch (e) { handleErr(e); }
    } }, 'Rücklastschrift'))));
  await breiterDialog(`Lastschrift zum ${deDate(l.faellig)}`, h('div', { class: 'tablewrap', style: 'max-height:420px;overflow:auto' }, h('table', null,
    h('thead', null, h('tr', null, ['Kontoinhaber', 'Mandat', 'Betrag', ''].map((x) => h('th', null, x)))), h('tbody', null, zeilen))),
  [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function adminSepa() {
  if (S.sepa == null) { if (!S.sepaLoading) loadSepa(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.sepa, e = d.einstellungen || {};
  const heute = new Date();
  const glaeubiger = h('input', { type: 'text', value: e.glaeubigerId || '', maxlength: 35, placeholder: 'DE98ZZZ09999999999' });
  const tagSel = h('select', null, Array.from({ length: 28 }, (_, i) => h('option', { value: i + 1, selected: (e.faelligkeitstag || 1) === i + 1 }, `${i + 1}.`)));
  const einstSpeichern = async () => {
    try { await api('PUT', '/api/admin/sepa/einstellungen', { glaeubigerId: glaeubiger.value, faelligkeitstag: Number(tagSel.value) }); toast('Gespeichert', 'ok'); S.sepa = null; render(); } catch (er) { handleErr(er); }
  };
  const art = h('select', null, [['vorauszahlung', 'Monatliche Vorauszahlung'], ['rechnungen', 'Jahresrechnungen (Restbetrag)']].map(([v, t]) => h('option', { value: v, selected: (S.sepaArt || 'vorauszahlung') === v }, t)));
  const faellig = h('input', { type: 'date', value: S.sepaFaellig || sepaFaelligVorschlag(S.sepaArt || 'vorauszahlung', e.faelligkeitstag) });
  art.addEventListener('change', () => { S.sepaArt = art.value; S.sepaFaellig = null; S.sepaVorschau = null; render(); });
  faellig.addEventListener('change', () => { S.sepaFaellig = faellig.value; S.sepaVorschau = null; render(); });
  const vorschau = async () => {
    try { S.sepaVorschau = await api('GET', `/api/admin/sepa/vorschau?art=${art.value}&faellig=${faellig.value}`); render(); } catch (er) { handleErr(er); }
  };
  const erstellen = async () => {
    const v = S.sepaVorschau;
    if (!(await confirmBox(`${anz(v.posten.length, 'Lastschrift', 'Lastschriften')} über ${eur(v.summe)} zum ${deDate(v.faellig)} erstellen? ${v.art === 'vorauszahlung' ? 'Die Beträge werden als Vorauszahlung auf die Jahresrechnung gebucht.' : 'Die Rechnungen werden als bezahlt vermerkt.'} Danach die Datei im Online-Banking hochladen.`, 'Erstellen'))) return;
    try {
      const l = await api('POST', '/api/admin/sepa/lauf', { art: v.art, faellig: v.faellig });
      S.sepaVorschau = null; S.sepa = null; await reload();
      await laufDatei(l);
    } catch (er) { handleErr(er); }
  };
  const v = S.sepaVorschau;
  const status = (m) => (m.widerrufen ? h('span', { class: 'badge' }, `widerrufen ${deDate(m.widerrufen)}`) : m.verfallen ? h('span', { class: 'badge err' }, 'verfallen') : h('span', { class: 'badge ok' }, 'gültig'));
  const neuMandat = async () => { if (await mandatDialog(null)) { S.sepa = null; render(); } };
  return h('div', null,
    h('p', { class: 'hint' }, 'Monatliche Vorauszahlungen und die Jahresrechnung per SEPA-Lastschrift einziehen. Das Programm erstellt die Datei für das Online-Banking und bucht gleich mit: Vorauszahlungen werden auf der Jahresrechnung abgezogen, eingezogene Rechnungen als bezahlt vermerkt.'),
    d.hinweis ? h('div', { class: 'banner warn' }, d.hinweis) : null,
    h('details', { class: 'card', open: !!d.hinweis },
      h('summary', null, h('b', null, 'Einstellungen'), e.glaeubigerId ? h('span', { class: 'hint' }, ` · Gläubiger-ID ${e.glaeubigerId} · Vorauszahlung zum ${e.faelligkeitstag || 1}. des Monats`) : null),
      h('div', { class: 'form', style: 'margin-top:10px' },
        h('div', { class: 'field' }, h('label', null, 'Gläubiger-Identifikationsnummer'), glaeubiger, h('span', { class: 'fhint' }, 'beantragt der Verein kostenlos online bei der Deutschen Bundesbank')),
        h('div', { class: 'field' }, h('label', null, 'Vorauszahlung fällig am'), tagSel, h('span', { class: 'fhint' }, 'Tag im Monat')),
        h('div', { class: 'field' }, h('label', null, ' '), h('button', { class: 'btn primary', onclick: einstSpeichern }, 'Speichern')))),

    h('h3', null, 'Lastschrift erstellen'),
    h('div', { class: 'card' },
      h('div', { class: 'toolbar' }, h('label', null, 'Art'), art, h('label', null, 'Fällig am'), faellig,
        h('button', { class: 'btn', disabled: !!d.hinweis, onclick: vorschau }, 'Vorschau')),
      v ? h('div', null,
        v.hinweise.length ? h('div', { class: 'banner info' }, h('b', null, 'Nicht dabei: '), v.hinweise.join(' · ')) : null,
        v.posten.length ? [
          h('div', { class: 'tablewrap', style: 'max-height:300px;overflow:auto' }, h('table', null,
            h('thead', null, h('tr', null, ['Kontoinhaber', 'IBAN', 'Verwendungszweck', 'Betrag'].map((x) => h('th', null, x)))),
            h('tbody', null, v.posten.map((x) => h('tr', null, h('td', null, x.name), h('td', { class: 'klein' }, ibanKurz(x.iban)), h('td', { class: 'klein wrap' }, x.zweck), h('td', { class: 'num' }, eur(x.betrag))))))),
          h('div', { class: 'toolbar', style: 'margin-top:10px' }, h('b', null, `${anz(v.posten.length, 'Lastschrift', 'Lastschriften')} · ${eur(v.summe)}`), h('div', { class: 'spacer' }),
            h('button', { class: 'btn primary', onclick: erstellen }, 'Lauf erstellen und Datei speichern'))]
          : h('p', { class: 'hint' }, 'Nichts einzuziehen.')) : h('p', { class: 'hint' }, 'Fälligkeitsdatum wählen und »Vorschau« klicken. Die Bank braucht die Datei rechtzeitig vor dem Termin (meist einen Bankarbeitstag vorher). Pächter müssen die Termine vorher kennen – dafür gibt es die Vorabankündigung.')),

    h('h3', null, `Mandate (${d.mandate.length})`),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: neuMandat }, '+ Mandat erfassen'),
      h('button', { class: 'btn', disabled: !!d.hinweis, title: 'Je Pächter mit Vorauszahlung eine Seite mit allen Abbuchungsterminen', onclick: () => vorabDrucken(heute.getFullYear()) }, `Vorabankündigung ${heute.getFullYear()}`),
      heute.getMonth() >= 9 ? h('button', { class: 'btn', disabled: !!d.hinweis, onclick: () => vorabDrucken(heute.getFullYear() + 1) }, `Vorabankündigung ${heute.getFullYear() + 1}`) : null),
    d.mandate.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Garten', 'Kontoinhaber', 'Mandatsreferenz', 'IBAN', 'Monatlich', 'Jahresrechnung', 'Status', ''].map((x) => h('th', null, x)))),
      h('tbody', null, d.mandate.map((m) => h('tr', null,
        h('td', null, m.gartennr), h('td', null, m.kontoinhaber, m.name && m.name !== m.kontoinhaber ? h('div', { class: 'hint' }, m.name) : null),
        h('td', { class: 'klein' }, m.referenz, h('div', { class: 'hint' }, `unterschrieben ${deDate(m.unterschrieben)}`)),
        h('td', { class: 'klein' }, ibanKurz(m.iban)), h('td', { class: 'num' }, m.monatsbetrag ? eur(m.monatsbetrag) : '–'),
        h('td', null, m.rechnungen ? 'ja' : 'nein'), h('td', null, status(m)),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', onclick: async () => { if (await mandatDialog(m)) { S.sepa = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([
            { label: 'Mandatsformular drucken', aktion: () => mandatsformularDrucken(m.paechterId) },
            m.zuletztGenutzt ? null : { label: 'Löschen', gefahr: true, aktion: async () => {
              if (!(await confirmBox(`Mandat ${m.referenz} löschen?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/sepa/mandate/${m.id}`); S.sepa = null; render(); } catch (er) { handleErr(er); }
            } }])))))))
      : h('div', { class: 'card' }, leer('euro', 'Noch keine Mandate', 'Pächter, die per Lastschrift zahlen möchten, unterschreiben ein SEPA-Mandat. Das Formular druckst du nach dem Anlegen über »Mehr« oder vorab für jeden Pächter.', '+ Mandat erfassen', neuMandat)),

    d.laeufe.length ? [h('h3', null, 'Bisherige Lastschriften'),
      h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Fällig', 'Art', 'Posten', 'Summe', ''].map((x) => h('th', null, x)))),
        h('tbody', null, d.laeufe.map((l) => h('tr', null,
          h('td', null, deDate(l.faellig)), h('td', null, l.art === 'vorauszahlung' ? 'Vorauszahlung' : 'Jahresrechnungen'),
          h('td', null, String(l.posten.length), l.zurueckAnzahl ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, `${l.zurueckAnzahl} zurück`) : null),
          h('td', { class: 'num' }, eur(l.summe)),
          h('td', { class: 'aktionen' }, h('button', { class: 'btn small', disabled: l.bereinigt, onclick: () => laufDatei(l) }, 'Datei'), ' ',
            h('button', { class: 'btn small', disabled: l.bereinigt, onclick: () => laufDetails(l) }, 'Einzelposten')))))))] : null,
    h('p', { class: 'hint', style: 'margin-top:14px' }, 'Gut zu wissen: Ein Mandat verfällt, wenn 36 Monate lang nicht damit eingezogen wurde. Zahlungspflichtige können eine Lastschrift innerhalb von acht Wochen zurückgeben – dann hier unter »Einzelposten« die Rücklastschrift buchen. Die Datei hat das Format pain.008 (SEPA-Basislastschrift), das deutsche Banken im Online-Banking annehmen.'));
}

// ------------------------------------------------------------------ Kontoauszug einlesen

const KONTO_STATUS = {
  passt: ['ok', 'Nummer und Betrag passen'], teilbetrag: ['warn', 'Teil- oder Mehrzahlung'], vorschlag: ['neu', 'Vorschlag – bitte prüfen'],
  vorauszahlung: ['ok', 'Vorauszahlung'], spende: ['neu', 'Spende?'], bezahlt: ['', 'schon bezahlt'], doppelt: ['', 'schon übernommen'], offen: ['', 'nicht zugeordnet'],
};

function kontoauszugEinlesen() {
  const datei = h('input', { type: 'file', accept: '.csv,.txt,text/csv', style: 'display:none' });
  datei.addEventListener('change', async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    let v;
    try { v = await api('POST', '/api/admin/kontoauszug', fd, true); } catch (e) { handleErr(e); return; } finally { datei.remove(); }
    if (await kontoauszugVorschau(v)) await reload();
  });
  document.body.append(datei);
  datei.click();
}

function kontoauszugVorschau(v) {
  if (!v.buchungen.length) { toast(`Keine Eingänge in der Datei gefunden${v.ausgaben ? ` (${v.ausgaben} Abbuchungen übersprungen)` : ''}.`, 'err'); return false; }
  const optionen = (sel) => [h('option', { value: '' }, '– nicht zuordnen –'), h('option', { value: 'spende', selected: sel === 'spende' }, '→ als Spende erfassen'),
    ...(v.vorauszahler || []).map((p) => h('option', { value: 'vz:' + p.id, selected: sel === 'vz:' + p.id }, `→ Vorauszahlung: ${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name} (${eur(p.soll)}/Monat)`)),
    ...v.offene.map((r) => h('option', { value: r.id, selected: r.id === sel }, `${r.nummer} · ${r.name} · offen ${eur(r.offen)}${r.jahr ? ` (${r.jahr})` : ''}`))];
  const zeilen = v.buchungen.map((b) => {
    const fest = b.status === 'doppelt' || b.status === 'bezahlt';
    const wahl = h('select', { disabled: fest, class: 'konto-wahl' }, optionen(b.status === 'spende' ? 'spende' : b.status === 'vorauszahlung' ? 'vz:' + b.paechterId : b.rechnungId));
    const an = h('input', { type: 'checkbox', disabled: fest, checked: b.status === 'passt' || b.status === 'teilbetrag' || b.status === 'vorauszahlung', 'aria-label': 'übernehmen' });
    wahl.addEventListener('change', () => { an.checked = !!wahl.value; zaehlen(); });
    an.addEventListener('change', () => { if (an.checked && !wahl.value) { an.checked = false; toast('Bitte zuerst eine Rechnung auswählen.', 'err'); } zaehlen(); });
    const [cls, text] = KONTO_STATUS[b.status] || ['', b.status];
    return { b, wahl, an, tr: h('tr', { class: fest ? 'vorbei' : '' },
      h('td', null, an), h('td', null, deDate(b.datum)),
      h('td', { class: 'wrap klein' }, h('b', null, b.name || '–'), h('div', { class: 'hint' }, b.zweck)),
      h('td', { class: 'num' }, eur(b.betrag)),
      h('td', null, wahl, h('div', null, h('span', { class: 'badge ' + cls }, text), b.hinweis ? h('span', { class: 'hint', style: 'margin-left:6px' }, b.hinweis) : null))) };
  });
  const zaehlerEl = h('b', null);
  const zaehlen = () => {
    const l = zeilen.filter((z) => z.an.checked);
    zaehlerEl.textContent = `${anz(l.length, 'Zahlung', 'Zahlungen')} · ${eur(l.reduce((a, z) => a + z.b.betrag, 0))}`;
  };
  zaehlen();
  const n = (st) => v.buchungen.filter((b) => b.status === st).length;
  const body = h('div', null,
    h('p', { class: 'hint' }, `${v.buchungen.length} Eingänge gelesen${v.ausgaben ? `, ${v.ausgaben} Abbuchungen übersprungen` : ''}: ${n('passt')} passen genau, ${n('teilbetrag')} Teilzahlungen, ${n('vorschlag')} Vorschläge über den Namen, ${n('vorauszahlung')} Vorauszahlungen, ${n('spende')} mögliche Spenden, ${n('offen')} nicht zugeordnet${n('doppelt') ? `, ${n('doppelt')} schon früher übernommen` : ''}. Übernommen wird nur, was angehakt ist. Die Datei selbst wird nicht gespeichert.`),
    h('div', { class: 'toolbar' }, h('div', { class: 'spacer' }), zaehlerEl),
    h('div', { class: 'tablewrap', style: 'max-height:420px;overflow:auto' }, h('table', { class: 'konto-tab' },
      h('thead', null, h('tr', null, ['', 'Datum', 'Von / Verwendungszweck', 'Betrag', 'Rechnung'].map((x) => h('th', null, x)))),
      h('tbody', null, zeilen.map((z) => z.tr)))));
  return breiterDialog('Kontoauszug einlesen', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Angehakte übernehmen', cls: 'primary', value: true, action: async () => {
      const zahlungen = zeilen.filter((z) => z.an.checked && z.wahl.value).map((z) => {
        const basis = { datum: z.b.datum, betrag: z.b.betrag, fingerprint: z.b.fingerprint };
        if (z.wahl.value === 'spende') return { ...basis, spende: true, name: z.b.name };
        if (z.wahl.value.startsWith('vz:')) return { ...basis, vorauszahlung: z.wahl.value.slice(3), name: z.b.name };
        return { ...basis, rechnungId: z.wahl.value, name: z.b.name };
      });
      if (!zahlungen.length) { toast('Es ist nichts angehakt.', 'err'); return false; }
      try {
        const r = await api('POST', '/api/admin/kontoauszug/buchen', { zahlungen });
        toast(`${anz(r.gebucht, 'Zahlung', 'Zahlungen')} über ${eur(r.summe)} übernommen${r.vorauszahlungen ? ` – davon ${anz(r.vorauszahlungen, 'Vorauszahlung', 'Vorauszahlungen')}` : ''}${r.spenden ? ` – davon ${r.spenden} als Spende; Anschrift unter »Spenden« ergänzen` : ''}`, 'ok',
          r.spenden ? { label: 'Zu den Spenden', fn: () => zielOeffnen('admin:spenden') } : null);
      } catch (e) { handleErr(e); return false; }
    } }]);
}

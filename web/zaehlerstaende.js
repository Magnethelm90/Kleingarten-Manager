'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Zählerstände und Rechnungen. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Tab: Zählerstände

function statusBadge(res) {
  if (res.vollstaendig) return h('span', { class: 'badge ok' }, '✓ OK');
  return h('span', { class: 'badge ' + (res.status === 'Angaben fehlen' ? 'warn' : 'err'), title: res.status }, res.status === 'Angaben fehlen' ? 'unvollständig' : res.status);
}

function updateCalc(tr, res) {
  tr.querySelector('.c-wv').textContent = res.wasserVerbrauch == null ? '' : nfFlex.format(res.wasserVerbrauch);
  tr.querySelector('.c-sv').textContent = res.energieVerbrauch == null ? '' : nfFlex.format(res.energieVerbrauch);
  const g = tr.querySelector('.gesamt');
  g.className = 'gesamt' + (res.vollstaendig && res.gesamt < 0 ? ' guthaben' : '');
  g.title = res.vollstaendig && res.gesamt < 0 ? 'Guthaben zu Gunsten des Pächters' : '';
  g.textContent = res.vollstaendig ? eur(res.gesamt) : '–';
  const inf = ((S.state && S.state.issued) || {})[tr.dataset.id];
  const hints = res.hinweise || ((S.state && S.state.hinweise) || {})[tr.dataset.id];
  tr.querySelector('.status').replaceChildren(...[statusBadge(res),
    inf ? h('span', { class: 'badge ' + (inf.geaendert ? 'warn' : 'neu'), title: inf.geaendert ? 'Rechnung wurde ausgestellt, danach wurden Werte geändert' : 'Rechnung ist ausgestellt' }, inf.geaendert ? '⚠ nach Ausstellung geändert' : 'ausgestellt') : null,
    hints && hints.length ? h('span', { class: 'badge warn', title: hints.join('\n') }, 'Stimmt das? Verbrauch prüfen') : null].filter(Boolean));
}

// Nach einer Änderung neu vom Programm holen, ob eine ausgestellte Rechnung davon betroffen ist.
async function refreshIssued(tr) {
  try {
    const st = await api('GET', `/api/state?year=${S.year}`);
    S.state.issued = st.issued; S.state.archive = st.archive;
    updateCalc(tr, S.state.results[tr.dataset.id]);
  } catch (e) { /* nicht kritisch */ }
}

let savedTimer;
function showSaved() {
  const el = document.querySelector('.saveind');
  if (!el) return;
  el.classList.add('show');
  clearTimeout(savedTimer);
  savedTimer = setTimeout(() => el.classList.remove('show'), 1800);
}

function hasDetails(a) {
  return !!(a && (a.versicherung || a.grundsteuer || a.auslagen || (a.hinweis && a.hinweis.trim())));
}

async function saveRow(tr, p) {
  const f = (k) => tr.querySelector(`[data-f="${k}"]`);
  const keys = ['wasserVJ', 'wasserAkt', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
  const vals = {};
  let bad = false;
  for (const k of keys) {
    const v = parseNum(f(k).value);
    const wrong = Number.isNaN(v) || (v !== null && v < 0);
    f(k).classList.toggle('err', wrong);
    bad = bad || wrong;
    vals[k] = v;
  }
  if (bad) { toast('Bitte nur Zahlen ab 0 eingeben (Komma ist erlaubt).', 'err'); return; }
  try {
    const wz = f('wz').value.trim(), sz = f('sz').value.trim();
    if (wz !== p.wasserzaehlerNr || sz !== p.stromzaehlerNr) {
      const np = await api('PUT', `/api/zaehler/${p.id}`, { wasserzaehlerNr: wz, stromzaehlerNr: sz });
      Object.assign(p, np);
    }
    const cur = S.state.ablesungen[p.id] || {};
    const body = { ...cur, wasserVJ: vals.wasserVJ, wasserAkt: vals.wasserAkt, stromVJ: vals.stromVJ, stromAkt: vals.stromAkt,
      stunden: vals.stunden, abschlag: vals.abschlag ?? 0 };
    const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
    S.state.ablesungen[p.id] = body;
    S.state.results[p.id] = res;
    S.state.hinweise = S.state.hinweise || {};
    verbrauchToast(p, res);
    if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
    updateCalc(tr, res);
    showSaved();
    if ((S.state.issued || {})[p.id]) refreshIssued(tr);
  } catch (e) { handleErr(e); }
}

const WECHSEL_WASSER = { wKey: 'wasserWechsel', field: 'wz', label: 'Wasser', unit: 'm³' };
const WECHSEL_STROM = { wKey: 'stromWechsel', field: 'sz', label: 'Strom', unit: 'kWh' };

async function saveWechsel(tr, p, art, wechsel) {
  const cur = S.state.ablesungen[p.id] || {};
  const body = { ...cur, [art.wKey]: wechsel };
  const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
  S.state.ablesungen[p.id] = body;
  S.state.results[p.id] = res;
  S.state.hinweise = S.state.hinweise || {};
  if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
  updateCalc(tr, res);
  if (wechsel && wechsel.neueNr) {
    p[art.field === 'wz' ? 'wasserzaehlerNr' : 'stromzaehlerNr'] = wechsel.neueNr;
    const el = tr.querySelector(`[data-f="${art.field}"]`);
    if (el) el.value = wechsel.neueNr;
  }
  const btn = tr.querySelector(`.wechselbtn-${art.field}`);
  if (btn) btn.classList.toggle('primary', !!wechsel);
  showSaved();
  if ((S.state.issued || {})[p.id]) refreshIssued(tr);
}

function wechselDialog(tr, p, art) {
  const cur = S.state.ablesungen[p.id] || {};
  const w = cur[art.wKey] || {};
  const altEnde = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.altEnde), autocomplete: 'off' });
  const neueNr = h('input', { type: 'text', maxlength: 40, value: w.neueNr || '', autocomplete: 'off' });
  const neuStart = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.neuStart != null ? w.neuStart : 0), autocomplete: 'off' });
  const fld = (label, input, unit) => h('div', { class: 'field' }, h('label', null, label),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const hatWechsel = w.altEnde != null || w.neuStart != null || (w.neueNr && w.neueNr.trim());
  const body = h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('p', { class: 'hint', style: 'margin:0' },
      `Wurde der ${art.label}zähler unterjährig getauscht? Endstand des alten und Anfangsstand des neuen Zählers eintragen – der Verbrauch wird dann aus beiden Zählern zusammengerechnet. Leer lassen, wenn es keinen Wechsel gab.`)),
    fld('Endstand alter Zähler', altEnde, art.unit), fld('Neue Zähler-Nr.', neueNr), fld('Anfangsstand neuer Zähler', neuStart, art.unit));
  return modal(`Zählerwechsel ${art.label} – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    hatWechsel ? { label: 'Wechsel entfernen', cls: 'danger', value: 'del', action: async () => {
      try { await saveWechsel(tr, p, art, null); } catch (e) { handleErr(e); return false; }
    } } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const ae = parseNum(altEnde.value), ns = parseNum(neuStart.value);
      const leer = ae === null && ns === null && !neueNr.value.trim();
      if (!leer && (Number.isNaN(ae) || (ae !== null && ae < 0) || Number.isNaN(ns) || (ns !== null && ns < 0))) {
        toast('Bitte gültige Zahlen ab 0 eingeben.', 'err'); return false;
      }
      try { await saveWechsel(tr, p, art, leer ? null : { altEnde: ae, neueNr: neueNr.value.trim(), neuStart: ns }); } catch (e) { handleErr(e); return false; }
    } },
  ].filter(Boolean));
}

async function detailsDialog(tr, p) {
  const cur = S.state.ablesungen[p.id] || {};
  const num = (val, id) => h('input', { type: 'text', inputmode: 'decimal', id, value: numIn(val || null), autocomplete: 'off' });
  const vers = num(cur.versicherung, 'd-vers'), grund = num(cur.grundsteuer, 'd-grund'), ausl = num(cur.auslagen, 'd-ausl');
  const hint = h('textarea', { rows: 3, maxlength: 300, style: 'width:100%', id: 'd-hint' }, cur.hinweis || '');
  const field = (label, input, unit, text) => h('div', { class: 'field' }, h('label', null, label, text ? hilfe(text) : null),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const body = h('div', { class: 'form' },
    field('Versicherung', vers, '€', 'Anteil an einer Versicherung, die der Verein für den Garten bezahlt hat (z. B. Lauben- oder Glasversicherung) und weiterberechnet.'),
    field('Grundsteuer', grund, '€', 'Anteilige Grundsteuer, die der Verpächter dem Verein berechnet und die auf die Gärten umgelegt wird.'),
    field('Sonstige Auslagen', ausl, '€', 'Weitere Beträge nur für diesen Garten, z. B. ein vom Verein bezahlter Zählertausch. Am besten im Hinweis darunter erläutern.'),
    h('div', { class: 'field wide' }, h('label', null, 'Hinweis / Erläuterung (steht auf der Rechnung)'), hint));
  await modal(`Weitere Angaben – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const vs = [vers, grund, ausl].map((i) => parseNum(i.value));
      if (vs.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Beträge ab 0 eingeben.', 'err'); return false; }
      const body2 = { ...cur, versicherung: vs[0] ?? 0, grundsteuer: vs[1] ?? 0, auslagen: vs[2] ?? 0, hinweis: hint.value.trim() };
      try {
        const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body2);
        S.state.ablesungen[p.id] = body2;
        S.state.results[p.id] = res;
        updateCalc(tr, res);
        tr.querySelector('.detailbtn').classList.toggle('primary', hasDetails(body2));
        showSaved();
        if ((S.state.issued || {})[p.id]) refreshIssued(tr);
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function verlaufDialog(p) {
  const hist = ((S.state && S.state.history) || {})[p.id] || [];
  const row = (e) => h('tr', null,
    h('td', null, e.jahr),
    h('td', { class: 'r' }, e.wasser == null ? '–' : nfFlex.format(e.wasser) + ' m³'),
    h('td', { class: 'r' }, e.strom == null ? '–' : nfFlex.format(e.strom) + ' kWh'),
    h('td', { class: 'r' }, e.stunden == null ? '–' : nfFlex.format(e.stunden)),
    h('td', { class: 'r' }, e.gesamt == null ? '–' : eur(e.gesamt)),
    h('td', null, e.ausgestellt ? h('span', { class: 'badge ok' }, 'ausgestellt') : h('span', { class: 'badge neu' }, 'berechnet')));
  const chrono = [...hist].reverse();
  const body = hist.length
    ? h('div', null,
        h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Gesamtbetrag je Jahr:'),
        barChart(chrono.filter((e) => e.gesamt != null).map((e) => ({ label: String(e.jahr), value: e.gesamt })), { valueFmt: (v) => eur(v) }),
        h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Jahr', 'Wasser', 'Strom', 'Stunden', 'Gesamt', ''].map((t) => h('th', null, t)))),
          h('tbody', null, hist.map(row)))))
    : h('p', { class: 'hint' }, 'Für diesen Pächter liegen noch keine Werte aus Vorjahren vor.');
  return modal(`Jahresverlauf – ${p.name}`, body, [{ label: 'Schließen', value: true }]);
}

function eingabeRow(p, ro) {
  const a = S.state.ablesungen[p.id] || {};
  const res = S.state.results[p.id];
  const tr = h('tr', { 'data-id': p.id });
  const save = () => saveRow(tr, p);
  const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off',
    'data-f': key, value: numIn(val), disabled: ro, onchange: save });
  const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key,
    value: val || '', disabled: ro, onchange: save });
  tr.append(
    h('td', { class: 'sticky s1' }, p.mitgliedsnr),
    h('td', { class: 'name sticky s2', title: p.name }, p.name, schlussBadge(p)),
    h('td', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_WASSER) }, '⇄')),
    h('td', null, inp('wasserVJ', a.wasserVJ)),
    h('td', null, inp('wasserAkt', a.wasserAkt)),
    h('td', { class: 'calc c-wv' }),
    h('td', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_STROM) }, '⇄')),
    h('td', null, inp('stromVJ', a.stromVJ)),
    h('td', null, inp('stromAkt', a.stromAkt)),
    h('td', { class: 'calc c-sv' }),
    h('td', null, inp('stunden', a.stunden, 'small')),
    h('td', null, inp('abschlag', a.abschlag || null, 'small')),
    h('td', { class: 'gesamt' }),
    h('td', { class: 'status mid' }),
    h('td', null, h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro,
      title: 'Versicherung, Grundsteuer, Auslagen, Hinweis', onclick: () => detailsDialog(tr, p) }, 'Weitere'), ' ',
      h('button', { class: 'btn small', title: 'Verbrauch und Betrag der Vorjahre', onclick: () => verlaufDialog(p) }, 'Verlauf')),
  );
  updateCalc(tr, res);
  // Enter springt in die Zeile darunter (gleiche Spalte)
  tr.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
    e.preventDefault();
    e.target.dispatchEvent(new Event('change'));
    let next = tr.nextElementSibling;
    while (next && next.hidden) next = next.nextElementSibling;
    const t = next && next.querySelector(`[data-f="${e.target.dataset.f}"]`);
    if (t) { t.focus(); t.select(); }
  });
  return tr;
}

// Hinweis über den Zählerständen: Hauptzähler gegen die Summe der Gartenzähler (nur wenn ein Hauptzählerwert eingetragen ist).
function schwundBanner() {
  const st = S.state;
  if (st.hasPassword && !st.loggedIn) return null;
  if (!S.schwundEingabe || S.schwundEingabe.jahr !== st.year) {
    if (!S.schwundEingabeLaedt) {
      S.schwundEingabeLaedt = true;
      api('GET', `/api/admin/schwund?year=${st.year}`).then((d) => { S.schwundEingabe = { jahr: st.year, daten: d }; })
        .catch(() => { S.schwundEingabe = { jahr: st.year, daten: null }; })
        .finally(() => { S.schwundEingabeLaedt = false; render(); });
    }
    return null;
  }
  const d = S.schwundEingabe.daten;
  const j = d && d.jahre.find((x) => x.jahr === st.year);
  if (!j || j.wasser.prozent == null) return null;
  const w = j.wasser;
  const stufe = w.prozent >= d.warnAb ? 'err' : w.prozent < 0 ? 'warn' : 'info';
  const text = w.prozent < 0
    ? `Die Gärten haben zusammen mehr Wasser gezählt (${nfFlex.format(w.gaerten)} m³) als der Hauptzähler (${nfFlex.format(w.haupt)} m³). Prüfe, ob ein Zählerstand falsch eingetippt wurde.`
    : `Hauptzähler ${nfFlex.format(w.haupt)} m³, Gärten zusammen ${nfFlex.format(w.gaerten)} m³ – ${nfFlex.format(w.menge)} m³ (${nfFlex.format(w.prozent)} %) kommen in keinem Garten an.${w.prozent >= d.warnAb ? ' Das ist viel: undichte Leitung oder falsch abgelesener Zähler?' : ''}${j.unvollstaendig ? ` ${j.unvollstaendig === 1 ? 'Ein Garten fehlt' : j.unvollstaendig + ' Gärten fehlen'} noch – der Wert wird mit ihnen kleiner.` : ''}`;
  return h('div', { class: 'banner ' + stufe, style: 'margin-bottom:10px' }, h('b', null, 'Wasser-Vergleich: '), text);
}

function viewEingabe() {
  const st = S.state;
  return h('div', null,
    h('h2', null, `Zählerstände ${st.year}`, ' ', seitenHilfe('eingabe')),
    schwundBanner(),
    h('div', { class: 'toolbar' },
      h('div', { class: 'subtabs', style: 'margin:0' },
        h('button', { class: S.eingabeMode === 'schnell' ? 'active' : '', onclick: () => { S.eingabeMode = 'schnell'; render(); } }, 'Schnelleingabe (Nummer eintippen)'),
        h('button', { class: S.eingabeMode === 'tabelle' ? 'active' : '', onclick: () => { S.eingabeMode = 'tabelle'; render(); } }, 'Tabelle (alle Pächter)')),
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', disabled: !st.paechter.length, title: 'Liste aller Gärten mit Zählernummern und Vorjahresständen zum Ausdrucken, mit leeren Feldern für den Ablese-Rundgang',
        onclick: () => ablesebogenDrucken(st.year) }, 'Ablesebogen drucken'),
      (!st.hasPassword || st.loggedIn) ? h('button', { class: 'btn', disabled: !st.paechter.length, title: 'Je Garten ein Zettel zum Ausfüllen: Pächter tragen ihren Zählerstand ein und werfen ihn in den Vereinsbriefkasten',
        onclick: () => zaehlerzettelDialog(st.year) }, 'Zettel zum Einwerfen') : null,
      (!st.hasPassword || st.loggedIn) && modulAn('eichung') ? h('button', { class: 'btn', disabled: !st.paechter.length, title: 'Wann müssen die Zähler nach dem Eichgesetz getauscht werden?', onclick: eichungDialog }, 'Eichfristen') : null),
    S.eingabeMode === 'schnell' ? viewSchnell() : viewTabelle());
}

// Eichfristen: Eichjahr je Zähler pflegen, Status sehen, Tauschliste drucken.
async function eichungDialog() {
  let d;
  try { d = await api('GET', '/api/admin/eichung'); } catch (e) { handleErr(e); return; }
  const STATUS = { abgelaufen: ['err', 'abgelaufen'], faellig: ['warn', 'dieses Jahr tauschen'], bald: ['neu', 'nächstes Jahr'], unbekannt: ['', 'Eichjahr fehlt'], ok: ['ok', 'gültig'] };
  const box = h('div');
  const zeichne = () => {
    const fw = h('input', { type: 'number', min: 1, max: 30, value: d.fristen.wasser, style: 'width:70px' });
    const fs = h('input', { type: 'number', min: 1, max: 30, value: d.fristen.strom, style: 'width:70px' });
    const fristen = async () => {
      try { d = await api('PUT', '/api/admin/eichung/fristen', { wasser: Number(fw.value), strom: Number(fs.value) }); zeichne(); } catch (e) { handleErr(e); }
    };
    fw.addEventListener('change', fristen); fs.addEventListener('change', fristen);
    const tauschen = d.zeilen.filter((z) => ['abgelaufen', 'faellig', 'bald'].includes(z.status)).length;
    box.replaceChildren(
      h('p', { class: 'hint' }, 'Nach dem Mess- und Eichgesetz dürfen Zähler nur begrenzt messen: Kaltwasserzähler 6 Jahre, elektronische Stromzähler 8 Jahre (Ferraris-Zähler 16). Die Frist endet mit Ablauf des Jahres. Trag das Eichjahr ein (steht meist auf dem Zähler oder der Plombe) – beim Zählerwechsel setzt das Programm es selbst.'),
      h('div', { class: 'toolbar' }, h('label', null, 'Eichfrist Wasser ', fw, ' Jahre'), h('label', null, 'Strom ', fs, ' Jahre'), h('div', { class: 'spacer' }),
        h('button', { class: 'btn', disabled: !tauschen, onclick: () => pdfDrucken({ url: '/api/admin/eichung/tauschliste', oeffnenUrl: '/api/admin/eichung/tauschliste/oeffnen', titel: 'Tauschliste', meldung: () => 'Die Tauschliste wurde im PDF-Programm geöffnet.' }) },
          `Tauschliste drucken${tauschen ? ` (${tauschen})` : ''}`),
        h('button', { class: 'btn', title: 'Alle im Abrechnungsjahr getauschten Zähler mit Endstand und Anfangsstand, zum Gegenzeichnen',
          onclick: () => pdfDrucken({ url: `/api/admin/zaehlerwechsel/protokoll?year=${S.year}`, oeffnenUrl: `/api/admin/zaehlerwechsel/protokoll/oeffnen?year=${S.year}`, titel: 'Zählerwechsel-Protokoll', meldung: () => 'Das Protokoll wurde im PDF-Programm geöffnet.' }) },
        'Wechsel-Protokoll')),
      h('div', { class: 'tablewrap', style: 'max-height:55vh' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Garten', 'Pächter', 'Zähler', 'Eichjahr', 'Gültig bis', ''].map((t) => h('th', null, t)))),
        h('tbody', null, d.zeilen.map((z) => {
          const inp = h('input', { type: 'number', min: 1950, max: d.jahr + 1, value: z.eichjahr || '', placeholder: '–', style: 'width:80px' });
          inp.addEventListener('change', async () => {
            try { d = await api('PUT', `/api/admin/eichung/${z.paechterId}`, { art: z.art, eichjahr: Number(inp.value || 0) }); zeichne(); }
            catch (e) { handleErr(e); inp.value = z.eichjahr || ''; }
          });
          const [cls, txt] = STATUS[z.status] || ['', z.status];
          return h('tr', null, h('td', null, z.gartennr), h('td', null, z.name), h('td', null, (z.art === 'wasser' ? 'Wasser ' : 'Strom ') + z.nr),
            h('td', null, inp), h('td', null, z.gueltigBis ? `31.12.${z.gueltigBis}` : ''), h('td', null, h('span', { class: 'badge ' + cls }, txt)));
        })))));
  };
  zeichne();
  await breiterDialog('Eichfristen der Zähler', box, [{ label: 'Schließen', cls: 'primary', value: true }]);
  S.aufgaben = null;
}

// Schnelleingabe: Nummer eintippen, Name/Adresse/Größe/Zählernummern kommen aus den Stammdaten.
function viewSchnell() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const box = h('div');
  const sugg = h('div', { class: 'sugg' });
  const find = (q) => {
    q = q.trim().toLowerCase();
    if (!q) return [];
    const exact = list.filter((p) => (p.mitgliedsnr || '').toLowerCase() === q);
    if (exact.length) return exact;
    const g = list.filter((p) => (p.gartennr || '').toLowerCase() === q);
    if (g.length) return g;
    return list.filter((p) => [p.mitgliedsnr, p.gartennr, p.name].some((x) => (x || '').toLowerCase().includes(q))).slice(0, 8);
  };
  const choose = (p) => { S.quickId = p.id; S.quickQ = ''; input.value = ''; sugg.replaceChildren(); drawCard(true); };
  const input = h('input', { type: 'search', class: 'bignr', placeholder: 'Nummer oder Name', autocomplete: 'off', style: 'width:280px', id: 'quicknr',
    oninput: () => {
      const m = find(input.value);
      sugg.replaceChildren();
      if (input.value.trim() && !m.length) sugg.append(h('span', { class: 'hint' }, 'Nichts gefunden.'));
      if (m.length > 1 || (m.length === 1 && input.value.trim().toLowerCase() !== (m[0].mitgliedsnr || '').toLowerCase()))
        for (const p of m) sugg.append(h('button', { class: 'btn small', onclick: () => choose(p) }, `${p.mitgliedsnr}  ${p.name}`));
    },
    onkeydown: (e) => {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      const m = find(input.value);
      if (m.length === 1) choose(m[0]);
      else if (m.length > 1) toast('Mehrere Treffer – bitte einen anklicken oder die Nummer genauer eingeben.', 'err');
      else if (input.value.trim()) {
        // die Karte des vorherigen Pächters ausblenden, damit nichts versehentlich beim Falschen landet
        S.quickId = null;
        drawCard();
        toast('Nummer nicht gefunden.', 'err');
      }
    } });

  const drawCard = (focusFirst) => {
    box.replaceChildren();
    const p = list.find((x) => x.id === S.quickId);
    if (!p) {
      box.append(list.length ? h('div', { class: 'card empty' }, 'Tippe oben eine Nummer ein und drücke Enter. Alle Angaben des Pächters erscheinen dann automatisch.')
        : leer('personen', 'Noch keine Pächter angelegt', 'Lege zuerst die Pächter an oder übernimm sie aus einer Excel-Liste. Danach trägst du hier die Zählerstände ein.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }));
      return;
    }
    const a = st.ablesungen[p.id] || {};
    const card = h('div', { class: 'card quick', 'data-id': p.id });
    const save = () => saveRow(card, p);
    const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off', 'data-f': key, value: numIn(val), disabled: ro, onchange: save });
    const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key, value: val || '', disabled: ro, onchange: save });
    const fld = (label, el, unit) => h('div', { class: 'field' }, h('label', null, label), unit ? h('div', { class: 'inputunit' }, el, h('span', null, unit)) : el);
    const info = (k, v) => v ? h('span', { class: 'chip' }, h('i', null, k + ' '), v) : null;
    card.append(
      h('div', { class: 'qhead' },
        h('div', null, h('h3', null, `${p.mitgliedsnr} – ${p.name}`, schlussBadge(p)), h('div', { class: 'hint' }, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
          p.notiz ? h('div', { class: 'hint', style: 'margin-top:2px' }, 'Notiz: ', p.notiz) : null),
        h('div', { class: 'chips' }, info('Garten', p.gartennr), info('Größe', p.gartengroesse ? `${nfFlex.format(p.gartengroesse)} m²` : ''),
          info('Umlage', eur(p.umlageAbweichend != null ? p.umlageAbweichend : st.settings.umlageStandard)))),
      h('div', { class: 'qgrid' },
        h('fieldset', null, h('legend', null, 'Wasser'),
          fld('Zähler-Nr.', h('div', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_WASSER) }, '⇄'))),
          fld('Stand Vorjahr', inp('wasserVJ', a.wasserVJ), 'm³'), fld('Stand aktuell', inp('wasserAkt', a.wasserAkt), 'm³'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-wv qv' }))),
        h('fieldset', null, h('legend', null, 'Strom'),
          fld('Zähler-Nr.', h('div', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_STROM) }, '⇄'))),
          fld('Stand Vorjahr', inp('stromVJ', a.stromVJ), 'kWh'), fld('Stand aktuell', inp('stromAkt', a.stromAkt), 'kWh'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-sv qv' }))),
        h('fieldset', null, h('legend', null, 'Sonstiges'), fld('Arbeitsstunden', inp('stunden', a.stunden, 'small'), 'h'),
          fld('Abschlag', inp('abschlag', a.abschlag || null, 'small'), '€'),
          h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro, onclick: () => detailsDialog(card, p) }, 'Weitere Angaben'))),
      h('div', { class: 'qfoot' },
        h('div', null, h('span', { class: 'hint' }, 'Rechnungsbetrag: '), h('b', { class: 'gesamt' }), ' ', h('span', { class: 'status' })),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn', onclick: () => verlaufDialog(p) }, 'Verlauf'),
        h('button', { class: 'btn', onclick: () => { S.tab = 'rechnungen'; S.selInvoice = p.id; S.invTab = 'pruefen'; S.invView = 'archiv'; render(); } }, 'Rechnung ansehen'),
        h('button', { class: 'btn primary', onclick: () => {
          // zum nächsten Pächter mit fehlenden Angaben springen (nach der Nummer), sonst zurück ins Suchfeld
          const aktuell = S.state.results;
          const ab = list.findIndex((x) => x.id === p.id);
          const unvoll = (x) => !(aktuell[x.id] && aktuell[x.id].vollstaendig);
          const naechster = list.slice(ab + 1).find(unvoll) || list.find((x) => x.id !== p.id && unvoll(x));
          if (naechster) choose(naechster); else { toast('Alle Pächter sind vollständig.', 'ok'); input.focus(); }
        } }, 'Fertig – nächster offener')));
    updateCalc(card, st.results[p.id]);
    box.append(card);

    // Enter: nächstes Feld; Pflichtfelder immer, Zählernummer/Vorjahresstände nur wenn noch leer
    const seq = ['wz', 'wasserVJ', 'wasserAkt', 'sz', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
    const must = new Set(['wasserAkt', 'stromAkt', 'stunden', 'abschlag']);
    const want = (k) => { const el = card.querySelector(`[data-f="${k}"]`); return el && !el.disabled && (must.has(k) || el.value.trim() === ''); };
    const goto = (from) => {
      for (let i = from; i < seq.length; i++) if (want(seq[i])) { const el = card.querySelector(`[data-f="${seq[i]}"]`); el.focus(); el.select(); return; }
      input.focus();
    };
    card.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
      e.preventDefault();
      e.target.dispatchEvent(new Event('change'));
      goto(seq.indexOf(e.target.dataset.f) + 1);
    });
    if (focusFirst && !ro) goto(0);
  };
  drawCard(false);

  return h('div', null,
    h('p', { class: 'hint' }, 'Nummer eintippen, Enter drücken: Name, Adresse, Gartengröße, Umlage und Zählernummern kommen automatisch aus den Stammdaten, der Vorjahresstand ist schon eingetragen. Du gibst nur noch die neuen Stände ein. Mit Enter springst du zum nächsten Feld, nach dem letzten wieder zur Nummer. Alles wird sofort gespeichert.'),
    h('div', { class: 'toolbar' }, input, h('span', { class: 'stat' }, `${done} von ${list.length} vollständig`), h('span', { class: 'saveind' }, '✓ gespeichert')),
    sugg, box);
}

function viewTabelle() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;

  const tbody = h('tbody');
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    const shown = list.filter((p) => (!q || [p.mitgliedsnr, p.name, p.gartennr].some((x) => (x || '').toLowerCase().includes(q))) &&
      (!S.nurUnvollstaendig || !(st.results[p.id] && st.results[p.id].vollstaendig)));
    for (const p of shown) tbody.append(eingabeRow(p, ro));
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 15, class: 'empty' },
      list.length ? 'Keine Treffer.' : leer('personen', 'Noch keine Pächter angelegt', 'Lege zuerst die Pächter an oder übernimm sie aus einer Excel-Liste.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }))));
  };
  fill();

  const search = h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.filter, style: 'width:240px',
    oninput: (e) => { S.filter = e.target.value; fill(); } });
  const nurOffen = h('label', { class: 'hint' },
    h('input', { type: 'checkbox', checked: S.nurUnvollstaendig, onchange: (e) => { S.nurUnvollstaendig = e.target.checked; fill(); } }),
    ` nur unvollständig${list.length - done ? ` (${list.length - done})` : ''}`);

  const th = (t, cls, extra) => h('th', { class: cls || '', ...(extra || {}) }, t);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier trägst du pro Pächter die Zählernummern, Zählerstände und Arbeitsstunden ein. Alles wird sofort gespeichert und die Beträge werden automatisch berechnet.'),
    h('div', { class: 'toolbar' }, search, nurOffen,
      h('span', { class: 'stat' }, `${list.length} Pächter · ${done} vollständig`),
      h('span', { class: 'saveind' }, '✓ gespeichert'),
      h('div', { class: 'spacer' }),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, 'Übersicht als Excel'),
    ),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null,
        h('tr', { class: 'group' }, th('', 'sticky s1', { colspan: 2 }), th('Wasser', '', { colspan: 4 }), th('Strom / Energie', '', { colspan: 4 }),
          th('', '', { colspan: 5 })),
        h('tr', { class: 'sub' }, th('Nr.', 'sticky s1'), th('Name', 'sticky s2'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch m³'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch kWh'),
          th('Arbeits-stunden'), th('Abschlag €'), th('Gesamt-betrag'), th('Status'), th('')),
      ), tbody)),
  );
}

// ------------------------------------------------------------------ Tab: Rechnungen

const fmtWhen = (iso) => { const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' }); };

function issueSummary(r) {
  return h('div', null,
    h('p', null, `${anz(r.created.length, 'Rechnung', 'Rechnungen')} ausgestellt und gespeichert. Die PDF-Dateien liegen hier:`), h('p', { class: 'mono' }, r.folder),
    (r.skipped || []).length ? h('div', { class: 'banner warn' }, h('b', null, `${r.skipped.length} nicht ausgestellt:`),
      h('ul', { class: 'plain' }, r.skipped.map((x) => h('li', null, `${x.name}: ${x.reason}`)))) : null);
}

async function issue(ids, replace) {
  const st = S.state;
  const r = await api('POST', `/api/invoices/issue?year=${st.year}`, { ids, replace: !!replace });
  await reload();
  // Nächste Schritte gleich im Abschluss anbieten: Briefe drucken, E-Mails vorbereiten
  const neu = S.state;
  const nPost = (neu.archive || []).filter((e) => e.status === 'gueltig' && String(e.versand || '').toLowerCase() === 'postversand').length;
  const mailOffen = neu.paechter.filter((p) => p.versand === 'Emailsendung' && p.email && (neu.issued || {})[p.id] && !neu.issued[p.id].emailAm);
  const wahl = await modal('Fertig', issueSummary(r), [
    { label: 'Schließen', value: false },
    { label: 'Ordner öffnen', value: 'ordner' },
    nPost && !neu.readOnly ? { label: `Postversand drucken (${nPost})`, value: 'post', cls: mailOffen.length ? '' : 'primary' } : null,
    mailOffen.length && !neu.readOnly ? { label: `E-Mails vorbereiten (${mailOffen.length})`, value: 'mail', cls: 'primary' } : null]);
  if (wahl === 'ordner') await api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr);
  else if (wahl === 'post') await postRechnungenDrucken(neu.year);
  else if (wahl === 'mail') { S.selInvoice = mailOffen[0].id; S.invView = 'archiv'; await emailVorbereiten(neu.issued[mailOffen[0].id].id); }
}

// Zählerstand-Zettel: je Garten ein Zettel zum Ausfüllen (drei je Seite), wahlweise nur für Gärten ohne neuen Stand.
async function zaehlerzettelDialog(year) {
  const frist = h('input', { type: 'date', value: isoInTagen(21) });
  const nurOffen = h('input', { type: 'checkbox', checked: true });
  const ok = await modal('Zettel zum Einwerfen', h('div', { class: 'form' },
    h('p', { class: 'hint' }, 'Jeder Garten bekommt einen Zettel mit seinen Zählernummern und dem Stand vom Vorjahr. Der Pächter schreibt den neuen Stand dazu und wirft den Zettel in den Vereinsbriefkasten. Drei Zettel passen auf eine Seite.'),
    h('div', { class: 'field' }, h('label', null, 'Bitte einwerfen bis'), frist),
    h('label', { class: 'inline' }, nurOffen, ' Nur für Gärten, bei denen noch ein Zählerstand fehlt')),
  [{ label: 'Abbrechen', value: false }, { label: 'Zettel erstellen', cls: 'primary', value: true }]);
  if (!ok) return;
  const q = `?year=${year}&frist=${encodeURIComponent(frist.value)}${nurOffen.checked ? '&nur=offen' : ''}`;
  pdfDrucken({ url: `/api/admin/zaehlerzettel${q}`, oeffnenUrl: `/api/admin/zaehlerzettel/oeffnen${q}`, titel: 'Zählerstand-Zettel',
    meldung: () => 'Die Zettel wurden im PDF-Programm geöffnet – dort drucken und ausschneiden.' });
}

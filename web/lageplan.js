'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Lageplan. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Tab: Lageplan

// Kachelübersicht aller Gärten mit Status-Farbcode, sortiert nach Gartennummer
// (natürliche Sortierung, z. B. 2 vor 10). Ein echter geometrischer Lageplan
// bräuchte Koordinaten, die es nicht gibt – die Kacheln geben trotzdem auf
// einen Blick den Überblick, welcher Garten noch offen, unvollständig oder
// erledigt ist.
// Belegungshistorie eines Gartens: welcher Pächter hatte ihn wann. Bleibt
// auch nach endgültigem Löschen eines Pächters erhalten.
async function gartenHistorieDialog(gartennr) {
  let hist;
  try { hist = await api('GET', `/api/garten-historie?gartennr=${encodeURIComponent(gartennr)}`); } catch (e) { handleErr(e); return; }
  const chrono = [...hist].reverse();
  const row = (e) => h('tr', null, h('td', null, e.mitgliedsnr), h('td', null, e.name),
    h('td', null, deDate(e.seit)), h('td', null, e.bis ? deDate(e.bis) : h('span', { class: 'badge ok' }, 'aktuell')));
  const body = chrono.length
    ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Seit', 'Bis'].map((t) => h('th', null, t)))),
        h('tbody', null, chrono.map(row))))
    : h('p', { class: 'hint' }, 'Für diesen Garten liegt noch keine Historie vor.');
  await modal(`Verlauf Garten ${gartennr}`, body, [{ label: 'Schließen', value: true }]);
}

// Lageplan: ohne gespeicherte Anordnung ein Raster nach Gartennummer, sonst eine Karte, auf der jeder
// Garten an seinem Platz auf dem Gelände liegt. Im Admin-Bereich lassen sich die Kacheln per Maus anordnen.
const LP_W = 120, LP_H = 64, LP_RASTER = 10;

function viewLageplan() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const issuedOf = (p) => (st.issued || {})[p.id];
  // wer seinen Garten schon übergeben hat, steht nicht mehr auf der Karte (der Garten gehört dem Nachfolger)
  const list = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) =>
    (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));

  const statusOf = (p) => {
    const res = st.results[p.id];
    if (!res || !res.vollstaendig) return 'unvollstaendig';
    const inf = issuedOf(p);
    if (!inf) return 'berechnet';
    const e = (st.archive || []).find((x) => x.id === inf.id);
    if (e && e.gesamt >= 0 && payIsOpen(e)) return 'offen';
    return 'bezahlt';
  };
  const labels = { unvollstaendig: 'Angaben fehlen', berechnet: 'berechnet, noch nicht ausgestellt', offen: 'Rechnung offen', bezahlt: 'bezahlt / kein offener Betrag' };
  const counts = { unvollstaendig: 0, berechnet: 0, offen: 0, bezahlt: 0, frei: 0 };
  const status = new Map(list.map((p) => { const x = statusOf(p); counts[x]++; return [p.id, x]; }));
  const edit = !!S.lageEntwurf && istAdmin && !st.readOnly;
  const kachel = (p, extra) => h('div', { class: 'kachel kachel-' + status.get(p.id) + (edit ? ' ziehbar' : ''), title: `${p.name}${p.notiz ? ' · Notiz: ' + p.notiz : ''} – ${labels[status.get(p.id)]}`,
    onclick: edit ? null : () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); }, ...(extra || {}) },
    p.gartennr && !edit ? h('button', { class: 'kachel-hist', title: istAdmin ? `Gartenakte von Garten ${p.gartennr}` : `Verlauf von Garten ${p.gartennr}`,
      onclick: (e) => { e.stopPropagation(); if (istAdmin) gartenakteDialog(p.gartennr.trim()); else gartenHistorieDialog(p.gartennr); } }, icon(istAdmin ? 'akte' : 'uhr', 14)) : null,
    h('div', { class: 'kachel-nr' }, p.gartennr || p.mitgliedsnr), h('div', { class: 'kachel-name' }, p.name));
  const vergeben = istAdmin && !st.readOnly && !edit;
  // freier Garten: steht auf der Karte, hat aber keinen aktiven Pächter (z. B. nach einer Kündigung)
  const kachelFrei = (nr) => h('div', { class: 'kachel kachel-frei' + (edit ? ' ziehbar' : ''),
    title: vergeben ? `Garten ${nr} ist frei – klicken, um ihn an jemanden von der Warteliste zu vergeben` : `Garten ${nr} ist frei`,
    onclick: vergeben ? () => freienGartenVergeben(nr) : null },
  edit ? h('button', { class: 'kachel-hist', title: 'Garten von der Karte nehmen', onpointerdown: (e) => e.stopPropagation(),
    onclick: (e) => { e.stopPropagation(); delete S.lageEntwurf[nr]; render(); } }, '✕') : null,
  h('div', { class: 'kachel-nr' }, nr), h('div', { class: 'kachel-name' }, 'frei'));
  const legend = (cls, label) => h('span', { class: 'hint', style: 'margin-right:14px' },
    h('span', { class: 'kachel-dot kachel-' + cls }), ` ${label} (${counts[cls]})`);

  const gespeichert = st.lageplan || {};
  const pos = edit ? S.lageEntwurf : gespeichert;
  const hatKarte = Object.keys(gespeichert).length > 0;
  const mitNr = list.filter((p) => (p.gartennr || '').trim());
  const ohneNr = list.filter((p) => !(p.gartennr || '').trim());
  const belegt = new Set(mitNr.map((p) => p.gartennr.trim()));
  const freie = Object.keys(pos).filter((nr) => !belegt.has(nr)).sort((a, b) => a.localeCompare(b, 'de', { numeric: true }));
  counts.frei = freie.length;

  let kartenAnsicht;
  const kb = st.lageKarte || {};
  const hatHintergrund = !!kb.bild || (kb.objekte || []).length > 0 || (kb.leitungen || []).length > 0;
  if (!hatKarte && !edit && !hatHintergrund) {
    kartenAnsicht = h('div', { class: 'kachelgrid' }, list.map((p) => kachel(p)));
  } else {
    const K = edit ? S.lageKarteEntwurf : kb;
    const wz = edit ? S.lageWerkzeug : null;
    const platziert = mitNr.filter((p) => pos[p.gartennr.trim()]);
    const offen = mitNr.filter((p) => !pos[p.gartennr.trim()]);
    const ms = K.massstab || 1;
    const bildW = kb.bild ? Math.round(kb.bildBreite * ms) : 0, bildH = kb.bild ? Math.round(kb.bildHoehe * ms) : 0;
    let maxX = bildW - LP_W - 40, maxY = bildH - LP_H - 30;
    for (const q of Object.values(pos)) {
      maxX = Math.max(maxX, q.x); maxY = Math.max(maxY, q.y);
      for (const u of q.umriss || []) { maxX = Math.max(maxX, u.x - LP_W); maxY = Math.max(maxY, u.y - LP_H); }
    }
    for (const o of K.objekte || []) { maxX = Math.max(maxX, o.x - LP_W); maxY = Math.max(maxY, o.y - LP_H); }
    for (const l of K.leitungen || []) for (const u of l.punkte) { maxX = Math.max(maxX, u.x - LP_W); maxY = Math.max(maxY, u.y - LP_H); }
    const breite = Math.max(0, maxX) + LP_W + 40, hoehe = Math.max(0, maxY) + LP_H + (edit ? 160 : 30);
    const flaeche = h('div', { class: 'lagekarte' + (edit ? ' bearbeiten' : '') + (wz ? ' werkzeug' : '') + (kb.bild ? ' mit-bild' : ''), style: `width:${breite}px;height:${hoehe}px` });
    if (kb.bild) {
      flaeche.append(h('img', { class: 'lagekarte-bild', src: `/api/lageplan-bild?v=${encodeURIComponent(kb.bild)}`, alt: '', draggable: 'false',
        style: `width:${bildW}px;height:${bildH}px;opacity:${K.deckkraft || 0.85}` }));
      if (kb.quelle) flaeche.append(h('div', { class: 'lagekarte-quelle' }, kb.quelle));
    }
    const svg = svgEl('svg', { class: 'lage-svg', width: breite, height: hoehe });
    flaeche.append(svg);
    const punkteText = (pts) => pts.map((u) => `${u.x},${u.y}`).join(' ');
    // Leitungen
    (K.leitungen || []).forEach((l, i) => {
      const art = LP_LINIEN[l.art] || LP_LINIEN.wasser;
      const g = svgEl('g', { class: 'lage-linie' + (edit && !wz ? ' klickbar' : '') },
        svgEl('title', null, l.bez ? `${art[0]}: ${l.bez}` : art[0]),
        svgEl('polyline', { points: punkteText(l.punkte), class: 'trefferflaeche' }),
        svgEl('polyline', { points: punkteText(l.punkte), stroke: art[1], 'stroke-dasharray': art[2] || null }));
      if (edit && !wz) g.addEventListener('click', () => leitungDialog(i));
      svg.append(g);
    });
    // Gärten mit Umriss: Fläche plus kleines Schild in der Mitte
    const mitte = (u) => {
      let a = 0, cx = 0, cy = 0;
      for (let i = 0; i < u.length; i++) {
        const j = (i + 1) % u.length, k = u[i].x * u[j].y - u[j].x * u[i].y;
        a += k; cx += (u[i].x + u[j].x) * k; cy += (u[i].y + u[j].y) * k;
      }
      if (Math.abs(a) < 1) return { x: u.reduce((s, q) => s + q.x, 0) / u.length, y: u.reduce((s, q) => s + q.y, 0) / u.length };
      return { x: cx / (3 * a), y: cy / (3 * a) };
    };
    const umrissSetzen = (nr, el, klasse, aktion) => {
      const u = pos[nr].umriss;
      const poly = svgEl('polygon', { points: punkteText(u), class: 'umriss ' + klasse });
      if (!edit && aktion) { poly.classList.add('klickbar'); poly.addEventListener('click', aktion); }
      svg.append(poly);
      const m = mitte(u);
      el.classList.add('kachel-mini');
      el.style.left = Math.round(m.x) + 'px'; el.style.top = Math.round(m.y) + 'px';
      if (edit) umrissZiehbar(el, nr, poly);
      return el;
    };
    const kachelSetzen = (nr, el, klasse, aktion) => {
      if ((pos[nr].umriss || []).length >= 3) return umrissSetzen(nr, el, klasse, aktion);
      el.style.left = pos[nr].x + 'px'; el.style.top = pos[nr].y + 'px';
      if (edit) {
        ziehbarMachen(el, nr, flaeche);
        el.append(h('button', { class: 'kachel-umriss', title: `Umriss von Garten ${nr} auf der Karte nachzeichnen`, onpointerdown: (e) => e.stopPropagation(),
          onclick: (e) => { e.stopPropagation(); S.lageWerkzeug = { art: 'umriss', nr, punkte: [] }; render(); } }, '⬠'));
      }
      return el;
    };
    for (const p of platziert) {
      const nr = p.gartennr.trim();
      flaeche.append(kachelSetzen(nr, kachel(p), 'kachel-' + status.get(p.id), () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); }));
    }
    for (const nr of freie) flaeche.append(kachelSetzen(nr, kachelFrei(nr), 'kachel-frei', vergeben ? () => freienGartenVergeben(nr) : null));
    // Einrichtungen
    (K.objekte || []).forEach((o, i) => {
      const art = LP_OBJEKTE[o.art] || LP_OBJEKTE.sonstiges;
      const el = h('div', { class: 'lage-obj' + (edit ? ' ziehbar' : ''), style: `left:${o.x}px;top:${o.y}px;background:${art[2]}`, title: o.bez ? `${art[1]}: ${o.bez}` : art[1] }, art[0],
        o.bez ? h('span', { class: 'lage-obj-bez' }, o.bez) : null);
      if (edit) ziehenOderKlicken(el, {
        bewegen: (dx, dy, start) => { o.x = Math.max(0, Math.round(start.x + dx)); o.y = Math.max(0, Math.round(start.y + dy)); el.style.left = o.x + 'px'; el.style.top = o.y + 'px'; },
        start: () => ({ x: o.x, y: o.y }), ende: () => render(), klick: () => objektDialog(i) });
      flaeche.append(el);
    });
    if (wz) werkzeugAnbinden(flaeche, svg, pos);
    const wrap = h('div', { class: 'lagekarte-wrap', onscroll: (e) => { S.lageScroll = { l: e.target.scrollLeft, t: e.target.scrollTop }; } }, flaeche);
    if (S.lageScroll) { const sc = S.lageScroll; queueMicrotask(() => { wrap.scrollLeft = sc.l; wrap.scrollTop = sc.t; }); }
    kartenAnsicht = h('div', null, wrap,
      offen.length || ohneNr.length ? h('div', { style: 'margin-top:14px' },
        h('h3', { class: 'sect' }, edit ? 'Noch nicht auf der Karte – auf die Fläche ziehen' : 'Noch nicht auf der Karte'),
        h('div', { class: 'kachelgrid' }, offen.map((p) => {
          const el = kachel(p);
          if (edit) ziehbarMachen(el, p.gartennr.trim(), flaeche, true);
          return el;
        }), ohneNr.map((p) => kachel(p, { title: `${p.name}: ohne Gartennummer, kann nicht platziert werden` }))))
        : null);
  }

  if (S.lage3d && !edit && list.length) {
    const daten = {
      gaerten: [...mitNr.filter((p) => pos[p.gartennr.trim()] || !hatKarte).map((p, i) => {
        const nr = p.gartennr.trim(), q = pos[nr] || { x: 20 + (i % 8) * (LP_W + 20), y: 20 + Math.floor(i / 8) * (LP_H + 30) };
        return { nr, name: istAdmin ? p.name : '', status: status.get(p.id), text: labels[status.get(p.id)], x: q.x, y: q.y, umriss: q.umriss || null };
      }), ...freie.map((nr) => ({ nr, frei: true, status: 'frei', text: 'frei', x: pos[nr].x, y: pos[nr].y, umriss: pos[nr].umriss || null }))],
      objekte: kb.objekte || [], leitungen: kb.leitungen || [],
      bild: kb.bild ? { url: `/api/lageplan-bild?v=${encodeURIComponent(kb.bild)}`, w: kb.bildBreite * (kb.massstab || 1), h: kb.bildHoehe * (kb.massstab || 1), deckkraft: kb.deckkraft || 0.85 } : null,
      dachfarbe: parseInt((getComputedStyle(document.documentElement).getPropertyValue('--green').trim() || '#2f6f3e').replace('#', ''), 16) || 0x2f6f3e,
    };
    kartenAnsicht = h('div', null, lageplan3dAnsicht(daten),
      h('p', { class: 'hint', style: 'margin-top:8px' }, 'Ziehen dreht die Anlage, Mausrad zoomt, rechte Maustaste verschiebt. Ein Klick auf einen Garten öffnet ihn.'));
  }

  let leiste = null;
  if (istAdmin && !st.readOnly && list.length) {
    leiste = edit
      ? (S.lageWerkzeug ? werkzeugLeiste() : h('div', null, h('div', { class: 'banner info lage-leiste' },
        h('span', null, 'Ziehe die Gärten an ihren Platz. ⬠ auf einer Kachel zeichnet den Umriss der Parzelle nach. Gespeichert wird erst mit »Anordnung speichern«.'),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn small', title: 'Einen Garten ohne Pächter auf die Karte setzen, z. B. nach einer Kündigung', onclick: freienGartenHinzufuegen }, '+ Freier Garten'), ' ',
        h('button', { class: 'btn small', onclick: () => { S.lageEntwurf = null; S.lageKarteEntwurf = null; render(); } }, 'Abbrechen'), ' ',
        hatKarte ? h('button', { class: 'btn small danger', onclick: async () => {
          if (!(await confirmBox('Die Anordnung und alle Umrisse löschen und die Gärten wieder als Raster nach Nummer zeigen? Hintergrund, Leitungen und Einrichtungen bleiben.', 'Zurück zum Raster', true))) return;
          try { await api('PUT', '/api/admin/lageplan', { positionen: {} }); S.lageEntwurf = null; S.lageKarteEntwurf = null; toast('Lageplan zurückgesetzt', 'ok'); await reload(); } catch (e) { handleErr(e); }
        } }, 'Zurück zum Raster') : null, ' ',
        h('button', { class: 'btn small primary', onclick: async () => {
          try {
            await api('PUT', '/api/admin/lageplan', { positionen: S.lageEntwurf, karte: S.lageKarteEntwurf });
            S.lageEntwurf = null; S.lageKarteEntwurf = null; toast('Anordnung gespeichert', 'ok'); await reload();
          } catch (e) { handleErr(e); }
        } }, 'Anordnung speichern')),
      werkzeugKasten()))
      : h('button', { class: 'btn', onclick: () => { S.lage3d = false; S.lageEntwurf = startEntwurf(mitNr, gespeichert); S.lageKarteEntwurf = startKarteEntwurf(); S.lageWerkzeug = null; render(); } },
        h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('karte', 18), hatKarte || hatHintergrund ? 'Karte bearbeiten' : 'Gärten wie auf dem Gelände anordnen'));
  }

  const umschalter = list.length && !edit ? h('div', { class: 'segment', role: 'group', 'aria-label': 'Ansicht' },
    h('button', { class: S.lage3d ? '' : 'an', onclick: () => { S.lage3d = false; render(); } }, 'Plan'),
    h('button', { class: S.lage3d ? 'an' : '', title: 'Die Anlage als 3D-Modell zum Drehen und Zoomen', onclick: () => { S.lage3d = true; render(); } }, '3D')) : null;
  const druck = list.length && !edit ? h('button', { class: 'btn', title: 'Lageplan als Aushang zum Ausdrucken (ohne Abrechnungsstand)', onclick: () => lageplanDrucken(istAdmin) },
    h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('drucker', 18), 'Drucken')) : null;
  const wartende = st.wartelisteAnzahl || 0;
  return h('div', null,
    h('div', { class: 'toolbar' }, h('h2', { style: 'margin:0' }, `Lageplan ${st.year}`, ' ', seitenHilfe('lageplan')), h('div', { class: 'spacer' }), umschalter, druck, edit || S.lage3d ? null : leiste),
    freie.length && !edit ? h('div', { class: 'banner info' },
      h('span', null, `${freie.length === 1 ? `Garten ${freie[0]} ist` : `${freie.length} Gärten sind`} frei. `,
        wartende ? `${wartende} Interessent${wartende === 1 ? '' : 'en'} auf der Warteliste.` : 'Die Warteliste ist leer.'),
      istAdmin ? [' ', h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Zur Warteliste')] : null) : null,
    h('p', { class: 'hint' }, hatKarte || edit ? 'Jeder Garten liegt an seinem Platz auf dem Gelände. Die Farbe zeigt den Abrechnungsstand, ein Klick öffnet die Schnelleingabe dieses Pächters.'
      : 'Kachel pro Garten, sortiert nach Gartennummer. Farbe zeigt den Abrechnungsstatus, Klick auf eine Kachel springt zur Schnelleingabe dieses Pächters.'),
    edit ? leiste : null,
    h('div', { style: 'margin-bottom:12px' }, legend('unvollstaendig', 'Angaben fehlen'), legend('berechnet', 'berechnet'), legend('offen', 'Rechnung offen'), legend('bezahlt', 'bezahlt'),
      freie.length ? legend('frei', 'frei') : null),
    list.length ? kartenAnsicht : leer('personen', 'Noch keine Gärten', 'Sobald Pächter mit Gartennummer angelegt sind, erscheint hier jeder Garten als Kachel mit seinem Abrechnungsstand.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }));
}

// Einen freien Garten (ohne Pächter) auf die Karte setzen, unter die vorhandenen Gärten.
async function freienGartenHinzufuegen() {
  const nrIn = h('input', { type: 'text', maxlength: 20, placeholder: 'z. B. 14' });
  const nr = await modal('Freien Garten auf die Karte setzen', h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'Gartennummer'), nrIn,
      h('span', { class: 'fhint' }, 'Für einen Garten, der gerade keinen Pächter hat. Danach an seinen Platz ziehen und »Anordnung speichern«.'))),
  [{ label: 'Abbrechen', value: '' }, { label: 'Hinzufügen', cls: 'primary', value: true, action: () => {
    const v = nrIn.value.trim();
    if (!v) { toast('Bitte eine Gartennummer eintragen.', 'err'); return false; }
    if (S.lageEntwurf[v]) { toast(`Garten ${v} ist schon auf der Karte.`, 'err'); return false; }
    return true;
  } }]);
  if (!nr) return;
  const v = nrIn.value.trim();
  const maxY = Math.max(0, ...Object.values(S.lageEntwurf).map((q) => q.y));
  S.lageEntwurf[v] = { x: 20, y: Object.keys(S.lageEntwurf).length ? maxY + LP_H + 30 : 20 };
  render();
}

// Freien Garten an jemanden von der Warteliste vergeben (ohne Warteliste: Pächter von Hand anlegen).
async function freienGartenVergeben(nr) {
  let liste = [];
  try { liste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); return; }
  if (!liste.length) {
    const ok = await modal(`Garten ${nr} ist frei`, h('p', null, 'Auf der Warteliste steht niemand. Du kannst den neuen Pächter direkt anlegen.'),
      [{ label: 'Abbrechen', value: false }, { label: 'Pächter anlegen', cls: 'primary', value: true }]);
    if (ok && await paechterDialog(null, { gartennr: nr, mitgliedsnr: naechsteMitgliedsnr() })) await reload();
    return;
  }
  const zahl = (ph) => { const el = h('input', { type: 'text', placeholder: ph || '', autocomplete: 'off' }); el.setAttribute('inputmode', 'decimal'); return el; };
  // Vorschlag: Reihenfolge der Anmeldung, dazu welche Wünsche zum Garten passen (Größe, Laube); wer abgesagt hat, fehlt
  let vorschlag = null;
  try { vorschlag = await api('GET', `/api/admin/freie-gaerten/${encodeURIComponent(nr)}/vorschlag`); } catch (e) { vorschlag = null; }
  const vz = new Map(((vorschlag && vorschlag.zeilen) || []).map((z) => [z.id, z]));
  const hinweisText = (b) => {
    const z = vz.get(b.id);
    if (!z) return b.besichtigungErgebnis === 'kein Interesse' || b.besichtigungErgebnis === 'nicht erschienen' ? ' [hat abgesagt]' : '';
    return (z.passt.length ? ` ✓ ${z.passt.join(', ')}` : '') + (z.passtNicht.length ? ` ✗ ${z.passtNicht.join(', ')} passt nicht` : '') + (z.empfohlen ? ' (Vorschlag)' : '');
  };
  const f = {
    wer: h('select', null, liste.map((b) => h('option', { value: b.id, selected: !!(vz.get(b.id) && vz.get(b.id).empfohlen) }, `${b.platz}. ${b.name}${b.wunsch ? ' – ' + b.wunsch : ''}${hinweisText(b)}`))),
    seit: h('input', { type: 'date', value: todayIso() }),
    mitgliedsnr: h('input', { type: 'text', value: naechsteMitgliedsnr(), maxlength: 20 }),
    groesse: zahl('m²'),
    wz: h('input', { type: 'text', maxlength: 40 }), sz: h('input', { type: 'text', maxlength: 40 }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  let neuerPaechter = null;
  const ok = await modal(`Garten ${nr} vergeben`, h('div', { class: 'form' },
    vorschlag && vorschlag.groesse ? h('p', { class: 'hint' }, `Garten ${nr}: ${nfFlex.format(vorschlag.groesse)} m²${vorschlag.laube ? ', mit Laube' : ', keine Laube erfasst'}. Der Vorschlag folgt der Reihenfolge der Anmeldung und prüft die Wünsche gegen diese Angaben.`) : null,
    fld('An', f.wer, 'Name, Anschrift und Kontakt kommen von der Warteliste; der Eintrag wird danach entfernt.', true),
    h('div', { class: 'field wide' }, h('button', { type: 'button', class: 'btn small', onclick: () => pdfDrucken({
      url: `/api/admin/warteliste/${encodeURIComponent(f.wer.value)}/brief?art=angebot&garten=${encodeURIComponent(nr)}`,
      oeffnenUrl: `/api/admin/warteliste/${encodeURIComponent(f.wer.value)}/brief/oeffnen?art=angebot&garten=${encodeURIComponent(nr)}`,
      titel: 'Gartenangebot', meldung: () => 'Das Angebot wurde im PDF-Programm geöffnet.' }) }, 'Angebotsschreiben drucken'), ' ',
      h('button', { type: 'button', class: 'btn small', onclick: () => pdfDrucken({
        url: `/api/admin/warteliste/${encodeURIComponent(f.wer.value)}/brief?art=aufnahmeantrag&garten=${encodeURIComponent(nr)}`,
        oeffnenUrl: `/api/admin/warteliste/${encodeURIComponent(f.wer.value)}/brief/oeffnen?art=aufnahmeantrag&garten=${encodeURIComponent(nr)}`,
        titel: 'Aufnahmeantrag', meldung: () => 'Der Aufnahmeantrag wurde im PDF-Programm geöffnet.' }) }, 'Aufnahmeantrag beilegen')),
    fld('Übergeben am', f.seit), fld('Mitgliedsnummer *', f.mitgliedsnr), fld('Gartengröße in m² *', f.groesse),
    fld('Wasserzähler-Nr.', f.wz), fld('Stromzähler-Nr.', f.sz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Garten vergeben', cls: 'primary', value: true, action: async () => {
    const g = parseNum(f.groesse.value);
    if (!g || Number.isNaN(g) || g <= 0) { toast('Bitte die Gartengröße eintragen.', 'err'); return false; }
    try {
      neuerPaechter = await api('POST', `/api/admin/warteliste/${f.wer.value}/vergeben`, { gartennr: nr, seit: f.seit.value, mitgliedsnr: f.mitgliedsnr.value, gartengroesse: g, wasserzaehlerNr: f.wz.value, stromzaehlerNr: f.sz.value });
    } catch (e) { handleErr(e); return false; }
  } }]);
  if (!ok) return;
  S.warteliste = null;
  await reload();
  if (neuerPaechter && await modal(`Garten ${nr} vergeben`, h('p', null, `Garten ${nr} gehört jetzt ${neuerPaechter.name}. Möchtest du gleich die Willkommensmappe drucken?`),
    [{ label: 'Später', value: false }, { label: 'Willkommensmappe', cls: 'primary', value: true }])) await willkommenDrucken(neuerPaechter);
}

async function lageplanDrucken(istAdmin) {
  let namen = false;
  if (istAdmin) {
    const w = await modal('Lageplan drucken', h('div', null,
      h('p', null, 'Für den Aushang ohne Namen, für den Vorstand auf Wunsch mit den Namen der Pächter. Der Abrechnungsstand (bezahlt, offen) wird nie gedruckt.')),
    [{ label: 'Abbrechen', value: '' }, { label: 'Mit Namen (Vorstand)', value: 'namen' }, { label: 'Ohne Namen (Aushang)', cls: 'primary', value: 'aushang' }]);
    if (!w) return;
    namen = w === 'namen';
  }
  const q = namen ? '?namen=1' : '';
  await pdfDrucken({ url: `/api/lageplan-druck${q}`, oeffnenUrl: `/api/lageplan-druck/oeffnen${q}`, titel: 'Lageplan',
    meldung: () => 'Der Lageplan wurde im PDF-Programm geöffnet – dort drucken.' });
}

// Entwurf zum Anordnen: gespeicherte Positionen übernehmen; gibt es noch keine, alle Gärten als Raster vorlegen.
function startEntwurf(mitNr, gespeichert) {
  const e = {};
  for (const [k, v] of Object.entries(gespeichert)) e[k] = { x: v.x, y: v.y, umriss: (v.umriss || []).map((u) => ({ x: u.x, y: u.y })) };
  if (!Object.keys(e).length) {
    mitNr.forEach((p, i) => { e[p.gartennr.trim()] = { x: 20 + (i % 8) * (LP_W + 20), y: 20 + Math.floor(i / 8) * (LP_H + 30) }; });
  }
  return e;
}

// 3D-Ansicht: ein Element, das bei jedem Neuaufbau der Seite wiederverwendet wird (die Szene bleibt erhalten).
function lageplan3dAnsicht(daten) {
  if (!S.lage3dZ) S.lage3dZ = { el: h('div', { class: 'lage3d' }, h('div', { class: 'lage3d-laden' }, '3D-Ansicht wird aufgebaut …')), api: null, schluessel: '' };
  const z = S.lage3dZ, schluessel = JSON.stringify(daten);
  const klick = (nr) => {
    const p = S.state.paechter.find((x) => !x.ausgeschiedenAm && (x.gartennr || '').trim() === nr);
    if (p) { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); return; }
    if (!S.state.hasPassword || S.state.loggedIn) freienGartenVergeben(nr);
  };
  if (!z.api && !z.laedt) {
    z.laedt = true;
    import('/lageplan3d.js').then((m) => { z.api = m.erstelle(z.el, daten, { klick }); z.schluessel = schluessel; })
      .catch(() => { z.el.textContent = 'Die 3D-Ansicht ließ sich nicht laden.'; })
      .finally(() => { z.laedt = false; });
  } else if (z.api && z.schluessel !== schluessel) { z.api.aktualisiere(daten); z.schluessel = schluessel; }
  else if (z.api) z.api.weiter();
  return z.el;
}

// ---- Karte unter dem Lageplan: Hintergrundbild, Umrisse, Leitungen und Einrichtungen
// Kürzel, Name, Farbe (wie objektArten/linienArten in lagekarte.go)
const LP_OBJEKTE = {
  wasserzaehler: ['W', 'Wasserzähler', '#1d6ec4'], absperr: ['A', 'Absperrhahn', '#0e468c'], schacht: ['S', 'Schacht', '#6e6e6e'],
  strom: ['E', 'Stromverteiler', '#d68c00'], tor: ['T', 'Tor / Eingang', '#7d5028'], vereinsheim: ['V', 'Vereinsheim', '#2f6f3e'],
  kompost: ['K', 'Kompost / Container', '#5f7828'], parkplatz: ['P', 'Parkplatz', '#3c3ca0'], sonstiges: ['•', 'Sonstiges', '#963c78'],
};
const LP_LINIEN = { wasser: ['Wasserleitung', '#1d6ec4', ''], strom: ['Stromleitung', '#d68c00', ''], abwasser: ['Abwasser', '#785028', '9 5'], weg: ['Weg', '#8a8a8a', '9 5'] };

function startKarteEntwurf() {
  const kb = S.state.lageKarte || {};
  return { massstab: kb.massstab || 1, massstabStart: kb.massstab || 1, deckkraft: kb.deckkraft || 0.85, quelle: kb.quelle || '',
    objekte: (kb.objekte || []).map((o) => ({ ...o })), leitungen: (kb.leitungen || []).map((l) => ({ ...l, punkte: l.punkte.map((u) => ({ ...u })) })) };
}

// Ziehen mit der Maus; ein Klick ohne Bewegung ruft klick auf.
function ziehenOderKlicken(el, o) {
  el.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0 || ev.target.closest('button')) return;
    ev.preventDefault(); ev.stopPropagation();
    const sx = ev.clientX, sy = ev.clientY, start = o.start();
    let bewegt = false;
    el.setPointerCapture(ev.pointerId);
    const mv = (e) => {
      const dx = e.clientX - sx, dy = e.clientY - sy;
      if (!bewegt && Math.hypot(dx, dy) < 4) return;
      bewegt = true; el.classList.add('zieht');
      o.bewegen(dx, dy, start);
    };
    const up = (e) => {
      el.removeEventListener('pointermove', mv); el.removeEventListener('pointerup', up); el.removeEventListener('pointercancel', up);
      el.classList.remove('zieht');
      if (bewegt) o.ende(); else if (e.type === 'pointerup') o.klick();
    };
    el.addEventListener('pointermove', mv); el.addEventListener('pointerup', up); el.addEventListener('pointercancel', up);
  });
}

// Garten mit Umriss: das Schild in der Mitte verschiebt die ganze Fläche.
function umrissZiehbar(el, nr, poly) {
  let orig;
  ziehenOderKlicken(el, {
    start: () => { orig = S.lageEntwurf[nr].umriss.map((u) => ({ ...u })); return { x: parseFloat(el.style.left), y: parseFloat(el.style.top) }; },
    bewegen: (dx, dy, p0) => {
      dx = Math.round(Math.max(dx, -Math.min(...orig.map((u) => u.x)))); dy = Math.round(Math.max(dy, -Math.min(...orig.map((u) => u.y))));
      const u = orig.map((q) => ({ x: q.x + dx, y: q.y + dy }));
      S.lageEntwurf[nr] = { x: Math.min(...u.map((q) => q.x)), y: Math.min(...u.map((q) => q.y)), umriss: u };
      poly.setAttribute('points', u.map((q) => `${q.x},${q.y}`).join(' '));
      el.style.left = (p0.x + dx) + 'px'; el.style.top = (p0.y + dy) + 'px';
    },
    ende: () => render(),
    klick: () => umrissMenue(nr),
  });
}

async function umrissMenue(nr) {
  const w = await modal(`Garten ${nr}`, h('p', null, 'Der Garten ist als Fläche eingezeichnet. Zum Verschieben das Schild mit der Nummer ziehen.'),
    [{ label: 'Abbrechen', value: '' }, { label: 'Wieder als Kachel', value: 'kachel' }, { label: 'Umriss neu zeichnen', cls: 'primary', value: 'neu' }]);
  if (w === 'kachel') { const q = S.lageEntwurf[nr]; S.lageEntwurf[nr] = { x: q.x, y: q.y }; render(); }
  if (w === 'neu') { S.lageWerkzeug = { art: 'umriss', nr, punkte: [] }; render(); }
}

// Zeichenwerkzeug auf der Fläche: Klicks setzen Einrichtungen oder Punkte einer Linie bzw. eines Umrisses.
// Ecken vorhandener Umrisse rasten ein, damit benachbarte Parzellen lückenlos aneinanderliegen.
function werkzeugAnbinden(flaeche, svg, pos) {
  const wz = S.lageWerkzeug;
  const linie = wz.art === 'linie' ? LP_LINIEN[wz.typ] : null;
  const vorschau = svgEl(wz.art === 'umriss' ? 'polygon' : 'polyline', { class: 'lage-vorschau', stroke: linie ? linie[1] : null });
  const ecken = svgEl('g');
  svg.append(vorschau, ecken);
  const zeichne = () => {
    vorschau.setAttribute('points', wz.punkte.map((u) => `${u.x},${u.y}`).join(' '));
    ecken.replaceChildren(...wz.punkte.map((u, i) => svgEl('circle', { cx: u.x, cy: u.y, r: i === 0 ? 6 : 4, class: 'lage-vorschau-punkt' })));
  };
  S.lageVorschau = zeichne;
  if (wz.punkte) zeichne();
  const fang = [];
  for (const [nr, q] of Object.entries(pos)) if (wz.art !== 'umriss' || nr !== wz.nr) for (const u of q.umriss || []) fang.push(u);
  flaeche.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0 || ev.target.closest('button')) return;
    ev.preventDefault();
    const f = flaeche.getBoundingClientRect();
    let x = Math.max(0, Math.round(ev.clientX - f.left)), y = Math.max(0, Math.round(ev.clientY - f.top));
    let best = 9;
    for (const u of fang) { const d = Math.hypot(u.x - x, u.y - y); if (d < best) { best = d; x = u.x; y = u.y; } }
    if (wz.art === 'objekt') { S.lageKarteEntwurf.objekte.push({ art: wz.typ, x, y, bez: '' }); render(); return; }
    if (wz.art === 'umriss' && wz.punkte.length >= 3 && Math.hypot(wz.punkte[0].x - x, wz.punkte[0].y - y) < 10) { werkzeugFertig(); return; }
    wz.punkte.push({ x, y });
    zeichne();
  });
  flaeche.addEventListener('dblclick', (ev) => { ev.preventDefault(); werkzeugFertig(); });
}

function werkzeugFertig() {
  const wz = S.lageWerkzeug;
  if (!wz) return;
  if (wz.art === 'objekt') { S.lageWerkzeug = null; render(); return; }
  const pts = wz.punkte.filter((u, i, a) => i === 0 || Math.hypot(u.x - a[i - 1].x, u.y - a[i - 1].y) > 3);
  if (wz.art === 'umriss') {
    if (pts.length < 3) { toast('Ein Umriss braucht mindestens 3 Ecken.', 'err'); return; }
    S.lageEntwurf[wz.nr] = { x: Math.min(...pts.map((u) => u.x)), y: Math.min(...pts.map((u) => u.y)), umriss: pts };
  } else {
    if (pts.length < 2) { toast('Eine Linie braucht mindestens 2 Punkte.', 'err'); return; }
    S.lageKarteEntwurf.leitungen.push({ art: wz.typ, punkte: pts, bez: '' });
  }
  S.lageWerkzeug = null;
  render();
}

function werkzeugLeiste() {
  const wz = S.lageWerkzeug;
  const text = wz.art === 'objekt' ? `Klicke auf die Karte, um »${LP_OBJEKTE[wz.typ][1]}« zu setzen. Mehrere nacheinander gehen auch.`
    : wz.art === 'linie' ? `${LP_LINIEN[wz.typ][0]}: Punkt für Punkt auf die Karte klicken. Mit Doppelklick oder »Fertig« abschließen.`
      : `Umriss von Garten ${wz.nr}: die Ecken der Parzelle nacheinander anklicken. Ecken benachbarter Umrisse rasten ein. Klick auf die erste Ecke, Doppelklick oder »Fertig« schließt den Umriss.`;
  return h('div', { class: 'banner info lage-leiste' }, h('span', null, text), h('div', { class: 'spacer' }),
    wz.art !== 'objekt' ? h('button', { class: 'btn small', onclick: () => { wz.punkte.pop(); if (S.lageVorschau) S.lageVorschau(); } }, 'Letzten Punkt zurück') : null, ' ',
    wz.art !== 'objekt' ? h('button', { class: 'btn small', onclick: () => { S.lageWerkzeug = null; render(); } }, 'Abbrechen') : null, ' ',
    h('button', { class: 'btn small primary', onclick: werkzeugFertig }, 'Fertig'));
}

function werkzeugKasten() {
  const kb = S.state.lageKarte || {}, K = S.lageKarteEntwurf;
  const bildEl = () => document.querySelector('.lagekarte-bild');
  const regler = (label, min, max, step, wert, setze, anzeige) => {
    const out = h('span', { class: 'lage-regler-wert' }, anzeige(wert));
    return h('label', { class: 'lage-regler' }, label,
      h('input', { type: 'range', min, max, step, value: wert,
        oninput: (e) => { const v = Number(e.target.value); setze(v); out.textContent = anzeige(v); },
        onchange: () => render() }), out);
  };
  const stufe = Math.log2((K.massstab || 1) / (K.massstabStart || 1));
  return h('div', { class: 'lage-werkzeuge' },
    h('button', { class: 'btn small', onclick: lageHintergrundDialog }, h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('karte', 16),
      kb.bild ? 'Hintergrund ändern' : 'Plan als Hintergrund')),
    kb.bild ? regler('Bildgröße', -2, 2, 0.05, stufe, (v) => {
      K.massstab = Math.round((K.massstabStart || 1) * Math.pow(2, v) * 10000) / 10000;
      const b = bildEl(); if (b) { b.style.width = Math.round(kb.bildBreite * K.massstab) + 'px'; b.style.height = Math.round(kb.bildHoehe * K.massstab) + 'px'; }
    }, (v) => Math.round(Math.pow(2, v) * 100) + ' %') : null,
    kb.bild ? regler('Deckkraft', 0.1, 1, 0.05, K.deckkraft, (v) => { K.deckkraft = v; const b = bildEl(); if (b) b.style.opacity = v; }, (v) => Math.round(v * 100) + ' %') : null,
    h('select', { class: 'lage-auswahl', onchange: (e) => { if (e.target.value) { S.lageWerkzeug = { art: 'objekt', typ: e.target.value }; render(); } } },
      h('option', { value: '' }, '+ Einrichtung setzen …'), Object.entries(LP_OBJEKTE).map(([k, v]) => h('option', { value: k }, `${v[0]} – ${v[1]}`))),
    h('select', { class: 'lage-auswahl', onchange: (e) => { if (e.target.value) { S.lageWerkzeug = { art: 'linie', typ: e.target.value, punkte: [] }; render(); } } },
      h('option', { value: '' }, '+ Leitung oder Weg zeichnen …'), Object.entries(LP_LINIEN).map(([k, v]) => h('option', { value: k }, v[0]))));
}

async function objektDialog(i) {
  const o = S.lageKarteEntwurf.objekte[i];
  const art = h('select', null, Object.entries(LP_OBJEKTE).map(([k, v]) => h('option', { value: k, selected: k === o.art }, `${v[0]} – ${v[1]}`)));
  const bez = h('input', { type: 'text', maxlength: 60, value: o.bez || '', placeholder: 'z. B. Hauptzähler, Hahn Weg B' });
  const w = await modal('Einrichtung', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Art'), art),
    h('div', { class: 'field' }, h('label', null, 'Bezeichnung'), bez, h('span', { class: 'fhint' }, 'Steht neben dem Zeichen auf der Karte und im Ausdruck.'))),
  [{ label: 'Entfernen', cls: 'danger', value: 'weg' }, { label: 'Abbrechen', value: '' }, { label: 'Übernehmen', cls: 'primary', value: 'ok' }]);
  if (w === 'weg') S.lageKarteEntwurf.objekte.splice(i, 1);
  if (w === 'ok') { o.art = art.value; o.bez = bez.value.trim(); }
  if (w) render();
}

async function leitungDialog(i) {
  const l = S.lageKarteEntwurf.leitungen[i];
  const art = h('select', null, Object.entries(LP_LINIEN).map(([k, v]) => h('option', { value: k, selected: k === l.art }, v[0])));
  const bez = h('input', { type: 'text', maxlength: 60, value: l.bez || '', placeholder: 'z. B. Hauptleitung, 1 Zoll, 2019 erneuert' });
  const w = await modal('Leitung / Weg', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Art'), art),
    h('div', { class: 'field' }, h('label', null, 'Bezeichnung'), bez, h('span', { class: 'fhint' }, 'Erscheint beim Darüberfahren mit der Maus.'))),
  [{ label: 'Entfernen', cls: 'danger', value: 'weg' }, { label: 'Abbrechen', value: '' }, { label: 'Übernehmen', cls: 'primary', value: 'ok' }]);
  if (w === 'weg') S.lageKarteEntwurf.leitungen.splice(i, 1);
  if (w === 'ok') { l.art = art.value; l.bez = bez.value.trim(); }
  if (w) render();
}

// Hintergrund: ein eigenes Bild, z. B. der eingescannte Vereinsplan.
async function lageHintergrundDialog() {
  const kb = S.state.lageKarte || {};
  let fertig = null;
  const nachLaden = async (k) => {
    if (S.lageKarteEntwurf) Object.assign(S.lageKarteEntwurf, { massstab: k.massstab, massstabStart: k.massstab, deckkraft: k.deckkraft, quelle: k.quelle || '' });
    await reload();
    if (fertig) fertig();
  };
  const datei = h('input', { type: 'file', accept: '.png,.jpg,.jpeg', style: 'display:none', onchange: async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    try { const k = await api('POST', '/api/admin/lageplan/bild', fd, true); toast('Bild gespeichert', 'ok'); await nachLaden(k); } catch (e) { handleErr(e); }
  } });
  const quelle = h('input', { type: 'text', maxlength: 200, value: (S.lageKarteEntwurf && S.lageKarteEntwurf.quelle) || kb.quelle || '', placeholder: 'z. B. Plan: Vorstand 2024',
    oninput: () => { if (S.lageKarteEntwurf) S.lageKarteEntwurf.quelle = quelle.value; } });
  const body = h('div', { class: 'lage-hg' },
    h('p', { class: 'hint' }, 'Ein Bild der Anlage als Hintergrund, zum Beispiel ein Foto oder Scan des Vereinsplans (PNG oder JPG). Darauf ziehst du die Gärten an ihren Platz und zeichnest Umrisse, Leitungen und Einrichtungen ein. Achte bei fremden Karten auf die Nutzungsrechte.'),
    h('button', { class: 'btn primary', onclick: () => datei.click() }, kb.bild ? 'Anderes Bild wählen …' : 'Bild wählen …'), datei,
    kb.bild ? h('div', { style: 'margin-top:18px' },
      h('h4', null, 'Quellenvermerk'),
      h('div', { class: 'form' }, h('div', { class: 'field wide' }, quelle,
        h('span', { class: 'fhint' }, 'Steht klein auf dem Plan und im Ausdruck (freiwillig). Wird mit »Anordnung speichern« übernommen.'))),
      h('button', { class: 'btn danger', style: 'margin-top:10px', onclick: async () => {
        if (!(await confirmBox('Das Hintergrundbild entfernen? Gärten, Umrisse, Leitungen und Einrichtungen bleiben.', 'Entfernen', true))) return;
        try { const k = await api('DELETE', '/api/admin/lageplan/bild'); toast('Hintergrund entfernt', 'ok'); await nachLaden(k); } catch (e) { handleErr(e); }
      } }, 'Hintergrund entfernen')) : null);
  const p = modal('Hintergrund des Lageplans', body, [{ label: 'Schließen', value: true }]);
  const dlg = document.querySelector('dialog:last-of-type');
  fertig = () => { if (dlg && dlg.open) dlg.dispatchEvent(new Event('cancel')); };
  await p;
}

// Kachel per Maus (oder Finger) verschieben; Position rastet auf 10 px ein. Aus der Liste »noch nicht auf der
// Karte« gezogene Kacheln landen dort auf der Fläche, wo sie losgelassen werden.
function ziehbarMachen(el, nr, flaeche, ausListe) {
  el.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0) return;
    ev.preventDefault();
    const start = el.getBoundingClientRect();
    const dx = ev.clientX - start.left, dy = ev.clientY - start.top;
    const geist = ausListe ? el.cloneNode(true) : el;
    if (ausListe) { geist.classList.add('schwebt'); geist.style.position = 'fixed'; geist.style.width = LP_W + 'px'; document.body.append(geist); }
    el.setPointerCapture(ev.pointerId);
    el.classList.add('zieht');
    const bewegen = (e) => {
      if (ausListe) { geist.style.left = (e.clientX - dx) + 'px'; geist.style.top = (e.clientY - dy) + 'px'; return; }
      const f = flaeche.getBoundingClientRect();
      const x = Math.max(0, Math.round((e.clientX - f.left - dx) / LP_RASTER) * LP_RASTER);
      const y = Math.max(0, Math.round((e.clientY - f.top - dy) / LP_RASTER) * LP_RASTER);
      el.style.left = x + 'px'; el.style.top = y + 'px';
      S.lageEntwurf[nr] = { x, y };
    };
    const ende = (e) => {
      el.removeEventListener('pointermove', bewegen);
      el.removeEventListener('pointerup', ende);
      el.removeEventListener('pointercancel', ende);
      el.classList.remove('zieht');
      if (ausListe) {
        geist.remove();
        const f = flaeche.getBoundingClientRect();
        if (e.type === 'pointerup' && e.clientX >= f.left && e.clientX <= f.right && e.clientY >= f.top && e.clientY <= f.bottom) {
          S.lageEntwurf[nr] = { x: Math.max(0, Math.round((e.clientX - f.left - dx) / LP_RASTER) * LP_RASTER),
            y: Math.max(0, Math.round((e.clientY - f.top - dy) / LP_RASTER) * LP_RASTER) };
        }
      }
      render();
    };
    el.addEventListener('pointermove', bewegen);
    el.addEventListener('pointerup', ende);
    el.addEventListener('pointercancel', ende);
  });
}

function viewRechnungen() {
  const st = S.state;
  const sub = reiterLeiste([
    { label: 'Rechnungen prüfen & ausstellen', aktiv: S.invTab !== 'archiv', geh: () => { S.invTab = 'pruefen'; render(); } },
    { label: `Archiv (${(st.archive || []).length})`, aktiv: S.invTab === 'archiv', geh: () => { S.invTab = 'archiv'; render(); } }]);
  return h('div', null, h('h2', null, `Rechnungen ${st.year}`, ' ', seitenHilfe('rechnungen')), sub, S.invTab === 'archiv' ? viewArchiv() : viewAusstellen());
}

async function loadArchivAlle() {
  if (S.archivAlleLoading) return;
  S.archivAlleLoading = true;
  try { S.archivAlle = await api('GET', '/api/archiv-alle'); } catch (e) { handleErr(e); }
  S.archivAlleLoading = false;
  render();
}

// Öffnet die gespeicherte Rechnungs-PDF im PDF-Programm des Rechners (dort gibt es den vollen Druckdialog).
async function imPdfProgrammOeffnen(id) {
  try { await api('POST', `/api/open-invoice/${id}`); toast('Die Rechnung wurde im PDF-Programm geöffnet – dort drucken.', 'ok'); }
  catch (e) { handleErr(e); }
}

// Öffnet das E-Mail-Programm mit Empfänger, Betreff und Text zur Rechnung und zeigt die PDF-Datei im
// Ordner an (zum Anhängen). Verschickt wird nichts automatisch.
async function emailVorbereiten(id) {
  try {
    const r = await api('POST', `/api/invoice-email/${id}`);
    toast(`E-Mail an ${r.email} ist im E-Mail-Programm vorbereitet. Bitte die markierte Datei »${r.datei}« anhängen und abschicken.`, 'ok');
    await reload();
  } catch (e) { handleErr(e); }
}

// Öffnet ein serverseitig erzeugtes PDF zum Drucken: im Programmfenster im PDF-Programm des Rechners
// (voller Druckdialog), sonst als Vorschau bzw. neuer Tab im Browser.
// body: Eingaben, die per POST mitgehen (z. B. freier Text eines Aushangs, der nicht in die Adresse gehört).
async function pdfDrucken({ url, oeffnenUrl, titel, meldung, body }) {
  try {
    if (inNativeWindow()) {
      const r = await api('POST', oeffnenUrl, body);
      toast(meldung(r), 'ok');
      return true;
    }
    const r = await fetch(url, body === undefined ? { headers: { 'X-GA-Request': '1' } }
      : { method: 'POST', headers: { 'X-GA-Request': '1', 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (!r.ok) {
      const d = await r.json().catch(() => null);
      const err = new Error((d && d.error) || `Fehler ${r.status}`);
      err.status = r.status;
      throw err;
    }
    const blobUrl = URL.createObjectURL(await r.blob());
    openDocument(blobUrl, titel);
    setTimeout(() => URL.revokeObjectURL(blobUrl), 60000);
    return true;
  } catch (e) { handleErr(e); return false; }
}

// Einheitlicher Druck-Knopf: öffnet das PDF (dort drucken oder speichern). Mit »speichern« (Dateiname) kommt
// »Als PDF speichern« dazu – nur für PDFs, die per einfacher Adresse abrufbar sind (nicht für POST mit Text).
function druckKnopf(opts, { label = 'Drucken', speichern, primary, klein } = {}) {
  const knopf = h('button', { class: 'btn' + (primary ? ' primary' : '') + (klein ? ' small' : ''), title: 'Öffnet das PDF – dort drucken oder speichern',
    onclick: () => pdfDrucken({ meldung: () => `${opts.titel} im PDF-Programm geöffnet – dort drucken oder speichern.`, ...opts }) },
  icon('drucker', klein ? 14 : 16), ' ', label);
  if (!speichern) return knopf;
  return h('span', { class: 'druck-gruppe' }, knopf,
    h('a', { class: 'btn' + (klein ? ' small' : ''), href: opts.url, download: speichern, title: 'Die PDF-Datei auf dem Rechner ablegen' }, 'Als PDF speichern'));
}

// Alle Rechnungen mit Postversand als eine PDF-Datei zum Ausdrucken (nach Mitgliedsnummer sortiert).
// Danach als »verschickt« vermerken (für den Jahres-Fahrplan auf der Übersicht).
async function postRechnungenDrucken(year) {
  const ok = await pdfDrucken({
    url: `/api/admin/rechnungen-druck?year=${year}`, oeffnenUrl: `/api/admin/rechnungen-druck/oeffnen?year=${year}`,
    titel: 'Rechnungen Postversand', meldung: (r) => `${anz(r.anzahl, 'Rechnung', 'Rechnungen')} im PDF-Programm geöffnet – dort drucken.`,
  });
  if (!ok) return;
  try { const r = await api('POST', `/api/admin/rechnungen-druck/vermerken?year=${year}`); if (r.vermerkt) await reload(); } catch (e) { /* nur für die Übersicht */ }
}

// Ablesebogen für den Rundgang: alle Gärten mit Zählernummern und Vorjahresständen, leere Felder zum Eintragen.
const ablesebogenDrucken = (year) => pdfDrucken({
  url: `/api/ablesebogen?year=${year}`, oeffnenUrl: `/api/ablesebogen/oeffnen?year=${year}`,
  titel: `Ablesebogen ${year}`, meldung: () => 'Der Ablesebogen wurde im PDF-Programm geöffnet – dort drucken.',
});

function archivRow(a, mitJahr) {
  const url = `/api/archive/${a.id}.pdf`;
  return h('tr', { class: a.status === 'ersetzt' ? 'ersetzt' : '' },
    mitJahr ? h('td', { class: 'mid' }, a.jahr) : null,
    h('td', null, a.mitgliedsnr), h('td', null, a.name), h('td', { class: 'mono' }, a.nummer), h('td', { class: 'mid' }, a.version),
    h('td', { class: 'r' }, eur(a.gesamt)), h('td', null, fmtWhen(a.ausgestellt)),
    h('td', null, h('span', { class: 'badge ' + (a.status === 'gueltig' ? 'ok' : 'warn') }, a.status === 'gueltig' ? 'gültig' : 'ersetzt')),
    h('td', null, h('a', { class: 'btn small', href: url, target: '_blank' }, 'Ansehen'), ' ',
      h('a', { class: 'btn small', href: url, download: a.datei ? a.datei.split('/').pop() : 'Rechnung.pdf' }, 'Speichern'),
      inNativeWindow() && a.datei ? [' ', h('button', { class: 'btn small', title: 'Im PDF-Programm öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(a.id) }, 'Drucken')] : null));
}

function viewArchiv() {
  const st = S.state;
  const q = S.archivSuche.trim().toLowerCase();
  const suchModus = q.length > 0;
  if (suchModus && !S.archivAlle && !S.archivAlleLoading) loadArchivAlle();

  const suchfeld = h('input', { type: 'search', placeholder: 'Suche über alle Jahre (Nummer oder Name)', value: S.archivSuche, style: 'width:300px',
    oninput: (e) => { S.archivSuche = e.target.value; render(); } });

  let body;
  if (suchModus) {
    if (!S.archivAlle) {
      body = h('div', { class: 'card empty' }, 'Wird geladen …');
    } else {
      const treffer = S.archivAlle.filter((a) => [a.mitgliedsnr, a.name, a.nummer].some((x) => (x || '').toLowerCase().includes(q)));
      body = treffer.length
        ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
            h('thead', null, h('tr', null, ['Jahr', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
            h('tbody', null, treffer.map((a) => archivRow(a, true)))))
        : h('div', { class: 'card empty' }, 'Keine Treffer.');
    }
  } else {
    const rows = st.archive || [];
    body = rows.length
      ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows.map((a) => archivRow(a, false)))))
      : leer('rechnung', 'Noch keine Rechnung ausgestellt', `Sobald du im Bereich »Rechnungen prüfen & ausstellen« Rechnungen ausstellst, liegen sie hier für ${st.year} – mit den Werten von damals, unveränderlich.`, 'Zum Ausstellen', () => { S.invTab = 'pruefen'; render(); });
  }

  return h('div', null,
    h('p', { class: 'hint' }, `Hier liegen alle ausgestellten Rechnungen des Jahres ${st.year}. Sie sind mit den Preisen und Angaben von damals gespeichert und ändern sich nie, auch wenn du später Preise, Namen oder Zählerstände änderst. Wurde eine Rechnung neu ausgestellt, bleibt die alte als »ersetzt« erhalten.`),
    h('div', { class: 'toolbar' }, suchfeld, h('div', { class: 'spacer' }),
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    body);
}

function viewAusstellen() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  if (!list.some((p) => p.id === S.selInvoice)) S.selInvoice = list.length ? list[0].id : null;
  const right = h('div');
  const issuedOf = (p) => (st.issued || {})[p.id];

  const drawPreview = () => {
    right.replaceChildren();
    const p = list.find((x) => x.id === S.selInvoice);
    if (!p) { right.append(leer('rechnung', 'Noch keine Pächter', 'Rechnungen gibt es, sobald Pächter angelegt und ihre Zählerstände eingetragen sind.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); })); return; }
    const res = st.results[p.id];
    const inf = issuedOf(p);
    const showLive = !inf || S.invView === 'live';
    const url = showLive ? `/api/invoice/${p.id}.pdf?year=${st.year}` : `/api/archive/${inf.id}.pdf`;
    const canShow = showLive ? res.vollstaendig : true;

    const bar = h('div', { class: 'toolbar' }, h('b', null, `${p.mitgliedsnr} – ${p.name}`), schlussBadge(p), h('div', { class: 'spacer' }));
    if (!inf && res.vollstaendig) {
      bar.append(h('button', { class: 'btn primary', onclick: async () => { try { await issue([p.id], false); } catch (e) { handleErr(e); } } }, 'Rechnung ausstellen & speichern'));
    }
    if (canShow) {
      bar.append(h('a', { class: 'btn', href: url, target: '_blank', title: 'Öffnet das PDF – dort drucken oder speichern' }, icon('drucker', 16), ' Drucken'),
        h('a', { class: 'btn', href: url, download: `Rechnung_${p.mitgliedsnr}.pdf`, title: 'Die PDF-Datei auf dem Rechner ablegen' }, 'Als PDF speichern'));
      if (inNativeWindow() && inf && !showLive) {
        bar.append(h('button', { class: 'btn', title: 'Im PDF-Programm des Rechners öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(inf.id) }, 'Im PDF-Programm öffnen'));
      }
      if (inf && !showLive) {
        bar.append(h('button', { class: 'btn', disabled: !p.email,
          title: p.email ? `Öffnet das E-Mail-Programm mit einer fertigen Nachricht an ${p.email}; die PDF-Datei wird im Ordner markiert und muss nur noch angehängt werden`
            : 'Für diesen Pächter ist keine E-Mail-Adresse eingetragen (unter »Pächter«)',
          onclick: () => emailVorbereiten(inf.id) }, inf.emailAm ? 'E-Mail erneut vorbereiten' : 'E-Mail vorbereiten'));
      }
    }
    right.append(bar);

    if (inf) {
      right.append(h('div', { class: 'banner info' },
        `Ausgestellt am ${fmtWhen(inf.ausgestellt)} · Rechnungs-Nr. ${inf.nummer}${inf.version > 1 ? ` (Version ${inf.version})` : ''} · ${eur(inf.gesamt)}. Diese Rechnung ist gespeichert und ändert sich nicht mehr.${inf.emailAm ? ` E-Mail vorbereitet am ${fmtWhen(inf.emailAm)}.` : ''}`));
      if (inf.geaendert) {
        right.append(h('div', { class: 'banner warn' },
          'Seit der Ausstellung wurden Werte geändert (Zählerstände, Preise oder Pächterdaten). Die ausgestellte Rechnung bleibt unverändert. ',
          h('button', { class: 'btn small', onclick: () => { S.invView = showLive ? 'archiv' : 'live'; drawPreview(); } }, showLive ? 'Ausgestellte Rechnung ansehen' : 'Mit aktuellen Werten ansehen'), ' ',
          res.vollstaendig ? h('button', { class: 'btn small', onclick: async () => {
            const ok = await confirmBox(`Für ${p.name} wird eine neue Rechnung mit den aktuellen Werten ausgestellt. Die bisherige bleibt als »ersetzt« im Archiv. Fortfahren?`, 'Neu ausstellen');
            if (ok) { try { S.invView = 'archiv'; await issue([p.id], true); } catch (e) { handleErr(e); } }
          } }, 'Neu ausstellen (ersetzt die alte)') : null));
      }
    }
    if (!canShow) {
      right.append(h('div', { class: 'banner warn' }, `Für diese Rechnung fehlt noch etwas: ${res.status}. Bitte im Tab »Zählerstände« ergänzen.`,
        ' ', h('button', { class: 'btn small', onclick: () => { S.tab = 'eingabe'; S.filter = p.mitgliedsnr; render(); } }, 'Zu den Zählerständen')));
      return;
    }
    right.append(h('div', { class: 'preview' }, h('iframe', { src: `${url}${showLive ? '&t=' + Date.now() : ''}#view=FitH`, title: 'Rechnungsvorschau' })));
  };

  const items = h('div', { class: 'card list' });
  const drawList = () => {
    items.replaceChildren();
    for (const p of list) {
      const ok = st.results[p.id] && st.results[p.id].vollstaendig;
      const inf = issuedOf(p);
      const cls = inf ? (inf.geaendert ? 'chg' : 'iss') : ok ? 'ok' : '';
      const tip = inf ? (inf.geaendert ? 'ausgestellt, danach geändert' : 'ausgestellt') : ok ? 'bereit zum Ausstellen' : 'unvollständig';
      items.append(h('button', { class: 'item' + (p.id === S.selInvoice ? ' active' : ''), onclick: () => { S.selInvoice = p.id; S.invView = 'archiv'; drawList(); drawPreview(); } },
        h('span', { class: 'dot ' + cls, title: tip }),
        h('span', { class: 'nr' }, p.mitgliedsnr), h('span', null, p.name, schlussBadge(p)),
        inf && inf.emailAm ? h('span', { title: `E-Mail vorbereitet am ${fmtWhen(inf.emailAm)}`, style: 'margin-left:auto' }, icon('brief', 15)) : null));
    }
    if (!list.length) items.append(h('div', { class: 'empty' }, 'Keine Pächter.'));
  };
  drawList();
  drawPreview();

  const open = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig && !issuedOf(p));
  const nIss = list.filter((p) => issuedOf(p)).length;
  const allBtn = h('button', { class: 'btn primary', disabled: !open.length, onclick: async () => {
    const ok = await confirmBox(`${anz(open.length, 'Rechnung wird', 'Rechnungen werden')} jetzt ausgestellt und als PDF gespeichert. Unvollständige Rechnungen werden übersprungen. Fortfahren?`, 'Ausstellen');
    if (!ok) return;
    allBtn.disabled = true;
    try { await issue([], false); } catch (e) { handleErr(e); }
    allBtn.disabled = false;
  } }, `Alle offenen ausstellen (${open.length})`);

  // gezählt wird wie im PDF: nach der Versandart auf der ausgestellten Rechnung, nicht nach den heutigen Stammdaten
  const nPost = (st.archive || []).filter((e) => e.status === 'gueltig' && String(e.versand || '').toLowerCase() === 'postversand').length;
  const postBtn = h('button', { class: 'btn', disabled: !nPost,
    title: 'Alle ausgestellten Rechnungen mit Versandart Postversand als eine PDF-Datei, nach Mitgliedsnummer sortiert',
    onclick: () => postRechnungenDrucken(st.year) }, `Postversand drucken (${nPost})`);
  // E-Mail-Versand nacheinander: springt zur nächsten ausgestellten Rechnung mit Versandart Emailsendung,
  // deren E-Mail noch nicht vorbereitet wurde
  const mailOffen = list.filter((p) => p.versand === 'Emailsendung' && p.email && issuedOf(p) && !issuedOf(p).emailAm);
  const ohneAdresse = list.filter((p) => p.versand === 'Emailsendung' && !p.email && issuedOf(p)).length;
  const mailBtn = h('button', { class: 'btn', disabled: !mailOffen.length,
    title: `Bereitet die E-Mail für die nächste Rechnung mit Versandart Emailsendung vor${ohneAdresse ? ` (${ohneAdresse} ohne eingetragene E-Mail-Adresse)` : ''}`,
    onclick: async () => { const p = mailOffen[0]; S.selInvoice = p.id; S.invView = 'archiv'; await emailVorbereiten(issuedOf(p).id); } },
  `Nächste E-Mail (${mailOffen.length} offen)`);

  return h('div', null,
    h('p', { class: 'hint' }, 'Wähle links einen Pächter und prüfe die Rechnung. Mit »Ausstellen« wird sie festgeschrieben, als PDF gespeichert und ins Archiv gelegt. Punkte: grau = unvollständig, grün = bereit, blau = ausgestellt, orange = ausgestellt, danach geändert.'),
    h('div', { class: 'toolbar' }, allBtn, h('span', { class: 'hint' }, `${nIss} von ${list.length} ausgestellt`), h('div', { class: 'spacer' }), st.readOnly ? null : mailBtn, postBtn,
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    h('div', { class: 'split' }, items, right));
}

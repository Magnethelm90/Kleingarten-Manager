'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Übersicht (Aufgaben, Vereinsjahr, Besichtigungen) und Start. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Übersicht: Was steht an?

async function loadAufgaben() {
  if (S.aufgabenLoading) return;
  S.aufgabenLoading = true;
  try {
    const alle = await api('GET', '/api/admin/aufgaben?alle=1');
    S.aufgaben = alle.filter((x) => !x.zurueckBis);
    S.aufgabenZurueck = alle.filter((x) => x.zurueckBis);
  } catch (e) { S.aufgaben = []; S.aufgabenZurueck = []; }
  S.aufgabenLoading = false;
  render();
}

// Ziel aus Cockpit oder Vereinsjahr öffnen: admin:<bereich>[:<unterseite>] oder tab:<reiter>[:eichung]
function zielOeffnen(ziel) {
  const [art, wohin, unter] = ziel.split(':');
  if (art === 'admin') adminTabWaehlen(wohin); else S.tab = wohin;
  if (wohin === 'inventar') S.invSeite = unter || 'schluessel';
  if (wohin === 'verwaltung') S.verwSeite = unter || 'beschluesse';
  if (wohin === 'kassenbericht') S.kasseNurOhneBeleg = unter === 'ohnebeleg';
  if (wohin === 'maengel') S.maengelSeite = unter || 'maengel';
  if (wohin === 'einstellungen' && unter) { S.esOffen = S.esOffen || new Set(); S.esOffen.add('es-' + unter); }
  render();
  window.scrollTo(0, 0);
  if (wohin === 'einstellungen' && unter) { const el = document.getElementById('es-' + unter); if (el) el.scrollIntoView({ block: 'start' }); }
  if (unter === 'eichung') eichungDialog();
}

// ------------------------------------------------------------------ Übersicht: Vereinsjahr

function vereinsjahrKarte(st, istAdmin) {
  if (!istAdmin || st.leer) return null;
  const jahr = S.vjJahr || new Date().getFullYear();
  if (!S.vj || S.vj.jahr !== jahr) {
    if (!S.vjLaedt) {
      S.vjLaedt = true;
      api('GET', `/api/admin/vereinsjahr?jahr=${jahr}`).then((d) => { S.vj = d; }).catch(() => { S.vj = { jahr, fehler: true }; })
        .finally(() => { S.vjLaedt = false; render(); });
    }
    return null;
  }
  const d = S.vj;
  if (d.fehler) return null;
  const heuteJahr = Number(d.heute.slice(0, 4)), heuteMonat = Number(d.heute.slice(5, 7));
  const vergangen = (m) => d.jahr < heuteJahr || (d.jahr === heuteJahr && m < heuteMonat);
  const sel = S.vjMonat || (d.jahr === heuteJahr ? heuteMonat : 1);
  const monat = d.monate[sel - 1];
  const alle = d.monate.flatMap((m) => m.punkte), fertig = alle.filter((p) => p.erledigt).length;
  const haken = async (p, an) => {
    try { S.vj = await api('PUT', '/api/admin/vereinsjahr', { jahr: d.jahr, key: p.key, erledigt: an }); render(); } catch (e) { handleErr(e); }
  };
  const jahrWechseln = (n) => { S.vjJahr = d.jahr + n; S.vjMonat = null; render(); };
  return h('div', { class: 'vereinsjahr' },
    h('div', { class: 'fs-kopf' },
      h('b', null, h('button', { class: 'linkbtn', 'aria-label': 'Vorjahr', onclick: () => jahrWechseln(-1) }, '‹'), ` ${d.jahr} `,
        h('button', { class: 'linkbtn', 'aria-label': 'Folgejahr', onclick: () => jahrWechseln(1) }, '›')),
      h('span', { class: 'hint' }, `${fertig} von ${alle.length} erledigt`)),
    h('div', { class: 'vj-monate', role: 'tablist' }, d.monate.map((m) => {
      const offen = m.punkte.filter((p) => !p.erledigt).length;
      const zustand = !m.punkte.length ? '' : offen === 0 ? ' fertig' : vergangen(m.monat) ? ' rueckstand' : '';
      return h('button', { class: 'vj-monat' + zustand + (m.monat === sel ? ' an' : '') + (d.jahr === heuteJahr && m.monat === heuteMonat ? ' jetzt' : ''),
        role: 'tab', 'aria-selected': m.monat === sel ? 'true' : 'false', title: `${MONATE[m.monat - 1]}: ${m.punkte.length ? (offen ? `${offen} offen` : 'alles erledigt') : 'nichts Besonderes'}`,
        onclick: () => { S.vjMonat = m.monat; render(); } }, h('span', null, MONATE[m.monat - 1].slice(0, 3)), h('i', null));
    })),
    h('div', { class: 'vj-inhalt' },
      h('h4', null, MONATE[sel - 1]),
      monat.punkte.length ? h('ul', { class: 'vj-liste' }, monat.punkte.map((p) => {
        const cb = h('input', { type: 'checkbox', checked: p.erledigt, disabled: p.erkannt, 'aria-label': `${p.titel} erledigt` });
        cb.addEventListener('change', () => haken(p, cb.checked));
        return h('li', { class: p.erledigt ? 'erledigt' : '' },
          h('label', { class: 'vj-haken', title: p.erkannt ? 'Vom Programm erkannt' : 'Von Hand abhaken' }, cb),
          h('div', { class: 'vj-text' }, h('b', null, p.titel), p.erkannt ? h('span', { class: 'badge ok', style: 'margin-left:6px' }, 'erledigt') : null, h('div', { class: 'hint' }, p.text)),
          h('button', { class: 'btn small', onclick: () => zielOeffnen(p.ziel) }, p.knopf + ' →'));
      })) : h('p', { class: 'hint' }, 'In diesem Monat steht nichts Besonderes an.'),
      h('div', { class: 'toolbar' }, h('button', { class: 'btn small', title: 'Neuen Termin in diesem Monat eintragen', onclick: async () => {
        const mm = String(sel).padStart(2, '0'), tag = d.jahr === heuteJahr && sel === heuteMonat ? d.heute : `${d.jahr}-${mm}-01`;
        if (await terminDialog({ datum: tag })) { S.vj = null; await reload(); }
      } }, `+ Termin im ${MONATE[sel - 1]}`)),
      monat.termine.length ? h('div', { class: 'vj-termine' }, h('span', { class: 'hint' }, 'Termine: '),
        monat.termine.map((t, i) => [i ? ' · ' : '', h('span', null, `${deDate(t.datum).slice(0, 6)} ${t.titel}`)])) : null));
}

const AUFGABE_ICON = { err: 'warnung', warn: 'uhr', info: 'bericht' };

// Eine Karte, zwei Sichten: was jetzt ansteht (Fristen, Zahlungen, Warnungen) und der Fahrplan durchs Vereinsjahr.
function anstehendKarte(st, istAdmin) {
  if (!istAdmin || st.leer) return null;
  const jahr = S.ueberSeite === 'jahr';
  const inhalt = jahr ? vereinsjahrKarte(st, istAdmin) : cockpitKarte(st, istAdmin);
  const n = S.aufgaben ? S.aufgaben.length : 0;
  return h('div', { class: 'card anstehend' },
    h('div', { class: 'subtabs' },
      h('button', { class: jahr ? '' : 'active', onclick: () => { S.ueberSeite = 'jetzt'; render(); } }, 'Was steht an?', n ? h('span', { class: 'zahl' }, String(n)) : null),
      h('button', { class: jahr ? 'active' : '', onclick: () => { S.ueberSeite = 'jahr'; render(); } }, 'Vereinsjahr')),
    inhalt || h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
}

const COCKPIT_BEREICHE = [['geld', 'Geld und Rechnungen'], ['anlage', 'Anlage und Gärten'], ['verein', 'Verein und Termine']];
function cockpitBereich(ziel) {
  const w = (ziel || '').split(':')[1] || '';
  if (['zahlungen', 'rechnungen', 'vorauszahlung', 'lastschrift', 'kassenbericht', 'haushalt', 'spenden', 'eingabe', 'berichte', 'kasse'].includes(w)) return 'geld';
  if (['inventar', 'maengel', 'bauten', 'lageplan', 'paechter', 'warteliste', 'kuendigungen', 'jubilaeen'].includes(w)) return 'anlage';
  return 'verein';
}

function cockpitKarte(st, istAdmin) {
  if (!istAdmin || st.leer) return null;
  if (S.aufgaben == null) { if (!S.aufgabenLoading) loadAufgaben(); return null; }
  const geh = zielOeffnen;
  if (!S.aufgaben.length) {
    return h('div', { class: 'cockpit-leer' }, h('span', { class: 'cockpit-ico ok' }, icon('fertig', 20)),
      h('div', null, h('b', null, 'Gerade steht nichts an'), h('div', { class: 'hint' }, 'Termine, Fristen, Zahlungen und Spenden sind im grünen Bereich.'),
        (S.aufgabenZurueck || []).length ? h('div', { class: 'hint' }, `${anz(S.aufgabenZurueck.length, 'Hinweis ist', 'Hinweise sind')} zurückgestellt.`) : null));
  }
  const l = S.aufgaben;
  const zurueck = async (a, tage) => {
    try { await api('POST', '/api/admin/aufgaben/zurueckstellen', { schluessel: a.schluessel, tage }); S.aufgaben = null; render(); } catch (e) { handleErr(e); }
  };
  const zeile = (a) => {
    const wahl = h('select', { class: 'zurueck-wahl', 'aria-label': `Hinweis zurückstellen: ${a.titel}`, title: 'Diesen Hinweis für einige Tage ausblenden',
      onchange: (e) => { const t = Number(e.target.value); if (t) zurueck(a, t); } },
    h('option', { value: '' }, 'Später …'), h('option', { value: '7' }, '7 Tage'), h('option', { value: '30' }, '30 Tage'));
    return h('li', { class: 'cockpit-' + a.stufe },
      h('span', { class: 'cockpit-ico ' + a.stufe }, icon(AUFGABE_ICON[a.stufe] || 'fertig', 18)),
      h('div', { class: 'cockpit-text' }, h('b', null, a.titel), a.text ? h('div', { class: 'hint' }, a.text) : null),
      wahl,
      h('button', { class: 'btn small', onclick: () => geh(a.ziel) }, a.knopf + ' →'));
  };
  // Die fünf wichtigsten Hinweise stehen offen da (dringend zuerst), alles Weitere ist eingeklappt und nach Bereich sortiert.
  const rang = { err: 0, warn: 1, info: 2 };
  const sortiert = [...l].sort((x, y) => rang[x.stufe] - rang[y.stufe]);
  const oben = sortiert.slice(0, 5), rest = sortiert.slice(5);
  const teile = [h('ul', { class: 'cockpit-liste' }, oben.map(zeile))];
  if (rest.length) {
    const bereiche = COCKPIT_BEREICHE.map((b) => [b, rest.filter((a) => cockpitBereich(a.ziel) === b[0])]).filter(([, g]) => g.length);
    const mehr = h('details', { class: 'cockpit-bereich', open: !!S.cockpitMehr },
      h('summary', null, h('b', null, anz(rest.length, 'weiterer Hinweis', 'weitere Hinweise'))),
      bereiche.map(([b, g]) => [bereiche.length > 1 ? h('div', { class: 'cockpit-gruppe' }, b[1]) : null, h('ul', { class: 'cockpit-liste' }, g.map(zeile))]));
    mehr.addEventListener('toggle', () => { S.cockpitMehr = mehr.open; });
    teile.push(mehr);
  }
  const zb = S.aufgabenZurueck || [];
  const zbBox = zb.length ? h('details', { class: 'cockpit-zurueck' },
    h('summary', null, `${zb.length} zurückgestellt`),
    h('ul', { class: 'cockpit-liste' }, zb.map((a) => h('li', { class: 'cockpit-' + a.stufe },
      h('div', { class: 'cockpit-text' }, h('b', null, a.titel), h('div', { class: 'hint' }, `Ausgeblendet bis ${isoDatum(a.zurueckBis).toLocaleDateString('de-DE')}`)),
      h('button', { class: 'btn small', onclick: () => zurueck(a, 0) }, 'Wieder anzeigen'))))) : null;
  return h('div', null, ...teile,
    zbBox,
    h('p', { class: 'hint', style: 'margin:8px 0 0' }, h('a', { href: '/api/admin/fristen.ics', download: 'Vereinsfristen.ics', title: 'Mängelfristen, Kündigungstermine und Zahlungsfristen als Kalenderdatei mit Erinnerung am Vortag' }, 'Fristen als Kalenderdatei')));
}

// ------------------------------------------------------------------ Übersicht: Besichtigungen

async function loadDashWarteliste() {
  if (S.dashWartelisteLoading) return;
  S.dashWartelisteLoading = true;
  try { S.dashWarteliste = await api('GET', '/api/admin/warteliste'); } catch (e) { S.dashWarteliste = []; }
  S.dashWartelisteLoading = false;
  render();
}

function besichtigungenKarte(st, istAdmin) {
  if (!istAdmin || !st.wartelisteAnzahl || !modulAn('warteliste')) return null;
  if (S.dashWarteliste == null) { if (!S.dashWartelisteLoading) loadDashWarteliste(); return null; }
  const heute = todayIso();
  const l = S.dashWarteliste.filter((b) => b.besichtigungAm && b.besichtigungAm >= heute && !b.besichtigungErgebnis)
    .sort((a, b) => (a.besichtigungAm + (a.besichtigungUm || '')).localeCompare(b.besichtigungAm + (b.besichtigungUm || ''))).slice(0, 4);
  if (!l.length) return null;
  return h('div', { class: 'card naechste-termine' },
    h('div', { class: 'fs-kopf' }, h('b', null, 'Besichtigungen'),
      h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Zur Warteliste →')),
    h('ul', { class: 'termin-liste' }, l.map((b) => {
      const d = isoDatum(b.besichtigungAm);
      return h('li', { class: 'termin' },
        h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, `${WOCHENTAGE[d.getDay()]} · ${MONATE[d.getMonth()].slice(0, 3)}`)),
        h('div', { class: 'termin-text' }, h('b', null, b.name),
          h('div', { class: 'hint' }, [b.besichtigungUm ? b.besichtigungUm + ' Uhr' : '', b.besichtigungGarten ? 'Garten ' + b.besichtigungGarten : '', b.telefon].filter(Boolean).join(' · '))),
        h('span', { class: 'chip termin-art' }, 'Besichtigung'));
    })));
}

// ------------------------------------------------------------------ Start

// Letzte Seite merken: nach dem Neustart des Programms öffnet sich die zuletzt besuchte Seite (nur im Browserspeicher dieses Rechners).
function seiteMerken(st) {
  if (st.gesperrt || st.leer || (st.hasPassword && !st.loggedIn)) return;
  try { localStorage.setItem('km-seite', JSON.stringify({ tab: S.tab, admin: S.tab === 'admin' ? S.adminTab : null })); } catch (e) { /* ohne Speicher einfach nicht merken */ }
}
function seiteWiederherstellen() {
  const st = S.state;
  if (st.gesperrt || st.leer || (st.hasPassword && !st.loggedIn)) return;
  let z; try { z = JSON.parse(localStorage.getItem('km-seite') || 'null'); } catch (e) { z = null; }
  if (!z || typeof z !== 'object') return;
  const eintrag = NAV.flatMap((g) => g.items).flatMap((x) => x.reiter || [x]).find((y) => (z.admin ? y.admin === z.admin : y.tab && y.tab === z.tab));
  if (!eintrag || !navSichtbar(eintrag, st) || z.tab === 'uebersicht') return;
  if (eintrag.admin) adminTabWaehlen(eintrag.admin); else S.tab = eintrag.tab;
}

// »Was hat sich geändert?«: einmal nach einem Update. Die letzte gesehene Version merkt sich der Browser.
const NEUERUNGEN = [
  { version: '1.5.1', punkte: [
    'Ratenplan, Gutschrift und Zahlung stehen in einem gemeinsamen Dialog.',
    'Mahn-Vorschlag auf der Seite »Zahlungen«: zeigt, wer jetzt eine Erinnerung oder Mahnung bekommen sollte.',
    'Pflegeplan für Gemeinschaftsflächen unter »Anlage«: Hecke, Wege, Spielplatz – mit Erinnerung im Cockpit.',
    'Haushaltsplan: Posten ab 90 % werden gelb, das Cockpit meldet sie.',
    'Geburtstage (nur mit Einverständnis) in der Pächter-Mappe, Erinnerung im Cockpit.',
    'Auf der Seite »Zählerstände« steht jetzt der Wasser-Vergleich mit dem Hauptzähler.',
    'Die Wiederherstellungs-Probe läuft alle 30 Tage von selbst.',
    'Suche findet auch Wörter mit einem Tippfehler (Meyer = Meier).',
    'Über jeder Tabelle steht »Tabelle drucken«.',
    'Bei vielen Hinweisen ist das Cockpit nach Geld, Anlage und Verein sortiert.',
    'In den Einstellungen kannst du die »Letzten Änderungen« ansehen und zurücknehmen.',
    'Nach einem Update prüft sich das Programm selbst (Ergebnis siehst du hier).',
    'Eigene Brieftexte in den Einstellungen – mit »Probebrief ansehen«.',
    'Zählerstand-Zettel zum Einwerfen: drei Zettel je Seite mit Vorjahresstand.',
    'Mehrere Pächter auf einmal ändern: Haken setzen, dann »Auswahl ändern« (Versandart, Umlage).',
    'Fristen aus der Satzung in den Einstellungen: Das Cockpit sagt, bis wann die Einladung zur Versammlung raus muss.',
    'Stehen noch Musterwerte in den Vereinsangaben, fragt das Programm vor dem Ausstellen der Rechnungen nach.',
    'Garten ohne Strom: Haken beim Pächter (unter »Weitere Angaben«) – dann steht kein Strom auf der Rechnung.',
    'Einstellungen aufgeräumt: Preise oben, alles Seltene unter »Weitere Einstellungen« zum Aufklappen.',
    'Pächter-Menü »Mehr«: ein Eintrag »Garten abgeben …« statt drei – mit Kündigung oder sofort.',
    'Übersicht: Jahres-Fahrplan ganz oben mit neuem Schritt »Verschicken«.',
    'Cockpit zeigt die fünf wichtigsten Hinweise, der Rest ist eingeklappt.',
    'Kontoauszug einlesen geht jetzt auch direkt bei Spenden und Vorauszahlungen.',
    'Berichte: »Jahreszahlen« bündelt Jahresbericht, Jahresmeldung, Statistik und Kostenvergleich.',
    'Druck-Knöpfe einheitlich: »Drucken« und – wo sinnvoll – »Als PDF speichern«.',
    'Zählerstände: »Abgelesen am« – das Programm trägt den Tag selbst ein, du kannst ihn ändern.'] }];
function neuerungenPruefen() {
  const st = S.state;
  if (st.gesperrt || (st.hasPassword && !st.loggedIn)) return;
  let alt = null;
  try { alt = localStorage.getItem('km-version'); localStorage.setItem('km-version', st.version); } catch (e) { return; }
  if (alt == null || alt === st.version || document.querySelector('dialog[open]')) return;
  const neu = NEUERUNGEN.find((n) => n.version === st.version);
  if (!neu) return;
  modal(`Neu in Version ${st.version}`, h('div', null,
    h('p', { class: 'hint' }, 'Das Programm wurde aktualisiert. Das ist neu:'),
    h('ul', null, neu.punkte.map((t) => h('li', null, t))),
    st.selbsttest ? h('div', { class: 'banner ' + (st.selbsttest.ok ? 'info' : 'err'), style: 'margin-top:10px' },
      h('b', null, st.selbsttest.ok ? 'Selbsttest: alles in Ordnung. ' : 'Selbsttest: Es gibt Hinweise. '),
      st.selbsttest.zeilen.filter((z) => z.stufe !== 'ok').map((z) => h('div', null, `${z.titel}: ${z.text}`))) : null),
  [{ label: 'Verstanden', cls: 'primary', value: true }]);
}

(async function start() {
  document.body.append(h('div', { id: 'toasts', role: 'status', 'aria-live': 'polite' }));
  try { await load(); seiteWiederherstellen(); render(); neuerungenPruefen(); } catch (e) {
    document.getElementById('app').replaceChildren(h('div', { class: 'center card' }, h('h2', null, 'Verbindung fehlgeschlagen'), h('p', null, e.message)));
  }
})();

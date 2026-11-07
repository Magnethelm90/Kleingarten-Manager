// 3D-Ansicht des Lageplans: die Anlage als kleines Modell zum Drehen und Zoomen. Parzellen in der Farbe des
// Abrechnungsstands, Lauben auf belegten Gärten, Wege, Leitungen und Einrichtungen; darunter auf Wunsch der
// hinterlegte Plan. Läuft komplett lokal (three.js liegt im Programm). Maßstab: 10 Bildpunkte des Plans = 1 Einheit.
import * as THREE from './vendor/three/three.module.min.js';
import { OrbitControls } from './vendor/three/OrbitControls.js';

const S3 = 0.1;
// Rand der Parzelle in der Farbe des Abrechnungsstands (wie die Legende im Programm), die Fläche ist Rasen
const FARBE = {
  bezahlt: 0x3f9a4f, offen: 0xd9a227, unvollstaendig: 0xc8553d, berechnet: 0x7f8db0, frei: 0xd9d6c8,
};
const RASEN = [0xa8cf8c, 0x9fc884, 0xb1d494, 0xa3cb8a];
const OBJEKT_FARBE = { wasserzaehler: 0x1d6ec4, absperr: 0x0e468c, schacht: 0x6e6e6e, strom: 0xd68c00, tor: 0x7d5028, vereinsheim: 0x2f6f3e, kompost: 0x5f7828, parkplatz: 0x3c3ca0, sonstiges: 0x963c78 };
const LINIEN_FARBE = { wasser: 0x1d6ec4, strom: 0xd68c00, abwasser: 0x785028, weg: 0xbdb6a6 };

function reduziert() { return window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches; }

// deterministischer Zufall, damit Bäume bei jedem Aufbau an derselben Stelle stehen
function zufall(seed) { let s = seed >>> 0; return () => { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296; }; }

function schild(text, farbe, groesse) {
  const c = document.createElement('canvas');
  const px = 64, pad = 22;
  const ctx = c.getContext('2d');
  ctx.font = `700 ${px}px system-ui, sans-serif`;
  const w = Math.ceil(ctx.measureText(text).width) + pad * 2, h = px + pad;
  c.width = w; c.height = h;
  ctx.font = `700 ${px}px system-ui, sans-serif`;
  ctx.fillStyle = 'rgba(255,255,255,0.94)';
  const r = h / 2;
  ctx.beginPath(); ctx.moveTo(r, 0); ctx.lineTo(w - r, 0); ctx.arc(w - r, r, r, -Math.PI / 2, Math.PI / 2); ctx.lineTo(r, h); ctx.arc(r, r, r, Math.PI / 2, Math.PI * 1.5); ctx.fill();
  ctx.fillStyle = farbe || '#24402c';
  ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
  ctx.fillText(text, w / 2, h / 2 + 3);
  const tex = new THREE.CanvasTexture(c);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.anisotropy = 4;
  const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, depthTest: false, transparent: true }));
  const g = groesse || 1.1;
  sp.scale.set(g * w / h, g, 1);
  sp.renderOrder = 10;
  return sp;
}

function laube(breite, holz, dach) {
  const g = new THREE.Group();
  const w = breite, d = breite * 0.8, h = breite * 0.62;
  const wand = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), new THREE.MeshStandardMaterial({ color: holz, roughness: 0.8 }));
  wand.position.y = h / 2; wand.castShadow = wand.receiveShadow = true;
  g.add(wand);
  const form = new THREE.Shape();
  const ue = w * 0.12, rh = w * 0.34;
  form.moveTo(-w / 2 - ue, 0); form.lineTo(w / 2 + ue, 0); form.lineTo(0, rh); form.closePath();
  const dachGeo = new THREE.ExtrudeGeometry(form, { depth: d + ue * 2, bevelEnabled: false });
  dachGeo.translate(0, 0, -(d + ue * 2) / 2);
  const dachM = new THREE.Mesh(dachGeo, new THREE.MeshStandardMaterial({ color: dach, roughness: 0.6 }));
  dachM.position.y = h; dachM.castShadow = true;
  g.add(dachM);
  const tuer = new THREE.Mesh(new THREE.BoxGeometry(w * 0.22, h * 0.7, 0.02), new THREE.MeshStandardMaterial({ color: 0x8a5a33 }));
  tuer.position.set(-w * 0.2, h * 0.35, d / 2 + 0.01);
  g.add(tuer);
  const fenster = new THREE.Mesh(new THREE.BoxGeometry(w * 0.26, h * 0.26, 0.02), new THREE.MeshStandardMaterial({ color: 0xbfe0ef, roughness: 0.2 }));
  fenster.position.set(w * 0.2, h * 0.55, d / 2 + 0.01);
  g.add(fenster);
  return g;
}

function baum(r, x, z, rnd) {
  const g = new THREE.Group();
  const s = 0.8 + rnd() * 0.6;
  const stamm = new THREE.Mesh(new THREE.CylinderGeometry(0.12 * s, 0.16 * s, 1.4 * s, 8), new THREE.MeshStandardMaterial({ color: 0x6b4a2e, roughness: 0.9 }));
  stamm.position.y = 0.7 * s; stamm.castShadow = true;
  g.add(stamm);
  const gruen = [0x5f9a52, 0x4f8a48, 0x6aa65c][Math.floor(rnd() * 3)];
  for (let i = 0; i < 3; i++) {
    const k = new THREE.Mesh(new THREE.IcosahedronGeometry((0.9 - i * 0.15) * s * r, 1), new THREE.MeshStandardMaterial({ color: gruen, roughness: 0.85, flatShading: true }));
    k.position.set((rnd() - 0.5) * 0.5 * s, (1.7 + i * 0.45) * s, (rnd() - 0.5) * 0.5 * s);
    k.castShadow = true;
    g.add(k);
  }
  g.position.set(x, 0, z);
  return g;
}

export function erstelle(container, daten, opt) {
  const breite = () => container.clientWidth || 800, hoehe = () => container.clientHeight || 520;
  let renderer;
  try {
    renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
  } catch (e) {
    container.textContent = 'Die 3D-Ansicht braucht WebGL, das auf diesem Rechner nicht verfügbar ist.';
    return { aktualisiere() {}, weiter() {}, zerstoere() {} };
  }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.setSize(breite(), hoehe());
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.05;
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  container.replaceChildren(renderer.domElement);
  const tip = document.createElement('div');
  tip.className = 'lage3d-tipp';
  container.append(tip);

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0xe9f0e6);
  const camera = new THREE.PerspectiveCamera(38, breite() / hoehe(), 0.5, 2000);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.08;
  controls.maxPolarAngle = Math.PI * 0.44;
  controls.minDistance = 8;
  controls.screenSpacePanning = false;

  scene.add(new THREE.HemisphereLight(0xffffff, 0x8aa07a, 1.25));
  const sonne = new THREE.DirectionalLight(0xfff3e0, 2.1);
  sonne.castShadow = true;
  sonne.shadow.mapSize.set(2048, 2048);
  sonne.shadow.bias = -0.0004;
  sonne.shadow.radius = 4;
  scene.add(sonne, sonne.target);

  let welt = new THREE.Group();
  scene.add(welt);
  let parzellen = [];
  let mitte = new THREE.Vector3();
  let ausdehnung = new THREE.Vector3(40, 0, 30);
  let groesse = 40;

  function aufbauen(d) {
    scene.remove(welt);
    welt.traverse((o) => { if (o.geometry) o.geometry.dispose(); if (o.material) { (Array.isArray(o.material) ? o.material : [o.material]).forEach((m) => { if (m.map) m.map.dispose(); m.dispose(); }); } });
    welt = new THREE.Group();
    scene.add(welt);
    parzellen = [];
    const rnd = zufall(1234);
    // Grenzen
    let minX = Infinity, minZ = Infinity, maxX = -Infinity, maxZ = -Infinity;
    const erw = (x, z) => { minX = Math.min(minX, x); maxX = Math.max(maxX, x); minZ = Math.min(minZ, z); maxZ = Math.max(maxZ, z); };
    for (const g of d.gaerten) {
      if (g.umriss && g.umriss.length >= 3) g.umriss.forEach((p) => erw(p.x * S3, p.y * S3));
      else { erw(g.x * S3, g.y * S3); erw((g.x + 120) * S3, (g.y + 64) * S3); }
    }
    for (const o of d.objekte || []) erw(o.x * S3, o.y * S3);
    for (const l of d.leitungen || []) l.punkte.forEach((p) => erw(p.x * S3, p.y * S3));
    if (d.bild) { erw(0, 0); erw(d.bild.w * S3, d.bild.h * S3); }
    if (!isFinite(minX)) { minX = 0; minZ = 0; maxX = 40; maxZ = 30; }
    const rand = 6;
    mitte = new THREE.Vector3((minX + maxX) / 2, 0, (minZ + maxZ) / 2);
    groesse = Math.max(maxX - minX, maxZ - minZ) + rand * 2;
    ausdehnung = new THREE.Vector3(maxX - minX + rand * 2, 0, maxZ - minZ + rand * 2);

    // Boden (mit Plan als Textur, wenn vorhanden)
    const bw = maxX - minX + rand * 2, bt = maxZ - minZ + rand * 2;
    const sockel = new THREE.Mesh(new THREE.BoxGeometry(bw, 1.2, bt), new THREE.MeshStandardMaterial({ color: 0x7a5a3e, roughness: 1 }));
    sockel.position.set(mitte.x, -0.62, mitte.z);
    sockel.receiveShadow = true;
    welt.add(sockel);
    const rasen = new THREE.Mesh(new THREE.PlaneGeometry(bw, bt), new THREE.MeshStandardMaterial({ color: 0xbfd8ae, roughness: 0.95 }));
    rasen.rotation.x = -Math.PI / 2;
    rasen.position.set(mitte.x, 0, mitte.z);
    rasen.receiveShadow = true;
    welt.add(rasen);
    if (d.bild) {
      const tex = new THREE.TextureLoader().load(d.bild.url, () => zeichnen());
      tex.colorSpace = THREE.SRGBColorSpace;
      tex.anisotropy = 8;
      const plan = new THREE.Mesh(new THREE.PlaneGeometry(d.bild.w * S3, d.bild.h * S3),
        new THREE.MeshStandardMaterial({ map: tex, transparent: true, opacity: d.bild.deckkraft, roughness: 1 }));
      plan.rotation.x = -Math.PI / 2;
      plan.position.set(d.bild.w * S3 / 2, 0.01, d.bild.h * S3 / 2);
      plan.receiveShadow = true;
      welt.add(plan);
    }

    // Wege und Leitungen
    for (const l of d.leitungen || []) {
      const pts = l.punkte.map((p) => new THREE.Vector3(p.x * S3, 0.04, p.y * S3));
      if (l.art === 'weg') {
        for (let i = 1; i < pts.length; i++) {
          const a = pts[i - 1], b = pts[i], len = a.distanceTo(b);
          const m = new THREE.Mesh(new THREE.BoxGeometry(len + 1.2, 0.06, 1.6), new THREE.MeshStandardMaterial({ color: LINIEN_FARBE.weg, roughness: 1 }));
          m.position.copy(a.clone().add(b).multiplyScalar(0.5));
          m.rotation.y = -Math.atan2(b.z - a.z, b.x - a.x);
          m.receiveShadow = true;
          welt.add(m);
        }
      } else {
        const kurve = new THREE.CatmullRomCurve3(pts, false, 'catmullrom', 0);
        const rohr = new THREE.Mesh(new THREE.TubeGeometry(kurve, Math.max(8, pts.length * 12), 0.09, 6, false),
          new THREE.MeshStandardMaterial({ color: LINIEN_FARBE[l.art] || 0x1d6ec4, roughness: 0.4, emissive: LINIEN_FARBE[l.art] || 0x1d6ec4, emissiveIntensity: 0.15 }));
        welt.add(rohr);
      }
    }

    // Parzellen
    const holz = 0xc8956a, dach = d.dachfarbe || 0x2f6f3e;
    for (const g of d.gaerten) {
      const form = new THREE.Shape();
      let cx, cz, w, t;
      if (g.umriss && g.umriss.length >= 3) {
        g.umriss.forEach((p, i) => (i ? form.lineTo(p.x * S3, p.y * S3) : form.moveTo(p.x * S3, p.y * S3)));
        const xs = g.umriss.map((p) => p.x * S3), zs = g.umriss.map((p) => p.y * S3);
        cx = xs.reduce((a, b) => a + b, 0) / xs.length; cz = zs.reduce((a, b) => a + b, 0) / zs.length;
        w = Math.max(...xs) - Math.min(...xs); t = Math.max(...zs) - Math.min(...zs);
      } else {
        const x0 = g.x * S3 + 0.25, z0 = g.y * S3 + 0.25, x1 = (g.x + 120) * S3 - 0.25, z1 = (g.y + 64) * S3 - 0.25, r = 0.35;
        form.moveTo(x0 + r, z0); form.lineTo(x1 - r, z0); form.quadraticCurveTo(x1, z0, x1, z0 + r); form.lineTo(x1, z1 - r);
        form.quadraticCurveTo(x1, z1, x1 - r, z1); form.lineTo(x0 + r, z1); form.quadraticCurveTo(x0, z1, x0, z1 - r); form.lineTo(x0, z0 + r); form.quadraticCurveTo(x0, z0, x0 + r, z0);
        cx = (x0 + x1) / 2; cz = (z0 + z1) / 2; w = x1 - x0; t = z1 - z0;
      }
      const geo = new THREE.ExtrudeGeometry(form, { depth: 0.28, bevelEnabled: true, bevelSize: 0.06, bevelThickness: 0.06, bevelSegments: 2 });
      geo.rotateX(Math.PI / 2); // Form liegt in x/z, Höhe nach oben
      geo.translate(0, 0.34, 0);
      const farbe = FARBE[g.frei ? 'frei' : g.status] || FARBE.berechnet;
      const mat = new THREE.MeshStandardMaterial({ color: g.frei ? 0xeceadf : RASEN[parzellen.length % RASEN.length], roughness: 0.95 });
      const m = new THREE.Mesh(geo, mat);
      m.castShadow = m.receiveShadow = true;
      const gruppe = new THREE.Group();
      gruppe.add(m);
      // farbiger Rand: dieselbe Form etwas größer und flacher darunter
      const rand = new THREE.Mesh(geo.clone(), new THREE.MeshStandardMaterial({ color: farbe, roughness: 0.7 }));
      rand.scale.set(1 + 0.7 / Math.max(w, 1), 0.62, 1 + 0.7 / Math.max(t, 1));
      rand.position.set(cx * (1 - rand.scale.x), 0, cz * (1 - rand.scale.z));
      rand.receiveShadow = true;
      gruppe.add(rand);
      if (!g.frei) {
        const lb = laube(Math.max(1.4, Math.min(w, t) * 0.32), holz, dach);
        lb.position.set(cx - w * 0.25, 0.34, cz - t * 0.12);
        lb.rotation.y = 0.05;
        gruppe.add(lb);
        // zwei Beete
        for (let i = 0; i < 2; i++) {
          const beet = new THREE.Mesh(new THREE.BoxGeometry(w * 0.32, 0.18, t * 0.14), new THREE.MeshStandardMaterial({ color: 0x8a6244, roughness: 1 }));
          beet.position.set(cx + w * 0.2, 0.43, cz - t * 0.15 + i * t * 0.28);
          beet.castShadow = beet.receiveShadow = true;
          gruppe.add(beet);
        }
      }
      const sp = schild(g.nr, g.frei ? '#7a857a' : '#24402c', 1.7);
      sp.position.set(cx, 3.6, cz);
      gruppe.add(sp);
      gruppe.userData = { garten: g, mat, farbe };
      welt.add(gruppe);
      parzellen.push({ gruppe, mesh: m });
    }

    // Einrichtungen als Pins
    for (const o of d.objekte || []) {
      const f = OBJEKT_FARBE[o.art] || 0x963c78;
      const pin = new THREE.Group();
      const stab = new THREE.Mesh(new THREE.CylinderGeometry(0.06, 0.06, 1.6, 8), new THREE.MeshStandardMaterial({ color: 0xffffff }));
      stab.position.y = 0.8;
      const kopf = new THREE.Mesh(new THREE.SphereGeometry(0.38, 20, 14), new THREE.MeshStandardMaterial({ color: f, roughness: 0.4 }));
      kopf.position.y = 1.75; kopf.castShadow = true;
      pin.add(stab, kopf);
      if (o.bez) { const sp = schild(o.bez, '#333', 0.7); sp.position.set(0, 2.6, 0); pin.add(sp); }
      pin.position.set(o.x * S3, 0, o.y * S3);
      welt.add(pin);
    }

    // Bäume rund um die Anlage
    const umfang = 2 * (bw + bt);
    const n = Math.min(60, Math.round(umfang / 4.5));
    for (let i = 0; i < n; i++) {
      const u = (i + rnd() * 0.6) / n * umfang;
      let x, z;
      if (u < bw) { x = mitte.x - bw / 2 + u; z = mitte.z - bt / 2 + 1.6; }
      else if (u < bw + bt) { x = mitte.x + bw / 2 - 1.6; z = mitte.z - bt / 2 + (u - bw); }
      else if (u < 2 * bw + bt) { x = mitte.x + bw / 2 - (u - bw - bt); z = mitte.z + bt / 2 - 1.6; }
      else { x = mitte.x - bw / 2 + 1.6; z = mitte.z + bt / 2 - (u - 2 * bw - bt); }
      if (rnd() < 0.35) continue;
      welt.add(baum(0.9 + rnd() * 0.5, x, z, rnd));
    }

    // Licht passend zur Größe
    const r = groesse;
    sonne.position.set(mitte.x - r * 0.4, r * 0.9, mitte.z - r * 0.55);
    sonne.target.position.copy(mitte);
    const sc = sonne.shadow.camera;
    sc.left = -r * 0.75; sc.right = r * 0.75; sc.top = r * 0.75; sc.bottom = -r * 0.75; sc.near = 1; sc.far = r * 3;
    sc.updateProjectionMatrix();
    scene.fog = new THREE.Fog(0xe9f0e6, r * 1.6, r * 3.2);
    controls.maxDistance = r * 2.2;
  }

  function kameraStart(animiert) {
    const ziel = mitte.clone();
    // Abstand so wählen, dass die ganze Anlage ins Bild passt (Blick schräg von vorn, ca. 42° von oben)
    const aspekt = breite() / hoehe(), vfov = THREE.MathUtils.degToRad(camera.fov);
    const hfov = 2 * Math.atan(Math.tan(vfov / 2) * aspekt);
    const neig = THREE.MathUtils.degToRad(42);
    const abstand = Math.max((ausdehnung.x / 2) / Math.tan(hfov / 2), (ausdehnung.z * Math.sin(neig) / 2 + 2) / Math.tan(vfov / 2)) * 0.98 + ausdehnung.z * 0.12;
    const ende = new THREE.Vector3(mitte.x, Math.sin(neig) * abstand, mitte.z + Math.cos(neig) * abstand);
    controls.target.copy(ziel);
    if (!animiert || reduziert()) { camera.position.copy(ende); controls.update(); return; }
    const start = new THREE.Vector3(mitte.x, abstand * 1.5, mitte.z + abstand * 0.08);
    const t0 = performance.now(), dauer = 1500;
    camera.position.copy(start);
    flug = (jetzt) => {
      const x = Math.min(1, (jetzt - t0) / dauer), e = 1 - Math.pow(1 - x, 3);
      camera.position.lerpVectors(start, ende, e);
      if (x >= 1) flug = null;
    };
  }

  // Maus: Garten hervorheben, Tooltip, Klick
  const ray = new THREE.Raycaster(), maus = new THREE.Vector2();
  let ueber = null, flug = null, laeuft = false, gedrueckt = null;
  function treffer(ev) {
    const r = renderer.domElement.getBoundingClientRect();
    maus.set(((ev.clientX - r.left) / r.width) * 2 - 1, -((ev.clientY - r.top) / r.height) * 2 + 1);
    ray.setFromCamera(maus, camera);
    const hit = ray.intersectObjects(parzellen.map((p) => p.mesh), false)[0];
    return hit ? parzellen.find((p) => p.mesh === hit.object) : null;
  }
  function markieren(p, ev) {
    if (ueber && ueber !== p) { ueber.gruppe.position.y = 0; ueber.gruppe.userData.mat.emissive.setHex(0x000000); }
    ueber = p;
    renderer.domElement.style.cursor = p ? 'pointer' : 'grab';
    if (!p) { tip.style.opacity = '0'; return; }
    p.gruppe.position.y = 0.18;
    p.gruppe.userData.mat.emissive.setHex(0x1a2a14);
    const g = p.gruppe.userData.garten;
    tip.textContent = `Garten ${g.nr}${g.name ? ' – ' + g.name : ''} · ${g.text}`;
    const r = container.getBoundingClientRect();
    tip.style.left = (ev.clientX - r.left + 14) + 'px';
    tip.style.top = (ev.clientY - r.top + 14) + 'px';
    tip.style.opacity = '1';
  }
  renderer.domElement.addEventListener('pointermove', (ev) => markieren(treffer(ev), ev));
  renderer.domElement.addEventListener('pointerleave', () => markieren(null));
  renderer.domElement.addEventListener('pointerdown', (ev) => { gedrueckt = { x: ev.clientX, y: ev.clientY }; });
  renderer.domElement.addEventListener('pointerup', (ev) => {
    if (!gedrueckt || Math.hypot(ev.clientX - gedrueckt.x, ev.clientY - gedrueckt.y) > 5) return;
    const p = treffer(ev);
    if (p && opt && opt.klick) opt.klick(p.gruppe.userData.garten.nr);
  });

  const ro = new ResizeObserver(() => {
    renderer.setSize(breite(), hoehe());
    camera.aspect = breite() / hoehe();
    camera.updateProjectionMatrix();
  });
  ro.observe(container);

  function zeichnen() { renderer.render(scene, camera); }
  function schleife(jetzt) {
    if (!container.isConnected) { laeuft = false; return; }
    if (flug) flug(jetzt);
    controls.update();
    zeichnen();
    requestAnimationFrame(schleife);
  }
  function weiter() {
    if (laeuft) return;
    laeuft = true;
    requestAnimationFrame(schleife);
  }

  aufbauen(daten);
  kameraStart(true);
  weiter();

  return {
    aktualisiere(d) { aufbauen(d); weiter(); },
    weiter,
    ansichtZuruecksetzen() { kameraStart(true); weiter(); },
    zerstoere() { ro.disconnect(); renderer.dispose(); },
  };
}

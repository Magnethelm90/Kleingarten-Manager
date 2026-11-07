# Illustrationen des Kleingarten-Managers: prozedurale 3D-Szenen (Laube, Beete, Obstbaum, Zaun, Sonnenblumen,
# Gießkanne) im Stil eines Modellbaus, gerendert mit Cycles.
# Aufruf mit Blender als Python-Modul (pip install bpy==4.5.15, Python 3.11):
#   python szene.py <ausgabeordner> <motiv> [groesse] [samples]
#   motiv: hero | laube | kanne | beet | baum | sonne
# Verwendet: hero 1600 128, die übrigen 700 96. Danach auf den sichtbaren Bereich zuschneiden und als WebP
# (Qualität 86, hero 1200 px breit, sonst 420 px) unter web/bilder speichern; hero wird zu garten.webp.
import bpy, math, random, sys, os
from mathutils import Vector

OUT = sys.argv[1] if len(sys.argv) > 1 else '/tmp'
WAS = sys.argv[2] if len(sys.argv) > 2 else 'hero'
GROESSE = int(sys.argv[3]) if len(sys.argv) > 3 else 1200
SAMPLES = int(sys.argv[4]) if len(sys.argv) > 4 else 96
random.seed(7)

def leeren():
    bpy.ops.wm.read_factory_settings(use_empty=True)

leeren()
MAT = {}
def mat(name, farbe, rough=0.6, sss=0.0, metal=0.0):
    if name in MAT:
        return MAT[name]
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    b = m.node_tree.nodes['Principled BSDF']
    b.inputs['Base Color'].default_value = (*farbe, 1)
    b.inputs['Roughness'].default_value = rough
    b.inputs['Metallic'].default_value = metal
    if sss:
        b.inputs['Subsurface Weight'].default_value = sss
        b.inputs['Subsurface Radius'].default_value = (0.2, 0.1, 0.05)
    MAT[name] = m
    return m

def hex2(h):
    h = h.lstrip('#')
    c = [int(h[i:i+2], 16) / 255 for i in (0, 2, 4)]
    return tuple(x ** 2.2 for x in c)  # sRGB -> linear

def setze(o, m, bevel=0.0, glatt=False, segs=3):
    o.data.materials.clear()
    o.data.materials.append(m)
    if bevel:
        b = o.modifiers.new('bevel', 'BEVEL'); b.width = bevel; b.segments = segs; b.limit_method = 'ANGLE'
    if glatt:
        for p in o.data.polygons: p.use_smooth = True
    return o

def box(name, loc, dim, m, bevel=0.02, rot=(0, 0, 0)):
    bpy.ops.mesh.primitive_cube_add(location=loc, rotation=rot)
    o = bpy.context.object; o.name = name
    o.scale = (dim[0] / 2, dim[1] / 2, dim[2] / 2)
    bpy.ops.object.transform_apply(scale=True)
    return setze(o, m, bevel)

def kugel(name, loc, r, m, sub=3, scale=(1, 1, 1)):
    bpy.ops.mesh.primitive_ico_sphere_add(subdivisions=sub, radius=r, location=loc)
    o = bpy.context.object; o.name = name; o.scale = scale
    bpy.ops.object.transform_apply(scale=True)
    return setze(o, m, glatt=True)

def zyl(name, loc, r, h, m, verts=24, rot=(0, 0, 0), bevel=0.01):
    bpy.ops.mesh.primitive_cylinder_add(vertices=verts, radius=r, depth=h, location=loc, rotation=rot)
    o = bpy.context.object; o.name = name
    setze(o, m, bevel=bevel, glatt=True)
    return o

def kegel(name, loc, r1, r2, h, m, verts=24, rot=(0, 0, 0)):
    bpy.ops.mesh.primitive_cone_add(vertices=verts, radius1=r1, radius2=r2, depth=h, location=loc, rotation=rot)
    o = bpy.context.object; o.name = name
    return setze(o, m, glatt=True)

# ---------------------------------------------------------------- Farben (passend zum Programm: Grün #2f6f3e)
GRAS = mat('gras', hex2('#7fb35b'), 0.85)
GRAS2 = mat('gras2', hex2('#6aa14a'), 0.85)
ERDE = mat('erde', hex2('#7a5236'), 0.95)
ERDE_D = mat('erde_d', hex2('#5b3b26'), 0.95)
HOLZ = mat('holz', hex2('#c08a55'), 0.7)
HOLZ_D = mat('holz_d', hex2('#8a5a33'), 0.7)
DACH = mat('dach', hex2('#2f6f3e'), 0.55)
WEISS = mat('weiss', hex2('#f4f1e8'), 0.5)
FENSTER = mat('fenster', hex2('#a9d3e8'), 0.15)
STEIN = mat('stein', hex2('#d8d2c4'), 0.8)
LAUB = mat('laub', hex2('#4f9a44'), 0.7, sss=0.15)
LAUB2 = mat('laub2', hex2('#3f8a3c'), 0.7, sss=0.15)
APFEL = mat('apfel', hex2('#d9433b'), 0.35)
GELB = mat('gelb', hex2('#f4c430'), 0.5, sss=0.1)
BRAUN = mat('braun', hex2('#5a3a1e'), 0.8)
KANNE = mat('kanne', hex2('#2f8f8a'), 0.3, metal=0.3)
KUERBIS = mat('kuerbis', hex2('#e8862e'), 0.45)
MOEHRE = mat('moehre', hex2('#f08a24'), 0.5)
SALAT = mat('salat', hex2('#9ccc5a'), 0.6, sss=0.2)
TOMATE = mat('tomate', hex2('#e2402f'), 0.3)
ROSA = mat('rosa', hex2('#f29bb0'), 0.5, sss=0.1)
LILA = mat('lila', hex2('#9b7fd1'), 0.5, sss=0.1)

# ---------------------------------------------------------------- Bausteine
def laube(x, y, s=1.0, rot=0.0):
    teile = []
    w, d, h = 1.6 * s, 1.3 * s, 1.0 * s
    teile.append(box('laube_sockel', (x, y, 0.06 * s), (w + 0.2 * s, d + 0.2 * s, 0.12 * s), STEIN, 0.02))
    teile.append(box('laube_wand', (x, y, 0.12 * s + h / 2), (w, d, h), HOLZ, 0.03))
    # Bretter (Rillen)
    for i in range(7):
        teile.append(box('brett', (x - w / 2 + (i + 0.5) * w / 7, y - d / 2 - 0.006, 0.12 * s + h / 2), (0.015 * s, 0.012, h * 0.98), HOLZ_D, 0))
    # Tür und Fenster vorn (−y)
    teile.append(box('tuer', (x - 0.35 * s, y - d / 2 - 0.02, 0.12 * s + 0.4 * s), (0.42 * s, 0.04, 0.8 * s), HOLZ_D, 0.015))
    teile.append(kugel('knauf', (x - 0.22 * s, y - d / 2 - 0.05, 0.12 * s + 0.4 * s), 0.025 * s, GELB, 2))
    teile.append(box('fensterrahmen', (x + 0.38 * s, y - d / 2 - 0.02, 0.12 * s + 0.6 * s), (0.5 * s, 0.04, 0.38 * s), WEISS, 0.015))
    teile.append(box('fenster', (x + 0.38 * s, y - d / 2 - 0.04, 0.12 * s + 0.6 * s), (0.4 * s, 0.02, 0.28 * s), FENSTER, 0.005))
    teile.append(box('blumenkasten', (x + 0.38 * s, y - d / 2 - 0.1, 0.12 * s + 0.36 * s), (0.52 * s, 0.14 * s, 0.1 * s), HOLZ_D, 0.01))
    for i in range(5):
        teile.append(kugel('bluete', (x + 0.18 * s + i * 0.1 * s, y - d / 2 - 0.1, 0.12 * s + 0.45 * s), 0.05 * s, [ROSA, GELB, LILA][i % 3], 2))
    # Satteldach: Giebel als dreieckiges Prisma, darauf zwei Dachflächen
    dz = 0.12 * s + h
    rh = 0.5 * s
    ueber = 0.18 * s
    import bmesh
    me = bpy.data.meshes.new('giebel')
    vs = []
    for vy in (-d / 2, d / 2):
        vs += [(x - w / 2, y + vy, dz), (x + w / 2, y + vy, dz), (x, y + vy, dz + rh)]
    me.from_pydata(vs, [], [(0, 1, 2), (5, 4, 3), (0, 3, 4, 1), (1, 4, 5, 2), (2, 5, 3, 0)])
    g = bpy.data.objects.new('giebel', me); bpy.context.collection.objects.link(g)
    teile.append(setze(g, HOLZ, 0))
    winkel = math.atan2(rh, w / 2)
    lang = (w / 2 + ueber) / math.cos(winkel)
    for seite in (-1, 1):
        mx = x + seite * (w / 2 + ueber) / 2
        mz = dz + rh - math.tan(winkel) * (w / 2 + ueber) / 2 + 0.045 * s
        teile.append(box('dach', (mx, y, mz), (lang, d + 2 * ueber, 0.07 * s), DACH, 0.025, rot=(0, seite * winkel, 0)))
    teile.append(box('first', (x, y, dz + rh + 0.05), (0.1 * s, d + 2 * ueber + 0.02, 0.08 * s), HOLZ_D, 0.02))
    # Schornstein-Ersatz: kleines Vordach-Schild weglassen; Regentonne
    teile.append(zyl('regentonne', (x + w / 2 + 0.22 * s, y - d / 2 + 0.15 * s, 0.25 * s), 0.17 * s, 0.5 * s, HOLZ_D, 20, bevel=0.02))
    teile.append(zyl('tonnenwasser', (x + w / 2 + 0.22 * s, y - d / 2 + 0.15 * s, 0.49 * s), 0.15 * s, 0.02, FENSTER, 20, bevel=0))
    for t in teile:
        t.rotation_euler.z += 0
    return teile

def baum(x, y, s=1.0, mit_aepfeln=True):
    zyl('stamm', (x, y, 0.55 * s), 0.07 * s, 1.1 * s, BRAUN, 12)
    kronen = [(0, 0, 1.35, 0.55), (0.32, 0.1, 1.15, 0.38), (-0.3, -0.12, 1.2, 0.4), (0.05, 0.28, 1.55, 0.36), (-0.1, -0.25, 1.6, 0.33)]
    for i, (dx, dy, z, r) in enumerate(kronen):
        k = kugel('krone', (x + dx * s, y + dy * s, z * s), r * s, LAUB if i % 2 else LAUB2, 3)
        # leicht verbeulen
        disp = k.modifiers.new('beulen', 'DISPLACE')
        tex = bpy.data.textures.new('wolke%d' % i, 'CLOUDS'); tex.noise_scale = 0.35
        disp.texture = tex; disp.strength = 0.07 * s
    if mit_aepfeln:
        for i in range(14):
            a = random.uniform(0, 2 * math.pi); b = random.uniform(-0.2, 0.9)
            r = 0.58 * s
            kugel('apfel', (x + math.cos(a) * r * math.cos(b), y + math.sin(a) * r * math.cos(b), 1.35 * s + math.sin(b) * r * 0.8), 0.055 * s, APFEL, 2)

def sonnenblume(x, y, h=1.0, neig=0.0):
    zyl('stiel', (x, y, h / 2), 0.02, h, LAUB2, 8)
    for i in range(2):
        kugel('blatt', (x + (0.08 if i else -0.08), y, h * (0.35 + i * 0.2)), 0.07, LAUB, 2, scale=(1.6, 0.6, 0.25))
    kopf_z = h + 0.02
    zyl('kopf', (x, y - 0.03, kopf_z), 0.11, 0.05, BRAUN, 20, rot=(math.radians(70 + neig), 0, 0))
    for i in range(14):
        a = i / 14 * 2 * math.pi
        bpy.ops.mesh.primitive_uv_sphere_add(segments=10, ring_count=6, radius=0.06, location=(0, 0, 0))
        p = bpy.context.object
        p.scale = (1.0, 0.4, 0.12)
        bpy.ops.object.transform_apply(scale=True)
        p.location = (0.15, 0, 0)
        bpy.ops.object.transform_apply(location=True)
        p.rotation_euler = (0, 0, a)
        bpy.ops.object.transform_apply(rotation=True)
        p.rotation_euler = (math.radians(70 + neig), 0, 0)
        p.location = (x, y - 0.03, kopf_z)
        setze(p, GELB, glatt=True)

def beet(x, y, w, d, pflanze='salat'):
    box('beetrand', (x, y, 0.09), (w + 0.12, d + 0.12, 0.18), HOLZ_D, 0.02)
    box('beeterde', (x, y, 0.12), (w, d, 0.18), ERDE, 0.03)
    reihen = max(2, int(d / 0.28))
    pro = max(3, int(w / 0.28))
    for r in range(reihen):
        for c in range(pro):
            px = x - w / 2 + (c + 0.5) * w / pro + random.uniform(-0.02, 0.02)
            py = y - d / 2 + (r + 0.5) * d / reihen + random.uniform(-0.02, 0.02)
            if pflanze == 'salat':
                kugel('salat', (px, py, 0.26), 0.1, SALAT, 2, scale=(1, 1, 0.7))
            elif pflanze == 'moehre':
                for k in range(3):
                    kegel('kraut', (px + random.uniform(-0.03, 0.03), py, 0.32), 0.025, 0.0, 0.18, LAUB, 6, rot=(random.uniform(-0.3, 0.3), random.uniform(-0.3, 0.3), 0))
                kegel('moehre', (px, py, 0.22), 0.035, 0.0, 0.08, MOEHRE, 10, rot=(math.pi, 0, 0))
            elif pflanze == 'tomate':
                zyl('stab', (px, py, 0.5), 0.012, 0.75, HOLZ, 6)
                kugel('busch', (px, py, 0.5), 0.13, LAUB2, 2, scale=(1, 1, 1.6))
                for k in range(3):
                    kugel('tomate', (px + random.uniform(-0.1, 0.1), py - 0.11, 0.35 + k * 0.13), 0.045, TOMATE, 2)
            elif pflanze == 'kuerbis':
                if (r + c) % 2 == 0:
                    kugel('kuerbis', (px, py, 0.3), 0.13, KUERBIS, 3, scale=(1.15, 1.15, 0.8))
                kugel('kuerbisblatt', (px + 0.08, py + 0.06, 0.24), 0.11, LAUB, 2, scale=(1.2, 1.2, 0.3))

def zaun(x0, y0, x1, y1, h=0.55):
    n = int(math.hypot(x1 - x0, y1 - y0) / 0.16)
    for i in range(n + 1):
        t = i / max(n, 1)
        px, py = x0 + (x1 - x0) * t, y0 + (y1 - y0) * t
        box('latte', (px, py, h / 2), (0.08, 0.04, h), WEISS, 0.012)
        kegel('spitze', (px, py, h + 0.03), 0.055, 0, 0.07, WEISS, 4, rot=(0, 0, math.pi / 4))
    for z in (0.18, 0.42):
        lang = math.hypot(x1 - x0, y1 - y0)
        box('querlatte', ((x0 + x1) / 2, (y0 + y1) / 2 + 0.03 * (1 if y0 == y1 else 0), z), (lang if y0 == y1 else 0.05, 0.03 if y0 == y1 else lang, 0.06), WEISS, 0.01)

def giesskanne(x, y, s=1.0, rot=0.0):
    k = zyl('kanne', (x, y, 0.22 * s), 0.17 * s, 0.4 * s, KANNE, 32, bevel=0.03)
    zyl('deckel', (x, y, 0.43 * s), 0.12 * s, 0.03 * s, KANNE, 32)
    bpy.ops.mesh.primitive_torus_add(major_radius=0.14 * s, minor_radius=0.022 * s, location=(x - 0.02 * s, y, 0.47 * s), rotation=(math.pi / 2, 0, 0))
    setze(bpy.context.object, KANNE, glatt=True)
    tuelle = zyl('tuelle', (x + 0.27 * s, y, 0.33 * s), 0.028 * s, 0.38 * s, KANNE, 16, rot=(0, math.radians(55), 0))
    zyl('brause', (x + 0.43 * s, y, 0.45 * s), 0.06 * s, 0.05 * s, KANNE, 20, rot=(0, math.radians(55), 0))

def weg(punkte):
    for (px, py) in punkte:
        bpy.ops.mesh.primitive_cylinder_add(vertices=10, radius=random.uniform(0.13, 0.17), depth=0.05, location=(px, py, 0.02))
        o = bpy.context.object; o.scale.x = random.uniform(1.0, 1.25); o.rotation_euler.z = random.uniform(0, 3)
        setze(o, STEIN, bevel=0.015, glatt=True)

def grasflaeche(w, d, dicke=0.35):
    box('erdsockel', (0, 0, -dicke / 2 - 0.02), (w, d, dicke), ERDE_D, 0.12)
    box('rasen', (0, 0, -0.05), (w, d, 0.1), GRAS, 0.08)
    # Grasbüschel
    for i in range(140):
        px, py = random.uniform(-w / 2 + 0.15, w / 2 - 0.15), random.uniform(-d / 2 + 0.15, d / 2 - 0.15)
        kegel('halm', (px, py, 0.04), 0.03, 0, 0.1, GRAS2, 5, rot=(random.uniform(-0.3, 0.3), random.uniform(-0.3, 0.3), 0))

# ---------------------------------------------------------------- Licht, Kamera, Render
def licht_und_kamera(ziel, abstand, ortho, winkel=(58, 0, 38), breite=1.0, hoehe=1.0):
    w = bpy.data.worlds.new('welt'); bpy.context.scene.world = w
    w.use_nodes = True
    bg = w.node_tree.nodes['Background']; bg.inputs['Color'].default_value = (0.82, 0.9, 0.95, 1); bg.inputs['Strength'].default_value = 0.75
    bpy.ops.object.light_add(type='SUN', rotation=(math.radians(48), math.radians(12), math.radians(28)))
    sun = bpy.context.object.data; sun.energy = 3.4; sun.angle = math.radians(10); sun.color = (1.0, 0.95, 0.86)
    bpy.ops.object.light_add(type='AREA', location=(-4, -5, 5))
    a = bpy.context.object; a.data.energy = 300; a.data.size = 6; a.data.color = (0.85, 0.92, 1.0)
    a.rotation_euler = (Vector(ziel) - a.location).to_track_quat('-Z', 'Y').to_euler()
    bpy.ops.object.camera_add()
    cam = bpy.context.object; bpy.context.scene.camera = cam
    rx, ry, rz = [math.radians(v) for v in winkel]
    richtung = Vector((math.sin(rx) * math.sin(rz), -math.sin(rx) * math.cos(rz), math.cos(rx)))
    cam.location = Vector(ziel) + richtung * abstand
    cam.rotation_euler = (Vector(ziel) - cam.location).to_track_quat('-Z', 'Y').to_euler()
    cam.data.type = 'ORTHO'; cam.data.ortho_scale = ortho
    sc = bpy.context.scene
    sc.render.engine = 'CYCLES'
    sc.cycles.device = 'CPU'; sc.cycles.samples = SAMPLES; sc.cycles.use_denoising = True
    sc.cycles.max_bounces = 6
    sc.render.film_transparent = True
    sc.render.resolution_x = int(GROESSE * breite); sc.render.resolution_y = int(GROESSE * hoehe)
    sc.render.image_settings.file_format = 'PNG'; sc.render.image_settings.color_mode = 'RGBA'
    sc.view_settings.view_transform = 'AgX'; sc.view_settings.look = 'AgX - Medium High Contrast'
    # Schatten auf transparentem Hintergrund behalten
    sc.cycles.film_transparent_glass = True

def render(name):
    bpy.context.scene.render.filepath = os.path.join(OUT, name + '.png')
    bpy.ops.render.render(write_still=True)

# ---------------------------------------------------------------- Szenen
if WAS == 'hero':
    grasflaeche(6.4, 4.6)
    laube(-1.6, 1.0, 1.0)
    baum(1.9, 1.3, 1.0)
    beet(-0.1, -0.9, 1.5, 0.75, 'salat')
    beet(1.75, -0.9, 1.3, 0.75, 'tomate')
    beet(-0.1, 0.3, 1.5, 0.6, 'moehre')
    beet(1.0, 0.25, 0.7, 0.6, 'kuerbis')
    weg([(-1.5 + i * 0.05, -0.2 - i * 0.38) for i in range(5)] + [(-1.1 + i * 0.4, -1.75 + 0.0) for i in range(0)])
    zaun(-3.1, -2.2, 3.1, -2.2)
    for i, (sx, sy, h) in enumerate([(-2.8, -1.7, 1.15), (-2.55, -1.85, 0.95), (-2.95, -1.25, 1.3)]):
        sonnenblume(sx, sy, h, neig=-8 * i)
    giesskanne(0.85, -1.75, 0.8)
    licht_und_kamera((0.0, -0.05, 0.35), 30, 8.0, winkel=(58, 0, 32), breite=1.0, hoehe=0.7)
    render('hero')
elif WAS == 'laube':
    grasflaeche(2.6, 2.3)
    laube(0, 0.15, 1.0)
    sonnenblume(-1.0, -0.75, 1.0)
    weg([(0.35 - i * 0.05, -0.75 - i * 0.0) for i in range(1)])
    licht_und_kamera((0, 0, 0.6), 30, 3.6)
    render('laube')
elif WAS == 'kanne':
    box('teller', (0, 0, -0.04), (1.3, 1.3, 0.08), GRAS, 0.3)
    giesskanne(-0.08, 0, 1.4)
    licht_und_kamera((0.1, 0, 0.35), 30, 1.9, winkel=(62, 0, 20))
    render('kanne')
elif WAS == 'beet':
    box('teller', (0, 0, -0.06), (2.0, 1.5, 0.12), GRAS, 0.3)
    beet(0, 0.2, 1.4, 0.55, 'moehre')
    beet(0, -0.45, 1.4, 0.45, 'salat')
    licht_und_kamera((0, 0, 0.15), 30, 2.9)
    render('beet')
elif WAS == 'baum':
    box('teller', (0, 0, -0.06), (1.6, 1.6, 0.12), GRAS, 0.6)
    baum(0, 0, 1.0)
    licht_und_kamera((0, 0, 0.85), 30, 3.2, winkel=(65, 0, 25))
    render('baum')
elif WAS == 'sonne':
    box('teller', (0, 0, -0.05), (1.2, 1.0, 0.1), GRAS, 0.3)
    for i, (sx, sy, h) in enumerate([(0, 0, 1.25), (-0.3, 0.15, 1.0), (0.32, 0.1, 0.9)]):
        sonnenblume(sx, sy, h, neig=-6 * i)
    licht_und_kamera((0, 0, 0.65), 30, 2.5, winkel=(70, 0, 15))
    render('sonne')

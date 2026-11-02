# Stilisierte Kleingartenanlage (Low-Poly, isometrisch) für die Oberfläche des Kleingarten-Managers.
# Erzeugt web/bilder/garten.webp (nach dem Rendern zuschneiden und als WebP speichern).
# Aufruf mit Blender als Python-Modul (pip install bpy, Python 3.13):
#   python garten.py <ausgabe.png> <breite> <hoehe> [samples]
# Verwendet wurde: python garten.py garten.png 1200 760 40
import math
import random
import sys

import bpy

out, W, H = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
SAMPLES = int(sys.argv[4]) if len(sys.argv) > 4 else 64
random.seed(7)

bpy.ops.wm.read_factory_settings(use_empty=True)
scene = bpy.context.scene


def mat(name, rgb, rough=0.8, alpha=1.0, transm=0.0):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    b = m.node_tree.nodes["Principled BSDF"]
    b.inputs["Base Color"].default_value = (*rgb, 1)
    b.inputs["Roughness"].default_value = rough
    if transm:
        b.inputs["Transmission Weight"].default_value = transm
        b.inputs["IOR"].default_value = 1.2
    if alpha < 1:
        b.inputs["Alpha"].default_value = alpha
    return m


def hex2(h):
    h = h.lstrip("#")
    c = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    return tuple(x ** 2.2 for x in c)  # sRGB → linear


M = {
    "wiese": mat("wiese", hex2("8fbf6a")),
    "parzelle": mat("parzelle", hex2("7cb35a")),
    "weg": mat("weg", hex2("e8dcc0")),
    "erde": mat("erde", hex2("7a5534")),
    "holz": mat("holz", hex2("c98f55")),
    "holzdunkel": mat("holzdunkel", hex2("8a5a33")),
    "dach": mat("dach", hex2("b5473a")),
    "dach2": mat("dach2", hex2("3f6f8f")),
    "fenster": mat("fenster", hex2("cfe8f2"), rough=0.1),
    "weiss": mat("weiss", hex2("f4f1ea")),
    "stamm": mat("stamm", hex2("6b4a2b")),
    "laub1": mat("laub1", hex2("3f8a43")),
    "laub2": mat("laub2", hex2("5aa64a")),
    "laub3": mat("laub3", hex2("2f6f3e")),
    "apfel": mat("apfel", hex2("d8412f"), rough=0.4),
    "glas": mat("glas", hex2("d8f0f5"), rough=0.05, transm=0.85),
    "alu": mat("alu", hex2("dfe3e6"), rough=0.3),
    "tonne": mat("tonne", hex2("2f6fbd"), rough=0.5),
    "blume1": mat("blume1", hex2("f2c230")),
    "blume2": mat("blume2", hex2("e35d8a")),
    "blume3": mat("blume3", hex2("ffffff")),
    "gemuese": mat("gemuese", hex2("67b84e")),
    "kuerbis": mat("kuerbis", hex2("e8892c")),
    "zaun": mat("zaun", hex2("f2ede2")),
}


def box(name, loc, size, m, bevel=0.0):
    bpy.ops.mesh.primitive_cube_add(location=loc)
    o = bpy.context.object
    o.name = name
    o.scale = (size[0] / 2, size[1] / 2, size[2] / 2)
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    if bevel:
        mod = o.modifiers.new("bevel", "BEVEL")
        mod.width = bevel
        mod.segments = 2
    o.data.materials.append(m)
    return o


def zyl(loc, r, h, m, verts=10):
    bpy.ops.mesh.primitive_cylinder_add(vertices=verts, radius=r, depth=h, location=loc)
    o = bpy.context.object
    o.data.materials.append(m)
    return o


def kugel(loc, r, m, sub=1, quetsch=1.0):
    bpy.ops.mesh.primitive_ico_sphere_add(subdivisions=sub, radius=r, location=loc)
    o = bpy.context.object
    o.scale.z = quetsch
    o.data.materials.append(m)
    bpy.ops.object.shade_flat()
    return o


def dach(loc, breite, tiefe, hoehe, m, ueberstand=0.25):
    # Satteldach als Prisma (First entlang der Tiefe)
    x, y, z = loc
    bw, bt = breite / 2 + ueberstand, tiefe / 2 + ueberstand
    verts = [(-bw, -bt, 0), (bw, -bt, 0), (0, -bt, hoehe), (-bw, bt, 0), (bw, bt, 0), (0, bt, hoehe)]
    faces = [(0, 1, 2), (3, 5, 4), (0, 3, 4, 1), (1, 4, 5, 2), (0, 2, 5, 3)]
    me = bpy.data.meshes.new("dach")
    me.from_pydata(verts, [], faces)
    o = bpy.data.objects.new("dach", me)
    o.location = (x, y, z)
    bpy.context.collection.objects.link(o)
    o.data.materials.append(m)
    sol = o.modifiers.new("dicke", "SOLIDIFY")
    sol.thickness = 0.08
    return o


def baum(x, y, groesse=1.0, aepfel=True):
    zyl((x, y, 0.55 * groesse), 0.09 * groesse, 1.1 * groesse, M["stamm"], 7)
    laub = random.choice([M["laub1"], M["laub2"], M["laub3"]])
    kugel((x, y, 1.35 * groesse), 0.62 * groesse, laub, sub=1, quetsch=0.9)
    kugel((x + 0.28 * groesse, y - 0.15 * groesse, 1.15 * groesse), 0.4 * groesse, laub, sub=1)
    if aepfel:
        for _ in range(7):
            a = random.uniform(0, 2 * math.pi)
            e = random.uniform(-0.2, 0.6)
            r = 0.6 * groesse
            kugel((x + r * math.cos(a) * math.cos(e), y + r * math.sin(a) * math.cos(e), 1.35 * groesse + r * 0.9 * math.sin(e)), 0.06 * groesse, M["apfel"], sub=1)


def strauch(x, y, r=0.3, m=None, blueten=None):
    kugel((x, y, r * 0.75), r, m or M["laub2"], sub=1, quetsch=0.8)
    if blueten:
        for _ in range(5):
            a = random.uniform(0, 2 * math.pi)
            kugel((x + r * 0.8 * math.cos(a), y + r * 0.8 * math.sin(a), r * 0.9), 0.06, blueten, sub=1)


def beet(x, y, b, t, reihen=3, pflanze=None):
    box("beet", (x, y, 0.08), (b, t, 0.16), M["erde"], bevel=0.03)
    for i in range(reihen):
        yy = y - t / 2 + t * (i + 0.5) / reihen
        n = max(2, int(b / 0.32))
        for k in range(n):
            xx = x - b / 2 + b * (k + 0.5) / n
            if pflanze == "kuerbis" and k % 2 == 0:
                kugel((xx, yy, 0.25), 0.14, M["kuerbis"], sub=1, quetsch=0.75)
            else:
                kugel((xx, yy, 0.24), 0.11, M["gemuese"], sub=1)


def zaun(x0, y0, x1, y1, hoehe=0.45):
    laenge = math.hypot(x1 - x0, y1 - y0)
    n = max(2, int(laenge / 0.35))
    for i in range(n + 1):
        t = i / n
        box("latte", (x0 + (x1 - x0) * t, y0 + (y1 - y0) * t, hoehe / 2), (0.06, 0.06, hoehe), M["zaun"])
    winkel = math.atan2(y1 - y0, x1 - x0)
    for hz in (0.15, 0.36):
        o = box("riegel", ((x0 + x1) / 2, (y0 + y1) / 2, hz), (laenge, 0.04, 0.05), M["zaun"])
        o.rotation_euler.z = winkel


def laube(x, y, b=2.2, t=1.8, h=1.3, dachm=None, holz=None):
    box("laube", (x, y, h / 2), (b, t, h), holz or M["holz"], bevel=0.03)
    dach((x, y, h), b, t, 0.75, dachm or M["dach"])
    box("tuer", (x - b / 2 - 0.005, y + 0.25, 0.5), (0.02, 0.45, 1.0), M["holzdunkel"])
    box("fenster", (x - b / 2 - 0.006, y - 0.45, 0.78), (0.02, 0.5, 0.4), M["fenster"])
    box("rahmen", (x - b / 2 - 0.004, y - 0.45, 0.78), (0.02, 0.56, 0.46), M["weiss"])
    box("fenster2", (x + 0.3, y - t / 2 - 0.005, 0.78), (0.6, 0.02, 0.4), M["fenster"])
    box("terrasse", (x - b / 2 - 0.55, y, 0.04), (1.0, t + 0.4, 0.08), M["holzdunkel"])


def gewaechshaus(x, y):
    box("gh", (x, y, 0.5), (1.2, 0.9, 1.0), M["glas"])
    dach((x, y, 1.0), 1.2, 0.9, 0.4, M["glas"], ueberstand=0.0)
    for dx in (-0.6, 0.6):
        for dy in (-0.45, 0.45):
            box("pfosten", (x + dx, y + dy, 0.5), (0.04, 0.04, 1.0), M["alu"])
    for _ in range(4):
        kugel((x + random.uniform(-0.35, 0.35), y + random.uniform(-0.25, 0.25), 0.25), 0.15, M["laub2"], sub=1)
        kugel((x + random.uniform(-0.3, 0.3), y + random.uniform(-0.2, 0.2), 0.42), 0.05, M["apfel"], sub=1)


# ---------------------------------------------------------------- Gelände
box("wiese", (0, 0, -0.1), (22, 16, 0.2), M["wiese"])
# Hauptweg (in y-Richtung) und Querweg
box("weg", (0, 0, 0.005), (1.2, 16, 0.02), M["weg"])
box("querweg", (0, -0.2, 0.006), (22, 1.0, 0.02), M["weg"])

# vier Parzellen
parzellen = [(-5.2, 4.0), (5.2, 4.0), (-5.2, -4.4), (5.2, -4.4)]
for i, (px, py) in enumerate(parzellen):
    box("parzelle", (px, py, 0.02), (9.0, 6.6, 0.04), M["parzelle"])
    # Zaun rundherum, an der Wegseite mit Tor-Lücke
    xa, xb, ya, yb = px - 4.5, px + 4.5, py - 3.3, py + 3.3
    zaun(xa, ya, xb, ya)
    zaun(xa, yb, xb, yb)
    weg_seite = xb if px < 0 else xa
    andere = xa if px < 0 else xb
    zaun(andere, ya, andere, yb)
    zaun(weg_seite, ya, weg_seite, py - 0.6)
    zaun(weg_seite, py + 0.6, weg_seite, yb)
    # Gartenweg zur Laube
    box("pfad", (px + (1.5 if px < 0 else -1.5), py, 0.045), (3.0, 0.7, 0.02), M["weg"])

# Parzelle 1: Laube, Obstbäume, Beete
laube(-7.0, 5.0)
baum(-2.6, 6.1, 1.0)
baum(-8.4, 1.8, 0.85)
beet(-4.6, 2.3, 2.4, 1.2, 2)
beet(-4.6, 5.8, 1.6, 1.0, 2, "kuerbis")
strauch(-1.6, 1.6, 0.35, M["laub1"], M["blume2"])
strauch(-1.6, 3.1, 0.3, M["laub2"], M["blume1"])

# Parzelle 2: Laube mit blauem Dach, Gewächshaus, Regentonne
laube(7.6, 5.2, dachm=M["dach2"], holz=M["weiss"])
gewaechshaus(3.4, 6.2)
zyl((6.1, 6.6, 0.35), 0.28, 0.7, M["tonne"], 14)
baum(8.8, 1.6, 1.15)
beet(4.3, 2.2, 2.8, 1.4, 3)
strauch(2.0, 4.3, 0.32, M["laub3"], M["blume3"])

# Parzelle 3: Laube, Rasen mit Blumen, Baum
laube(-7.4, -3.4, b=2.0, t=1.6)
baum(-2.4, -6.0, 1.2)
baum(-4.2, -2.1, 0.75)
for k in range(6):
    strauch(-1.4 - (k % 3) * 0.7, -2.0 - (k // 3) * 0.9, 0.22, M["laub2"], [M["blume1"], M["blume2"], M["blume3"]][k % 3])
beet(-5.8, -6.4, 2.2, 0.9, 2)

# Parzelle 4: freier Garten (frisch umgegraben) mit Schild
box("umgegraben", (5.2, -4.4, 0.05), (6.0, 4.0, 0.06), M["erde"])
for r in range(7):
    box("furche", (5.2, -6.1 + r * 0.55, 0.09), (5.6, 0.08, 0.04), M["holzdunkel"])
zyl((3.0, -1.7, 0.45), 0.04, 0.9, M["holzdunkel"], 6)
box("schild", (3.0, -1.7, 0.95), (0.06, 0.8, 0.45), M["weiss"], bevel=0.02)
baum(9.0, -7.0, 0.9, aepfel=False)

# Bäume am Rand
for (x, y, g) in [(-10.3, 7.2, 1.3), (10.4, 7.0, 1.2), (-10.5, -7.3, 1.1), (10.2, -0.9, 1.0), (-10.4, -0.6, 0.95)]:
    baum(x, y, g, aepfel=False)

# ---------------------------------------------------------------- Licht, Kamera, Rendern
world = bpy.data.worlds.new("welt")
scene.world = world
world.use_nodes = True
bg = world.node_tree.nodes["Background"]
bg.inputs["Color"].default_value = (0.92, 0.96, 1.0, 1)
bg.inputs["Strength"].default_value = 0.9

bpy.ops.object.light_add(type="SUN", location=(0, 0, 10))
sonne = bpy.context.object
sonne.data.energy = 3.4
sonne.data.angle = math.radians(8)
sonne.rotation_euler = (math.radians(48), math.radians(14), math.radians(-38))

bpy.ops.object.camera_add()
cam = bpy.context.object
cam.data.type = "ORTHO"
cam.data.ortho_scale = 28.5
# klassische Iso-Ansicht
cam.rotation_euler = (math.radians(58), 0, math.radians(45))
d = 30
cam.location = (d * math.sin(math.radians(45)) * math.sin(math.radians(58)), -d * math.cos(math.radians(45)) * math.sin(math.radians(58)), d * math.cos(math.radians(58)))
cam.location = (cam.location.x, cam.location.y, cam.location.z)
scene.camera = cam

scene.render.engine = "CYCLES"
scene.cycles.device = "CPU"
scene.cycles.samples = SAMPLES
scene.cycles.use_denoising = True
scene.render.film_transparent = True
scene.render.resolution_x = W
scene.render.resolution_y = H
scene.render.resolution_percentage = 100
scene.view_settings.view_transform = "AgX"
scene.view_settings.look = "AgX - Medium High Contrast"
scene.render.image_settings.file_format = "PNG"
scene.render.image_settings.color_mode = "RGBA"
scene.render.filepath = out
bpy.ops.render.render(write_still=True)
print("fertig:", out)

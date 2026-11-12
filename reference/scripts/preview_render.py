"""Tiny software renderer: render a GLB mesh with a given texture from several angles.
usage: render.py model.glb texture.png out.png [size]"""
import sys, struct, json
import numpy as np
from PIL import Image


def load_glb(path):
    data = open(path, 'rb').read()
    off = 12
    js = binc = None
    while off < len(data):
        ln, typ = struct.unpack_from('<II', data, off)
        chunk = data[off + 8: off + 8 + ln]
        if typ == 0x4E4F534A:
            js = json.loads(chunk)
        else:
            binc = chunk
        off += 8 + ln
    return js, binc


def accessor(js, binc, idx):
    a = js['accessors'][idx]
    bv = js['bufferViews'][a['bufferView']]
    comp = {5126: np.float32, 5125: np.uint32, 5123: np.uint16, 5121: np.uint8}[a['componentType']]
    n = {'SCALAR': 1, 'VEC2': 2, 'VEC3': 3, 'VEC4': 4}[a['type']]
    start = bv.get('byteOffset', 0) + a.get('byteOffset', 0)
    stride = bv.get('byteStride', 0)
    isz = np.dtype(comp).itemsize * n
    if stride and stride != isz:
        raw = np.frombuffer(binc, np.uint8, count=stride * a['count'], offset=start).reshape(a['count'], stride)[:, :isz]
        arr = raw.copy().view(comp).reshape(a['count'], n)
    else:
        arr = np.frombuffer(binc, comp, count=a['count'] * n, offset=start).reshape(a['count'], n)
    return arr.astype(np.float64 if comp == np.float32 else np.int64)


def load_mesh(path):
    js, binc = load_glb(path)
    P, UV, I = [], [], []
    base = 0
    mesh = js['meshes'][0]
    for prim in mesh['primitives']:
        at = prim['attributes']
        p = accessor(js, binc, at['POSITION'])
        uv = accessor(js, binc, at['TEXCOORD_0'])
        idx = accessor(js, binc, prim['indices']).reshape(-1, 3)
        P.append(p); UV.append(uv); I.append(idx + base); base += len(p)
    return np.vstack(P), np.vstack(UV), np.vstack(I)


def rot(yaw, pitch):
    cy, sy = np.cos(yaw), np.sin(yaw)
    cp, sp = np.cos(pitch), np.sin(pitch)
    Ry = np.array([[cy, 0, sy], [0, 1, 0], [-sy, 0, cy]])
    Rx = np.array([[1, 0, 0], [0, cp, -sp], [0, sp, cp]])
    return Rx @ Ry


def render(P, UV, I, tex, R, size=400):
    tex = np.asarray(tex.convert('RGB'), np.float64) / 255
    th, tw = tex.shape[:2]
    c = (P.max(0) + P.min(0)) / 2
    Q = (P - c) @ R.T
    s = size * 0.45 / np.abs(Q[:, :2]).max()
    X = Q[:, 0] * s + size / 2
    Y = -Q[:, 1] * s + size / 2
    Z = Q[:, 2]
    img = np.full((size, size, 3), 0.15)
    zb = np.full((size, size), -np.inf)
    light = np.array([0.3, 0.5, 0.8]); light /= np.linalg.norm(light)
    for a, b, cc in I:
        xs = np.array([X[a], X[b], X[cc]]); ys = np.array([Y[a], Y[b], Y[cc]])
        n = np.cross(Q[b] - Q[a], Q[cc] - Q[a]); nn = np.linalg.norm(n)
        if nn == 0: continue
        n /= nn
        x0, x1 = int(max(xs.min(), 0)), int(min(xs.max() + 1, size))
        y0, y1 = int(max(ys.min(), 0)), int(min(ys.max() + 1, size))
        if x0 >= x1 or y0 >= y1: continue
        gx, gy = np.meshgrid(np.arange(x0, x1) + .5, np.arange(y0, y1) + .5)
        d = (ys[1] - ys[2]) * (xs[0] - xs[2]) + (xs[2] - xs[1]) * (ys[0] - ys[2])
        if abs(d) < 1e-12: continue
        w0 = ((ys[1] - ys[2]) * (gx - xs[2]) + (xs[2] - xs[1]) * (gy - ys[2])) / d
        w1 = ((ys[2] - ys[0]) * (gx - xs[2]) + (xs[0] - xs[2]) * (gy - ys[2])) / d
        w2 = 1 - w0 - w1
        m = (w0 >= 0) & (w1 >= 0) & (w2 >= 0)
        if not m.any(): continue
        z = w0 * Z[a] + w1 * Z[b] + w2 * Z[cc]
        sub = zb[y0:y1, x0:x1]
        m &= z > sub
        if not m.any(): continue
        u = w0 * UV[a, 0] + w1 * UV[b, 0] + w2 * UV[cc, 0]
        v = w0 * UV[a, 1] + w1 * UV[b, 1] + w2 * UV[cc, 1]
        tx = (np.mod(u, 1) * tw).astype(int).clip(0, tw - 1)
        ty = (np.mod(v, 1) * th).astype(int).clip(0, th - 1)
        col = tex[ty, tx] * (0.45 + 0.55 * abs(n @ light))
        sub[m] = z[m]
        img[y0:y1, x0:x1][m] = col[m]
    return Image.fromarray((img * 255).clip(0, 255).astype(np.uint8))


if __name__ == '__main__':
    P, UV, I = load_mesh(sys.argv[1])
    tex = Image.open(sys.argv[2])
    size = int(sys.argv[4]) if len(sys.argv) > 4 else 400
    print('verts', len(P), 'tris', len(I), 'bbox', P.min(0), P.max(0), 'uv', UV.min(0), UV.max(0))
    views = [(0, 0), (np.pi / 2, 0), (np.pi, 0), (-np.pi / 2, 0), (0, np.pi / 2), (0.6, 0.5)]
    imgs = [render(P, UV, I, tex, rot(y, p), size) for y, p in views]
    out = Image.new('RGB', (size * 3, size * 2))
    for k, im in enumerate(imgs):
        out.paste(im, ((k % 3) * size, (k // 3) * size))
    out.save(sys.argv[3])

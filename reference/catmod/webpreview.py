"""Self-contained HTML page with an interactive three.js preview of the sphere.

The sphere is generated in JS with the same formula/UV convention as catmod.mesh, and the texture
is embedded, so the page works offline-ish (three.js comes from a CDN) and is the seed of the
future website's live preview. Material/lighting is an approximation of the game's NPR shader.
"""
from __future__ import annotations

import base64
import io

from PIL import Image

PAGE = """<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Soul Container Preview</title>
<style>
  :root { --bg:#1d1f22; --fg:#e8e8e8; --muted:#9aa0a6; }
  html,body { margin:0; height:100%; background:var(--bg); color:var(--fg); font:14px system-ui,sans-serif; }
  #c { width:100vw; height:100vh; display:block; }
  #hud { position:fixed; left:16px; top:12px; right:16px; pointer-events:none; }
  #hud b { font-size:16px; } #hud span { color:var(--muted); }
  #ui { position:fixed; left:16px; bottom:16px; display:flex; gap:12px; flex-wrap:wrap; }
  label { background:#0006; padding:6px 10px; border-radius:6px; }
</style>
<script type="importmap">{"imports":{"three":"https://cdn.jsdelivr.net/npm/three@0.160.0/build/three.module.js",
"three/addons/":"https://cdn.jsdelivr.net/npm/three@0.160.0/examples/jsm/"}}</script>
</head><body>
<canvas id="c"></canvas>
<div id="hud"><b>Soul Container preview</b> <span>— arrastra para girar, rueda para zoom. Iluminación aproximada (no es el shader del juego).</span></div>
<div id="ui">
  <label><input type="checkbox" id="spin" checked> girar</label>
  <label><input type="checkbox" id="lit" checked> iluminación</label>
</div>
<script type="module">
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
const TEX = "data:image/jpeg;base64,__TEX__";
const R = __RADIUS__, NU = 96, NV = 48;
const pos = [], uv = [], idx = [];
for (let j = 0; j <= NV; j++) for (let i = 0; i <= NU; i++) {
  const u = i / NU, v = j / NV, th = v * Math.PI, ph = u * 2 * Math.PI;
  const x = Math.sin(th) * Math.cos(ph), y = Math.sin(th) * Math.sin(ph), z = Math.cos(th);
  pos.push(R * x, R * z, -R * y);           // Source (x fwd, y left, z up) -> three (Y up)
  uv.push(u, v);
}
for (let j = 0; j < NV; j++) for (let i = 0; i < NU; i++) {
  const a = j * (NU + 1) + i, b = a + 1, c = a + NU + 1, d = c + 1;
  idx.push(a, c, b, b, c, d);              // same outward winding as catmod.mesh
}
const geo = new THREE.BufferGeometry();
geo.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
geo.setAttribute('uv', new THREE.Float32BufferAttribute(uv, 2));
geo.setIndex(idx); geo.computeVertexNormals();
const tex = new THREE.TextureLoader().load(TEX);
tex.flipY = false; tex.colorSpace = THREE.SRGBColorSpace; tex.anisotropy = 8;
const litMat = new THREE.MeshStandardMaterial({ map: tex, roughness: __ROUGH__, metalness: 0 });
const flatMat = new THREE.MeshBasicMaterial({ map: tex });
const mesh = new THREE.Mesh(geo, litMat);
const scene = new THREE.Scene(); scene.background = new THREE.Color(0x1d1f22); scene.add(mesh);
scene.add(new THREE.HemisphereLight(0xffffff, 0x404040, 1.6));
const sun = new THREE.DirectionalLight(0xffffff, 1.8); sun.position.set(3, 5, 4); scene.add(sun);
const canvas = document.getElementById('c');
const renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
renderer.setPixelRatio(devicePixelRatio);
const cam = new THREE.PerspectiveCamera(40, 1, 0.1, 1000); cam.position.set(R * 3.2, R * 0.8, 0);
const controls = new OrbitControls(cam, canvas); controls.enableDamping = true;
function resize() { renderer.setSize(innerWidth, innerHeight, false); cam.aspect = innerWidth / innerHeight; cam.updateProjectionMatrix(); }
addEventListener('resize', resize); resize();
const spin = document.getElementById('spin'), lit = document.getElementById('lit');
lit.onchange = () => mesh.material = lit.checked ? litMat : flatMat;
renderer.setAnimationLoop(() => { if (spin.checked) mesh.rotation.y += 0.004; controls.update(); renderer.render(scene, cam); });
</script></body></html>
"""


def preview_html(texture: Image.Image, radius: float = 6.3, roughness: float = 0.69, tex_size: int = 1024) -> str:
    buf = io.BytesIO()
    texture.convert("RGB").resize((tex_size, tex_size), Image.LANCZOS).save(buf, "JPEG", quality=88)
    b64 = base64.b64encode(buf.getvalue()).decode()
    return (PAGE.replace("__TEX__", b64).replace("__RADIUS__", f"{radius}")
            .replace("__ROUGH__", f"{roughness}"))

// 3D preview of the soul container, as close to the game as we can get in a browser:
//   - the exact mesh the game draws (/api/mesh: same vertices, UVs decoded from the stored half
//     floats, same triangles), not a generic three.js sphere
//   - the texture produced by the same Go code that builds the mod (/api/texture)
//   - a shader driven by the real material values (soulMaterial.js) + the glow/ember particles
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { createSoulMaterial, createOutlineMaterial } from './soulMaterial.js';
import { createGlow, createEmbers } from './effects.js';

// Source is Z-up (x forward, y left); three.js is Y-up: (x, y, z) -> (x, z, -y).
export function geometryFromMesh(m) {
  const n = m.positions.length / 3;
  const pos = new Float32Array(n * 3), nrm = new Float32Array(n * 3);
  for (let i = 0; i < n; i++) {
    const x = m.positions[i * 3], y = m.positions[i * 3 + 1], z = m.positions[i * 3 + 2];
    pos.set([x, z, -y], i * 3);
    if (m.normals) { // the normals the game unpacks (every shape, not only the sphere)
      const a = m.normals[i * 3], b = m.normals[i * 3 + 1], c = m.normals[i * 3 + 2];
      nrm.set([a, c, -b], i * 3);
    } else {
      const len = Math.hypot(x, y, z) || 1;
      nrm.set([x / len, z / len, -y / len], i * 3);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
  geo.setAttribute('normal', new THREE.BufferAttribute(nrm, 3));
  geo.setAttribute('uv', new THREE.BufferAttribute(new Float32Array(m.uvs), 2));
  geo.setIndex(new THREE.BufferAttribute(new Uint32Array(m.indices), 1));
  geo.computeBoundingSphere();
  geo.computeBoundingBox();
  return geo;
}

function placeholderTexture() {
  const c = document.createElement('canvas');
  c.width = 512; c.height = 256;
  const g = c.getContext('2d');
  g.fillStyle = '#18788c';
  g.fillRect(0, 0, 512, 256);
  const tex = new THREE.CanvasTexture(c);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

export function createViewer(canvas, { mesh, background = 0x14181c } = {}) {
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, preserveDrawingBuffer: true });
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.0;

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(background);
  const camera = new THREE.PerspectiveCamera(35, 1, 0.1, 500);
  const controls = new OrbitControls(camera, canvas);
  controls.enableDamping = true;
  controls.enablePan = false;
  let userMoved = false;
  controls.addEventListener('start', () => { userMoved = true; });

  const material = createSoulMaterial(placeholderTexture());
  const outline = createOutlineMaterial();
  const orb = new THREE.Group();
  scene.add(orb);
  let body = null, hull = null, radius = 6.3;

  const glow = createGlow(radius);
  const embers = createEmbers(radius);
  scene.add(glow, embers);

  // soft contact shadow so the orb does not float in a void
  const shadowTex = (() => {
    const c = document.createElement('canvas'); c.width = c.height = 128;
    const g = c.getContext('2d'), gr = g.createRadialGradient(64, 64, 0, 64, 64, 64);
    gr.addColorStop(0, 'rgba(0,0,0,0.55)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
    g.fillStyle = gr; g.fillRect(0, 0, 128, 128);
    return new THREE.CanvasTexture(c);
  })();
  const shadow = new THREE.Mesh(new THREE.PlaneGeometry(1, 1),
    new THREE.MeshBasicMaterial({ map: shadowTex, transparent: true, depthWrite: false }));
  shadow.rotation.x = -Math.PI / 2;
  scene.add(shadow);

  function setMesh(m) {
    if (body) { orb.remove(body, hull); body.geometry.dispose(); }
    const geo = geometryFromMesh(m);
    radius = geo.boundingSphere.radius;
    body = new THREE.Mesh(geo, material);
    hull = new THREE.Mesh(geo, outline);
    orb.add(hull, body);
    shadow.scale.setScalar(radius * 2.6);
    shadow.position.y = geo.boundingBox.min.y - radius * 0.02;
    controls.target.set(0, 0, 0);
    controls.minDistance = radius * 1.8;
    controls.maxDistance = radius * 12;
    frameSphere();
  }

  // Distance at which the sphere (plus a margin for the glow) fits the narrower side of the view.
  const viewDir = new THREE.Vector3(3.6, 1.3, 2.2).normalize();
  function frameSphere() {
    const vfov = (camera.fov * Math.PI) / 180;
    const hfov = 2 * Math.atan(Math.tan(vfov / 2) * camera.aspect);
    const dist = (radius * 1.35) / Math.sin(Math.min(vfov, hfov) / 2);
    camera.position.copy(viewDir).multiplyScalar(dist);
    controls.update();
  }
  if (mesh) setMesh(mesh);

  async function setTextureBlob(blob) {
    const bitmap = await createImageBitmap(blob, { imageOrientation: 'none' });
    const tex = new THREE.Texture(bitmap);
    tex.flipY = false; // row 0 of the texture is the top pole (v = 0), as in the game
    tex.colorSpace = THREE.SRGBColorSpace;
    tex.anisotropy = renderer.capabilities.getMaxAnisotropy();
    tex.generateMipmaps = true;
    tex.minFilter = THREE.LinearMipmapLinearFilter;
    tex.needsUpdate = true;
    const old = material.uniforms.uMap.value;
    material.uniforms.uMap.value = tex;
    old.dispose();
  }

  const options = { spin: true, effects: true, pulse: true, outline: true };
  function set(opts) { Object.assign(options, opts); }

  function resize() {
    const w = canvas.clientWidth, h = canvas.clientHeight;
    if (canvas.width !== Math.floor(w * renderer.getPixelRatio()) || canvas.height !== Math.floor(h * renderer.getPixelRatio())) {
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      if (!userMoved) frameSphere(); // keep it framed until the user takes control
    }
  }

  const timer = new THREE.Timer();
  let time = 0;
  function frame(timestamp) {
    resize();
    timer.update(timestamp);
    const dt = Math.min(timer.getDelta(), 0.1);
    time += dt;
    if (options.spin) { orb.rotation.y += dt * 0.35; orb.rotation.x = Math.sin(time * 0.4) * 0.35; }
    material.uniforms.uTime.value = time;
    material.uniforms.uPulse.value = options.pulse ? 1 : 0;
    if (hull) hull.visible = options.outline;
    glow.visible = embers.visible = options.effects;
    if (options.effects) embers.userData.update(dt, canvas.clientHeight * renderer.getPixelRatio(), camera.fov);
    controls.update();
    renderer.render(scene, camera);
  }
  renderer.setAnimationLoop(frame);

  return {
    setMesh, setTextureBlob, set,
    material, // exposed for calibration (uniforms)
    snapshot: () => canvas.toDataURL('image/png'),
    dispose() { renderer.setAnimationLoop(null); renderer.dispose(); controls.dispose(); timer.dispose?.(); },
  };
}

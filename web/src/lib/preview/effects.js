// The two child particles the game draws around the soul container: a soft teal glow and
// rising embers. Numbers come from the .vpcf files (see gameParams.js), scaled to our model.
import * as THREE from 'three';
import { EMBERS, GLOW, MODEL_SCALE } from './gameParams.js';

// soft = wide falloff (glow halo); otherwise a bright core with a short falloff (ember spark)
function radialTexture(soft = true) {
  const c = document.createElement('canvas');
  c.width = c.height = 128;
  const g = c.getContext('2d');
  const grad = g.createRadialGradient(64, 64, 0, 64, 64, 64);
  const stops = soft ? [[0, 1], [0.35, 0.45], [1, 0]] : [[0, 1], [0.12, 0.95], [0.3, 0.3], [0.6, 0.06], [1, 0]];
  for (const [at, a] of stops) grad.addColorStop(at, `rgba(255,255,255,${a})`);
  g.fillStyle = grad;
  g.fillRect(0, 0, 128, 128);
  const tex = new THREE.CanvasTexture(c);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

export function createGlow(radius) {
  const group = new THREE.Group();
  const map = radialTexture();
  for (let i = 0; i < GLOW.sprites; i++) {
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({
      map, color: new THREE.Color(...GLOW.color), transparent: true, opacity: 0.22,
      blending: THREE.AdditiveBlending, depthWrite: false,
    }));
    sprite.scale.setScalar(radius * (2.6 + i * 0.9));
    group.add(sprite);
  }
  return group;
}

const rand = (a, b) => a + Math.random() * (b - a);

// Embers: CPU-simulated points. Positions are in the viewer's Y-up space around the origin.
export function createEmbers(sphereRadius, max = 64) {
  // game units -> our units: in game the 6.3 sphere is drawn at MODEL_SCALE, so one game unit
  // next to it corresponds to `unit` units around our full-size sphere
  const unit = sphereRadius / (6.3 * MODEL_SCALE);
  const pos = new Float32Array(max * 3), col = new Float32Array(max * 4), size = new Float32Array(max);
  const vel = new Float32Array(max * 3), age = new Float32Array(max).fill(1e9), life = new Float32Array(max).fill(1);
  const baseSize = new Float32Array(max), baseAlpha = new Float32Array(max);
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
  geo.setAttribute('color', new THREE.BufferAttribute(col, 4));
  geo.setAttribute('size', new THREE.BufferAttribute(size, 1));
  const mat = new THREE.ShaderMaterial({
    transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
    uniforms: { uMap: { value: radialTexture(false) }, uScale: { value: 1 } },
    vertexShader: /* glsl */ `
      attribute float size; attribute vec4 color; varying vec4 vColor; uniform float uScale;
      void main() {
        vColor = color;
        vec4 mv = modelViewMatrix * vec4(position, 1.0);
        gl_PointSize = size * uScale / -mv.z;
        gl_Position = projectionMatrix * mv;
      }`,
    fragmentShader: /* glsl */ `
      uniform sampler2D uMap; varying vec4 vColor;
      void main() { gl_FragColor = vColor * texture2D(uMap, gl_PointCoord); }`,
  });
  const points = new THREE.Points(geo, mat);
  points.frustumCulled = false;
  let carry = 0, next = 0;

  function spawn(i) {
    const d = new THREE.Vector3().randomDirection().multiplyScalar(rand(...EMBERS.spawnRadius) * unit);
    pos.set([d.x, d.y, d.z], i * 3);
    vel.set([0, 0, 0], i * 3);
    age[i] = 0;
    life[i] = rand(...EMBERS.lifetime);
    baseSize[i] = rand(...EMBERS.size) * unit;
    baseAlpha[i] = rand(...EMBERS.alpha);
    const t = Math.random(), a = EMBERS.colorA, b = EMBERS.colorB;
    col.set([a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t, 0], i * 4);
  }

  points.userData.update = (dt, viewportHeight, fov) => {
    mat.uniforms.uScale.value = viewportHeight / (2 * Math.tan((fov * Math.PI) / 360)) * 2;
    carry += dt * EMBERS.ratePerSecond;
    while (carry >= 1) { spawn(next); next = (next + 1) % max; carry -= 1; }
    const g = EMBERS.gravityZ * unit, drag = Math.pow(1 - EMBERS.drag, dt * 60);
    for (let i = 0; i < max; i++) {
      age[i] += dt;
      const t = age[i] / life[i];
      if (t >= 1) { size[i] = 0; col[i * 4 + 3] = 0; continue; }
      vel[i * 3 + 1] = (vel[i * 3 + 1] + g * dt) * drag; // game +Z = our +Y
      pos[i * 3 + 1] += vel[i * 3 + 1] * dt;
      size[i] = baseSize[i] * (1 - t);                    // C_OP_RadiusDecay / InterpolateRadius
      col[i * 4 + 3] = baseAlpha[i] * (1 - t) * Math.min(1, t * 8); // fade in, fade out
    }
    geo.attributes.position.needsUpdate = geo.attributes.color.needsUpdate = geo.attributes.size.needsUpdate = true;
  };
  return points;
}

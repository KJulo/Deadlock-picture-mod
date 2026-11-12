// Approximation of Deadlock's NPR "pbr" shader for the soul container.
//
// What is real (from the material): self-illumination colour, its -0.2*sin(3t) pulse, the Fresnel
// exponent, the transmissive (rim) colour, outline on, roughness. What is approximated: the exact
// NPR lighting ramp and the scene lights (the game's are per-map); those are uniforms so they can be
// calibrated against in-game screenshots.
import * as THREE from 'three';
import { MATERIAL } from './gameParams.js';

const v3 = (a) => new THREE.Vector3(a[0], a[1], a[2]);

export function createSoulMaterial(map) {
  return new THREE.ShaderMaterial({
    uniforms: {
      uMap: { value: map },
      uTime: { value: 0 },
      uPulse: { value: 1 }, // 0 disables the self-illum pulse
      uLightDir: { value: new THREE.Vector3(0.45, 0.8, 0.4).normalize() },
      uLightColor: { value: new THREE.Color(1.0, 0.96, 0.9).multiplyScalar(1.15) },
      uSkyColor: { value: new THREE.Color(0.42, 0.47, 0.52) },
      uGroundColor: { value: new THREE.Color(0.16, 0.15, 0.14) },
      uToon: { value: 0.65 },     // 0 = smooth half-lambert, 1 = hard two-tone
      uRoughness: { value: MATERIAL.roughness },
      uSelfIllumTint: { value: v3(MATERIAL.selfIllumTint) },
      uSelfIllumAmp: { value: MATERIAL.selfIllumAmplitude },
      uSelfIllumFreq: { value: MATERIAL.selfIllumFrequency },
      uFresnelExp: { value: MATERIAL.selfIllumFresnelExponent },
      uFresnelTint: { value: v3(MATERIAL.selfIllumFresnelTint) },
      uTransmissive: { value: v3(MATERIAL.transmissiveColor) },
      uTransmissiveStrength: { value: 0.55 },
    },
    vertexShader: /* glsl */ `
      varying vec2 vUv;
      varying vec3 vNormal;
      varying vec3 vWorldPos;
      void main() {
        vUv = uv;
        vec4 world = modelMatrix * vec4(position, 1.0);
        vWorldPos = world.xyz;
        vNormal = normalize(mat3(modelMatrix) * normal);
        gl_Position = projectionMatrix * viewMatrix * world;
      }`,
    fragmentShader: /* glsl */ `
      uniform sampler2D uMap;
      uniform float uTime, uPulse, uToon, uRoughness, uSelfIllumAmp, uSelfIllumFreq, uFresnelExp;
      uniform float uTransmissiveStrength;
      uniform vec3 uLightDir, uLightColor, uSkyColor, uGroundColor;
      uniform vec3 uSelfIllumTint, uFresnelTint, uTransmissive;
      varying vec2 vUv;
      varying vec3 vNormal;
      varying vec3 vWorldPos;
      void main() {
        vec3 albedo = texture2D(uMap, vUv).rgb;          // sRGB texture -> linear by the GPU
        vec3 N = normalize(vNormal);
        vec3 V = normalize(cameraPosition - vWorldPos);
        vec3 L = normalize(uLightDir);
        float NdotL = dot(N, L);
        float NdotV = clamp(dot(N, V), 0.0, 1.0);

        // NPR diffuse: wrapped (half-lambert) light pushed towards two soft tones
        float wrap = NdotL * 0.5 + 0.5;
        float toon = smoothstep(0.42, 0.58, wrap);
        float diffuse = mix(wrap * wrap, mix(0.35, 1.0, toon), uToon);
        vec3 ambient = mix(uGroundColor, uSkyColor, N.y * 0.5 + 0.5);
        vec3 color = albedo * (ambient + uLightColor * diffuse);

        // broad, weak highlight (roughness 0.69)
        vec3 H = normalize(L + V);
        float gloss = mix(48.0, 6.0, uRoughness);
        color += uLightColor * pow(max(dot(N, H), 0.0), gloss) * (1.0 - uRoughness) * 0.35 * step(0.0, NdotL);

        // self-illumination: tint * (-0.2 sin 3t), stronger at grazing angles (Fresnel mask ^5)
        float fresnel = pow(1.0 - NdotV, uFresnelExp);
        float scale = -uSelfIllumAmp * sin(uSelfIllumFreq * uTime) * uPulse;
        color += uSelfIllumTint * scale * (1.0 + 4.0 * fresnel * uFresnelTint);

        // NPR transmission: light leaking through the silhouette, strongest when back-lit
        float back = clamp(dot(-L, V) * 0.5 + 0.5, 0.0, 1.0);
        float rim = pow(1.0 - NdotV, 2.5);
        color += uTransmissive * rim * (0.35 + 0.65 * back) * uTransmissiveStrength;

        gl_FragColor = vec4(max(color, 0.0), 1.0);
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
      }`,
  });
}

// Inverted-hull outline: the mesh drawn again, slightly inflated, back faces only, in a dark tone.
export function createOutlineMaterial(thickness = MATERIAL.outlineThickness) {
  return new THREE.ShaderMaterial({
    side: THREE.BackSide,
    uniforms: {
      uThickness: { value: thickness },
      uColor: { value: new THREE.Color(0.015, 0.035, 0.04) },
    },
    vertexShader: /* glsl */ `
      uniform float uThickness;
      void main() {
        vec3 p = position + normalize(normal) * uThickness;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
      }`,
    fragmentShader: /* glsl */ `
      uniform vec3 uColor;
      void main() {
        gl_FragColor = vec4(uColor, 1.0);
        #include <colorspace_fragment>
      }`,
  });
}

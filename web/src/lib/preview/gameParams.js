// Values taken from the game files (decompiled with Source2Viewer), so the preview is driven by
// the real material instead of guesses. Colours are linear RGB 0..1 as stored in the material.
//
// models/props_gameplay/soul_container/materials/soul_container.vmat
//   shader pbr.vfx, F_USE_NPR_LIGHTING 1, F_SELF_ILLUM 1, F_OVERRIDE_NPR_OUTLINE 1
// particles/generic/holding_gold_neutral_model{,_glow,_embers}.vpcf
export const MATERIAL = {
  outlineThickness: 0.05,                       // g_flOverrideNprOutlineThickness
  selfIllumTint: [0.094118, 0.215686, 0.231373], // g_vSelfIllumTint1
  selfIllumAmplitude: 0.2,                       // DynamicParams: g_flSelfIllumScale1 = -.2*sin(3*time())
  selfIllumFrequency: 3.0,
  selfIllumFresnelExponent: 5.0,                 // g_flSelfIllumFresnelMaskExponent
  selfIllumFresnelTint: [1, 1, 1],               // g_vSelfIllumFresnelMaskTint1
  transmissiveColor: [0.070588, 0.682353, 0.92549], // TextureNprTramsissiveColor1 (sic)
  roughness: 0.69,                               // our NormalRoughness texture (original average)
};

export const GLOW = {
  color: [41 / 255, 255 / 255, 201 / 255], // m_ConstantColor of the glow child
  sprites: 2,
};

export const EMBERS = {
  ratePerSecond: 20,                  // C_OP_ContinuousEmitter
  spawnRadius: [3, 10],               // C_INIT_CreateWithinSphereTransform, game units
  lifetime: [1, 2],                   // seconds
  size: [0.5, 1.0],                   // radius, game units
  gravityZ: 70,                       // C_OP_BasicMovement, units/s², upwards
  drag: 0.05,
  colorA: [14 / 255, 250 / 255, 222 / 255], // C_INIT_RandomColor
  colorB: [29 / 255, 255 / 255, 191 / 255],
  alpha: [0.784314, 1.0],
};

// In game the model particle starts at scale 0.5 (C_INIT_InitFloat), so the 6.3-unit sphere is
// ~3.15 units across in radius; ember distances above are relative to that.
export const MODEL_SCALE = 0.5;

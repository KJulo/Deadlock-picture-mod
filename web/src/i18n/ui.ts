// Every user-facing string on the site, per language. `es` is the source of truth: `en` must
// have exactly the same keys (TypeScript enforces it). Strings used with set:html may contain
// inline markup; everything else is plain text.

export const languages = { es: 'Español', en: 'English' } as const;
export type Lang = keyof typeof languages;
export const defaultLang: Lang = 'es';

const es = {
  meta: {
    title: '{name} — tu foto en el contenedor de almas',
    tagline: 'Encierra a quien quieras en el contenedor de almas.',
    description:
      'Convierte tu foto en el contenedor de almas de Deadlock: vista previa 3D fiel al juego y un .vpk listo para Deadlock Mod Manager.',
  },
  header: {
    home: '{name}, inicio',
    sections: 'Secciones',
    workshop: 'Taller',
    install: 'Instalar',
    faq: 'Preguntas',
    language: 'Idioma',
  },
  workshop: {
    title: 'Taller',
    canvas: 'Vista previa 3D del contenedor de almas. Arrastra para girarla.',
    viewerOptions: 'Opciones de la vista previa',
    spin: 'Girar',
    effects: 'Brillo',
    pulse: 'Pulso',
    outline: 'Contorno',
    lod: 'Detalle',
    lods: ['Cerca', 'Media', 'Lejos', 'Mínimo'],
    photo: 'La foto',
    dropHtml: '<strong>Elige una foto</strong> o arrástrala aquí',
    formats: 'JPEG · PNG · WebP · hasta',
    templatesLabel: 'o elige una',
    templates: {
      cat: 'Gato', eye: 'Ojo', skull: 'Calavera', heart: 'Corazón', moon: 'Luna', flame: 'Llama',
      eightBall: 'Bola ocho', duck: 'Pato', crystal: 'Cristal',
    },
    templateError: 'No se pudo preparar la plantilla.',
    thumbAlt: 'Tu foto. Arrastra el círculo para elegir qué parte queda en el centro.',
    hint: 'Arrastra el círculo.',
    framing: 'Encuadre',
    shape: 'La forma',
    shapesLegend: 'Modelo 3D',
    shapes: { sphere: 'Esfera', cube: 'Dado', gem: 'Gema', coin: 'Moneda', egg: 'Huevo', medallions: 'Medallones' },
    layoutLegend: 'Sellos',
    layouts: {
      cube6: { name: 'Seis sellos', note: 'La foto seis veces: se ve entera caiga como caiga.' },
      twice: { name: 'Dos caras', note: 'Dos fotos grandes, una por lado.' },
      wrap: { name: 'Envoltura', note: 'Una sola foto envolviendo toda la esfera.' },
    },
    stickerSize: 'Tamaño del sello',
    ringWidth: 'Grosor del aro',
    colors: 'Los colores',
    background: 'Fondo',
    ring: 'Aro',
    otherColor: 'Otro color',
    otherBackground: 'Otro color de fondo',
    swatches: {
      soul: 'Alma', abyss: 'Abismo', brass: 'Latón', blood: 'Sangre', bone: 'Hueso', onyx: 'Ónix',
    },
    forgeTitle: 'Forjar',
    forgeButton: 'Forjar el .vpk',
  },
  // Strings the workshop script shows at runtime. {placeholders} are filled in by the script.
  status: {
    needPhoto: 'Primero, una foto.',
    offline: 'El taller está cerrado por ahora (no responde el servidor). Vuelve en un rato.',
    noServer: 'Sin conexión con el servidor.',
    rendering: 'Grabando los sellos…',
    ready: 'Listo. Cuando te guste, fórjalo.',
    tooBig: 'La foto pesa {size} MB; el máximo es {max} MB.',
    forging: 'Forjando…',
    done: 'Hecho. Instálalo con Deadlock Mod Manager.',
  },
  // Backend error codes (backend/internal/api) → message. Unknown codes fall back to the server's text.
  errors: {
    bad_photo: 'No se pudo leer la foto; usa JPEG, PNG, GIF o WebP.',
    photo_too_large: 'La foto es demasiado grande.',
    busy: 'El servidor está ocupado; inténtalo de nuevo en unos segundos.',
    bad_params: 'Parámetros no válidos.',
    bad_shape: 'Modelo 3D desconocido.',
    bad_request: 'No se pudo leer el envío.',
    missing_photo: 'Falta la foto.',
    internal: 'Error interno del servidor.',
  },
  install: {
    title: 'Instalar',
    steps: [
      'En <strong>Deadlock Mod Manager</strong>, agrega el <code>.vpk</code> como mod local.',
      'Actívalo y apaga otros mods del contenedor.',
      'Para verlo al tiro, en la consola de una partida privada:',
    ],
  },
  faq: {
    title: 'Preguntas',
    items: [
      ['¿Lo ven los demás?', 'No. Solo cambia tu instalación.'],
      ['¿Guardan mi foto?', 'No. Se procesa en memoria y se descarta.'],
      ['¿Me pueden banear?', 'Es un mod visual local, como los de Deadlock Mod Manager. Igual, no es oficial.'],
    ] as [string, string][],
  },
  footer: {
    disclaimer: 'Hecho por fans. Sin relación con Valve.',
    code: 'Código en GitHub',
  },
};

export type Dict = typeof es;

const en: Dict = {
  meta: {
    title: '{name} — your photo on the soul container',
    tagline: 'Lock anyone you like inside the soul container.',
    description:
      "Turn your photo into Deadlock's soul container: a game-faithful 3D preview and a .vpk ready for Deadlock Mod Manager.",
  },
  header: {
    home: '{name}, home',
    sections: 'Sections',
    workshop: 'Workshop',
    install: 'Install',
    faq: 'FAQ',
    language: 'Language',
  },
  workshop: {
    title: 'Workshop',
    canvas: '3D preview of the soul container. Drag to rotate it.',
    viewerOptions: 'Preview options',
    spin: 'Spin',
    effects: 'Glow',
    pulse: 'Pulse',
    outline: 'Outline',
    lod: 'Detail',
    lods: ['Near', 'Medium', 'Far', 'Lowest'],
    photo: 'The photo',
    dropHtml: '<strong>Choose a photo</strong> or drop it here',
    formats: 'JPEG · PNG · WebP · up to',
    templatesLabel: 'or pick one',
    templates: {
      cat: 'Cat', eye: 'Eye', skull: 'Skull', heart: 'Heart', moon: 'Moon', flame: 'Flame',
      eightBall: 'Eight ball', duck: 'Duck', crystal: 'Crystal',
    },
    templateError: 'The template could not be prepared.',
    thumbAlt: 'Your photo. Drag the circle to choose which part ends up in the center.',
    hint: 'Drag the circle.',
    framing: 'Framing',
    shape: 'The shape',
    shapesLegend: '3D model',
    shapes: { sphere: 'Sphere', cube: 'Die', gem: 'Gem', coin: 'Coin', egg: 'Egg', medallions: 'Medallions' },
    layoutLegend: 'Seals',
    layouts: {
      cube6: { name: 'Six seals', note: 'The photo six times: it shows in full however it lands.' },
      twice: { name: 'Two sides', note: 'Two big photos, one per side.' },
      wrap: { name: 'Wrap', note: 'A single photo wrapped around the whole sphere.' },
    },
    stickerSize: 'Seal size',
    ringWidth: 'Ring width',
    colors: 'The colors',
    background: 'Background',
    ring: 'Ring',
    otherColor: 'Other color',
    otherBackground: 'Other background color',
    swatches: {
      soul: 'Soul', abyss: 'Abyss', brass: 'Brass', blood: 'Blood', bone: 'Bone', onyx: 'Onyx',
    },
    forgeTitle: 'Forge',
    forgeButton: 'Forge the .vpk',
  },
  status: {
    needPhoto: 'First, a photo.',
    offline: 'The workshop is closed for now (the server is not responding). Come back in a while.',
    noServer: 'No connection to the server.',
    rendering: 'Engraving the seals…',
    ready: 'Ready. Forge it when you like it.',
    tooBig: 'The photo is {size} MB; the maximum is {max} MB.',
    forging: 'Forging…',
    done: 'Done. Install it with Deadlock Mod Manager.',
  },
  errors: {
    bad_photo: 'The photo could not be read; use JPEG, PNG, GIF or WebP.',
    photo_too_large: 'The photo is too large.',
    busy: 'The server is busy; try again in a few seconds.',
    bad_params: 'Invalid parameters.',
    bad_shape: 'Unknown 3D model.',
    bad_request: 'The upload could not be read.',
    missing_photo: 'The photo is missing.',
    internal: 'Internal server error.',
  },
  install: {
    title: 'Install',
    steps: [
      'In <strong>Deadlock Mod Manager</strong>, add the <code>.vpk</code> as a local mod.',
      'Enable it and turn off other container mods.',
      'To see it right away, in a private match console:',
    ],
  },
  faq: {
    title: 'FAQ',
    items: [
      ['Can others see it?', 'No. It only changes your install.'],
      ['Do you keep my photo?', 'No. It is processed in memory and discarded.'],
      ['Can I get banned?', 'It is a local visual mod, like the ones in Deadlock Mod Manager. Still, it is not official.'],
    ],
  },
  footer: {
    disclaimer: 'Made by fans. Not affiliated with Valve.',
    code: 'Code on GitHub',
  },
};

export const ui: Record<Lang, Dict> = { es, en };

/** The dictionary for the page being rendered (from Astro.currentLocale). */
export function useDict(locale: string | undefined): Dict {
  return ui[locale && locale in ui ? (locale as Lang) : defaultLang];
}

/** Fills {placeholders}: fmt('{a} y {b}', { a: 1, b: 2 }). */
export function fmt(s: string, vars: Record<string, string | number>): string {
  return s.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m));
}

/** URL of the home page in a given language ('/' for the default, '/en/' otherwise). */
export function homeUrl(lang: Lang): string {
  return lang === defaultLang ? '/' : `/${lang}/`;
}

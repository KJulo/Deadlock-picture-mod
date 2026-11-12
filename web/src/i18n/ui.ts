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
  hero: {
    eyebrow: 'Un mod para Deadlock, hecho por fans',
    lede:
      'Cuando caes con almas sin asegurar, queda una esfera que el enemigo puede reclamar. Aquí puedes ponerle tu foto: la de tu gato, la de tu equipo o la de tu peor enemigo. La previsualizas en 3D tal como la verás en partida y te llevas un <code>.vpk</code> listo para instalar.',
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
    note: 'Misma malla y misma textura que recibe el juego. La luz es una aproximación: en partida depende del mapa.',
    photo: 'La foto',
    dropHtml: '<strong>Elige una foto</strong> o arrástrala aquí',
    formats: 'JPEG · PNG · WebP · hasta',
    sample: 'o prueba con el gato de ejemplo',
    thumbAlt: 'Tu foto. Arrastra el círculo para elegir qué parte queda en el centro.',
    hint: 'Arrastra el círculo: eso es lo que queda en cada sello.',
    framing: 'Encuadre',
    shape: 'La forma',
    layoutLegend: 'Distribución de la foto',
    layouts: {
      cube6: { name: 'Seis sellos', note: 'La foto seis veces: se ve entera caiga como caiga.' },
      twice: { name: 'Anverso y reverso', note: 'Dos fotos grandes, una por lado.' },
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
    done: 'Hecho. Ahora instálalo en Deadlock Mod Manager (abajo te explico cómo).',
  },
  // Backend error codes (backend/internal/api) → message. Unknown codes fall back to the server's text.
  errors: {
    bad_photo: 'No se pudo leer la foto; usa JPEG, PNG, GIF o WebP.',
    photo_too_large: 'La foto es demasiado grande.',
    busy: 'El servidor está ocupado; inténtalo de nuevo en unos segundos.',
    bad_params: 'Parámetros no válidos.',
    bad_request: 'No se pudo leer el envío.',
    missing_photo: 'Falta la foto.',
    internal: 'Error interno del servidor.',
  },
  install: {
    title: 'Instalarlo en Deadlock',
    steps: [
      'Abre <strong>Deadlock Mod Manager</strong> y elige <em>agregar un mod local</em>. Selecciona el <code>.vpk</code> que forjaste.',
      'Actívalo. Si tienes otro mod que cambie el contenedor de almas, desactívalo: con dos activos solo se verá uno de ellos.',
      'Entra al juego. Para verlo sin esperar a una partida, abre la consola en el modo sandbox o en una partida privada y escribe:',
    ],
    aside:
      'El primero activa los trucos, el segundo te da almas sin asegurar y el tercero te elimina para que la esfera aparezca.',
  },
  faq: {
    title: 'Preguntas',
    items: [
      ['¿Lo ven los demás jugadores?', 'No. El mod cambia archivos de tu instalación, así que solo tú ves tu foto. Para el resto sigue siendo el contenedor de siempre.'],
      ['¿Guardan mi foto?', 'No. La foto se procesa en memoria para generar la textura y el archivo, y se descarta en cuanto termina. Nunca se escribe en disco ni queda en los registros.'],
      ['¿Me pueden banear por esto?', 'Es un mod visual local, del mismo tipo que los que distribuye Deadlock Mod Manager. Aun así, los mods no son oficiales: úsalo bajo tu responsabilidad.'],
      ['¿Y si actualizan el juego?', 'Si Valve cambia el modelo del contenedor, el servidor lo detecta y deja de generar archivos hasta que se adapte. Un mod viejo simplemente deja de aplicarse.'],
      ['¿Por qué la foto sale seis veces?', 'La esfera cae y rueda en cualquier posición. Con seis sellos siempre hay una foto completa mirando hacia ti. Si prefieres, elige «Anverso y reverso» o «Envoltura».'],
    ] as [string, string][],
  },
  footer: {
    disclaimer:
      'Proyecto hecho por fans. No está afiliado ni respaldado por Valve Corporation; Deadlock es una marca de Valve. El mod solo cambia lo que ves tú: los demás jugadores siguen viendo el contenedor original.',
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
  hero: {
    eyebrow: 'A Deadlock mod, made by fans',
    lede:
      'When you die carrying unsecured souls, you drop an orb the enemy can claim. Here you can put your photo on it: your cat, your team or your worst enemy. Preview it in 3D exactly as it will look in a match and take home a <code>.vpk</code> ready to install.',
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
    note: 'Same mesh and same texture the game gets. The lighting is an approximation: in a match it depends on the map.',
    photo: 'The photo',
    dropHtml: '<strong>Choose a photo</strong> or drop it here',
    formats: 'JPEG · PNG · WebP · up to',
    sample: 'or try the sample cat',
    thumbAlt: 'Your photo. Drag the circle to choose which part ends up in the center.',
    hint: 'Drag the circle: that is what goes on each seal.',
    framing: 'Framing',
    shape: 'The shape',
    layoutLegend: 'Photo layout',
    layouts: {
      cube6: { name: 'Six seals', note: 'The photo six times: it shows in full however it lands.' },
      twice: { name: 'Front and back', note: 'Two big photos, one per side.' },
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
    done: 'Done. Now install it with Deadlock Mod Manager (see how below).',
  },
  errors: {
    bad_photo: 'The photo could not be read; use JPEG, PNG, GIF or WebP.',
    photo_too_large: 'The photo is too large.',
    busy: 'The server is busy; try again in a few seconds.',
    bad_params: 'Invalid parameters.',
    bad_request: 'The upload could not be read.',
    missing_photo: 'The photo is missing.',
    internal: 'Internal server error.',
  },
  install: {
    title: 'Installing it in Deadlock',
    steps: [
      'Open <strong>Deadlock Mod Manager</strong> and choose <em>add a local mod</em>. Pick the <code>.vpk</code> you forged.',
      'Enable it. If you have another mod that changes the soul container, disable it: with two enabled only one of them will show.',
      'Launch the game. To see it without waiting for a match, open the console in sandbox mode or in a private match and type:',
    ],
    aside:
      'The first enables cheats, the second gives you unsecured souls and the third kills you so the orb appears.',
  },
  faq: {
    title: 'FAQ',
    items: [
      ['Can other players see it?', 'No. The mod changes files in your own install, so only you see your photo. For everyone else it is still the usual container.'],
      ['Do you keep my photo?', 'No. The photo is processed in memory to make the texture and the file, and discarded as soon as that is done. It is never written to disk or logged.'],
      ['Can I get banned for this?', 'It is a local visual mod, the same kind Deadlock Mod Manager distributes. Still, mods are not official: use it at your own risk.'],
      ['What if the game updates?', 'If Valve changes the container model, the server detects it and stops generating files until it is adapted. An old mod simply stops applying.'],
      ['Why does the photo show up six times?', 'The orb falls and rolls in any position. With six seals there is always a full photo facing you. If you prefer, choose “Front and back” or “Wrap”.'],
    ],
  },
  footer: {
    disclaimer:
      'Fan-made project. Not affiliated with or endorsed by Valve Corporation; Deadlock is a trademark of Valve. The mod only changes what you see: other players still see the original container.',
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

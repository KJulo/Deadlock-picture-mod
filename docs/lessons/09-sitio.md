# Etapa 9: el sitio en Astro

**Código:** `web/` · **Correr:** `make web` (API + sitio en http://localhost:4321) · **Compilar:** `make web-build` → `web/dist/`

## Por qué Astro
El sitio es estático: una sola página que habla con la API. Astro genera HTML puro y solo manda JavaScript donde hace falta, en este caso el taller y el visor 3D. El resultado se sube a Cloudflare Pages como un montón de archivos, sin servidor.

Versiones fijadas con fecha (no "la última"): **Astro 7.4.3** y **three 0.186.1**. Las tipografías vienen de `@fontsource`: se sirven desde nuestro sitio y no desde Google Fonts, sin llamadas a terceros.

## Estructura
```
web/
  astro.config.mjs        sitio estático, puerto 4321 (el origen que la API acepta por defecto)
  .env.example            PUBLIC_API_BASE: la URL de la API (en producción, la de Cloud Run)
  public/                 favicon, foto y textura de ejemplo (la textura ya generada: 0 llamadas a la API al cargar)
  src/site.js             nombre, lema y enlaces del sitio (el nombre "Relicario" es provisional)
  src/styles/global.css   paleta, tipografías y grano de fondo
  src/layouts/Base.astro  <head>, fuentes y metadatos
  src/components/
    Plate.astro           panel de pergamino con doble filete de latón y esquinas déco
    Divider.astro         separador déco (líneas escalonadas y un rombo)
    SiteHeader / Hero / Workshop / Install / Faq / SiteFooter
  src/lib/preview/        el visor 3D (etapa 8) y el cliente de la API
  src/pages/index.astro   la página: arma los componentes
```

## Diseño: "Nueva York ocultista de los años 20"
- **Paleta:** noche turquesa de fondo, pergamino para los paneles, latón en los filetes y un único acento brillante, `#29ffc9`, que es **el color del brillo del contenedor en el juego**.
- **Tipografías:**
  - *Limelight*: cartel art déco, solo para el nombre.
  - *Cormorant Garamond*: texto con aire editorial antiguo.
  - *Josefin Sans*: versalitas espaciadas para las etiquetas, geométrica de los años 20.
- **Formas déco:**
  - el visor dentro de un **arco**
  - esquinas escalonadas y números romanos en hexágonos alargados
  - el botón "Forjar" con puntas de flecha
  - las muestras de color como rombos
- **Sin recursos de Valve:** ni logos, ni arte, ni tipografías del juego, y con aviso de proyecto fan en el pie.
- **El texto** está escrito para el tema ("sellos", "forjar", "relicario"), sin emojis ni frases de plantilla.

## El taller (`Workshop.astro`)
1. **La foto:** botón o arrastrar y soltar, o el gato de ejemplo. Sobre la miniatura se dibuja **el círculo que quedará en cada sello**; se arrastra con mouse o con el dedo (Pointer Events) y eso fija `center`. El control "Encuadre" cambia `radius`.
2. **La forma:** "Seis sellos", "Anverso y reverso" o "Envoltura", más el tamaño del sello y el grosor del aro.
3. **Los colores:** fondo y aro, con presets y un color libre.
4. **Forjar:** `POST /api/build` y la descarga del `.vpk`.

Cada cambio pide una textura nueva 250 ms después del último movimiento (*debounce*). Si llega otro cambio antes, el pedido anterior se cancela con `AbortController`, para no saturar el servidor.

**Detalles:**
- three.js se carga de forma diferida (`import()`), así el texto aparece antes.
- Si el usuario pidió menos movimiento en su sistema (`prefers-reduced-motion`), la esfera no gira sola.
- Se revisa el tamaño de la foto antes de subirla.
- Si la API no responde, se muestra un aviso sobre el visor.

## Cómo lo verificamos
- Con Chrome sin pantalla (puppeteer) en escritorio (1280×900) y celular (390×844): carga, el gato de ejemplo genera la textura con la API y el estado queda en "Listo".
- **Correcciones que salieron de mirar esas capturas:**
  - La esfera quedaba cortada en el arco vertical. Ahora la cámara calcula su distancia según el lado más angosto de la vista.
  - El encabezado era muy alto.
  - three.js avisaba que `Clock` está obsoleto; ahora se usa `Timer`.

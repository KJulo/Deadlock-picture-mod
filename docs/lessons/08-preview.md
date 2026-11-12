# Etapa 8: el preview fiel al juego

**Código:** `web/src/lib/preview/` (lo usa el sitio Astro, ver etapa 9) · **Probar:** `make web` y abrir http://localhost:4321

## Por qué el preview anterior "mentía"
El HTML de antes generaba su propia esfera en JavaScript con UV perfectas. El juego, en cambio, lee las UV del archivo **como half floats**. Cuando las guardábamos como enteros (SNORM), el navegador mostraba la foto bien y el juego la mostraba rota. Un preview que no usa los mismos datos que el juego no puede avisarte de ese tipo de errores.

## Qué usa ahora el visor
| Pieza | Fuente | ¿Igual al juego? |
|---|---|---|
| Geometría | `GET /api/mesh?lod=N`: los mismos vértices y triángulos que van en el `.vmdl_c` | ✅ idéntica, incluido el LOD |
| UV | Los bytes f16 del modelo, decodificados (`mesh.HalfToFloat`) | ✅ idénticas a lo que lee la GPU del juego |
| Textura | `POST /api/texture`: el mismo código Go que arma el mod | ✅ (el juego la ve en BC7: diferencia de ~49 dB, invisible) |
| Material | `soul_container.vmat` descompilado del juego | 🟡 valores reales, fórmula aproximada |
| Brillo y brasas | Los `.vpcf` de partículas del juego | 🟡 colores, tasas, vida y gravedad reales; sprite aproximado |
| Luz de la escena | Depende del mapa en el juego | ❌ aproximada (ajustable) |

## Lo que sacamos del juego (`gameParams.js`)
Con Source2Viewer descompilamos el material y las partículas:
- **Iluminación NPR** (estilo ilustración) más **contorno** de grosor 0,05.
- **Autoiluminación** color `(0.094, 0.216, 0.231)` con una fórmula dinámica: `return -.2*sin(3*time());`. Es decir, **pulsa** suavemente unas 0,5 veces por segundo. La replicamos tal cual.
- **Máscara Fresnel** de exponente 5: la autoiluminación pesa más en los bordes.
- **Color de transmisión** turquesa `(0.07, 0.68, 0.93)`: la luz que se "cuela" por la silueta.
- **Brasas:** 20 por segundo, nacen a 3–10 unidades, viven 1–2 s, suben con gravedad +70 y tienen colores turquesa entre `(14,250,222)` y `(29,255,191)`.
- **Halo:** 2 sprites de color `(41,255,201)`.

## Cómo está hecho (three.js)
- **`geometryFromMesh`:** convierte de Source (Z hacia arriba) a three.js (Y hacia arriba) con `(x, y, z) → (x, z, -y)`. Es una rotación, así que el orden de los triángulos, y con él qué lado es "afuera", se mantiene.
- **`createSoulMaterial`:** un `ShaderMaterial` con GLSL propio:
  - luz "envuelta" (half-lambert) empujada hacia dos tonos (`uToon`)
  - un brillo ancho y débil (rugosidad 0,69)
  - el pulso de autoiluminación con Fresnel
  - la transmisión en el borde, más fuerte a contraluz
- **Contorno "inverted hull":** se dibuja la malla otra vez, un poco inflada y solo por dentro (`BackSide`), en un tono oscuro. Es la técnica clásica de los juegos con estilo cartoon.
- **Brasas:** puntos simulados en la CPU con vida, gravedad, roce y desvanecimiento, dibujados con mezcla aditiva para que "brillen".

## Lo que falta: calibrar con una captura del juego
Las partes 🟡/❌ son uniforms ajustables. Desde la consola del navegador:
```js
viewer.material.uniforms.uToon.value = 0.8
viewer.material.uniforms.uTransmissiveStrength.value = 0.4
```
Con una captura de tu esfera en el juego, idealmente de cerca y con buena luz, se ajustan para que coincidan.

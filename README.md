# Deadlock Picture Mod — tu foto como Soul Container

Convierte el **soul container** de Deadlock (el objeto que queda cuando mueres con almas sin asegurar) en una **esfera con tu foto**, por ejemplo la de tu gato. Por defecto, la foto se repite en 6 calcomanías redondas, así siempre se ve completa sin importar cómo caiga o gire la esfera. El resultado es un `.vpk` que se instala con Deadlock Mod Manager.

![preview](docs/preview.png)

## Uso (Linux)

```bash
python3 -m venv .venv && .venv/bin/pip install -r reference/requirements.txt
.venv/bin/python reference/scripts/catmod_cli.py fetch                         # archivos base desde tu Deadlock
.venv/bin/python reference/scripts/catmod_cli.py build assets/cat.jpg --open   # VPK + preview.png + preview.html (3D, se abre en el navegador)
```

- **Revisar antes de probar en el juego:** `build/preview.html` es un visor 3D interactivo. Muestra bien el layout y el encuadre; la iluminación es aproximada y no reproduce el shader del juego.
- **Probar rápido en el juego, sin DMM:** agrega `--install` para copiar el VPK a `addons/pak09_dir.vpk` (`catmod_cli.py uninstall` lo quita). Hay que reiniciar Deadlock, y si DMM tiene otro mod del soul container activo, desactívalo, porque gana el de número menor.
- **Copia en Descargas:** `--downloads`. Para que DMM (Flatpak) pueda leer `~/Downloads`, dale permiso una sola vez: `flatpak override --user --filesystem=xdg-download:ro dev.stormix.deadlock-mod-manager`.
- **Calibración del material:** `catmod_cli.py calibrate assets/cat.jpg -o build/calibrate/pak01_dir.vpk` genera una esfera con franjas etiquetadas de alpha (A0…A255) y rugosidad (R0…R100). Con una sola prueba en el juego se ve qué combinación luce mejor.

Opciones de `build`:

| Opción | Qué hace |
|---|---|
| `--layout cube6\|twice\|wrap` | `cube6`: 6 calcomanías, legible desde cualquier ángulo (por defecto); `twice`: frente y atrás; `wrap`: una sola foto envolviendo toda la esfera |
| `--sticker-deg` / `--ring-deg` | tamaño angular de cada calcomanía (40° por defecto) y grosor del anillo (3°) |
| `--bg-color` / `--ring-color` | colores hex del fondo (turquesa "alma") y del anillo |
| `--color-alpha` / `--roughness` | material: 1.0 / 0.69 por defecto, igual que el original. Con `--color-alpha 0` la esfera se ve **negra** en el juego (probado) |
| `--center X Y` | punto de la foto (de 0 a 1) que queda al frente de la esfera |
| `--radius R` | cuánta foto envuelve el frente (fracción del lado corto; 0.5 por defecto, más grande = se ve más foto) |
| `--projection equidistant\|ortho` | `equidistant` reparte la foto de forma pareja (por defecto); `ortho` la deja idéntica vista de frente, pero muy estirada en los bordes |
| `--back mirror\|flip\|color` | solo para `wrap`: qué va en la parte de atrás |
| `--rotate GRADOS` | rota la foto antes de proyectarla |

Para probar en el juego, en sandbox o en una partida privada, abre la consola y ejecuta: `sv_cheats 1`, `citadel_give_gold 5000 1` (almas sin asegurar) y `kill`.

## Cómo funciona

No hacen falta herramientas de Valve: el mod se arma **parcheando los archivos compilados** del juego.

| Archivo | Cambio |
|---|---|
| `soul_container.vmdl_c` | Los vértices (meshopt códec v0) e índices de los 4 LOD se reemplazan por una esfera UV de radio 6.3. Se mantienen las mismas cantidades, así que los metadatos binarios (KV3), la física (esfera de radio 7) y el material no cambian. |
| `soul_container_color…vtex_c` | La foto proyectada en equirectangular, 2048², BC7 con 10 mips, reutilizando la cabecera original. El alpha de esta textura viene de `soul_container_metal.png` (el original vale 255). Con el shader NPR de Deadlock, alpha 0 hace que se vea negra; su efecto exacto se está calibrando con `calibrate`. |
| `soul_container_rough…vtex_c` | Normal plana y rugosidad uniforme (0.69 por defecto, igual que el original), para que no se imprima el relieve de la jaula. |
| `soul_container_ao…vtex_c` | Blanco (BC4), sin oclusión horneada. |

Detalles de formato descubiertos por ingeniería inversa (verificados contra Source 2 Viewer):
- **Vértice (24 B):** posición f32×3, UV, `CompressedTangentFrame` u32 y blend indices u8×4.
- **UV: el layout declara `R16G16_SNORM` (formato 37, semántica `LowPrecisionUv`), pero el juego la decodifica como `R16G16_FLOAT`.** Source 2 Viewer asume SNORM, así que no sirve para validar esto. Lo confirmamos en el juego con `catmod_cli.py uvtest`: una esfera mitad f16 (fondo naranja) y mitad snorm (turquesa); solo la naranja se ve bien.
- **`CompressedTangentFrame`:** `bit0` = signo del bitangente, `bits 1-11` = ángulo de la tangente en la base de Frisvad (×2047/2π), `bits 12-21` y `22-31` = normal octaédrica (×1023).
- **Contenedor de recursos:** offsets relativos y bloques alineados a 16, salvo `RERL`, que va alineado a 4. En `vtex_c`, los mips van después de los bloques, del más chico al más grande.

Estructura (monorepo):

```
reference/        implementación en Python: la fuente de verdad, probada en el juego
  catmod/         núcleo (build, photo, mesh, vmdl, vtex, resource, preview, templates)
  scripts/        CLI, make_golden.py, script para DMM
  tests/          pytest
backend/          backend en Go (en construcción, ver docs/lessons/)
web/              sitio Astro (pendiente)
testdata/         golden files: lo que genera el Python y contra lo que se compara el Go
templates/        archivos base del juego (gitignored, `make fetch`)
docs/             WEB_PLAN.md y lecciones
Makefile          make setup / fetch / golden / test
```

## Notas

- Solo cambia tu cliente: los demás jugadores no ven nada distinto.
- Si una actualización de Deadlock cambia el modelo, `templates.py` lo detecta por el sha256 y se niega a construir el mod. En ese caso hay que actualizar las cantidades de cada LOD y el hash.
- Los archivos de Valve van en `templates/`, que está en `.gitignore`.

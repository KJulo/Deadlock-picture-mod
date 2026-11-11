# Deadlock Picture Mod — tu foto como Soul Container

Convierte el **soul container** de Deadlock (el objeto que queda cuando mueres con almas sin asegurar) en una **esfera con tu foto**, por ejemplo la de tu gato. Por defecto, la foto se repite en 6 calcomanías redondas, así siempre se ve completa sin importar cómo caiga o gire la esfera. El resultado es un `.vpk` que se instala con Deadlock Mod Manager.

![preview](docs/preview.png)

## Uso (Linux)

```bash
python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
.venv/bin/python scripts/catmod_cli.py fetch           # extrae los archivos base desde tu instalación de Deadlock
.venv/bin/python scripts/catmod_cli.py build assets/cat.jpg --preview build/preview.png
./scripts/stage_for_dmm.sh                             # copia build/pak01_dir.vpk a donde DMM (Flatpak) puede leerlo
```

En DMM, agrega un mod local y elige `~/.var/app/dev.stormix.deadlock-mod-manager/import/catmod_sphere.vpk`. Como `.var` es una carpeta oculta, presiona Ctrl+H en el selector de archivos para verla.

Opciones de `build`:

| Opción | Qué hace |
|---|---|
| `--layout cube6\|twice\|wrap` | `cube6`: 6 calcomanías, legible desde cualquier ángulo (por defecto); `twice`: frente y atrás; `wrap`: una sola foto envolviendo toda la esfera |
| `--sticker-deg` / `--ring-deg` | tamaño angular de cada calcomanía (40° por defecto) y grosor del anillo (3°) |
| `--bg-color` / `--ring-color` | colores hex del fondo (turquesa "alma") y del anillo |
| `--metalness` / `--roughness` | material: 0 / 0.9 por defecto. **No subas el metal:** la foto se vuelve un reflejo cromado |
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
| `soul_container.vmdl_c` | Los vértices e índices de los 4 LOD (comprimidos con meshoptimizer) se reemplazan por una esfera UV de radio 6.3. Se mantienen las mismas cantidades, así que los metadatos binarios (KV3), la física (esfera de radio 7) y el material no cambian. |
| `soul_container_color…vtex_c` | La foto proyectada en equirectangular, 2048², BC7 con 10 mips, reutilizando la cabecera original. **El alpha de esta textura es el metalness**: el juego empaqueta ahí `soul_container_metal.png`, y la jaula original es 100% metálica. Lo dejamos en 0. |
| `soul_container_rough…vtex_c` | Normal plana y rugosidad uniforme (0.9 por defecto), para que no se imprima el relieve de la jaula original y la foto se vea mate. |
| `soul_container_ao…vtex_c` | Blanco (BC4), sin oclusión horneada. |

Detalles de formato descubiertos por ingeniería inversa (verificados contra Source 2 Viewer):
- **Vértice (24 B):** posición f32×3, UV `R16G16_SNORM`, `CompressedTangentFrame` u32 y blend indices u8×4.
- **`CompressedTangentFrame`:** `bit0` = signo del bitangente, `bits 1-11` = ángulo de la tangente en la base de Frisvad (×2047/2π), `bits 12-21` y `22-31` = normal octaédrica (×1023).
- **Contenedor de recursos:** offsets relativos y bloques alineados a 16, salvo `RERL`, que va alineado a 4. En `vtex_c`, los mips van después de los bloques, del más chico al más grande.

Estructura:

```
catmod/           núcleo reutilizable (lo importaría el backend web)
  build.py        build_mod(foto, PhotoParams) -> ModResult (.files, .texture, .vpk_bytes(), .preview_mesh())
  photo.py        proyección foto -> textura de la esfera
  mesh.py         esfera + empaquetado de normales/tangentes
  vmdl.py         parche del modelo
  vtex.py         reconstrucción de texturas BC7/BC4
  resource.py     contenedor de recursos de Source 2 (parse/serialize idénticos byte a byte)
  preview.py      renderer por software para previews
  templates.py    archivos base del juego (fijados por sha256; nunca se suben al repo)
scripts/          CLI, script para DMM y renderer de .glb
tests/            pytest
docs/WEB_PLAN.md  plan del sitio web
```

## Notas

- Solo cambia tu cliente: los demás jugadores no ven nada distinto.
- Si una actualización de Deadlock cambia el modelo, `templates.py` lo detecta por el sha256 y se niega a construir el mod. En ese caso hay que actualizar las cantidades de cada LOD y el hash.
- Los archivos de Valve van en `templates/`, que está en `.gitignore`.

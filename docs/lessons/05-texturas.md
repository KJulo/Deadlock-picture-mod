# Etapa 5: la foto, las texturas BC7 y el mod completo

**Código:** `backend/internal/photo/`, `bc7/`, `vtex/`, `build/`, `cmd/catmod/` · **Probar:** `make vpk PHOTO=assets/cat.jpg ARGS=-downloads`

## 1. La foto → la textura (`photo`)
- **`Load`** decodifica JPEG, PNG, GIF o WebP y aplica tres cosas:
  - **la orientación EXIF:** los celulares guardan la foto "acostada" y anotan cómo girarla; sin esto, las fotos verticales quedan de lado
  - **el contraste**
  - **la saturación:** copia exacta de las fórmulas de Pillow
- **`SphereTexture`** recorre cada píxel de la textura de 2048×2048, calcula a qué dirección de la esfera corresponde y decide qué va ahí:
  - la foto, si cae dentro de una calcomanía (con interpolación bilineal)
  - el color del anillo, si cae en el borde
  - el fondo turquesa, en cualquier otro caso

**Resultado:** idéntico byte a byte a Python en los 6 layouts de prueba. También coinciden las 8 orientaciones EXIF.

## 2. Comprimir en BC7 (`bc7`): cgo
Las GPUs no leen JPEG: usan formatos como **BC7**, que guarda cada cuadrado de 4×4 píxeles en 16 bytes (dos colores extremos más un índice por píxel). Comprimir bien en BC7 es difícil, así que usamos un encoder en C ya probado (`bc7enc`, de Rich Geldreich) y lo llamamos desde Go con **cgo**:

```go
/*
#include "glue.h"
*/
import "C"
...
C.glue_encode_rows(...)   // llamar a una función C como si fuera de Go
```

Detalles que vale la pena saber de cgo:
- **Cada llamada a C cuesta algo**, unas decenas de nanosegundos. Por eso `glue.c` codifica una franja completa de filas por llamada, no un bloque.
- **Paralelismo con goroutines:** la imagen se divide en franjas y cada una se codifica en su propia goroutine; `sync.WaitGroup` espera a que terminen todas. Así 2048×2048 tarda 0,15 s.
- **Memoria:** a C se le pueden pasar slices de bytes de Go mientras dure la llamada, pero no estructuras que contengan punteros de Go.
- **Compilar necesita gcc**, y el Docker de producción también lo va a necesitar.

**El bug del alpha 254:** el encoder dejaba el alpha en 254 en la mitad de los bloques. En el modo 6 de BC7, un mismo bit, el "p-bit", se comparte entre el color y el alpha, y el encoder lo elegía pensando solo en el color. Le aplicamos un parche de una línea (`catmod patch` en `bc7enc.c`, documentado en `VENDORED.md`) para que en bloques opacos use siempre el p-bit 1. Con eso el alpha queda exactamente en 255, como en el original.

## 3. Reconstruir las texturas (`vtex`)
Se conserva la cabecera del archivo original y se reemplazan solo los píxeles:
- **Color:** la foto más 9 mips. Cada mip es la mitad del anterior, calculado promediando cuadrados de 2×2, y se guardan del más chico al más grande.
- **Normal/rugosidad:** es un color constante, así que se codifica **un** bloque y se repite.
- **AO:** BC4 blanco, sin sombras horneadas. Sale idéntica a la de Python.

## 4. Todo junto (`build.Mod`) y el CLI
`build.Mod(plantillas, bytesDeLaFoto, params)` devuelve los 4 archivos y `Result.VPK()` arma el `.vpk`. Esto es lo que va a llamar el servidor HTTP en la etapa 6. Todo el proceso tarda **~0,9 s**, contra unos 3 s en Python.

```bash
make vpk PHOTO=assets/cat.jpg ARGS=-downloads     # copia en ~/Downloads/catmod_go.vpk
make vpk PHOTO=assets/cat.jpg ARGS=-install       # directo a addons/pak09 (reinicia Deadlock)
```

## Cómo sabemos que está bien
- **Proyección:** idéntica a Python, y el test detecta si el centro se mueve un 2%.
- **BC7:** sobre 40 dB en imágenes suaves; los colores constantes quedan a ±1 nivel; el resultado es el mismo con cualquier cantidad de goroutines.
- **Mod completo:** el modelo y la AO tienen el mismo sha256 que en Python, y la textura de color da 53,8 dB contra la imagen original.
- **Foto real del gato (JPEG con EXIF):** 49 dB contra la textura de Python.
- **Lo único que falta** es probarlo en el juego, porque el encoder BC7 cambió de etcpak a bc7enc. El formato es el mismo, así que debería verse igual.

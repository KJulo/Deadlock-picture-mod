# Lección 1: leer y escribir archivos binarios en Go

**Estado:** ✅ implementado en `backend/internal/resource/resource.go` (port de `reference/catmod/resource.py`). Este documento explica cómo funciona, para leerlo con el código al lado.

**Por qué empezamos aquí:** todo lo demás (modelo, texturas) está guardado dentro de este "contenedor". Además, es el ejercicio clásico para aprender Go: bytes, structs, slices y errores.

**Verificación:** `make test-go`. Los 5 tests pasan, incluido el roundtrip byte a byte de los archivos del juego.

## 1. El formato

Un archivo `*_c` de Source 2 es un índice seguido de los bloques:

```
┌───────────────────────── header (16 bytes) ─────────────────────────┐
│ u32 tamaño │ u16 hver │ u16 ver │ u32 offset tabla (=8) │ u32 count │
├──────────────────────── tabla (count × 12 B) ───────────────────────┤
│ "MVTX" │ u32 offset relativo │ u32 tamaño │   ← entrada 0           │
│ "MIDX" │ ...                                ← entrada 1             │
├──────────────────────────── bloques ────────────────────────────────┤
│ [padding hasta múltiplo de 16] payload MVTX [padding] payload MIDX… │
├──────────────────────────── trailer ────────────────────────────────┤
│ (en las texturas: los píxeles; no los cuenta el "tamaño")           │
└─────────────────────────────────────────────────────────────────────┘
```

Tres detalles que costaron descubrir y que tu código tiene que respetar:

1. **Los offsets son relativos a donde están escritos.** El offset de la entrada `i` se lee en `16 + i*12 + 4`, y el bloque empieza en *esa posición + el valor*.
2. **La alineación no siempre es 16.** El bloque `RERL` del modelo va alineado a 4. Por eso se *infiere* al leer (`Block.Align`) y se reproduce al escribir; si no, el archivo de salida no queda idéntico.
3. **El tamaño del header no incluye el trailer.**

## 2. Conceptos de Go que vas a usar

**Little-endian:** el entero `0x00000010` se guarda en disco como `10 00 00 00`, con el byte menos significativo primero.

```go
import "encoding/binary"
le := binary.LittleEndian
n := le.Uint32(data[8:12])   // leer
le.PutUint32(out[0:4], 123)  // escribir
```
Es el equivalente a `struct.unpack_from("<I", data, 8)` en Python.

**Slices:** `data[a:b]` **no copia**, es una "ventana" sobre el mismo arreglo, así que `Block.Data` apunta al buffer original sin gastar memoria. La contracara es que, si modificas `data`, también cambia el bloque. Si alguna vez necesitas una copia: `bytes.Clone(x)`.

**Panic vs. error:** en Python, `data[100:200]` sobre 50 bytes devuelve algo vacío sin quejarse. En Go, un índice fuera de rango hace **panic** y botaría el servidor. Como el backend recibe datos de afuera, la regla es **revisar `len(data)` antes de leer** y devolver `ErrTruncated`:

```go
if len(data) < 16 {
    return nil, ErrTruncated
}
```
Cuidado con el overflow: valida `count` antes de calcular `count*12`. Un archivo malicioso puede decir que tiene mil millones de bloques, y eso lo revisa `TestParseRejectsGarbage`.

**Construir la salida:** calcula primero dónde va cada bloque, reserva todo de una vez con `make([]byte, total)` y después rellena con `copy(out[pos:], payload)`. Al final, `append(out, r.Trailer...)`.

**Errores con contexto:** `fmt.Errorf("%w: bloque %d", ErrTruncated, i)` agrega información y conserva el error original, así que `errors.Is(err, ErrTruncated)` sigue funcionando.

## 3. Los tests

| Test | Qué comprueba |
|---|---|
| `TestSerializeSynthetic` | Un recurso armado a mano produce exactamente los bytes de header y tabla esperados |
| `TestParseSynthetic` | `Parse(Serialize(x))` devuelve `x`, y `BlockIndex` funciona |
| `TestParseRejectsGarbage` | Un input corrupto devuelve un error, nunca panic |
| `TestRoundtripGameFiles` | Los 4 archivos del juego salen idénticos byte a byte |
| `TestMatchesPythonReference` | Go ve los mismos bloques que el Python (`testdata/resource_layout.json`) |

Los dos últimos necesitan `templates/` (`make fetch`). En CI se saltan, porque los archivos de Valve nunca se suben.

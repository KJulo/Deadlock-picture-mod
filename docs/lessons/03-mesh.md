# Etapa 3: la esfera

**Código:** `backend/internal/mesh/mesh.go` · **Tests:** `make test-go`

## Qué hace
Genera la esfera que reemplaza a la jaula original, en el formato exacto que espera el juego. Cada **vértice** (punto de la malla) ocupa 24 bytes:

| Bytes | Qué es | Formato |
|---|---|---|
| 0-11 | Posición (x, y, z) | 3 floats de 32 bits |
| 12-15 | Coordenada de textura (u, v): qué punto de la foto se pega aquí | 2 **half floats** (16 bits) |
| 16-19 | Normal y tangente comprimidas: hacia dónde "mira" la superficie, para la luz | 1 entero de 32 bits |
| 20-23 | Hueso al que pertenece | siempre 0 |

Además están los **índices**: cada grupo de 3 números dice qué vértices forman un triángulo.

## Las partes interesantes

**La grilla.** Es una esfera tipo "globo terráqueo": `NU` cortes verticales (meridianos) y `NV` horizontales (paralelos). `SphereGrid` busca la esfera más detallada que cabe en el presupuesto de cada LOD. El modelo original tiene una cantidad fija de vértices y triángulos, y no podemos cambiarla sin reescribir los metadatos del modelo.

**El relleno.** Si sobran vértices, se repite el vértice 0. Si sobran índices, se rellenan con `0, 0, 0`, que forman triángulos de área cero: el juego los "dibuja", pero no se ven.

**Half floats (`Float16`).** Es el descubrimiento que hicimos probando en el juego: aunque el modelo dice que las UV son enteros, Deadlock las lee como floats de 16 bits. Go no tiene un tipo `float16`, así que la conversión está escrita a mano: separa exponente y mantisa, redondea al par más cercano y maneja los números muy chicos (subnormales).

**La normal comprimida (`PackTangentFrame`).** Hay que meter dos direcciones 3D en 32 bits:
- **La normal** se proyecta sobre un octaedro (un "diamante"), lo que deja 2 números de 10 bits cada uno.
- **La tangente** es perpendicular a la normal, así que basta guardar su **ángulo** (11 bits) dentro de una base que el juego reconstruye desde la normal (la base de Frisvad).
- **El bit restante** es el signo del bitangente.

**El orden de los triángulos (winding).** Si los 3 vértices van en el orden equivocado, el triángulo queda "mirando hacia adentro" y el juego lo esconde. `TestWindingOutward` lo comprueba.

## Ideas de Go que aparecen
- **Tipos propios con métodos:** `type Vec3 [3]float64` con `Add`, `Dot` y `Cross`. Es como una clase chiquita, pero es un arreglo de valor: se copia al pasarlo y no hay punteros.
- **Retornos con nombre** (`func FrisvadBasis(n Vec3) (b1, b2 Vec3)`) y retornos múltiples (`vb, idx, grid, err`).
- **Closures:** `quad := func(j, i int) ...` es una función definida dentro de otra.
- **`math.RoundToEven`:** redondea igual que numpy. Si se usara `math.Round`, algunos valores en la mitad exacta saldrían distintos al Python.

## Cómo sabemos que está bien
- **`TestMatchesPythonReference`:** los buffers de los 4 LOD son **idénticos byte a byte** a los del Python (`testdata/mesh_buffers.bin.gz`). Esa es la versión que ya se ve perfecta en el juego.
- **Tests de ida y vuelta:** las normales y tangentes comprimidas, al descomprimirlas, vuelven con menos de 0.25° y 0.5° de error.
- **Casos borde de `Float16`:** el redondeo en empates, los subnormales y el máximo.

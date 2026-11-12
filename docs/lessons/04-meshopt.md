# Etapa 4: compresión meshopt y el modelo completo

**Código:** `backend/internal/meshopt/`, `backend/internal/vmdl/`, `backend/internal/templates/` · **Tests:** `make test-go`

## Por qué comprimir
El juego no guarda los vértices "en crudo": usa **meshoptimizer**, una librería de compresión hecha para geometría 3D. Si le pasamos los bytes sin comprimir, el juego no los entiende. Por eso hay que escribir un compresor que produzca exactamente el formato que el juego sabe leer.

## Vértices (códec v0)
La idea es que los vértices vecinos se parecen. En vez de guardar cada valor, se guarda **cuánto cambió** respecto al vértice anterior:

1. **Por byte:** el byte 0 de todos los vértices forma una "columna", el byte 1 otra, y así. Cada columna se comprime por separado.
2. **Diferencias:** se reemplaza cada valor por la diferencia con el anterior. En una esfera suave las diferencias son números chicos.
3. **Zigzag:** convierte -1, +1, -2, +2… en 1, 2, 3, 4…, así los números negativos chicos también son chicos.
4. **Grupos de 16 bytes:** cada grupo se guarda con 0, 2, 4 u 8 bits por valor, el que ocupe menos. Los valores que no caben se marcan como "escapados" y se escriben enteros después del grupo.

## Índices (códec v1)
Los triángulos vecinos comparten aristas, así que el compresor recuerda las **últimas 16 aristas** y los **últimos 16 vértices**:
- Si un triángulo comparte una arista reciente, se describe con **1 byte**: "la arista #3 + el vértice siguiente".
- Si no, se usa un byte de código, a veces uno extra (el "codeaux"), y los índices que no se pueden adivinar se guardan como **diferencia en formato varint**, donde los números chicos ocupan 1 byte.

## La anécdota del desempate
El primer intento salió idéntico en 14 de 16 casos. En los otros 2 el largo coincidía, pero algunos bytes no. ¿La causa? Cuando un grupo ocupa **lo mismo** con 2 bits que con 4, hay que elegir uno. La versión antigua de meshoptimizer se quedaba con el primero; la nueva prefiere **el mismo que usó el grupo anterior**. Las dos versiones son válidas y el juego lee ambas, pero para quedar byte a byte iguales a la librería de Python había que usar la regla nueva. Lo encontramos bajando el código fuente original y comparando.

**Lección general:** cuando portas un algoritmo, compara contra la implementación de referencia con datos reales *y* con casos raros. Los casos raros (datos aleatorios, triángulos desordenados) fueron los que destaparon la diferencia.

## Juntando todo: `vmdl.BuildSphereModel`
1. `resource.Parse` abre el modelo original (etapa 1).
2. Por cada LOD: `mesh.BuildBuffers` genera la esfera (etapa 3), `meshopt` la comprime y se reemplazan los bloques `MVTX` y `MIDX`.
3. `res.Serialize()` vuelve a cerrar el archivo.

`templates.Load` lee el modelo original y revisa su sha256. Si Deadlock se actualiza y el archivo cambia, se niega a construir en vez de generar un mod roto.

## Cómo sabemos que está bien
- **Meshopt:** 16 casos (los 4 LOD más datos aleatorios, ruido, ceros, triángulos desordenados e índices enormes) dan **exactamente** los mismos bytes que la librería en C.
- **El modelo completo** generado en Go tiene el **mismo sha256** que el del Python, que es el que ya funciona en el juego. En `testdata/` solo se guarda el hash, porque el modelo contiene datos de Valve.

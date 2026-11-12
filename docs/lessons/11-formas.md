# Etapa 11: más modelos 3D (dado, gema, moneda, huevo)

**Código:** `backend/internal/mesh/shape.go` · **Probar:** `go run ./cmd/catmod -shape cube FOTO` o el selector "La forma" del sitio

## La idea: deformar la esfera en vez de hacer modelos nuevos
El modelo del juego tiene una cantidad **fija** de vértices y triángulos por LOD. En vez de inventar mallas nuevas, cada forma es **la misma grilla de la esfera con otro radio en cada dirección**: un vértice que iba en `d · radio` pasa a `d · radio · forma(d)`.

Eso trae tres ventajas:
- cabe siempre en el presupuesto de vértices de cada LOD
- las UV no cambian, así que **la foto, la textura y los sellos funcionan igual** en todas las formas
- con `forma = nil` sale la esfera de siempre, idéntica byte a byte (los goldens lo comprueban)

## Las formas: superelipsoides
Casi todas salen de una sola fórmula, el **superelipsoide**: `(|x/a|^(2/e2) + |y/b|^(2/e2))^(e2/e1) + |z/c|^(2/e1) = 1`.
- exponente chico → caras planas: **dado** (los tres ejes) y **moneda** (solo arriba y abajo)
- exponente 1 → elipsoide: el **huevo** (más un poco de asimetría arriba y abajo)
- exponente cerca de 2 → octaedro: la **gema**

Como la fórmula es "homogénea", el radio en cada dirección tiene solución directa: `r = f(d)^(-e1/2)`, sin buscar nada iterativamente.

## Normales
La luz necesita saber hacia dónde mira cada punto. En la esfera la normal es la misma dirección del vértice; en las otras formas no. Se calculan con **diferencias finitas**: se mueve un poquito el punto en `u` y en `v`, y el producto cruz de esos dos desplazamientos da la normal. En los polos ese truco falla (todos los vértices están en el mismo punto), así que ahí se usa el eje Z directamente.

## Límite de tamaño
Medimos la jaula original: cabe en una caja de **±6,34 unidades**, casi lo mismo que nuestra esfera (6,3). Ninguna forma se sale de esa caja (`TestShapesStayInBounds`). Si se saliera, el juego podría recortarla o hundirla en el suelo.

**Ojo:** la **física** sigue siendo la del modelo original. El dado o la moneda ruedan como una bola y pueden quedar un poquito flotando o metidos en el suelo según cómo caigan.

## En el sitio
- `GET /api/mesh?shape=cube&lod=0` devuelve la malla de esa forma, ahora con sus **normales**, y el visor las usa tal cual.
- Cambiar de forma no pide una textura nueva, porque la textura es la misma: solo cambia la geometría.

## Medallones: solo los 6 sellos
El material del contenedor **no tiene transparencia** (un alpha 0 sale negro), así que los huecos no pueden venir de la textura: tienen que venir de la **geometría**. `mesh/medallion.go` arma solo seis casquetes de esfera, uno por sello, y no genera nada entre ellos.

- **Atlas de textura:** cada medallón usa su propio disco en una grilla de 3×2 dentro de la textura, en vez de la proyección de la esfera. Así no hay costura ni polos. Una sola función define la correspondencia (punto del disco ↔ dirección ↔ UV), y la usan tanto la malla como la textura, así que no pueden desalinearse.
- **Mismo color que en la esfera:** cada píxel del atlas se pinta con la misma función que colorea esa dirección en la esfera (`stickerTexel`). Los sellos se ven idénticos; solo cambia dónde quedan en la textura.
- **Cara de atrás:** el juego no dibuja los triángulos vistos por detrás. Cada medallón va **dos veces**, la segunda con el orden de los vértices al revés y la normal invertida, para que no desaparezca al mirarlo por un hueco.
- **Tamaño:** el medallón mide lo que el sello más su aro, con un máximo de 45° para que los vecinos (a 90°) nunca se pisen. Por eso `/api/mesh?shape=medallions&cap_deg=…` se arma en cada pedido (tarda ~1 ms).

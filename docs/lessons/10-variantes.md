# Etapa 10: la variante "jaula original" (y un error que corregimos)

**Código:** `backend/internal/photo/cage.go`, `build.cage`, `build/lodtest.go` · **Probar:** `go run ./cmd/catmod -variant cage FOTO` y `go run ./cmd/catmod -lodtest`

## 1. Primero, una corrección importante: el formato de las UV
Para ubicar la foto en la jaula había que leer las UV del modelo original. Al hacerlo apareció algo que no calzaba:

| LOD | Leídas como SNORM | Leídas como f16 |
|---|---|---|
| 0, 1, 2 | ✅ entre 0 y 1 | ❌ valores absurdos |
| 3 | ❌ | ✅ entre 0 y 1 |

Al revisar la declaración de cada LOD dentro del modelo (`m_inputLayoutFields`), los LOD 0–2 dicen `37` (SNORM) y el LOD 3 dice `34` (FLOAT). **El juego respeta lo que declara cada LOD.** Nuestra conclusión anterior ("el juego siempre lee f16") venía de una prueba donde, sin saberlo, mirábamos el LOD 3: el contenedor es chico en pantalla, y ese es el LOD que se usa a distancia normal.

**Consecuencia:** el mod con f16 en todos los LOD se veía perfecto a distancia normal, pero roto **muy de cerca**. Ahora cada LOD se escribe en su formato, tanto en la referencia Python como en Go (goldens regenerados y Go otra vez idéntico byte a byte).

**El diagnóstico `-lodtest`:** cada LOD mapea la esfera a su propia franja de color con la etiqueta "LOD n". En el juego muestra qué LOD se dibuja a cada distancia, y si algún formato estuviera mal, esa franja se vería como manchas sin sentido.

**Lección:** una prueba que "funciona" puede estar probando otra cosa de la que crees. Conviene diseñar pruebas que **muestren qué están midiendo**, como la etiqueta con el número de LOD.

## 2. Análisis de la jaula
Con las UV bien leídas dibujamos el mapa de UV sobre la textura original y buscamos las "islas": grupos de triángulos conectados en la textura. Para cada isla vimos cuántas piezas 3D la usan y si son planas.

- **Las dos islas con forma de disco** son el **domo** ("lente") de las caras superior e inferior (±Z): una es la superficie exterior y la otra la interior. Cada isla la usan **las dos caras**, arriba y abajo.
- **Las islas grandes en forma de marco** son los marcos de las caras, también compartidos.
- **Ajuste por mínimos cuadrados** de `UV = centro + x·A[0] + y·A[1]`: así sabemos dónde está cada domo en la textura, su radio y su orientación. En el código solo quedan esas constantes (`photo.CageDiscs`), no la geometría de Valve.
- **Revisamos los 4 LOD:** en los LOD 0–2 el domo usa esas mismas zonas. En el LOD 3 está simplificado, pero al renderizarlo con la textura modificada la foto igual se ve (con el borde más poligonal).

## 3. La variante
`variant: "cage"`:
1. decodifica la textura de color original (BC7 → píxeles)
2. pinta la foto en los dos domos (`PaintDiscs`), con el mismo encuadre (centro y radio) y el aro opcional de las calcomanías, conservando el alpha original
3. la vuelve a comprimir en BC7 con sus mips

**El VPK lleva un solo archivo** (5,6 MB), porque el modelo y las otras texturas quedan los originales del juego.

**Limitaciones:**
- **La foto queda en espejo en la cara inferior.** Las dos caras comparten la misma zona de la textura, y no se puede evitar sin modificar el modelo.
- **Solo 2 de las 6 caras tienen domo.** Las otras son marcos abiertos, o el emblema triangular en el frente.

## 4. Pendiente: el visor del sitio
Para previsualizar la jaula, el navegador tendría que recibir la malla y la textura **originales** de Valve. Con la esfera eso no pasa, porque la malla y la textura son nuestras. Por eso `/api/texture` responde `no_preview` para esta variante hasta que se decida si se quiere servir eso desde el sitio.

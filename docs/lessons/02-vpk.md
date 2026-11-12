# Etapa 2: el archivo `.vpk`

**Código:** `backend/internal/vpk/vpk.go` · **Tests:** `make test-go`

## Qué es
Un `.vpk` es el "zip" de Valve, pero sin compresión. Es el archivo que importas en Deadlock Mod Manager. Adentro van nuestros 4 archivos (el modelo y 3 texturas), con su ruta dentro del juego.

```
[header 28 B] [árbol de nombres] [contenido de los archivos] [3 hashes MD5]
```

## El árbol de nombres
En vez de guardar la ruta completa de cada archivo, se agrupan para no repetir texto:

```
vtex_c                                       ← extensión
  models/props_gameplay/soul_container/materials   ← carpeta
    soul_container_color_png_1da67bfe  + 18 bytes   ← nombre + "dónde está y cuánto mide"
    soul_container_rough_png_59a0e409  + 18 bytes
vmdl_c
  models/props_gameplay/soul_container
    soul_container                     + 18 bytes
```

Cada texto termina en un byte `0`, y un `0` extra cierra cada grupo. Los 18 bytes son: CRC32 (para detectar corrupción), "este archivo viene adentro" (`0x7fff`), posición, tamaño y un terminador `0xffff`.

## Ideas de Go que aparecen
- **`map[string][]byte`:** la entrada de `Build` es un diccionario ruta → contenido, como un `dict` de Python.
- **`sort.Slice`:** en Go el orden de un map es **aleatorio a propósito**. Por eso ordenamos antes de escribir; si no, cada build daría un archivo distinto.
- **`bytes.Buffer`:** un arreglo de bytes que crece a medida que escribes. Se arman el árbol y los datos por separado y al final se juntan.
- **`crypto/md5`, `hash/crc32`:** vienen en la librería estándar, sin instalar nada.

## Cómo sabemos que está bien
- **`TestMatchesPythonReference`:** el VPK que genera Go es **idéntico byte a byte** al de la librería de Python que ya probamos en el juego y en DMM (`testdata/vpk_synthetic.vpk`).
- **`TestRoundtrip`:** un lector mínimo vuelve a abrir el archivo y comprueba contenidos, CRC y MD5.
- **`TestRejectsBadPaths`:** rutas sin extensión, absolutas o con `\` dan error en vez de un VPK roto.

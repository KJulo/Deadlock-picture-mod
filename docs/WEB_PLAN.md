# Plan: sitio web "sube tu foto → esfera → descarga el VPK"

## Flujo de usuario
1. Entra a `catmod.pages.dev` (o al dominio propio, si lo compramos) y sube una foto.
2. Elige una variante (esfera con calcomanías, jaula original con fotos en algunas caras, …), ajusta el encuadre y ve un **preview 3D fiel al juego**.
3. Presiona "Descargar", recibe el `.vpk` y lo importa como mod local en Deadlock Mod Manager.

## Arquitectura

```
Cloudflare Pages (gratis)                  Google Cloud Run (free tier, escala a cero)
web/  Astro estático                       backend/  Go (net/http) en Docker
├─ subir foto + controles     ──HTTPS──▶   POST /api/preview → textura + malla (para el visor)
├─ visor three.js: la MISMA malla          POST /api/build   → .vpk
│  que se genera para el juego,             └─ archivos base del juego dentro de la imagen,
│  UV decodificadas como f16                   en un registro PRIVADO (nunca en el repo)
└─ shader aproximado al NPR del juego
```

- **Monorepo, sin Turborepo:** son tres toolchains distintos (Go, Astro/npm, Python). Un `Makefile` en la raíz basta. En GitHub Actions se usan filtros por carpeta (`paths:`) para que cada parte construya solo lo suyo.
- **`reference/` (Python) es la fuente de verdad.** Cada módulo de Go se valida contra golden files generados por el Python (`make golden` → `testdata/`). En `testdata/` solo van metadatos y salidas sintéticas, nunca contenido de Valve.
- **URL del usuario:** la del frontend. La de Cloud Run (`https://catmod-api-<n>.<región>.run.app`) solo la usa el frontend por debajo.

## Etapas (Claude implementa; cada etapa trae una explicación breve en docs/lessons/)

| # | Etapa | Estado |
|---|---|---|
| 1 | `internal/resource`: contenedor `*_c`, idéntico byte a byte — [explicación](lessons/01-resource.md) | ✅ |
| 2 | `internal/vpk`: writer de VPK v2, idéntico byte a byte a la librería de Python — [explicación](lessons/02-vpk.md) | ✅ |
| 3 | `internal/mesh`: esfera, empaquetado de tangentes, UV f16; idéntica byte a byte a `mesh.py` — [explicación](lessons/03-mesh.md) | ✅ |
| 4 | Encoder meshopt (vértices v0 + índices v1) en Go puro, idéntico byte a byte a la librería en C; `vmdl_c` completo con el mismo sha256 que el Python — [explicación](lessons/04-meshopt.md) | ✅ |
| 5 | Texturas: proyección de la foto (idéntica a `photo.py`), **BC7 vía cgo** (`bc7enc` + parche de alpha), BC4, mips; `build.Mod` + CLI `cmd/catmod` (foto → VPK en ~0,9 s) — [explicación](lessons/05-texturas.md) | ✅ (probado en el juego) |
| 6 | API HTTP (`cmd/server`): `/api/build`, `/api/texture`, `/api/mesh`, `/api/defaults`, `/healthz`; límites, validación, cola con 503, timeouts, CORS, logs JSON, apagado ordenado; la foto nunca toca el disco — [explicación](lessons/06-api.md) | ✅ |
| 7 | Docker (estático + distroless, 38,6 MB) + scripts de deploy a Cloud Run (Artifact Registry privado, `--max-instances 2`), gzip en la API, CI — [guía](lessons/07-deploy.md) | 🟡 listo; faltan los pasos de tu cuenta de Google Cloud |
| 8 | Preview fiel: renderizar la malla real con un shader NPR aproximado (pulso turquesa, borde, rim) y calibrar contra capturas del juego | |
| 9 | `web/` en Astro con diseño propio inspirado en Deadlock: art déco, Nueva York ocultista de los años 20, pergamino, turquesa y latón, tipografías serif. Sin logos ni assets de Valve; aviso de fan-made | |
| 10 | Variantes: jaula original con UV recodificadas a f16 y fotos en discos o marcos (requiere prueba en el juego) | |

## Pendientes / riesgos
- Las actualizaciones de Deadlock pueden cambiar los archivos base. El sha256 lo detecta, y hay que volver a extraerlos y actualizar la imagen.
- Hipótesis por verificar: las UV originales de la jaula también se leen como f16, lo que es necesario para la etapa 10.
- Cloud Run pide tarjeta para verificar la cuenta. Configurar una alerta de presupuesto en $1 desde el día 1.

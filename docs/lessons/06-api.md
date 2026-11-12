# Etapa 6: la API HTTP

**Código:** `backend/internal/api/` (handlers y middlewares), `backend/cmd/server/` (arranque y configuración) · **Correr:** `make dev`

## Los endpoints

| Método y ruta | Recibe | Devuelve |
|---|---|---|
| `GET /healthz` | — | `{"status":"ok"}`, para que Cloud Run sepa que el servidor está vivo |
| `GET /api/defaults` | — | Los parámetros por defecto y los límites, para armar los controles del frontend |
| `GET /api/mesh` | — | La esfera tal como la lee el juego (posiciones, UV f16 decodificadas y triángulos), para el visor 3D |
| `POST /api/texture?size=1024` | foto (+ params) | La textura de la esfera en JPEG, para la vista previa |
| `POST /api/build` | foto (+ params) | El `.vpk` para DMM |

Los `POST` usan `multipart/form-data`, el mismo formato que un `<form>` con un archivo: un campo `photo` y, opcional, un campo `params` con JSON. Lo que mandes en `params` reemplaza los valores por defecto:

```bash
curl -F photo=@assets/cat.jpg -F 'params={"layout":"wrap","back":"flip"}' \
     localhost:8080/api/build -o mod.vpk
```

## Cómo está armado
**El router** es el de la librería estándar. Desde Go 1.22 acepta el método en el patrón, y si llega un `GET` a una ruta `POST` responde 405 solo:

```go
mux.HandleFunc("POST /api/build", s.buildHandler)
```

**Los middlewares** son funciones que envuelven a otras (`func(http.Handler) http.Handler`), como capas de cebolla. De afuera hacia adentro:
1. **`recoverer`:** si algo hace panic, responde 500 en vez de botar el servidor (test: `TestPanicRecovered`).
2. **`logger`:** escribe una línea JSON por request (método, ruta, estado, bytes, ms) con un ID. **Nunca registra el contenido**, porque son fotos de usuarios.
3. **`cors`:** el frontend vive en otro dominio (`catmod.pages.dev`) y el navegador solo lo deja llamar a la API si la API lo autoriza. La lista de dominios permitidos viene de `ALLOWED_ORIGINS`.
4. **`securityHeaders`:** un par de cabeceras de seguridad básicas.

**Errores claros:** cada error responde JSON `{"error": {"code": "...", "message": "..."}}` con el código HTTP correcto:
- **400:** parámetros malos, o un campo con typo; `DisallowUnknownFields` lo detecta en vez de ignorarlo.
- **413:** foto muy grande.
- **415:** la foto no se puede leer.
- **503:** el servidor está ocupado.
- **500:** solo para errores nuestros. El detalle va al log, no al usuario.

Para distinguir estos casos, el paquete `photo` define errores "centinela" (`ErrBadPhoto`, `ErrBadParams`) y la API los reconoce con `errors.Is`.

## Protecciones (servidor gratis = recursos chicos)
- **Límite de subida:** `http.MaxBytesReader` corta la lectura pasado el límite (15 MB por defecto), así nadie puede mandar 2 GB.
- **La foto nunca toca el disco:** se lee el multipart "en streaming" con `MultipartReader`. `ParseMultipartForm` guardaría los archivos grandes en `/tmp`.
- **Fotos gigantes:** una foto de 48 MP se reduce a 4096 px apenas se decodifica, así una sola foto no se come la memoria.
- **Cola de builds:** cada build usa todos los núcleos, así que se permiten N a la vez (`MAX_CONCURRENT_BUILDS`, por defecto 2). Es un **semáforo** hecho con un canal con buffer:

  ```go
  slots := make(chan struct{}, 2)
  slots <- struct{}{}   // toma un lugar (espera si están los 2 ocupados)
  <-slots               // lo libera
  ```

  Si alguien espera más de 20 s, recibe 503 con `Retry-After`.
- **Timeouts del servidor:** 10 s para las cabeceras (protege del ataque "slowloris"), 60 s para subir y 90 s para responder.
- **Apagado ordenado:** antes de apagar una instancia, Cloud Run manda `SIGTERM`. `signal.NotifyContext` lo captura y `Shutdown` termina los requests en curso antes de salir.

## Configuración (variables de entorno)
| Variable | Por defecto |
|---|---|
| `PORT` | `8080` (Cloud Run la define sola) |
| `CATMOD_TEMPLATES` | `../templates` |
| `ALLOWED_ORIGINS` | `http://localhost:4321` (el puerto de desarrollo de Astro) |
| `MAX_UPLOAD_MB` | `15` |
| `MAX_CONCURRENT_BUILDS` | `2` |

## Cómo sabemos que está bien
- **14 tests HTTP con `httptest`:** usan un "builder falso" para correr sin los archivos del juego, también en CI, y otro test usa el builder real. Cubren cada código de error, CORS, el 503 con la cola llena y el panic. Pasan con el detector de carreras (`go test -race`).
- **Prueba en vivo con `curl`:** el build con la foto del gato devuelve el VPK de 14 MB en 0,77 s, la textura en 0,15 s y la malla, y una foto inválida da 415.

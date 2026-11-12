# Etapa 7: Docker y deploy a Cloud Run

**Archivos:** `backend/Dockerfile`, `.dockerignore`, `backend/deploy/setup-gcp.sh`, `backend/deploy/deploy.sh`, `.github/workflows/backend.yml`

## Qué es cada cosa
- **Docker:** empaqueta el servidor con todo lo que necesita en una "imagen". Una imagen corre igual en tu PC y en Google.
- **Artifact Registry:** el "Docker Hub" privado de Google. Ahí se sube la imagen. **Tiene que ser privado**, porque la imagen contiene los archivos del juego.
- **Cloud Run:** toma la imagen y la expone en internet con HTTPS. Si nadie la usa, se apaga sola (escala a cero) y no cobra.

## El Dockerfile (dos etapas)
1. **`golang:1.27.3-trixie`:** compila el binario. Como usamos cgo (el BC7 está en C), se enlaza **estático**: el binario lleva adentro todo lo de C/C++ y no depende de nada del sistema.
2. **`distroless/static:nonroot`:** la imagen final. No tiene shell ni gestor de paquetes; solo el binario y `templates/`, y corre como usuario sin privilegios. Pesa **38,6 MB** en total.

`.dockerignore` deja entrar solo `backend/` y `templates/`. El resto del repo (Python, `.venv`, fotos) nunca llega a la imagen.

## Lo que medimos en local (con los límites de Cloud Run: 2 CPU, 1 GB)
- Un build con la foto del gato tarda ~2 s.
- **gzip:** el VPK de 14 MB viaja como 2,9 MB, porque dos texturas son de color constante. Eso importa: la transferencia de salida es lo primero que se cobra.
- Con 4 builds simultáneos, 2 se procesan y 2 esperan en la cola; todos terminan bien.
- La memoria quedó en ~160 MB.

## Costos (presupuesto $0)
Cloud Run tiene un nivel gratuito mensual generoso en CPU, memoria y requests. Cada build gasta ~4 vCPU-segundos, así que caben decenas de miles al mes. **La transferencia de salida** tiene un margen gratis más chico, y por eso agregamos gzip. Los límites exactos cambian, así que revísalos en la página de precios de Cloud Run antes de publicar.

`deploy.sh` además pone un **techo duro**: `--max-instances 2`. Aunque alguien bombardee la API, nunca habrá más de 2 servidores encendidos.

Usamos `us-central1` porque es región "Tier 1", la más barata si alguna vez pasas el nivel gratuito. Desde Chile agrega ~150 ms, que no se notan en un build de 2 s. Si prefieres Santiago, usa `GCP_REGION=southamerica-west1`.

## Pasos que tienes que hacer tú (yo no puedo entrar a tu cuenta)
Escribe cada comando en Claude Code con `!` adelante, así su salida queda en la conversación:

1. **Instala gcloud** (en CachyOS/Arch está en el AUR):
   ```
   ! paru -S google-cloud-cli
   ```
2. **Inicia sesión** (se abre el navegador):
   ```
   ! gcloud auth login
   ```
3. **Crea el proyecto y vincúlale la facturación** en https://console.cloud.google.com:
   - "Nuevo proyecto": el ID tiene que ser único, por ejemplo `catmod-<algo>`.
   - Facturación: asocia tu tarjeta al proyecto. No cobra mientras estés dentro del nivel gratuito.
   - **Alerta de presupuesto:** Facturación → Presupuestos y alertas → crear uno de **US$1** con aviso por correo. Si algo se descontrola, te enteras de inmediato.
4. **Setup único** (activa las APIs, crea el registro privado y configura Docker):
   ```
   ! GCP_PROJECT=tu-id-de-proyecto backend/deploy/setup-gcp.sh
   ```
5. **Deploy** (construye, sube y despliega, y al final muestra la URL):
   ```
   ! GCP_PROJECT=tu-id-de-proyecto make deploy
   ```
   Cuando exista el frontend, se agrega su dominio: `ALLOWED_ORIGINS=https://catmod.pages.dev make deploy`.

Para actualizar después, basta con repetir el paso 5.

## CI (GitHub Actions)
`.github/workflows/backend.yml` corre `gofmt`, `go vet` y `go test -race` en cada push que toque el backend. Los tests que necesitan los archivos del juego se saltan solos, y el resto, goldens incluidos, corre en la nube de GitHub.

Para subir este archivo, tu token de `gh` necesita el permiso `workflow` (una sola vez):
```
! gh auth refresh -h github.com -s workflow
```

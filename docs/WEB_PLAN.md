# Plan: sitio web "sube tu foto → esfera → descarga el VPK"

## Flujo de usuario
1. El usuario sube una foto.
2. Ajusta el encuadre (centro, zoom/radio, rotación, parte de atrás) y ve en vivo **un render 3D de la esfera** que puede girar con el mouse.
3. Presiona "Descargar" y recibe un `.vpk`, que importa como mod local en Deadlock Mod Manager. Ya comprobamos que DMM acepta un `.vpk` suelto.

## Arquitectura recomendada

```
navegador (estático)                         backend (Python)
├─ subir foto + controles                    POST /api/build  (foto + PhotoParams)
├─ preview 3D con three.js:                    └─ catmod.build.build_mod(...).vpk_bytes()
│  SphereGeometry + textura generada              -> application/octet-stream (≈14 MB)
│  en un <canvas> con la MISMA proyección
│  que catmod/photo.py (portada a JS)
└─ botón "Descargar" -> POST /api/build
```

- **Preview en el cliente:** es instantáneo y no le pega al servidor mientras el usuario ajusta. La proyección de `catmod/photo.py` es una fórmula simple por texel y se porta a JS/canvas o a un shader. Para que el preview coincida con el juego, la convención de UV de la esfera (`u = φ/2π`, `v = θ/π`, +Z arriba, frente en +X) debe ser la misma en three.js. Basta con rotar `SphereGeometry` o generar la malla con la misma fórmula.
- **Build en el servidor:** reutiliza el núcleo tal cual: BC7 con `etcpak`, meshoptimizer y el writer de VPK. Tarda unos 3 segundos por build. Endpoint sugerido: FastAPI, `POST /api/build` (multipart: `photo` + JSON de parámetros), que devuelve el `.vpk`.
- **Archivos base (`templates/`):** el servidor los necesita. Se extraen una vez desde una instalación del juego con `catmod_cli.py fetch` y se guardan en el servidor, no en el repo público. Después de cada actualización de Deadlock hay que volver a extraerlos y revisar el sha256.

## Alternativa: todo en el navegador (sin backend)
Es posible: meshoptimizer tiene un encoder oficial en JS/WASM, hay encoders BC7 en WASM y el formato VPK es simple. Pero implica portar `resource.py`, `vtex.py` y el writer de VPK a TypeScript, y además repartir los archivos base de Valve a cada visitante. Conviene evaluarla solo si el costo del servidor llega a importar.

## Pendientes antes del sitio
- [ ] Validar en el juego: iluminación, el brillo turquesa del material (autoiluminación y transmisión) y que la esfera se vea bien al caer o girar.
- [ ] Decidir si se neutraliza el brillo turquesa. Habría que parchear `soul_container.vmat_c`, cuyo KV3 binario es más complejo.
- [ ] Límites de subida (tamaño y formatos), y borrar la foto apenas se genera el VPK.
- [ ] Preview más fiel opcional: renderizar en three.js con un material emisivo turquesa parecido al del juego.

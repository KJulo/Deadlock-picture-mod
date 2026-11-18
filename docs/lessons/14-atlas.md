# Etapa 14: atlas de la esfera

La esfera y el dado usan seis caras equiangulares en una textura cuadrada de 3 × 2 islas. Cada isla tiene un margen de ocho píxeles a 2048: el color repite el borde más cercano y el mapa de normales queda plano fuera de las islas. La malla, el color y el relieve comparten la conversión entre UV y dirección; la tangente apunta hacia U creciente y el bitangente es normal × tangente. El layout envolvente sigue evaluándose por dirección.

El atlas aumenta la resolución horizontal y elimina los polos y la costura de longitud. La elección se mide para cada forma, incluso para formas nuevas: se compara el máximo error radial en LOD3 y se descarta un atlas que supere en más de 50 % el error equirectangular de otro LOD. La esfera elige atlas uniforme; el dado elige atlas con muestreo concentrado hacia las aristas mediante un exponente 0,5, sin cambiar el mapa UV; gema, moneda y huevo conservan la grilla equirectangular adaptativa. La decisión por nombre se guarda en memoria durante el proceso. Los medallones y la jaula no cambian.

A 2048, la densidad equirectangular es 5,69 a infinito px/grado en horizontal (diverge en los polos) y 11,38 en vertical. En el atlas es aproximadamente 7,41 a 10,48 px/grado en cada eje. La tabla muestra vértices/triángulos y la máxima separación angular en grados entre vecinos de la grilla: total / cruce de aristas del cubo / zona del borde de la moneda (|z| cerca de 0,42). El atlas de esta primera tabla es uniforme, antes del muestreo adaptado del dado. Son mediciones direccionales, no una medida del error geométrico de curvatura.

| Forma | LOD | Equirect V/T | Equirect total/arista/borde | Atlas V/T | Atlas total/arista/borde |
|---|---:|---:|---:|---:|---:|
| sphere | 0 | 3003/5776 | 4.74/4.74/4.74 | 3174/5808 | 4.09/4.09/4.09 |
| sphere | 1 | 1540/2916 | 6.67/6.67/6.67 | 1536/2700 | 6.00/6.00/6.00 |
| sphere | 2 | 630/1156 | 10.59/10.59/10.59 | 600/972 | 9.99/9.99/9.98 |
| sphere | 3 | 561/1024 | 11.25/11.25/11.25 | 600/972 | 9.99/9.99/9.98 |
| cube | 0 | 3066/5904 | 7.35/5.62/6.96 | 3174/5808 | 4.09/4.09/4.09 |
| cube | 1 | 1568/2976 | 10.94/6.30/10.02 | 1536/2700 | 6.00/6.00/6.00 |
| cube | 2 | 627/1152 | 16.19/9.86/14.81 | 600/972 | 9.99/9.99/9.98 |
| cube | 3 | 594/1088 | 16.83/10.08/15.25 | 600/972 | 9.99/9.99/9.98 |
| gem | 0 | 3066/5904 | 6.62/6.62/6.22 | 3174/5808 | 4.09/4.09/4.09 |
| gem | 1 | 1568/2976 | 9.88/9.88/8.94 | 1536/2700 | 6.00/6.00/6.00 |
| gem | 2 | 627/1152 | 14.68/14.68/14.11 | 600/972 | 9.99/9.99/9.98 |
| gem | 3 | 594/1088 | 14.97/14.97/14.97 | 600/972 | 9.99/9.99/9.98 |
| coin | 0 | 3066/5904 | 10.61/5.19/4.69 | 3174/5808 | 4.09/4.09/4.09 |
| coin | 1 | 1568/2976 | 13.92/7.49/7.10 | 1536/2700 | 6.00/6.00/6.00 |
| coin | 2 | 627/1152 | 23.11/11.45/10.41 | 600/972 | 9.99/9.99/9.98 |
| coin | 3 | 594/1088 | 24.32/11.39/10.50 | 600/972 | 9.99/9.99/9.98 |
| egg | 0 | 3066/5904 | 5.00/5.00/4.97 | 3174/5808 | 4.09/4.09/4.09 |
| egg | 1 | 1568/2976 | 7.50/7.50/6.99 | 1536/2700 | 6.00/6.00/6.00 |
| egg | 2 | 627/1152 | 11.33/11.33/11.29 | 600/972 | 9.99/9.99/9.98 |
| egg | 3 | 594/1088 | 11.99/11.82/11.99 | 600/972 | 9.99/9.99/9.98 |

## Error de superficie

Para cada triángulo no degenerado se miden los tres puntos medios y el centroide: el primer número es el máximo error radial, el segundo su RMS con igual peso por muestra, ambos como porcentaje del radio de la esfera; el tercero es el máximo ángulo en grados entre la normal verdadera (diferencias finitas) y la normal interpolada. Son muestras discretas, no una cota continua. El atlas con exponente 0,5 se probó para todas las formas; en la gema, sus caras pequeñas amplifican mucho el error cuando la grilla no incluye el centro.

| Forma | LOD | Equirect máx / RMS / normal | Atlas uniforme máx / RMS / normal | Atlas 0,5 máx / RMS / normal |
|---|---:|---:|---:|---:|
| sphere | 0 | 0.1706 / 0.1020 / 0.00 | 0.1621 / 0.0850 / 0.00 | 1.3855 / 0.2126 / 0.00 |
| sphere | 1 | 0.3381 / 0.2024 / 0.00 | 0.3412 / 0.1826 / 0.00 | 3.9772 / 0.5209 / 0.00 |
| sphere | 2 | 0.8513 / 0.5126 / 0.00 | 0.9064 / 0.5057 / 0.00 | 6.4887 / 1.1504 / 0.00 |
| sphere | 3 | 0.9561 / 0.5790 / 0.00 | 0.9064 / 0.5057 / 0.00 | 6.4887 / 1.1504 / 0.00 |
| cube | 0 | 0.3734 / 0.1086 / 1.16 | 0.8612 / 0.1654 / 1.59 | 0.2492 / 0.0747 / 0.46 |
| cube | 1 | 0.7046 / 0.2129 / 2.15 | 1.6657 / 0.3534 / 3.32 | 0.5226 / 0.1604 / 0.98 |
| cube | 2 | 1.7796 / 0.5587 / 4.78 | 3.6202 / 0.9523 / 8.13 | 1.3588 / 0.4435 / 2.64 |
| cube | 3 | 1.9347 / 0.5997 / 4.90 | 3.6202 / 0.9523 / 8.13 | 1.3588 / 0.4435 / 2.64 |
| gem | 0 | 0.1731 / 0.0441 / 15.47 | 0.4536 / 0.0608 / 21.54 | 1.5949 / 0.1708 / 24.29 |
| gem | 1 | 0.2846 / 0.0832 / 17.21 | 5.3629 / 0.5115 / 19.90 | 21.7539 / 1.9427 / 22.48 |
| gem | 2 | 0.5351 / 0.1969 / 20.77 | 9.3505 / 1.2018 / 20.93 | 27.3727 / 3.3658 / 22.91 |
| gem | 3 | 0.7871 / 0.2395 / 19.85 | 9.3505 / 1.2018 / 20.93 | 27.3727 / 3.3658 / 22.91 |
| coin | 0 | 0.1867 / 0.0895 / 1.00 | 1.1132 / 0.2208 / 3.91 | 1.4610 / 0.1962 / 4.14 |
| coin | 1 | 0.3717 / 0.1769 / 1.74 | 2.2578 / 0.4700 / 7.41 | 2.8867 / 0.4299 / 7.16 |
| coin | 2 | 0.8660 / 0.4638 / 4.31 | 5.5517 / 1.1857 / 15.96 | 6.3058 / 1.1065 / 17.50 |
| coin | 3 | 0.9824 / 0.4951 / 4.63 | 5.5517 / 1.1857 / 15.96 | 6.3058 / 1.1065 / 17.50 |
| egg | 0 | 0.1339 / 0.0819 / 0.02 | 0.1763 / 0.0736 / 0.02 | 1.8809 / 0.1884 / 0.15 |
| egg | 1 | 0.2722 / 0.1610 / 0.04 | 0.3790 / 0.1581 / 0.05 | 5.4067 / 0.4623 / 0.14 |
| egg | 2 | 0.6831 / 0.4227 / 0.11 | 1.0459 / 0.4377 / 0.13 | 8.7109 / 1.0181 / 0.25 |
| egg | 3 | 0.7177 / 0.4531 / 0.12 | 1.0459 / 0.4377 / 0.13 | 8.7109 / 1.0181 / 0.25 |

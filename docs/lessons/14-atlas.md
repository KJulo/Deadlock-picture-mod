# Etapa 14: atlas de la esfera

La esfera usa seis caras equiangulares en una textura cuadrada de 3 × 2 islas. Cada isla tiene un margen de ocho píxeles a 2048: el color repite el borde más cercano y el mapa de normales queda plano fuera de las islas. La malla, el color y el relieve comparten la conversión entre UV y dirección; la tangente apunta hacia U creciente y el bitangente es normal × tangente. El layout envolvente sigue evaluándose por dirección.

El atlas aumenta la resolución horizontal y elimina los polos y la costura de longitud. La esfera lo usa por defecto. Las demás formas conservan la grilla equirectangular adaptativa: el espacio angular por sí solo no demuestra que una grilla uniforme capture mejor sus curvas. Las formas nuevas también usan esa alternativa conservadora. Los medallones y la jaula no cambian.

A 2048, la densidad equirectangular es 5,69 a infinito px/grado en horizontal (diverge en los polos) y 11,38 en vertical. En el atlas es aproximadamente 7,41 a 10,48 px/grado en cada eje. La tabla muestra vértices/triángulos y la máxima separación angular en grados entre vecinos de la grilla: total / cruce de aristas del cubo / zona del borde de la moneda (|z| cerca de 0,42). Son mediciones direccionales, no una medida del error geométrico de curvatura.

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

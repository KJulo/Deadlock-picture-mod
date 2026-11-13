# Etapa 12: varios sellos y fotos

Cada sello guarda su foto, dirección y encuadre. El editor circular cambia al seleccionar un sello; arrastrar un sello sobre el modelo cambia su dirección. El visor convierte el punto de la malla de vuelta al espacio local antes de cambiar de ejes, así el giro de la esfera no altera la posición guardada.

La textura asigna cada dirección al centro de sello más cercano; si dos centros empatan, gana el primero. Durante el arrastre se pide una textura de 512 píxeles y al soltar se genera la de 2048. El preview y el VPK usan la misma proyección y las mismas fotos.

Los medallones mantienen sus seis posiciones fijas, aunque cada uno puede usar otra foto. La envoltura usa una sola foto. Las fotos se procesan en memoria y el límite de subida se aplica a la suma de todas.

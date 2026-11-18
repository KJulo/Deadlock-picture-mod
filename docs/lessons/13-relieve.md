# Etapa 13: relieve de los sellos

Con `relief` entre 0 y 1, el aro genera una normal tangente y una sombra AO suave sin añadir geometría. El mapa usa la dirección y el sello más cercano de la textura de color; los medallones usan su atlas. Con cero se conserva el camino original de texturas constantes y la jaula no cambia.

Se asume que el juego lee X e Y de la normal desde R y G, con bitangente positiva `normal × tangente`; B conserva la rugosidad. El orden y los signos de canales y la intensidad de AO requieren una prueba dentro del juego.

#include <string.h>
#include "bc7decomp.h"

extern "C" int glue_decode_block(const unsigned char *block, unsigned char *rgba_out) {
	bc7decomp::color_rgba px[16];
	bool ok = bc7decomp::unpack_bc7(block, px);
	for (int i = 0; i < 16; i++)
		for (int c = 0; c < 4; c++)
			rgba_out[i * 4 + c] = px[i][c];
	return ok ? 1 : 0;
}

#include <string.h>
#include "bc7enc.h"
#include "glue.h"

void glue_init(void) { bc7enc_compress_block_init(); }

// Same values as bc7enc_compress_block_params_init(), written out because the header's helpers are
// C99 "inline" definitions without an external symbol (they may not link when not inlined).
static void params_init(bc7enc_compress_block_params *p, int perceptual) {
	p->m_max_partitions_mode = BC7ENC_MAX_PARTITIONS1;
	p->m_try_least_squares = BC7ENC_TRUE;
	p->m_mode_partition_estimation_filterbank = BC7ENC_TRUE;
	p->m_uber_level = 0;
	p->m_use_mode5_for_alpha = BC7ENC_TRUE;
	p->m_use_mode7_for_alpha = BC7ENC_TRUE;
	if (perceptual) {
		p->m_perceptual = BC7ENC_TRUE;
		p->m_weights[0] = 128; p->m_weights[1] = 64; p->m_weights[2] = 16; p->m_weights[3] = 32;
	} else {
		p->m_perceptual = BC7ENC_FALSE;
		p->m_weights[0] = 1; p->m_weights[1] = 1; p->m_weights[2] = 1; p->m_weights[3] = 1;
	}
}

void glue_encode_rows(const uint8_t *rgba, int width, int by0, int by1, uint8_t *out, int perceptual) {
	bc7enc_compress_block_params params;
	params_init(&params, perceptual);
	uint8_t pixels[16 * 4];
	int bw = width / 4;
	for (int by = by0; by < by1; by++) {
		for (int bx = 0; bx < bw; bx++) {
			for (int y = 0; y < 4; y++)
				memcpy(pixels + y * 16, rgba + ((size_t)(by * 4 + y) * width + bx * 4) * 4, 16);
			bc7enc_compress_block(out, pixels, &params);
			out += 16;
		}
	}
}

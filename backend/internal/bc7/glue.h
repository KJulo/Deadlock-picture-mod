// Small C layer between Go and bc7enc, so Go makes one cgo call per strip of blocks
// instead of one per 4x4 block (each cgo call costs ~tens of ns).
#include <stdint.h>

void glue_init(void);

// Encodes block rows [by0, by1) of an RGBA8 image (width, height multiples of 4) into out,
// which points at the first block of row by0. perceptual=1 for photos, 0 for data textures.
void glue_encode_rows(const uint8_t *rgba, int width, int by0, int by1, uint8_t *out, int perceptual);

// Decodes one 16-byte BC7 block into 16 RGBA8 pixels. Returns 0 on an invalid block.
int glue_decode_block(const uint8_t *block, uint8_t *rgba_out);

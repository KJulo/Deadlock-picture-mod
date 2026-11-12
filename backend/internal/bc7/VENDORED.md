# Vendored: bc7enc

- Source: https://github.com/richgel999/bc7enc (branch `master`, fetched 2026-11-11)
- Files: `bc7enc.c`, `bc7enc.h`, `bc7decomp.cpp`, `bc7decomp.h`
- License: MIT or public domain, at your choice (`LICENSE-bc7enc`)
- `VENDORED.sha256` has the upstream hashes.

## Local patch
`bc7enc.c`, `find_optimal_solution`: opaque mode 6 blocks always use pbit 1, so alpha decodes as
exactly 255 (upstream leaves it at 254 in about half the blocks). Search for `catmod patch`.
`bc7enc.c` therefore no longer matches its upstream hash.

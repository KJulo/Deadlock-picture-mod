"""Minimal reader/writer for the Source 2 compiled-resource container (*_c files).

Layout: u32 file_size | u16 header_version | u16 version | u32 block_table_offset | u32 block_count
        block table: block_count * (char[4] type, u32 offset, u32 size)
Offsets are relative to the position of the field that stores them. Block data is usually 16-byte
aligned, but not always (RERL in vmdl_c is 4-aligned), so the alignment of each block is inferred on
parse and reproduced on write -> parse(x).serialize() == x for unmodified resources.
Anything after the last block (e.g. texture pixel data in vtex_c) is kept as `trailer`.
"""
from __future__ import annotations

import struct
from dataclasses import dataclass, field

ALIGN = 16


@dataclass
class Resource:
    header_version: int
    version: int
    blocks: list[tuple[str, bytes]] = field(default_factory=list)
    trailer: bytes = b""
    aligns: list[int] = field(default_factory=list)

    @classmethod
    def parse(cls, data: bytes) -> "Resource":
        _size, hver, ver, table_off, count = struct.unpack_from("<IHHII", data, 0)
        table = 8 + table_off
        blocks, aligns = [], []
        end = prev_end = table + count * 12
        for i in range(count):
            entry = table + i * 12
            kind = data[entry:entry + 4].decode("ascii")
            rel, size = struct.unpack_from("<II", data, entry + 4)
            start = entry + 4 + rel
            blocks.append((kind, data[start:start + size]))
            aligns.append(next(a for a in (16, 8, 4, 2, 1) if start == _align(prev_end, a)))
            prev_end = start + size
            end = max(end, start + size)
        return cls(hver, ver, blocks, data[end:], aligns)

    def block_index(self, kind: str, nth: int = 0) -> int:
        hits = [i for i, (k, _) in enumerate(self.blocks) if k == kind]
        return hits[nth]

    def serialize(self) -> bytes:
        count = len(self.blocks)
        table = 16  # header is 16 bytes, table follows immediately
        aligns = self.aligns + [ALIGN] * (count - len(self.aligns))
        pos = table + count * 12
        layout = []
        for (_, payload), a in zip(self.blocks, aligns):
            pos = _align(pos, a)
            layout.append(pos)
            pos += len(payload)
        body_end = layout[-1] + len(self.blocks[-1][1]) if self.blocks else table
        out = bytearray(body_end)
        struct.pack_into("<HHII", out, 4, self.header_version, self.version, table - 8, count)
        for i, ((kind, payload), start) in enumerate(zip(self.blocks, layout)):
            entry = table + i * 12
            out[entry:entry + 4] = kind.encode("ascii")
            struct.pack_into("<II", out, entry + 4, start - (entry + 4), len(payload))
            out[start:start + len(payload)] = payload
        # the size field covers the resource blocks only (vtex_c pixel data after it is not counted)
        struct.pack_into("<I", out, 0, len(out))
        return bytes(out) + self.trailer


def _align(n: int, a: int = ALIGN) -> int:
    return (n + a - 1) // a * a

"""Verify that a PE32+ executable has the icon group used by PdmView."""

import struct
import sys
from pathlib import Path


def check_icon(executable):
    data = executable.read_bytes()
    pe_offset, = struct.unpack_from("<I", data, 0x3C)
    optional_header = pe_offset + 24
    magic, = struct.unpack_from("<H", data, optional_header)
    if data[:2] != b"MZ" or data[pe_offset:pe_offset + 4] != b"PE\0\0" or magic != 0x20B:
        raise ValueError("expected a PE32+ executable")
    section_count, = struct.unpack_from("<H", data, pe_offset + 6)
    header_size, = struct.unpack_from("<H", data, pe_offset + 20)
    resource_rva, resource_size = struct.unpack_from("<II", data, optional_header + 128)
    if not resource_rva or not resource_size:
        raise ValueError("PE has no resource directory")

    sections = [
        struct.unpack_from("<III", data, optional_header + header_size + 40 * index + 12)
        for index in range(section_count)
    ]

    def read_rva(rva, size):
        for address, raw_size, offset in sections:
            if address <= rva and rva + size <= address + raw_size:
                start = offset + rva - address
                payload = data[start:start + size]
                if len(payload) == size:
                    return payload
        raise ValueError(f"invalid PE resource address 0x{rva:x}")

    resource = read_rva(resource_rva, resource_size)
    directory_flag = 0x80000000

    def entries(offset):
        named, numbered = struct.unpack_from("<HH", resource, offset + 12)
        return dict(
            struct.unpack_from("<II", resource, offset + 16 + 8 * index)
            for index in range(named, named + numbered)
        )

    def resource_data(kind, identifier):
        kinds = entries(0)
        identifiers = entries(kinds[kind] & ~directory_flag)
        languages = entries(identifiers[identifier] & ~directory_flag)
        data_entry = next(iter(languages.values()))
        rva, size = struct.unpack_from("<II", resource, data_entry)
        return read_rva(rva, size)

    # MyGo loads icon group 1 via LoadIconW.
    group = resource_data(14, 1)
    reserved, kind, count = struct.unpack_from("<HHH", group)
    if reserved != 0 or kind != 1 or len(group) != 6 + 14 * count:
        raise ValueError("invalid RT_GROUP_ICON header")

    sizes = set()
    for index in range(count):
        width, height, _, _, _, _, size, identifier = struct.unpack_from(
            "<BBBBHHIH", group, 6 + 14 * index
        )
        image = resource_data(3, identifier)
        if len(image) != size or not image.startswith(b"\x89PNG\r\n\x1a\n"):
            raise ValueError(f"invalid RT_ICON payload {identifier}")
        dimensions = width or 256, height or 256
        if dimensions[0] != dimensions[1] or struct.unpack_from(">II", image, 16) != dimensions:
            raise ValueError(f"invalid RT_ICON dimensions for {identifier}")
        sizes.add(dimensions[0])
    if not {16, 32, 48, 256} <= sizes:
        raise ValueError(f"missing standard icon sizes: {sorted(sizes)}")
    print("Windows icon group 1: " + ", ".join(f"{size}×{size}" for size in sorted(sizes)))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: check-windows-icon.py executable")
    try:
        check_icon(Path(sys.argv[1]))
    except (KeyError, OSError, StopIteration, ValueError, struct.error) as error:
        sys.exit(f"Windows icon check failed: {error}")

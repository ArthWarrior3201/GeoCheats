#!/usr/bin/env python3
"""Reject executables missing their multi-resolution icon or version resource."""
import struct, sys
from pathlib import Path
import pefile

for name in ['GeoCheats-Windows.exe','Dash-Windows.exe','Dash-CLI.exe']:
    pe=pefile.PE(str(Path(sys.argv[1])/name))
    types={entry.id:entry for entry in pe.DIRECTORY_ENTRY_RESOURCE.entries}
    assert 3 in types and 14 in types and 16 in types, f'{name}: missing icon/group/version resources'
    group=types[14].directory.entries[0].directory.entries[0].data.struct
    raw=pe.get_data(group.OffsetToData,group.Size)
    reserved,kind,count=struct.unpack_from('<HHH',raw)
    assert reserved==0 and kind==1 and count>=7,f'{name}: missing icon resolutions'
    sizes={raw[6+i*14] or 256 for i in range(count)}
    assert {16,24,32,48,64,128,256}<=sizes,(name,sizes)
    print(f'{name}: embedded ICO sizes {sorted(sizes)} and version metadata verified')

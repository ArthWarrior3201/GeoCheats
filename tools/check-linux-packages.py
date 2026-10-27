#!/usr/bin/env python3
"""Inspect actual DEB/RPM payloads for both launchers, app icons and GUI binaries."""
import subprocess,sys
from pathlib import Path

root=Path(sys.argv[1])
required=['usr/bin/geocheats','usr/bin/geocheats-dash','usr/bin/geocheats-dash-builder','usr/share/applications/geocheats.desktop','usr/share/applications/dash.desktop']
required += [f'usr/share/icons/hicolor/{size}x{size}/apps/{name}.png' for size in [256,512] for name in ['geocheats','geocheats-dash']]
for package in [*root.glob('*.deb'),*root.glob('*.rpm')]:
    command=['dpkg-deb','--contents',str(package)] if package.suffix=='.deb' else ['rpm','-qpl',str(package)]
    output=subprocess.check_output(command,text=True)
    for path in required:assert path in output,(package,path)
    print(f'{package.name}: desktop launchers, binaries and PNG icon payloads verified')
assert list(root.glob('*.deb')) and list(root.glob('*.rpm')),'Both Linux package types are required'

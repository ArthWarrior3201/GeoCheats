#!/usr/bin/env python3
"""Convert the supplied logos to platform icon formats; requires Pillow at build time."""
from pathlib import Path
from PIL import Image, ImageOps

root = Path(__file__).resolve().parents[1]
for name, slug in [('GeoCheats', 'geocheats'), ('Dash', 'dash')]:
    image = Image.open(root / 'assets' / f'{name}.png').convert('RGBA')
    image = ImageOps.contain(image, (1024, 1024), Image.Resampling.LANCZOS)
    square = Image.new('RGBA', (1024, 1024))
    square.alpha_composite(image, ((1024-image.width)//2, (1024-image.height)//2))
    mac = root / 'source-code-macos/packaging/icons'
    win = root / 'source-code-windows/packaging/icons'
    linux = root / 'source-code-linux/packaging/icons'
    for folder in [mac, win, linux]: folder.mkdir(parents=True, exist_ok=True)
    square.save(mac / f'{name}.icns', format='ICNS')
    square.save(win / f'{name}.ico', format='ICO', sizes=[(n,n) for n in [16,24,32,48,64,128,256]])
    for size in [256,512]:
        square.resize((size,size), Image.Resampling.LANCZOS).save(linux / f'{slug}-{size}.png', optimize=True)
    for platform in ['windows','macos','linux']:
        folder=root / f'source-code-{platform}/internal' / ('app' if name=='GeoCheats' else 'dashui')
        folder.mkdir(parents=True, exist_ok=True)
        square.resize((128,128),Image.Resampling.LANCZOS).save(folder / f'{slug}.png', optimize=True)
    print(f'{name}: ICNS, ICO, Linux PNGs and embedded UI icon generated')

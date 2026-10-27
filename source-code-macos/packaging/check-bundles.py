#!/usr/bin/env python3
"""Validate native app bundles and actually launch them through LaunchServices."""
import json, os, plistlib, subprocess, sys, tempfile
from pathlib import Path

stage = Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory(prefix='geocheats-launch-') as tmp:
    for name in ['GeoCheats', 'Dash']:
        app=stage/f'{name}.app'
        with (app/'Contents/Info.plist').open('rb') as stream: info=plistlib.load(stream)
        binary=app/'Contents/MacOS'/info['CFBundleExecutable']
        icon=app/'Contents/Resources'/info['CFBundleIconFile']
        assert os.access(binary,os.X_OK),f'{binary} is not executable'
        assert icon.read_bytes().startswith(b'icns'),f'{icon} is not an ICNS file'
        subprocess.run(['plutil','-lint',str(app/'Contents/Info.plist')],check=True)
        subprocess.run(['lipo','-verify_arch','x86_64','arm64',str(binary)],check=True)
        subprocess.run(['codesign','--verify','--deep','--strict','--verbose=2',str(app)],check=True)
        decoded=Path(tmp)/f'{name}.iconset'
        subprocess.run(['iconutil','--convert','iconset','--output',str(decoded),str(icon)],check=True)
        report=Path(tmp)/f'{name}.json'
        subprocess.run(['/usr/bin/open','-W','-n',str(app),'--args','--launch-check',str(report)],check=True,timeout=45)
        assert report.is_file(),f'{name} launched without producing its startup report'
        result=json.loads(report.read_text());assert result.get('ok'),result
        print(f'{name}: native LaunchServices, signature, universal binary, ICNS, UI and server checks passed')
    helper=stage/'Dash.app/Contents/Helpers/dash'
    subprocess.run(['lipo','-verify_arch','x86_64','arm64',str(helper)],check=True)
    subprocess.run([str(helper),'help'],check=True,stdout=subprocess.DEVNULL,timeout=15)

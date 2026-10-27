#!/usr/bin/env python3
"""Start real native windows under Xvfb and verify capture/startup reports."""
from pathlib import Path
import json,subprocess,tempfile

with tempfile.TemporaryDirectory(prefix='native-window-check-') as temporary:
    for binary,name in [('GeoCheats','GeoCheats'),('Dash-Builder','Dash')]:
        report=Path(temporary)/(name+'.json')
        subprocess.run([str(Path(binary).resolve()),'--launch-check',str(report)],check=True,timeout=40)
        state=json.loads(report.read_text())
        assert state.get('ok') and state.get('ui')=='native' and state.get('browser') is False,state
        assert Path(str(report)+'.png').stat().st_size>1000,'Native window screenshot missing'
        print(name+': native window, controls and capture passed')

# Windows icon resource

`generate.py` renders `assets/PdmView.png` at 16, 24, 32, 48, 64, 128 and
256 pixels with macOS `sips`, assembles `PdmView.ico` under `.out/windowsicon/`, and
uses pinned `github.com/akavel/rsrc@v0.10.2` to write the amd64 COFF resource
object embedded by PdmView.

Regenerate from the repository root:

```sh
python3 tools/windowsicon/generate.py
```

The output is `cmd/pdmview/pdmview_windows_amd64.syso`. The generator
needs macOS `sips` and downloads pinned `rsrc` only when regenerating. Normal
PdmView builds use the checked-in `.syso` and require no extra tool.

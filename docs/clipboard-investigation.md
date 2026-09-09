# Native clipboard and image paste investigation

Read-only investigation of the connected Tatsu running xochitl firmware 3.27.3.0,
Qt 6.8.2. No clipboard content, document data, service configuration, or process
memory was modified. The clipboard setter and image paste have not yet been
implemented or exercised.

## Evidence from this device

`/proc/386/maps` shows xochitl using
`/usr/lib/plugins/platforms/libepaper.so`, rather than an X11 or Wayland platform.
No DISPLAY or WAYLAND environment setting appeared in the inspected process,
and no matching X11/Wayland clipboard socket was found in the socket inspection.

The epaper plugin's EpaperIntegration vtable (ELF offset 0x4f610) has a relocation
at 0x4f688 to the inherited `QPlatformIntegration::clipboard() const`. There is
no EpaperIntegration clipboard override among the inspected dynamic symbols.

Upstream Qt 6.8.2 implements that accessor with a default QPlatformClipboard,
whose backing QMimeData is stored in a process-local Q_GLOBAL_STATIC. Therefore,
with this inherited backend, starting another Qt program to set its clipboard
would not set xochitl's clipboard. This conclusion uses the installed plugin's
relocations and matching upstream Qt source; the installed Qt library itself
was not fully disassembled.

The xochitl executable imports:

- QGuiApplication::clipboard()
- QClipboard::setMimeData(), mimeData(), image(), ownsClipboard()

Embedded method names and diagnostic strings include:

- systemClipboardDataChanged
- transferDataFromSystemClipboard
- clipboard image source: direct-image
- clipboard image source: file-url
- pasteClipboardContent
- pasteClipboardContentAsFloatingSceneImage
- insertImageFileAsSceneItem
- insertImageAsSceneItem

These provide strong evidence that native image clipboard handling and scene
insertion paths are compiled into this device's executable. They do not prove
that every path is enabled in the tablet UI or establish the callable signatures
and required state. Symbols for these application methods are not exported like
the Qt library functions; some names appear in Qt metadata or logging strings.

System-bus inspection found xochitl connections but no named clipboard service.
Introspection of xochitl's unique bus names was denied. No externally callable
clipboard method has been identified; denied introspection does not establish
that no such method exists.

## Recommended design

Keep the user-facing operation focused on the clipboard:

```sh
remarkablectl --host 192.168.0.113 clipboard set --image diagram.png
```

This command is proposed, not implemented. The likely architecture is:

1. The local CLI sends image bytes over SSH to remarkable-agent.
2. A small bridge loaded inside xochitl receives the request through a local
   Unix socket or another bounded local IPC channel.
3. The bridge decodes the image and sets QGuiApplication::clipboard() through
   Qt on the GUI thread, using a QImage/QMimeData image payload.
4. xochitl's existing clipboard-change and paste paths handle the image.
5. The user invokes native paste and positions the resulting object. A separate
   paste command can follow after the real UI behavior is verified.

A C++/Qt bridge is a natural choice for the in-process part; the CLI and SSH
helper can stay Go. It would need an explicit loading mechanism (for example,
a compatible extension loader or preload setup), validation on this firmware,
and potentially a one-time xochitl restart. None was installed during this
investigation.

First experiment: set a small diagnostic image through the bridge, then verify
native paste, movement, resizing, undo, and persistence after reopening the page.
Start by observing the clipboard's MIME formats and the app's reaction to a
normal copy operation if needed. Merely uploading a PNG or running a separate
clipboard executable is insufficient for the process-local backend.

## Sources

- Installed xochitl and libepaper.so, inspected through strings and ELF symbols/
  relocations; local copies kept outside the repository in
  /tmp/remarkable-clipboard.
- https://raw.githubusercontent.com/qt/qtbase/v6.8.2/src/gui/kernel/qplatformintegration.cpp
- https://raw.githubusercontent.com/qt/qtbase/v6.8.2/src/gui/kernel/qplatformclipboard.cpp
- https://doc.qt.io/qt-6/qclipboard.html

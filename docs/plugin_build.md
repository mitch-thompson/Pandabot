### Plugin Build Process

The PandaBot plugin is built as a C-shared library (DLL) using Go's `cgo` and a C wrapper for Ashita v4 compatibility.

#### Prerequisites

- Go 1.25.4 or later
- MinGW-w64 (32-bit for `386` architecture)
- `CGO_ENABLED=1`

#### Build Command

To build the plugin DLL, run the following command in PowerShell:

```powershell
$env:CGO_ENABLED = "1"
$env:GOARCH = "386"
C:\Users\mitch\sdk\go1.25.4\bin\go.exe build -buildmode=c-shared -o pandabot.dll ./cmd/plugin
```

#### Components

- `cmd/plugin/dll.go`: Contains the Go logic and exports functions to C.
- `cmd/plugin/dllmain.c`: C wrapper that implements the Ashita v4 `IPlugin` interface and delegates calls to Go.

#### Implementation Details

- **GetVersion Conflict**: The local `GetVersion` function in `dllmain.c` was renamed to `PluginGetVersion` to avoid conflicts with the Windows API `GetVersion` defined in `sysinfoapi.h`.
- **C/C++ Compatibility**: `dllmain.c` is compiled as C, so `extern "C"` blocks are not used. `stdbool.h` is included for `bool` support.

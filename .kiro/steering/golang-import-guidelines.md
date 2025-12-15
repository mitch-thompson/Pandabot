# Go Import Path Guidelines

This project uses relative module paths for internal imports to maintain portability and avoid dependency on external repositories.

## Required Patterns

### Module Name
- The module name is `PandaBot` (as defined in go.mod)
- All internal imports MUST use this module name as the base path

### Internal Package Imports
- Use: `import ("PandaBot/internal/server")`
- Use: `import ("PandaBot/internal/config")`
- Use: `import ("PandaBot/pkg/api")`

### Command Package Imports
- Use: `import ("PandaBot/cmd/pandabot")`
- Use: `import ("PandaBot/cmd/simple")`

## Prohibited Patterns

### GitHub-based Imports (NEVER use these)
- Never use: `import ("github.com/pandabot/internal/server")`
- Never use: `import ("github.com/username/pandabot/internal/config")`
- Never use: Any external GitHub repository references for internal packages

### Relative Path Imports
- Avoid: `import ("./internal/server")` 
- Avoid: `import ("../config")`

## Benefits
- **Portability**: Code works regardless of where the repository is hosted
- **Consistency**: All team members use the same import paths
- **Maintainability**: No need to update imports when repository location changes
- **Local Development**: Works seamlessly in local development environments

## Examples

### Correct Import Block
```go
import (
    "context"
    "fmt"
    "log"
    
    "PandaBot/internal/config"
    "PandaBot/internal/server"
    "PandaBot/internal/logger"
    "PandaBot/pkg/api"
)
```

### Package Structure Reference
```
PandaBot/
├── cmd/
│   ├── pandabot/
│   └── simple/
├── internal/
│   ├── config/
│   ├── server/
│   ├── logger/
│   └── ...
└── pkg/
    └── api/
```

## Go Environment Setup

### Required Environment Variables
- `GOROOT=C:\Users\mitch\sdk\go1.25.4`
- `GOPATH=C:\Users\mitch\go`
- Go executable: `C:\Users\mitch\sdk\go1.25.4\bin\go.exe`

### Build Commands
- Use: `C:\Users\mitch\sdk\go1.25.4\bin\go.exe build`
- Or ensure the Go bin directory is in PATH to use: `go build`

## Additional Rules
- Always group imports: standard library first, then external dependencies, then internal packages
- Use the exact module name `PandaBot` as specified in go.mod
- When creating new packages, ensure they follow the `PandaBot/internal/` or `PandaBot/pkg/` pattern
- Use the specific Go installation at `C:\Users\mitch\sdk\go1.25.4` for all builds and operations
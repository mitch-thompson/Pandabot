### Performance and Context Optimization
- **Minimal Context Usage**: To reduce token consumption, always prefer targeted searches (`search_project`) over reading entire directories. 
- **Large Registry Files**: Files like `internal/registry/spells.go`, `internal/registry/abilities.go`, and `internal/registry/items.go` contain many static definitions. Do NOT open these files unless specifically investigating a bug in the registration logic. Use `search_project` to find specific data points within them if needed.
- **Incremental Exploration**: Start by examining the file structure (`get_file_structure`) before opening files.
- **Specific Edits**: When editing, only include the necessary lines in `search_replace` to keep the context window clean.

### Development Guidelines
- **Go Best Practices**: Follow standard Go idioms (effective Go).
- **Comments**: Comments need to be restricted to codedocs.  If more details are required create a md file explaining it.
- **Concurrency**: Use `sync.RWMutex` for shared registries as established in the codebase.
- **Modularity**: Keep logic separated into internal packages (e.g., `spell`, `action`, `registry`).

### File Danger Levels

| Category              | Files / Patterns                              | Action Policy                                      | Reason                              |
|-----------------------|-----------------------------------------------|----------------------------------------------------|-------------------------------------|
| **Do NOT open**       | registry/spells.go, abilities.go, items.go    | Never open unless fixing registration bug          | 5k–20k+ lines, static data          |
| **High caution**      | casting.go, server.go, casting/*.go           | Open only if search didn't find answer             | Complex logic, long functions       |
| **Safe / small**      | action.go, spell.go, entity.go, job.go        | Usually safe to read                               | Short, foundational types           |
| **Configuration**     | config.go, guidelines.md, *.toml              | Safe                                               | Small, declarative                  |

### Large Registry Files — STRICT RULES

Files `internal/registry/spells.go`, `abilities.go`, `items.go` (and any future large static registries):

- **NEVER** open the entire file during normal development or debugging
- **NEVER** include more than ~20–30 lines in prompts / context
- **ONLY** open when the bug is CONFIRMED to be in the registration logic itself
  (wrong ID, missing field, duplicate entry, mutex race, init() failure)
- Use **search_project** instead:
    - `search_project "Cure V" in spells.go`
    - `search_project "MPCost:" near "Reraise III"`
    - `search_project "RegisterSpell(" "Cure IV"`

### ALWAYS

- Check `docs/` folder for documentation on a specific feature you're working on
- **ALWAYS** update documentation as features are updated
- **ALWAYS** add documentation for missing features
- **ALWAYS** create new documentation when a feature is added/changed and no matching doc exists
- **ALWAYS** create a new .md file in docs/ when no relevant documentation is found after checking
# V4 Implementation Plan - Production Ready CLI

## Current Status (v4)
✅ Basic init command with LLM validation
✅ Setup command with provider selection
✅ Help command
✅ LLM validation loop with file requests
✅ Environment variable detection and enhancement

## Missing from V1 (Priority Order)

### HIGH PRIORITY (Must Have)
1. **Global Flags**
   - [x] --verbose flag (show LLM responses, detailed logs)
   - [x] --quiet flag (minimal output)
   - [x] --version flag

2. **Init Command Enhancements**
   - [ ] --docker-run flag (start containers after generation)
   - [ ] --create-proxy flag (Caddy + /etc/hosts setup)
   - [ ] --cwd flag (custom workspace directory)
   - [ ] Quick start guide generation
   - [ ] Service detection summary display
   - [ ] Tree output display

3. **Generate Command**
   - [ ] Dry-run mode without starting containers
   - [ ] --output flag for custom directory
   - [ ] --agent flag to enable/disable AI

### MEDIUM PRIORITY (Should Have)
4. **Stats Command**
   - [ ] Live container dashboard with TUI
   - [ ] --project flag
   - [ ] --once flag for CI

5. **Nuke Command**
   - [ ] Complete cleanup (containers, volumes, images, /etc/hosts)
   - [ ] --project flag
   - [ ] --force flag

### LOW PRIORITY (Nice to Have)
6. **Diagnose Command**
   - [ ] AI-powered container diagnostics
   - [ ] --project flag

7. **Additional Features**
   - [ ] AI healer background process
   - [ ] Proxy management (Caddy)
   - [ ] Port conflict resolution display

## Implementation Strategy

### Phase 1: Flags & Output Control
1. Add global --verbose, --quiet, --version flags to root
2. Implement output level control in UI package
3. Add verbose LLM response logging

### Phase 2: Init Enhancements
1. Add --docker-run flag and docker compose up logic
2. Add --create-proxy flag and Caddy/hosts setup
3. Add --cwd flag for custom workspace
4. Generate and display quick start guide
5. Show service detection summary
6. Display tree output

### Phase 3: Generate Command
1. Implement generate command (clone + generate, no docker)
2. Add --output and --agent flags

### Phase 4: Stats & Nuke
1. Implement stats command with TUI dashboard
2. Implement nuke command with cleanup logic

### Phase 5: Polish
1. Add diagnose command
2. Add AI healer
3. Add proxy management
4. Final testing and bug fixes

## File Structure Additions Needed

```
v4/
├── cmd/
│   ├── root.go (add flags)
│   ├── init.go (enhance)
│   ├── generate.go (NEW)
│   ├── stats.go (NEW)
│   ├── nuke.go (NEW)
│   └── diagnose.go (NEW)
├── internal/
│   ├── docker/ (NEW - docker operations)
│   ├── proxy/ (NEW - Caddy + /etc/hosts)
│   ├── ports/ (NEW - port conflict resolution)
│   ├── healer/ (NEW - AI diagnostics)
│   └── nuke/ (NEW - cleanup operations)
```

## Notes
- Keep v4 simpler and more focused than v1
- Prioritize LLM validation quality over feature count
- Ensure all outputs respect --verbose and --quiet flags
- Make quick start guide comprehensive and helpful

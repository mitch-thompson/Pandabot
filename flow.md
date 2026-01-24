PandaBot Application Flow (High-Level Overview)

1. Startup & Initialization
   ├── logger.go                → Initializes global logger (file + stdout)
   ├── config.go                → Loads config.toml (or defaults) + fsnotify watcher
   ├── registry/spells.go       → Registers all spells (Cure, Protect, Shell, etc.)
   ├── registry/abilities.go    → Registers job abilities (Benediction, etc.)
   ├── registry/items.go        → Registers usable items (Echo Drops, Remedy)
   └── server.NewServer()       → Creates main Server struct
   ├── textParser           → Default trigger words → priorities
   ├── cureSelector, buffSelector, naSelector
   ├── statusMonitor        → Tracks party HP/MP/statuses
   ├── castingSystem        → Central casting engine + client manager
   └── autoActionService    → Decides next action when client is ready

2. GUI Startup (optional, runs in goroutine)
   └── gui.NewGUI(server) → ShowAndRun()
   └── refreshLoop() → every 1s → updatePartyInfo() → rebuilds party UI rows

3. Main Server Loop (TCP listener)
   Server.ListenAndServe()
   │
   ├── Accept new client connections (Ashita addon)
   │   └── ServerClientAdapter (implements ClientInterface)
   │
   └── For each connected client → spawns goroutine to handle messages

4. Incoming Message Handling (per client)
   ├── TypeChatLine           → Raw chat message
   │   └── textParser.Parse() → Detects trigger words ("stoned", "firebuffs", "panda", etc.)
   │        └── TriggerEvent → sent to triggerService.RouteTriggerEvents()
   │             └── castingSystem.ProcessTriggerEvent() → creates CastRequest(s)
   │
   ├── TypeStatusUpdate       → Party/own HP/MP/status/effects update
   │   ├── statusMonitor.Update() → tracks party state + desired buffs
   │   ├── entityService.ConvertPartyMembersToEntities() → for casting decisions
   │   └── castingEngine / prioritizer / autoActionService may react
   │
   ├── TypeActionComplete / TypeActionFailed
   │   └── castingEngine.NotifyActionComplete() → advances sequence or retries
   │
   └── TypeReadyForAction     → Client says "I'm ready for next command"
   └── autoActionService.DecideNextAction(playerName, statusMonitor)
   └── Returns *protocol.ExecuteCommand or nil
   └── Sent back to client: /ma "Cure IV" <t>, /item "Echo Drops" <me>, etc.

5. Automatic / Reactive Casting Flow (most important runtime loop)
   Client is ready (ReadyForAction) OR trigger detected
   ↓
   autoActionService.DecideNextAction()
   ├── Priority checks (in rough order):
   │   1. Self-silence → Use Echo Drops if available
   │   2. Critical/low HP party member → CureSelector.SelectOptimalCure()
   │   3. Negative status (stoned, paralyzed, etc.) → NaSelector
   │   4. Missing desired buffs (Protect, Shell, Bar*, Reraise) → BuffSelector
   │   5. Low-priority debuffs on enemies (if implemented)
   │   ↓
   CastingEngine.RequestCast(CastRequest)
   ↓
   → Select optimal spell/level (cure, protect, shell, reraise, na-remove)
   → Choose best client (if multi-client) based on job levels / MP / distance
   → Create ActionCommand (/ma "Cure IV" <t>)
   → Client.CheckReadyToCast() → wait for ReadyResponse
   → If ready → Send command to Ashita
   → Record in activeCasts + history
   ↓
   Ashita executes → sends ActionComplete / ActionFailed
   ↓
   CastingEngine.NotifyActionComplete() → clean up or queue next in sequence

6. Trigger-Based Flow (chat commands / calls for help)
   Chat: "(Player) stoned" or "panda protect" or "firebuffs"
   ↓
   textParser → TriggerEvent (type="stoned", sender="Player", priority=8)
   ↓
   triggerService → castingSystem.ProcessTriggerEvent()
   ↓
   trigger_processor → specific handler (processNaTrigger, processShellTrigger, etc.)
   ↓
   Creates CastRequest(Type: CastTypeNa / CastTypeShell / CastTypeProtect / ...)
   ↓
   Same casting flow as above

Key Decision Points Summary:
- Triggers (chat) → immediate high-priority casts
- Status updates → reactive healing / buffing / removal
- Client ready signal → proactive auto-action (cure lowest HP, maintain buffs, etc.)
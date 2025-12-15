# Implementation Plan

- [x] 1. Enhance protocol and message handling


  - Extend the existing MessagePack protocol to support all required message types
  - Add StatusUpdate, ChatLine, and enhanced ExecuteCommand message structures
  - Implement message validation and error handling
  - _Requirements: 7.3_

- [x] 1.1 Write property test for protocol compliance


  - **Property 23: Protocol compliance**
  - **Validates: Requirements 7.3**

- [x] 2. Implement server-side text parsing system


  - Create TextParser component to analyze incoming chat messages
  - Build configurable trigger word dictionary with spell mappings
  - Add sender authorization and filtering logic
  - _Requirements: 2.1, 2.2, 2.3, 2.5_

- [x] 2.1 Write property test for message forwarding


  - **Property 5: Message forwarding**
  - **Validates: Requirements 2.1**

- [x] 2.2 Write property test for unauthorized player filtering

  - **Property 7: Unauthorized player filtering**
  - **Validates: Requirements 2.5**

- [x] 3. Build spell prioritization engine


  - Implement SpellPrioritizer component with urgency-based ranking
  - Create priority calculation algorithms for healing vs buffs vs status removal
  - Add resource optimization logic for MP efficiency
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [x] 3.1 Write property test for action prioritization ranking


  - **Property 11: Action prioritization ranking**
  - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**

- [x] 3.2 Write property test for multi-trigger prioritization

  - **Property 6: Multi-trigger prioritization**
  - **Validates: Requirements 2.4**

- [x] 3.3 Write property test for resource optimization

  - **Property 12: Resource optimization**
  - **Validates: Requirements 4.5**

- [x] 4. Implement cure spell selection system


  - Create CureSelector component for optimal cure level calculation
  - Add HP-based cure spell selection algorithms
  - Implement MP validation and efficiency optimization
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 4.1 Write property test for optimal cure level calculation


  - **Property 13: Optimal cure level calculation**
  - **Validates: Requirements 5.1, 5.2, 5.3**

- [x] 4.2 Write property test for cure option optimization

  - **Property 14: Cure option optimization**
  - **Validates: Requirements 5.4**

- [x] 4.3 Write property test for MP validation before casting

  - **Property 15: MP validation before casting**
  - **Validates: Requirements 5.5**

- [x] 4.4 Implement curaga spell selection for multiple targets
  - Extend CureSelector component to detect when 3 or more party members need healing
  - Add logic to switch from individual cure spells to appropriate curaga spells
  - Implement curaga level selection based on average missing HP of affected party members
  - _Requirements: 5.6_

- [x] 4.5 Write property test for curaga selection for multiple targets
  - **Property 16: Curaga selection for multiple targets**
  - **Validates: Requirements 5.6**

- [x] 5. Build status effect and "na" spell system


  - Implement NaSpellSelector component for status effect removal
  - Create status effect to spell mapping database
  - Add severity-based prioritization for multiple status effects
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 5.1 Write property test for status effect to spell mapping


  - **Property 17: Status effect to spell mapping**
  - **Validates: Requirements 6.1**

- [x] 5.2 Write property test for status effect prioritization

  - **Property 18: Status effect prioritization**
  - **Validates: Requirements 6.2**

- [x] 5.3 Write property test for status removal command generation

  - **Property 19: Status removal command generation**
  - **Validates: Requirements 6.3**

- [x] 5.4 Write property test for action queuing for unavailable resources

  - **Property 20: Action queuing for unavailable resources**
  - **Validates: Requirements 6.4**

- [x] 5.5 Write property test for state tracking after status removal

  - **Property 21: State tracking after status removal**
  - **Validates: Requirements 6.5**

- [x] 6. Enhance Lua plugin with status monitoring



  - Extend existing Lua plugin to collect party member HP, MP, and status data
  - Implement periodic status reporting to Go server
  - Add game interface handlers for status data extraction
  - _Requirements: 3.1, 3.2_

- [x] 6.1 Write property test for periodic status reporting


  - **Property 8: Periodic status reporting**
  - **Validates: Requirements 3.1**

- [x] 6.2 Write property test for status message completeness

  - **Property 9: Status message completeness**
  - **Validates: Requirements 3.2**

- [x] 7. Implement command execution system in Lua plugin
  - Add command parsing and execution logic to Lua plugin
  - Implement priority-based command queue management
  - Add error handling and failure reporting back to server
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 7.1 Write property test for command execution and parsing
  - **Property 1: Command execution and parsing**
  - **Validates: Requirements 1.1, 1.2**

- [x] 7.2 Write property test for error handling for malformed commands
  - **Property 2: Error handling for malformed commands**
  - **Validates: Requirements 1.3**

- [x] 7.3 Write property test for command queue management
  - **Property 3: Command queue management**
  - **Validates: Requirements 1.4**

- [x] 7.4 Write property test for error reporting
  - **Property 4: Error reporting**
  - **Validates: Requirements 1.5**

- [x] 8. Enhance connection management and reliability
  - Improve existing connection handling with automatic reconnection
  - Add exponential backoff for reconnection attempts
  - Implement message queuing during disconnections
  - Add connection monitoring and health checks
  - _Requirements: 7.1, 7.2, 7.4, 7.5_

- [x] 8.1 Write property test for automatic reconnection with backoff
  - **Property 22: Automatic reconnection with backoff**
  - **Validates: Requirements 7.2**

- [x] 8.2 Write property test for connection monitoring and recovery
  - **Property 10: Connection monitoring and recovery**
  - **Validates: Requirements 3.5**

- [x] 8.3 Write property test for graceful degradation and message queuing
  - **Property 24: Graceful degradation and message queuing**
  - **Validates: Requirements 7.4**

- [x] 8.4 Write property test for state synchronization after reconnection
  - **Property 25: State synchronization after reconnection**
  - **Validates: Requirements 7.5**

- [x] 9. Integrate server-side status monitoring
  - Create StatusMonitor component to track party member states
  - Implement health threshold detection and cure triggering
  - Add status effect detection and "na" spell triggering
  - _Requirements: 3.3, 3.4, 3.5_

- [x] 10. Wire together complete system integration
  - Connect all server components (TextParser, SpellPrioritizer, CureSelector, NaSpellSelector, StatusMonitor)
  - Integrate enhanced Lua plugin with improved server
  - Add configuration management for thresholds and settings
  - Implement comprehensive logging and debugging support
  - _Requirements: All requirements integration_

- [x] 11. Filter chat message capture to tells and party messages only






  - Modify Lua addon chat handler to only capture tell messages (mode 3) and party messages (mode 4)
  - Update handle_chat_message function to filter out other chat types (say, yell, linkshell, etc.)
  - Ensure existing trigger word functionality continues to work with filtered messages
  - _Requirements: 2.1_

- [-] 12. Implement server-side command queuing and spell completion feedback



  - Modify server to maintain a command queue per client connection instead of sending all commands immediately
  - Add spell completion notification from Lua addon to server when spells finish casting
  - Implement command state tracking (queued, in-progress, completed, failed)
  - Only send next command in queue after receiving completion notification for previous command
  - Add timeout handling for commands that don't complete within expected time
  - _Requirements: 1.4, 4.1, 4.2_

- [x] 12.1 Write property test for server-side command queuing


  - **Property 26: Server-side command queuing**
  - **Validates: Requirements 1.4, 4.1**

- [ ] 12.2 Write property test for spell completion feedback


  - **Property 27: Spell completion feedback**
  - **Validates: Requirements 1.4**

- [x] 13. Enhance cure selection with actual HP/MP values and intelligent scoring
  - Updated Lua addon to send both actual HP/MP values (`party:GetMemberHP(i)`, `party:GetMemberMP(i)`) and percentage values
  - Enhanced protocol types to include `hp_actual` and `mp_actual` fields in PartyMember structure
  - Improved cure selector with healing appropriateness scoring (1.5x bonus for 70-120% coverage, penalties for insufficient/excessive healing)
  - Added job-level HP estimation with job-specific multipliers for fallback calculations
  - Updated status monitor and server processing to prioritize actual HP/MP values over percentages
  - _Requirements: 5.1, 5.2, 5.3, 9.1, 9.2, 9.3, 10.3_

- [x] 14. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
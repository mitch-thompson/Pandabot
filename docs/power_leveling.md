### Power Leveling Mode

Power Leveling (PL) Mode allows one player's bot to automatically monitor and heal another player's party. This is particularly useful when you have a high-level character (the PL target/healer) assisting a lower-level character (the PL source).

#### Activation
To activate PL mode, the player who wants to be power-leveled (PL source) must send a tell to the player who will be doing the healing (PL target) containing the phrase "power level".

**Example:**
`/tell Kiro power level`

#### Deactivation
To deactivate PL mode, either the PL source or the PL target can send a tell or a chat message containing "stop pl".

**Example:**
`/tell Kiro stop pl`

Both players must have PandaBot connected to the same server for this to function correctly.

#### Behavior
Once activated:
1.  **PL Source (Sender):** The bot will stop selecting and executing any automatic actions (spells, abilities, items). This ensures the character doesn't perform actions that might interfere with the PL process or draw unwanted attention.
2.  **PL Target (Healer):** The bot will continue its normal operations but will also monitor the status of all members in the PL source's party. It will prioritize cures and status removals for the PL source's party members as if they were in its own party.

#### Technical Implementation
-   **Detection:** The server monitors incoming tells (Mode 13/14) for the "power level" trigger in `internal/server/server.go`.
-   **State Management:** Each client (character) maintains its own `PLSource` and `PLTarget` fields. This allows multiple characters to power level different targets independently.
-   **Action Logic:** In `internal/autoActionService/autoActionService.go`, the `DecideNextAction` function checks these fields for the specific client:
    -   If the requesting client is the `PLSource`, it returns no action (unless cures are disabled, in which case it allows other actions).
    -   If the requesting client is the `PLTarget`, it retrieves the `StatusMonitor` from the `PLSource`'s client and merges their party members into the local healing consideration loop.
-   **Status Monitoring:** Each client has its own `StatusMonitor` instance to accurately track its local party's state, which is shared via the server for PL mode.

#### Per-Character Control
Both Power Leveling mode and Cure control are per-character. This means:
- You can enable PL on one character while another character continues regular play.
- You can "disable cures" on one character (e.g., your healer while you manually heal) without affecting other characters running PandaBot.
- Commands like "enable cures", "disable cures", "power level", and "stop pl" only affect the specific character that receives or sends the message.

#### Disable Cures Command
To prevent a character from casting Cures or status removal ("-na") spells, use the following commands:
-   **Disable Cures:** Send a chat message or tell containing "disable cures".
-   **Enable Cures:** Send a chat message or tell containing "enable cures".

When "Disable Cures" is active, the bot will skip all automatic Cure and status removal actions for that specific character. 

**Behavior in PL Mode:**
- If "Disable Cures" is **NOT** active (default), the PL Source (person being leveled) will stop all automatic actions.
- If "Disable Cures" **IS** active, the PL Source will be allowed to perform other automatic actions (like buffs or nukes) but will still skip Cures and status removals. This allows the power-leveled character to contribute to combat while letting the power-leveler handle all healing.

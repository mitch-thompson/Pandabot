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
-   **State Management:** The server maintains `PLSource` and `PLTarget` fields.
-   **Action Logic:** In `internal/autoActionService/autoActionService.go`, the `DecideNextAction` function checks these fields:
    -   If the requesting client is the `PLSource`, it returns no action.
    -   If the requesting client is the `PLTarget`, it retrieves the `StatusMonitor` from the `PLSource`'s client and merges their party members into the local healing consideration loop.
-   **Status Monitoring:** Each client now has its own `StatusMonitor` instance to accurately track its local party's state, which is shared via the server for PL mode.

### Red Mage Convert

PandaBot automatically manages the **Convert** ability for Red Mages (RDM).

#### Behavior
If the bot is playing as a Red Mage (RDM):
- **Level Requirement**: RDM Level 40 or higher.
- **Trigger Condition**: MP drops below **10%**.
- **Action**: The bot will automatically add `Convert` to the top of its action queue.

#### Priority
Convert is treated as a high-priority action (Priority 80), ensuring it is used quickly to restore MP for continued healing and support.

#### Availability
The bot tracks the 10-minute cooldown of Convert. If the ability is on cooldown, it will wait until it becomes available again before attempting to use it.

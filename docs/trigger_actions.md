### Trigger-Based Actions

PandaBot supports several actions that can be requested via chat triggers. These actions are typically handled as one-time requests and are not automatically maintained like permanent buffs.

#### Control Triggers

These triggers control the bot's internal state or queues.

| Trigger | Description |
|---------|-------------|
| `panda clear` | Clears all tracked buffs for everyone. |
| `panda clear <name>` | Clears tracked buffs for a specific player. |
| `panda clear <spell>` | Clears a specific tracked buff (e.g., `panda clear haste`). |
| `panda` | Clears the current casting queue and all tracked buffs. |

#### System Triggers

These triggers enable or disable specific bot modes.

| Trigger | Description |
|---------|-------------|
| `power level` | Enters Power Leveling mode (must be sent via direct tell to the bot). |
| `stop pl` | Disables Power Leveling mode. |
| `disable cures` | Suspends all automatic healing and status removal actions. |
| `enable cures` | Resumes automatic healing and status removal actions. |

#### Status Removal Triggers

These triggers cause the bot to cast a specific status removal spell on the sender.

| Trigger | Spell |
|---------|-------|
| `stoned` | Stona |
| `paralyzed` | Paralyna |
| `silenced` | Silena |
| `poisoned` | Poisona |
| `blinded` | Blindna |
| `erase` | Erase |
| `cursna`, `cursed`, `doom` | Cursna |
| `viruna`, `diseased`, `plagued` | Viruna |

#### Ability Triggers

These triggers cause the bot to use a Job Ability on the sender. These are one-time uses and will not be automatically recast when the cooldown expires.

| Trigger | Ability | Description |
|---------|---------|-------------|
| `devotion` | Devotion | Uses Devotion on the person who requested it. Requires WHM 75+. |

#### Buff Triggers

| Trigger | Action |
|---------|--------|
| `haste` | Casts Haste on the sender. |
| `auspice` | Casts Auspice (self-targeted). |
| `regen` | Casts the optimal Regen spell on the sender. |
| `refresh` | Casts optimal Refresh on sender, or target (e.g., `refresh player`). Limited to party members. Recast when it falls off. |
| `protect` | Casts the optimal Protect/Protectra spell. |
| `shell` | Casts the optimal Shell/Shellra spell. |
| `reraise` | Casts the optimal Reraise spell on the caster. |
| `solace` | Uses Afflatus Solace. |
| `misery` | Uses Afflatus Misery. |
| `lightarts` | Uses Light Arts. |
| `darkarts` | Uses Dark Arts. |

#### Healing Triggers

| Trigger | Action |
|---------|--------|
| `heal`, `cure`, `help` | Casts the optimal Cure spell on the sender based on their missing HP. |

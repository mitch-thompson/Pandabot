### Trigger-Based Actions

PandaBot supports several actions that can be requested via chat triggers. These actions are typically handled as one-time requests and are not automatically maintained like permanent buffs.

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

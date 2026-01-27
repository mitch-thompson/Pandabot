### Cure Selection Logic

PandaBot uses a sophisticated selection system for Cure and Curaga spells, balancing healing throughput, MP efficiency, and casting time. The core logic is implemented in the `cureSelector` package.

#### Selection Process

The selection of a cure spell level follows these primary steps:

1.  **Requirement Filtering**:
    *   **MP Availability**: Only spells within the caster's current MP pool are considered.
    *   **Job Level**: The caster must meet the level requirement for their current job or subjob.
        *   **Caster Level Determination**: The system maintains a mapping of all the caster's job levels (e.g., `{"WHM": 75, "BLM": 37}`). When evaluating a spell, it checks if any of the caster's jobs meet or exceed the required level for that specific spell in the registry. This allows the bot to seamlessly use spells available from both the main job and subjob.
        *   **Data Source**: This information is retrieved in real-time from the game client's state. When the bot receives a status update, it extracts the current job and level, updates its internal mapping, and passes this via the `ClientInterface` to the selector within a `CastContext`.
    *   **Spell Type**: Only spells categorized as `Healing` (with "Cure" or "Curaga" prefix) or specific `BlueMagic` healing spells (`Wild Carrot`, `Magic Fruit`, `Healing Breeze`) are evaluated.

2.  **Selection Modes**:
    Depending on the situation, the system uses different evaluation algorithms:

    *   **Urgency-Weighted Efficiency (`SelectOptimalCure`)**:
        Used for general healing. It calculates a "Weighted Efficiency" for each spell:
        `WeightedEfficiency = (ActualHeal * TargetUrgency) / MPCost`
        - `ActualHeal`: The amount the spell will heal (capped at target's missing HP).
        - `TargetUrgency`: `MissingHP / MaxHP`. A target at lower health has higher urgency, making larger, less efficient spells more attractive.
    
    *   **Damage-Matching (`SelectCureByDamage`)**:
        Used when responding to a specific amount of damage. It scores spells based on how well their `HealAmount` matches the `missingHP`:
        - **Good Match (0.8x - 1.5x missing HP)**: High score, penalized slightly for overheal.
        - **Overheal (> 1.5x missing HP)**: Significant penalty (`0.5 / healRatio`).
        - **Underheal (< 0.8x missing HP)**: Severe penalty, especially for large damage amounts (> 300 HP), to ensure adequate healing.

3.  **Efficiency vs. Emergency (`selectBestOption`)**:
    The system can prioritize either MP efficiency or casting speed:
    *   **Efficiency Mode**: Multiplies `MPEfficiency` by an `appropriatenessBonus` (1.5x for 70-120% coverage) and applies a light overheal penalty.
    *   **Emergency Mode**: Multiplies `TimeEfficiency` (`HealAmount / CastTime`) by a `healingScore` (favoring spells that cover at least 80% of missing HP) and applies a light overheal penalty.

#### Curaga Selection

Curaga level selection is based on the average missing HP of party members:

1.  **Threshold**: Curaga is considered if 2 or more party members need healing (3+ for specific multi-target calls).
2.  **Efficiency Check**: Curaga is only chosen if its MP cost is less than or equal to the estimated total cost of casting individual cures for each member.
3.  **Level Selection**: The system selects the Curaga level that maximizes:
    `Score = (HealAmount * MembersCount / MPCost) * (1.0 - OverhealWaste * 0.4)`
    - This favors spells that heal the group efficiently while penalizing significant overheal based on the average missing HP.
    - A 20% bonus is applied if more than 4 members are being healed.

#### Summary of Cure Levels

| Spell | HP Restored | Base MP Cost |
| :--- | :--- | :--- |
| Cure | 30 | 8 |
| Cure II | 100 | 24 |
| Cure III | 250 | 46 |
| Cure IV | 480 | 88 |
| Cure V | 780 | 135 |
| Cure VI | 900 | 180 |
| Curaga | 90 | 60 |
| Curaga II | 300 | 120 |
| Curaga III | 550 | 180 |
| Curaga IV | 800 | 260 |
| Curaga V | 1100 | 380 |

*Note: HP Restored values are base values from the registry and may be modified by gear or job traits in the future.*

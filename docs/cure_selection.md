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
    *   **Tier Filtering (n-2 Rule)**: To prevent casting inefficiently low-level spells, the system identifies the highest available tier of "Cure" or "Curaga" for the current job level. It then filters out any spells that are more than 2 tiers below this maximum (e.g., if Cure V is available, only Cure III, IV, and V are considered).
        *   This rule can be disabled by setting `is_powerleveling = true` in `config.toml`.
    *   **Recast Timers**: Spells currently on recast are not considered. The system tracks recast timers internally (based on base values) and synchronizes with real-time data from the game client via status updates.

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

1.  **Threshold**: Curaga is considered if the number of party members needing healing meets or exceeds the `curaga_threshold_count` (defined in `config.toml`, defaults to 3).
2.  **Significant Damage**: Only members with significant damage (missing > 50 HP or > 15% of Max HP) are counted towards the threshold to avoid using Curaga for minor chip damage.
3.  **Efficiency Check**: Curaga is only chosen if its MP cost is less than or equal to the estimated total cost of casting individual cures for each member.
4.  **Level Selection**: The system selects the Curaga level that maximizes:
    `Score = (HealAmount * MembersCount / MPCost) * (1.0 - OverhealWaste * 0.4)`
    - This favors spells that heal the group efficiently while penalizing significant overheal based on the average missing HP.
    - A 20% bonus is applied if more than 4 members are being healed.

#### Configuration Thresholds

The following thresholds can be adjusted in `config.toml`:

- **`cure_threshold_percent`** (Default: 70): The HP percentage below which a party member is considered to "need" a cure. This maps to the **Medium** health threshold.
- **`health_thresholds.critical`** (Default: 25): The HP percentage below which a party member is in critical condition.
- **`health_thresholds.low`** (Default: 50): The HP percentage below which a party member is in low health condition.
- **`curaga_threshold_count`** (Default: 3): The minimum number of party members needing healing to trigger a Curaga evaluation.
- **`is_powerleveling`** (Default: false): When set to true, disables the n-2 tier filtering rule, allowing all available tiers of cure spells to be used.
- **`disable_cures`** (Default: false): When set to true, prevents the bot from automatically casting any Cure or status removal spells. This can be toggled in-game using "disable cures" and "enable cures" commands.

#### Internal Thresholds (Hardcoded)

Some logic uses hardcoded values for safety and stability:

- **Significant Damage (Curaga)**: A member only counts towards the `curaga_threshold_count` if they are missing more than **50 HP** OR more than **15%** of their Max HP (`internal/cureSelector/cureSelector.go`).
- **Power Leveling Mode**: When Power Leveling (PL) mode is active, Curaga spells are completely disabled to prevent the healer from drawing excessive enmity from multiple targets. The bot will use individual Cure spells instead. See [Power Leveling Implementation](developer/power_leveling_implementation.md) for developer documentation.
- **Emergency Mode**: In emergency mode, the system favors spells that cover at least **80%** of missing HP (`internal/cureSelector/cureSelector.go`).
- **Efficiency Bonus**: A 1.5x appropriateness bonus is applied if a spell covers **70-120%** of missing HP (`internal/cureSelector/cureSelector.go`).

#### Summary of Cure Levels

| Spell | HP Restored | Base MP Cost | Base Recast |
| :--- | :--- | :--- | :--- |
| Cure | 30 | 8 | 2.0s |
| Cure II | 100 | 24 | 2.5s |
| Cure III | 250 | 46 | 3.0s |
| Cure IV | 480 | 88 | 3.5s |
| Cure V | 780 | 135 | 4.0s |
| Cure VI | 900 | 180 | 4.5s |
| Curaga | 90 | 60 | 4.5s |
| Curaga II | 300 | 120 | 5.25s |
| Curaga III | 550 | 180 | 6.0s |
| Curaga IV | 800 | 260 | 6.75s |
| Curaga V | 1100 | 380 | 7.5s |

*Note: HP Restored values are base values from the registry and may be modified by gear or job traits in the future.*

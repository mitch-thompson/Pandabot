### Blue Magic Healing Support

The bot now supports healing spells for Blue Mage (BLU). 

#### Supported Spells

| Spell Name | Level | MP Cost | Target | Type |
|------------|-------|---------|--------|------|
| Healing Breeze | 16 | 55 | Self (AoE) | Curaga-style |
| Wild Carrot | 30 | 37 | Party Member | Cure-style |
| Magic Fruit | 58 | 72 | Party Member | Cure-style |

#### Selection Logic

- **Wild Carrot** and **Magic Fruit** are integrated into the single-target cure selection logic (`getAvailableCureOptions`). They are evaluated alongside standard Cure spells based on efficiency and healing amount.
- **Healing Breeze** is integrated into the AoE cure selection logic (`getAvailableCuragaOptions`). It is treated as a Curaga-style spell and will be used when multiple party members need healing.

#### Implementation Details

- Spells are registered in `internal/registry/spells.go` with `spell.BlueMagic` type.
- `internal/cureSelector/cureSelector.go` has been updated to include `spell.BlueMagic` types in its search for available healing options.

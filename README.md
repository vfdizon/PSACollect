# PSA Collect

A Discord collection and turn-based battle game.

## Character types

- **Goon** (formerly Chud)
- **Harana**
- **Barkada**
- **Locked In** (formerly Lockin)

Legacy `Chud` and `Lockin` values are normalized to `Goon` and `Locked In` when character data is loaded.

## Rarities

Rarity controls spawn probability and multiplies a character's battle stats.

| Rarity | Spawn chance | Stat multiplier |
| --- | ---: | ---: |
| Common | 50% | 1x |
| Uncommon | 25% | 1.5x |
| Rare | 16% | 2x |
| Epic | 6% | 3x |
| Legendary | 2% | 5x |
| Mythic | 1% | 10x |

## Leveling system

- Players begin at level 1 with 0 XP.
- Winning a battle awards at least 25 XP.
- Defeating a higher-level character awards 10 additional XP per level of disadvantage.
- Winning despite a type disadvantage awards 15 additional XP.
- Participating in a lost battle awards 10 XP.
- A player gains one level for every 100 total XP.
- Battle XP is awarded to both the player and the character used.
- Leveling does not change a character's base power or toughness.
- Character levels are capped at 100. Extra XP carries toward the next level.

### Character XP requirements

The XP needed for the next level is the rarity's base requirement multiplied by the character's current level.

| Rarity | Base XP |
| --- | ---: |
| Common | 100 |
| Uncommon | 125 |
| Rare | 150 |
| Epic | 200 |
| Legendary | 300 |
| Mythic | 500 |

### Type advantages

Type advantages form a cycle:

- Goon beats Harana.
- Harana beats Barkada.
- Barkada beats Locked In.
- Locked In beats Goon.

Winning in the reverse direction counts as defeating an enemy while at a type disadvantage.

## Abilities and effects

The combat engine supports these status effects:

- **Weakened:** reduces power by 20% for two turns.
- **Vulnerable:** reduces toughness by 20% for two turns.
- **Injured:** prevents healing for three turns.
- **Exhausted:** prevents attacking for one turn.

Character-specific ability assignment is not yet configured in the character data.

## Stats

- **Power:** acts as the offensive multiplier. Effective power is base power multiplied by the rarity multiplier, and an attack deals that amount as damage. Leveling does not change power.
- **Toughness:** acts as HP. A character loses when toughness reaches zero. Damage is only tracked for the current battle, so toughness returns to full after every game. Leveling does not change toughness.
- **Healing:** restores one-third of maximum toughness and can be used once per battle.

## Battle command

Use `?battle @opponent` to challenge another player. After acceptance, both players select a character through DM and alternate between `attack`, `heal`, and `forfeit`.
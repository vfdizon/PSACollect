package main

type RarityMultiplier struct {
	Rarity     string
	Multiplier float64
}

type Effect struct {
	Name           string
	Description    string
	turnsRemaining int
	effectFunc     func(*FightingCharacter, *FightingCharacter)
}

type FightingCharacter struct {
	character        *Character
	currentPower     int
	currentToughness int
	currentLevel     int
	uuid             string

	canHeal   bool
	canAttack bool

	effects []Effect
}

type currentBattle struct {
	player1 *FightingCharacter
	player2 *FightingCharacter
	turn    int
}

var rarityMultipliers = []RarityMultiplier{
	{Rarity: "Common", Multiplier: 1.0},
	{Rarity: "Uncommon", Multiplier: 1.5},
	{Rarity: "Rare", Multiplier: 2.0},
	{Rarity: "Epic", Multiplier: 3.0},
	{Rarity: "Legendary", Multiplier: 5.0},
	{Rarity: "Mythic", Multiplier: 10.0},
}

var Effects = []Effect{
	{
		Name:           "Weakened",
		Description:    "Reduces power by 20%% for 2 turns.",
		turnsRemaining: 2,
		effectFunc: func(attacker, defender *FightingCharacter) {
			attacker.currentPower = int(float64(attacker.currentPower) * 0.8)
		},
	},
	{
		Name:           "Vulnerable",
		Description:    "Reduces toughness by 20%% for 2 turns.",
		turnsRemaining: 2,
		effectFunc: func(attacker, defender *FightingCharacter) {
			defender.currentToughness = int(float64(defender.currentToughness) * 0.8)
		},
	},
	{
		Name:           "Injured",
		Description:    "Disallows healing for 3 turns.",
		turnsRemaining: 3,
		effectFunc: func(attacker, defender *FightingCharacter) {
			defender.canHeal = false
		},
	},
	{
		Name:           "Exhausted",
		Description:    "Disallows attacking for 1 turn.",
		turnsRemaining: 1,
		effectFunc: func(attacker, defender *FightingCharacter) {
			attacker.canAttack = false
		},
	},
}

func attack(attacker, defender *FightingCharacter) {
	damage := attacker.currentPower - defender.currentToughness
	if damage < 0 {
		damage = 0
	}
	defender.currentToughness -= damage

	for i := range attacker.effects {
		attacker.effects[i].effectFunc(attacker, defender)
	}

	for i := range defender.effects {
		defender.effects[i].effectFunc(attacker, defender)
	}

	attacker.canAttack = false
}

func heal(character *FightingCharacter) {
	if !character.canHeal {
		return
	}
	healAmount := int(float64(character.character.Power) * 0.5)
	character.currentToughness += healAmount
	character.canHeal = false
}

func endTurn(battle *currentBattle) {
	battle.turn++

	for _, character := range []*FightingCharacter{battle.player1, battle.player2} {
		character.canAttack = true
		character.canHeal = true

		var remainingEffects []Effect
		for _, effect := range character.effects {
			effect.turnsRemaining--
			if effect.turnsRemaining > 0 {
				remainingEffects = append(remainingEffects, effect)
			}
		}
		character.effects = remainingEffects
	}
}

func applyEffect(character *FightingCharacter, effect Effect) {
	character.effects = append(character.effects, effect)
	effect.effectFunc(character, character)
}

func calculateRarityMultiplier(rarity string) float64 {
	for _, rm := range rarityMultipliers {
		if rm.Rarity == rarity {
			return rm.Multiplier
		}
	}
	return 1.0
}

func isBattleOver(battle *currentBattle) bool {
	return battle.player1.currentToughness <= 0 || battle.player2.currentToughness <= 0
}

func calculateLevelMultiplier(level int16) int {
	return int(float64(level) * 2.0)
}

func getBattleWinner(battle *currentBattle) string {
	if battle.player1.currentToughness <= 0 && battle.player2.currentToughness <= 0 {
		return "Draw"
	} else if battle.player1.currentToughness <= 0 {
		return "Player 2"
	} else if battle.player2.currentToughness <= 0 {
		return "Player 1"
	}
	return "None"
}

func startBattle(player1Char, player2Char *IndividalCharacter) *currentBattle {
	player1 := &FightingCharacter{
		character:        &player1Char.CharacterInfo,
		currentPower:     int(float64(player1Char.CharacterInfo.Power) * calculateRarityMultiplier(player1Char.CharacterInfo.Rarity)),
		currentToughness: int(float64(player1Char.CharacterInfo.Toughness) * calculateRarityMultiplier(player1Char.CharacterInfo.Rarity)),
		currentLevel:     player1Char.Level,
		uuid:             player1Char.UUID,
		canHeal:          true,
		canAttack:        true,
	}

	player2 := &FightingCharacter{
		character:        &player2Char.CharacterInfo,
		currentPower:     int(float64(player2Char.CharacterInfo.Power) * calculateRarityMultiplier(player2Char.CharacterInfo.Rarity)),
		currentToughness: int(float64(player2Char.CharacterInfo.Toughness) * calculateRarityMultiplier(player2Char.CharacterInfo.Rarity)),
		currentLevel:     player2Char.Level,
		uuid:             player2Char.UUID,
		canHeal:          true,
		canAttack:        true,
	}

	return &currentBattle{
		player1: player1,
		player2: player2,
		turn:    1,
	}
}

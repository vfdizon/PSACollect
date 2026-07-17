package main

import "testing"

func battleTestCharacter(name string, power, toughness int) *IndividalCharacter {
	return &IndividalCharacter{
		CharacterInfo: Character{Name: name, Rarity: "Common", Power: power, Toughness: toughness},
		UUID:          name,
		Level:         1,
	}
}

func TestAttackDealsPowerAsDamage(t *testing.T) {
	battle := startBattle(battleTestCharacter("Attacker", 3, 5), battleTestCharacter("Defender", 1, 7))

	attack(battle.player1, battle.player2)

	if battle.player2.currentToughness != 4 {
		t.Fatalf("expected 4 toughness after attack, got %d", battle.player2.currentToughness)
	}
}

func TestHealIsCappedAndCanOnlyBeUsedOnce(t *testing.T) {
	battle := startBattle(battleTestCharacter("Healer", 2, 6), battleTestCharacter("Opponent", 1, 6))
	battle.player1.currentToughness = 5

	heal(battle.player1)
	if battle.player1.currentToughness != 6 {
		t.Fatalf("expected healing to cap at 6, got %d", battle.player1.currentToughness)
	}

	battle.player1.currentToughness = 3
	heal(battle.player1)
	if battle.player1.currentToughness != 3 {
		t.Fatalf("expected second heal to do nothing, got %d", battle.player1.currentToughness)
	}
}

func TestBattleEndsAtZeroToughness(t *testing.T) {
	battle := startBattle(battleTestCharacter("Attacker", 10, 5), battleTestCharacter("Defender", 1, 4))

	attack(battle.player1, battle.player2)

	if !isBattleOver(battle) {
		t.Fatal("expected battle to be over")
	}
	if winner := getBattleWinner(battle); winner != "Player 1" {
		t.Fatalf("expected Player 1 to win, got %s", winner)
	}
}

func TestLevelsDoNotChangeBattleStats(t *testing.T) {
	levelOne := battleTestCharacter("Level One", 10, 20)
	levelTen := battleTestCharacter("Level Ten", 10, 20)
	levelTen.Level = 10

	battle := startBattle(levelOne, levelTen)

	if battle.player2.currentPower != battle.player1.currentPower {
		t.Fatalf("expected levels not to change power: level 1 has %d, level 10 has %d", battle.player1.currentPower, battle.player2.currentPower)
	}
	if battle.player2.maxToughness != battle.player1.maxToughness {
		t.Fatalf("expected levels not to change toughness: level 1 has %d, level 10 has %d", battle.player1.maxToughness, battle.player2.maxToughness)
	}
}

func TestPowerUsesRarityMultiplier(t *testing.T) {
	common := battleTestCharacter("Common", 4, 10)
	rare := battleTestCharacter("Rare", 4, 10)
	rare.CharacterInfo.Rarity = "Rare"

	battle := startBattle(common, rare)

	if battle.player1.currentPower != 4 {
		t.Fatalf("expected common power 4, got %d", battle.player1.currentPower)
	}
	if battle.player2.currentPower != 8 {
		t.Fatalf("expected rare power multiplier to produce 8, got %d", battle.player2.currentPower)
	}
}

func TestToughnessResetsForEachBattle(t *testing.T) {
	character := battleTestCharacter("Fighter", 3, 10)
	opponent := battleTestCharacter("Opponent", 2, 10)
	firstBattle := startBattle(character, opponent)
	firstBattle.player1.currentToughness = 1

	secondBattle := startBattle(character, opponent)

	if secondBattle.player1.currentToughness != secondBattle.player1.maxToughness {
		t.Fatalf("expected toughness to reset to %d, got %d", secondBattle.player1.maxToughness, secondBattle.player1.currentToughness)
	}
}

func TestBattleWinnerXPIncludesDisadvantageBonuses(t *testing.T) {
	winner := &FightingCharacter{
		character:    &Character{Type: CharacterTypeHarana},
		currentLevel: 2,
	}
	loser := &FightingCharacter{
		character:    &Character{Type: CharacterTypeGoon},
		currentLevel: 5,
	}

	got := calculateBattleWinnerXP(winner, loser)
	want := battleWinnerBaseXP + 3*levelDisadvantageXP + typeDisadvantageXP
	if got != want {
		t.Fatalf("expected %d XP with both disadvantages, got %d", want, got)
	}
}

func TestTypeAdvantagesFormCycle(t *testing.T) {
	tests := []struct {
		attacker string
		defender string
	}{
		{CharacterTypeGoon, CharacterTypeHarana},
		{CharacterTypeHarana, CharacterTypeBarkada},
		{CharacterTypeBarkada, CharacterTypeLockedIn},
		{CharacterTypeLockedIn, CharacterTypeGoon},
	}

	for _, test := range tests {
		if !hasTypeAdvantage(test.attacker, test.defender) {
			t.Errorf("expected %s to have an advantage over %s", test.attacker, test.defender)
		}
		if hasTypeAdvantage(test.defender, test.attacker) {
			t.Errorf("did not expect %s to have an advantage over %s", test.defender, test.attacker)
		}
	}
}

package main

import "testing"

func TestCharacterXPRequirementIncreasesByRarity(t *testing.T) {
	rarities := []string{"Common", "Uncommon", "Rare", "Epic", "Legendary", "Mythic"}
	previous := int64(0)
	for _, rarity := range rarities {
		required := characterXPForNextLevel(rarity, 1)
		if required <= previous {
			t.Fatalf("expected %s requirement %d to exceed previous requirement %d", rarity, required, previous)
		}
		previous = required
	}
}

func TestCharacterXPRequirementIncreasesByLevel(t *testing.T) {
	levelOne := characterXPForNextLevel("Rare", 1)
	levelTwo := characterXPForNextLevel("Rare", 2)
	if levelTwo != levelOne*2 {
		t.Fatalf("expected level two requirement %d to be twice level one requirement %d", levelTwo, levelOne)
	}
}

func TestApplyCharacterXPCarriesRemainderAcrossLevels(t *testing.T) {
	xp, level, levelsGained := applyCharacterXP(90, 1, "Common", 220)

	if level != 3 {
		t.Fatalf("expected level 3, got %d", level)
	}
	if levelsGained != 2 {
		t.Fatalf("expected 2 levels gained, got %d", levelsGained)
	}
	if xp != 10 {
		t.Fatalf("expected 10 remaining XP, got %d", xp)
	}
}

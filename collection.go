package main

import (
	"fmt"
	"strings"
)

var (
	playerCollectionPageSize = 5
)

type Player struct {
	ID         string            `json:"id"`
	Collection []CollectionEntry `json:"collection"`
	Level      int16             `json:"level"`
	XP         int64             `json:"xp"`
}

type IndividalCharacter struct {
	CharacterInfo Character `json:"character_info"`
	UUID          string    `json:"uuid"`
	Level         int       `json:"level"`
	Experience    int       `json:"experience"`
}

type CharacterQueryResult struct {
	indivudalCharacters []IndividalCharacter
	Total               int `json:"total"`
	Page                int `json:"page"`
	PageSize            int `json:"page_size"`
}

type CharacterXPProgress struct {
	Entry        CollectionEntry
	LevelsGained int16
}

func getPlayerCollection(playerID string) ([]CollectionEntry, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	return append([]CollectionEntry{}, player.Collection...), nil
}

func getPlayerCharacters(playerID string) ([]IndividalCharacter, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	var characters []IndividalCharacter
	for _, entry := range player.Collection {
		character, err := getCharacterByID(entry.CharacterID)
		if err != nil {
			return nil, err
		}

		characters = append(characters, IndividalCharacter{
			CharacterInfo: *character,
			UUID:          entry.UUID,
			Level:         int(entry.Level),
			Experience:    int(entry.XP),
		})
	}

	return characters, nil
}

func getCharacterByUUID(playerID, uuid string) (*IndividalCharacter, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	for _, entry := range player.Collection {
		if entry.UUID == uuid {
			character, err := getCharacterByID(entry.CharacterID)
			if err != nil {
				return nil, err
			}

			return &IndividalCharacter{
				CharacterInfo: *character,
				UUID:          entry.UUID,
				Level:         int(entry.Level),
				Experience:    int(entry.XP),
			}, nil
		}
	}

	errCharacterNotFound := fmt.Errorf("character with UUID %s not found in player %s's collection", uuid, playerID)
	return nil, errCharacterNotFound
}

func queryPlayerCharacters(playerID, characterName string) (*CharacterQueryResult, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	var matchingCharacters []IndividalCharacter
	for _, entry := range player.Collection {
		character, err := getCharacterByID(entry.CharacterID)
		if err != nil {
			return nil, err
		}

		if containsIgnoreCase(character.Name, characterName) {
			matchingCharacters = append(matchingCharacters, IndividalCharacter{
				CharacterInfo: *character,
				UUID:          entry.UUID,
				Level:         int(entry.Level),
				Experience:    int(entry.XP),
			})
		}
	}

	return &CharacterQueryResult{
		indivudalCharacters: matchingCharacters,
		Total:               len(matchingCharacters),
		Page:                1,
		PageSize:            len(matchingCharacters),
	}, nil
}

func spawnCharacter() (*Character, error) {
	return getRandomWeightedCharacter()
}

func catchCharacter(playerID, characterID string, xp int64, level int16) (*CollectionEntry, error) {
	return addCharacterToCollection(playerID, characterID, xp, level)
}

func releaseCharacter(playerID, entryUUID string) (*CollectionEntry, error) {
	return removeCharacterFromCollection(playerID, entryUUID)
}

func awardCharacterXP(playerID, entryUUID string, amount int64) (*CharacterXPProgress, error) {
	if amount < 0 {
		return nil, fmt.Errorf("character XP award cannot be negative")
	}

	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	entryIndex := -1
	for i := range player.Collection {
		if player.Collection[i].UUID == entryUUID {
			entryIndex = i
			break
		}
	}
	if entryIndex < 0 {
		return nil, errCollectionEntryNotFound
	}

	entry := player.Collection[entryIndex]
	character, err := getCharacterByID(entry.CharacterID)
	if err != nil {
		return nil, err
	}

	newXP, newLevel, levelsGained := applyCharacterXP(entry.XP, entry.Level, character.Rarity, amount)
	entry.XP = newXP
	entry.Level = newLevel
	player.Collection[entryIndex] = entry
	if _, err := updatePlayerCollection(playerID, player.Collection); err != nil {
		return nil, err
	}

	return &CharacterXPProgress{Entry: entry, LevelsGained: levelsGained}, nil
}

func applyCharacterXP(currentXP int64, currentLevel int16, rarity string, amount int64) (int64, int16, int16) {
	if currentLevel < 1 {
		currentLevel = 1
	}
	currentXP += amount
	startingLevel := currentLevel

	for currentLevel < 100 {
		requiredXP := characterXPForNextLevel(rarity, currentLevel)
		if currentXP < requiredXP {
			break
		}
		currentXP -= requiredXP
		currentLevel++
	}

	return currentXP, currentLevel, currentLevel - startingLevel
}

func characterXPForNextLevel(rarity string, currentLevel int16) int64 {
	baseXP := int64(100)
	switch strings.ToLower(strings.TrimSpace(rarity)) {
	case "uncommon":
		baseXP = 125
	case "rare":
		baseXP = 150
	case "epic":
		baseXP = 200
	case "legendary":
		baseXP = 300
	case "mythic":
		baseXP = 500
	}
	if currentLevel < 1 {
		currentLevel = 1
	}
	return baseXP * int64(currentLevel)
}

func containsIgnoreCase(str, substr string) bool {
	strLower := strings.ToLower(str)
	substrLower := strings.ToLower(substr)
	return strings.Contains(strLower, substrLower)
}

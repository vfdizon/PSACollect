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
	Characters []Character `json:"characters"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

func getPlayerCollection(playerID string) ([]CollectionEntry, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	return append([]CollectionEntry{}, player.Collection...), nil
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

	var matchingCharacters []Character
	for _, entry := range player.Collection {
		character, err := getCharacterByID(entry.CharacterID)
		if err != nil {
			return nil, err
		}

		if characterName == "" || containsIgnoreCase(character.Name, characterName) {
			matchingCharacters = append(matchingCharacters, *character)
		}
	}

	return &CharacterQueryResult{
		Characters: matchingCharacters,
		Total:      len(matchingCharacters),
		Page:       1,
		PageSize:   len(matchingCharacters),
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

func containsIgnoreCase(str, substr string) bool {
	strLower := strings.ToLower(str)
	substrLower := strings.ToLower(substr)
	return strings.Contains(strLower, substrLower)
}

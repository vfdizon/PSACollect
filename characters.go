package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	characterPageSize        = 10
	charactersOnce           sync.Once
	cachedCharacters         []Character
	cachedCharactersByID     map[string]Character
	cachedCharactersByRarity map[string][]Character
	cachedCharactersErr      error
)

const (
	CharacterTypeGoon     = "Goon"
	CharacterTypeHarana   = "Harana"
	CharacterTypeBarkada  = "Barkada"
	CharacterTypeLockedIn = "Locked In"
)

type Character struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ImagePath string `json:"image_path"`
	Type      string `json:"type"`
	Rarity    string `json:"rarity"`
	Toughness int    `json:"toughness"`
	Power     int    `json:"power"`
}

func getCharacterByID(id string) (*Character, error) {
	_, charactersByID, _, err := loadCharacters()
	if err != nil {
		return nil, err
	}

	character, exists := charactersByID[strings.TrimSpace(id)]
	if !exists {
		return nil, errors.New("character not found")
	}
	return &character, nil
}

func getRandomCharacter() (*Character, error) {
	characters, _, _, err := loadCharacters()
	if err != nil {
		return nil, err
	}

	if len(characters) == 0 {
		return nil, errors.New("no characters found")
	}

	character := characters[rand.Intn(len(characters))]
	return &character, nil
}

func getRandomWeightedCharacter() (*Character, error) {
	characters, _, charactersByRarity, err := loadCharacters()
	if err != nil {
		return nil, err
	}

	if len(characters) == 0 {
		return nil, errors.New("no characters found")
	}

	// Pick a rarity bucket first, then choose a random character from that bucket.
	// This keeps the distribution stable even as the number of characters in each
	// rarity changes.
	type rarityBucket struct {
		name   string
		weight float64
	}

	rarityWeights := []rarityBucket{
		{name: "Common", weight: 0.50},
		{name: "Uncommon", weight: 0.25},
		{name: "Rare", weight: 0.16},
		{name: "Epic", weight: 0.06},
		{name: "Legendary", weight: 0.02},
		{name: "Mythic", weight: 0.01},
	}

	totalWeight := 0.0
	for _, bucket := range rarityWeights {
		totalWeight += bucket.weight
	}

	roll := rand.Float64() * totalWeight
	selectedRarity := ""
	for _, bucket := range rarityWeights {
		if roll < bucket.weight {
			selectedRarity = bucket.name
			break
		}
		roll -= bucket.weight
	}

	if selectedRarity == "" {
		selectedRarity = "Common"
	}

	selectedCharacters := charactersByRarity[strings.ToLower(selectedRarity)]
	if len(selectedCharacters) == 0 {
		// Fall back to any character if the selected rarity is missing from the file.
		character := characters[rand.Intn(len(characters))]
		return &character, nil
	}

	character := selectedCharacters[rand.Intn(len(selectedCharacters))]
	return &character, nil
}

func getCharacterPages() ([][]Character, error) {
	characters, _, _, err := loadCharacters()
	if err != nil {
		return nil, err
	}

	var pages [][]Character
	for i := 0; i < len(characters); i += characterPageSize {
		end := i + characterPageSize
		if end > len(characters) {
			end = len(characters)
		}
		pages = append(pages, append([]Character(nil), characters[i:end]...))
	}

	return pages, nil
}

func loadCharacters() ([]Character, map[string]Character, map[string][]Character, error) {
	charactersOnce.Do(func() {
		charactersPath, err := charactersFilePath()
		if err != nil {
			cachedCharactersErr = fmt.Errorf("get characters file path: %w", err)
			return
		}

		data, err := os.ReadFile(charactersPath)
		if err != nil {
			cachedCharactersErr = fmt.Errorf("read characters file: %w", err)
			return
		}
		if err := json.Unmarshal(data, &cachedCharacters); err != nil {
			cachedCharactersErr = fmt.Errorf("parse characters file: %w", err)
			return
		}
		if len(cachedCharacters) == 0 {
			cachedCharactersErr = errors.New("no characters found")
			return
		}

		cachedCharactersByID = make(map[string]Character, len(cachedCharacters))
		cachedCharactersByRarity = make(map[string][]Character)
		for i := range cachedCharacters {
			character := &cachedCharacters[i]
			character.Type = canonicalCharacterType(character.Type)
			cachedCharactersByID[character.ID] = *character
			rarity := strings.ToLower(strings.TrimSpace(character.Rarity))
			cachedCharactersByRarity[rarity] = append(cachedCharactersByRarity[rarity], *character)
		}
	})

	return cachedCharacters, cachedCharactersByID, cachedCharactersByRarity, cachedCharactersErr
}

func canonicalCharacterType(characterType string) string {
	normalized := strings.ToLower(strings.TrimSpace(characterType))
	normalized = strings.Join(strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(normalized)), " ")

	switch normalized {
	case "chud", "goon":
		return CharacterTypeGoon
	case "harana":
		return CharacterTypeHarana
	case "barkada":
		return CharacterTypeBarkada
	case "lockin", "lock in", "locked in":
		return CharacterTypeLockedIn
	default:
		return strings.TrimSpace(characterType)
	}
}

func charactersFilePath() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		charactersPath := filepath.Join(workingDir, "config", "characters.json")
		if _, err := os.Stat(charactersPath); err == nil {
			return charactersPath, nil
		}

		parentDir := filepath.Dir(workingDir)
		if parentDir == workingDir {
			break
		}
		workingDir = parentDir
	}

	return "", fmt.Errorf("could not find config/characters.json from current working directory")
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

var (
	characterPageSize = 10
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
	// will search ../config/characters.json for the character with the given id and return it, if exists
	charactersPath, err := charactersFilePath()

	if err != nil {
		return nil, fmt.Errorf("get characters file path: %w", err)
	}

	data, err := os.ReadFile(charactersPath)
	if err != nil {
		return nil, fmt.Errorf("read characters file: %w", err)
	}

	var characters []Character
	if err := json.Unmarshal(data, &characters); err != nil {
		return nil, fmt.Errorf("parse characters file: %w", err)
	}

	for _, character := range characters {
		if character.ID == id {
			return &character, nil
		}
	}

	return nil, errors.New("character not found")
}

func getRandomCharacter() (*Character, error) {
	charactersPath, err := charactersFilePath()
	if err != nil {
		return nil, fmt.Errorf("get characters file path: %w", err)
	}

	data, err := os.ReadFile(charactersPath)
	if err != nil {
		return nil, fmt.Errorf("read characters file: %w", err)
	}

	var characters []Character
	if err := json.Unmarshal(data, &characters); err != nil {
		return nil, fmt.Errorf("parse characters file: %w", err)
	}

	if len(characters) == 0 {
		return nil, errors.New("no characters found")
	}

	randomIndex := rand.Intn(len(characters))
	return &characters[randomIndex], nil
}

func getRandomWeightedCharacter() (*Character, error) {
	charactersPath, err := charactersFilePath()
	if err != nil {
		return nil, fmt.Errorf("get characters file path: %w", err)
	}

	data, err := os.ReadFile(charactersPath)
	if err != nil {
		return nil, fmt.Errorf("read characters file: %w", err)
	}

	var characters []Character
	if err := json.Unmarshal(data, &characters); err != nil {
		return nil, fmt.Errorf("parse characters file: %w", err)
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
		{name: "Common", weight: 0.75},
		{name: "Uncommon", weight: 0.18},
		{name: "Rare", weight: 0.05},
		{name: "Epic", weight: 0.015},
		{name: "Legendary", weight: 0.004},
		{name: "Mythic", weight: 0.001},
	}

	charactersByRarity := make(map[string][]Character)
	for _, character := range characters {
		rarity := strings.TrimSpace(character.Rarity)
		charactersByRarity[rarity] = append(charactersByRarity[rarity], character)
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

	selectedCharacters := charactersByRarity[selectedRarity]
	if len(selectedCharacters) == 0 {
		// Fall back to any character if the selected rarity is missing from the file.
		randomIndex := rand.Intn(len(characters))
		return &characters[randomIndex], nil
	}

	randomIndex := rand.Intn(len(selectedCharacters))
	return &selectedCharacters[randomIndex], nil
}

func getCharacterPages() ([][]Character, error) {
	charactersPath, err := charactersFilePath()
	if err != nil {
		return nil, fmt.Errorf("get characters file path: %w", err)
	}

	data, err := os.ReadFile(charactersPath)
	if err != nil {
		return nil, fmt.Errorf("read characters file: %w", err)
	}

	var characters []Character
	if err := json.Unmarshal(data, &characters); err != nil {
		return nil, fmt.Errorf("parse characters file: %w", err)
	}

	var pages [][]Character
	for i := 0; i < len(characters); i += characterPageSize {
		end := i + characterPageSize
		if end > len(characters) {
			end = len(characters)
		}
		pages = append(pages, characters[i:end])
	}

	return pages, nil
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

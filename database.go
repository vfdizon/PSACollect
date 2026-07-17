package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var errPlayerNotFound = errors.New("player not found")
var errCollectionEntryNotFound = errors.New("collection entry not found")
var databaseHTTPClient = &http.Client{Timeout: 10 * time.Second}

type CollectionEntry struct {
	UUID        string `json:"uuid"`
	CharacterID string `json:"character_id"`
	XP          int64  `json:"xp"`
	Level       int16  `json:"level"`
}

type databaseConfig struct {
	playerTableURL string
	apiKey         string
}

func getPlayerByID(id string) (*Player, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("player id cannot be empty")
	}

	cfg, err := loadDatabaseConfig()
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(cfg.playerTableURL)
	if err != nil {
		return nil, fmt.Errorf("parse player table url: %w", err)
	}

	query := endpoint.Query()
	query.Set("select", "id,collection,level,xp")
	query.Set("id", "eq."+id)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build player query request: %w", err)
	}
	setDatabaseHeaders(request, cfg.apiKey)

	response, err := databaseHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query player: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, errPlayerNotFound
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("query player: unexpected status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var players []Player
	if err := json.NewDecoder(response.Body).Decode(&players); err != nil {
		return nil, fmt.Errorf("decode player query response: %w", err)
	}

	if len(players) == 0 {
		return nil, errPlayerNotFound
	}

	return &players[0], nil
}

func addPlayer(id string) (*Player, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("player id cannot be empty")
	}

	player, err := getPlayerByID(id)
	if err == nil {
		return player, nil
	}
	if !errors.Is(err, errPlayerNotFound) {
		return nil, err
	}

	return createPlayer(id)
}

func addCharacterToCollection(playerID, characterID string, xp int64, level int16) (*CollectionEntry, error) {
	player, err := ensurePlayer(playerID)
	if err != nil {
		return nil, err
	}

	entryUUID, err := newUUID()
	if err != nil {
		return nil, err
	}

	entry := CollectionEntry{
		UUID:        entryUUID,
		CharacterID: strings.TrimSpace(characterID),
		XP:          xp,
		Level:       level,
	}

	updatedCollection := append(append([]CollectionEntry{}, player.Collection...), entry)
	if _, err := updatePlayerCollection(playerID, updatedCollection); err != nil {
		return nil, err
	}

	return &entry, nil
}

func removeCharacterFromCollection(playerID, entryUUID string) (*CollectionEntry, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	entryIndex := -1
	for i, entry := range player.Collection {
		if entry.UUID == entryUUID {
			entryIndex = i
			break
		}
	}
	if entryIndex == -1 {
		return nil, errCollectionEntryNotFound
	}

	removedEntry := player.Collection[entryIndex]
	updatedCollection := append([]CollectionEntry{}, player.Collection[:entryIndex]...)
	updatedCollection = append(updatedCollection, player.Collection[entryIndex+1:]...)

	if _, err := updatePlayerCollection(playerID, updatedCollection); err != nil {
		return nil, err
	}

	return &removedEntry, nil
}

func transferCharacterBetweenPlayers(fromPlayerID, toPlayerID, entryUUID string) (*CollectionEntry, error) {
	fromPlayer, err := getPlayerByID(fromPlayerID)
	if err != nil {
		return nil, err
	}

	toPlayer, err := ensurePlayer(toPlayerID)
	if err != nil {
		return nil, err
	}

	entryIndex := -1
	for i, entry := range fromPlayer.Collection {
		if entry.UUID == entryUUID {
			entryIndex = i
			break
		}
	}
	if entryIndex == -1 {
		return nil, errCollectionEntryNotFound
	}

	entry := fromPlayer.Collection[entryIndex]
	updatedFromCollection := append([]CollectionEntry{}, fromPlayer.Collection[:entryIndex]...)
	updatedFromCollection = append(updatedFromCollection, fromPlayer.Collection[entryIndex+1:]...)

	if _, err := updatePlayerCollection(fromPlayerID, updatedFromCollection); err != nil {
		return nil, err
	}

	updatedToCollection := append(append([]CollectionEntry{}, toPlayer.Collection...), entry)
	if _, err := updatePlayerCollection(toPlayerID, updatedToCollection); err != nil {
		_, rollbackErr := updatePlayerCollection(fromPlayerID, fromPlayer.Collection)
		if rollbackErr != nil {
			return nil, fmt.Errorf("transfer character failed: %v (rollback also failed: %w)", err, rollbackErr)
		}
		return nil, fmt.Errorf("transfer character failed: %w", err)
	}

	return &entry, nil
}

func createPlayer(id string) (*Player, error) {
	cfg, err := loadDatabaseConfig()
	if err != nil {
		return nil, err
	}

	player := Player{
		ID:         id,
		Collection: []CollectionEntry{},
		Level:      1,
		XP:         0,
	}

	body, err := json.Marshal(player)
	if err != nil {
		return nil, fmt.Errorf("marshal player: %w", err)
	}

	endpoint, err := url.Parse(cfg.playerTableURL)
	if err != nil {
		return nil, fmt.Errorf("parse player table url: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "id,collection,level,xp")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build player create request: %w", err)
	}
	setDatabaseHeaders(request, cfg.apiKey)
	request.Header.Set("Prefer", "return=representation")

	response, err := databaseHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("create player: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("create player: unexpected status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var created []Player
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("decode player create response: %w", err)
	}

	if len(created) == 0 {
		return &player, nil
	}

	return &created[0], nil
}

func ensurePlayer(id string) (*Player, error) {
	return addPlayer(id)
}

func updatePlayerCollection(id string, collection []CollectionEntry) (*Player, error) {
	return updatePlayerFields(id, map[string]any{"collection": collection})
}

func awardPlayerXP(id string, amount int64) (*Player, error) {
	if amount < 0 {
		return nil, errors.New("XP award cannot be negative")
	}

	player, err := getPlayerByID(id)
	if err != nil {
		return nil, err
	}

	newXP := player.XP + amount
	newLevel := int16(1 + newXP/100)
	if newLevel < player.Level {
		newLevel = player.Level
	}

	return updatePlayerFields(id, map[string]any{
		"xp":    newXP,
		"level": newLevel,
	})
}

func updatePlayerFields(id string, fields map[string]any) (*Player, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("player id cannot be empty")
	}

	cfg, err := loadDatabaseConfig()
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("marshal player update: %w", err)
	}

	endpoint, err := url.Parse(cfg.playerTableURL)
	if err != nil {
		return nil, fmt.Errorf("parse player table url: %w", err)
	}

	query := endpoint.Query()
	query.Set("select", "id,collection,level,xp")
	query.Set("id", "eq."+id)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodPatch, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build player update request: %w", err)
	}
	setDatabaseHeaders(request, cfg.apiKey)
	request.Header.Set("Prefer", "return=representation")

	response, err := databaseHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("update player: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, errPlayerNotFound
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("update player: unexpected status %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	var updated []Player
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("decode player update response: %w", err)
	}
	if len(updated) == 0 {
		return getPlayerByID(id)
	}

	return &updated[0], nil
}

func loadDatabaseConfig() (databaseConfig, error) {
	playerTableURL := strings.TrimSpace(os.Getenv("DATABASE_PLAYER_TABLE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("DATABASE_API_KEY"))

	if playerTableURL == "" {
		return databaseConfig{}, errors.New("DATABASE_PLAYER_TABLE_URL is not set")
	}
	if apiKey == "" {
		return databaseConfig{}, errors.New("DATABASE_API_KEY is not set")
	}

	return databaseConfig{
		playerTableURL: playerTableURL,
		apiKey:         apiKey,
	}, nil
}

func setDatabaseHeaders(request *http.Request, apiKey string) {
	request.Header.Set("apikey", apiKey)
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
}

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := crand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:6]),
		hex.EncodeToString(bytes[6:8]),
		hex.EncodeToString(bytes[8:10]),
		hex.EncodeToString(bytes[10:16]),
	), nil
}

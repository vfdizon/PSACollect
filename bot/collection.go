package main

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

func getPlayerCollection(playerID string) ([]CollectionEntry, error) {
	player, err := getPlayerByID(playerID)
	if err != nil {
		return nil, err
	}

	return append([]CollectionEntry{}, player.Collection...), nil
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

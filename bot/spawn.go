package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	errNoActiveSpawn      = errors.New("no active spawn")
	errSpawnWrongChannel  = errors.New("spawn is in a different channel")
	errSpawnAlreadyCaught = errors.New("spawn has already been caught")
)

type spawnConfig struct {
	channelID       string
	minInterval     time.Duration
	maxInterval     time.Duration
	despawnInterval time.Duration
}

type liveSpawn struct {
	channelID       string
	messageID       string
	character       *Character
	despawnInterval time.Duration
	despawnTimer    *time.Timer
	done            chan struct{}
	once            sync.Once
	claimed         bool
}

func (spawn *liveSpawn) finish() {
	spawn.once.Do(func() {
		close(spawn.done)
	})
}

var spawnState = struct {
	sync.Mutex
	active *liveSpawn
}{}

func startCharacterSpawner(session *discordgo.Session) {
	cfg, err := loadSpawnConfig()
	if err != nil {
		log.Printf("character spawner disabled: %v", err)
		return
	}

	go func() {
		for {
			wait := randomDuration(cfg.minInterval, cfg.maxInterval)
			log.Printf("next character spawn in %s", wait)
			time.Sleep(wait)

			spawn, err := spawnWildCharacter(session, cfg)
			if err != nil {
				log.Printf("failed to spawn character: %v", err)
				continue
			}

			<-spawn.done
		}
	}()
}

func spawnWildCharacter(session *discordgo.Session, cfg spawnConfig) (*liveSpawn, error) {
	character, err := spawnCharacter()
	if err != nil {
		return nil, err
	}

	embed := &discordgo.MessageEmbed{
		Title: character.Name,
		Description: fmt.Sprintf(
			"A wild character appeared!\n\nType: %s\nRarity: %s\nToughness: %d\nPower: %d\n\nUse `?catch` to catch it before it despawns.",
			character.Type, character.Rarity, character.Toughness, character.Power,
		),
		Color: getRarityColor(character.Rarity),
	}

	message, err := session.ChannelMessageSendEmbed(cfg.channelID, embed)
	if err != nil {
		return nil, fmt.Errorf("send spawn message: %w", err)
	}

	spawn := &liveSpawn{
		channelID:       cfg.channelID,
		messageID:       message.ID,
		character:       character,
		despawnInterval: cfg.despawnInterval,
		despawnTimer:    time.NewTimer(cfg.despawnInterval),
		done:            make(chan struct{}),
	}

	spawnState.Lock()
	if spawnState.active != nil {
		spawnState.Unlock()
		return nil, errors.New("a character is already active")
	}
	spawnState.active = spawn
	spawnState.Unlock()

	go monitorSpawn(session, spawn)

	return spawn, nil
}

func monitorSpawn(session *discordgo.Session, spawn *liveSpawn) {
	select {
	case <-spawn.despawnTimer.C:
		spawnState.Lock()
		if spawnState.active == spawn && !spawn.claimed {
			spawnState.active = nil
		}
		spawnState.Unlock()

		if _, err := session.ChannelMessageSend(spawn.channelID, fmt.Sprintf("The wild %s has despawned!", spawn.character.Name)); err != nil {
			log.Printf("failed to send despawn message: %v", err)
		}

		spawn.finish()
	case <-spawn.done:
		return
	}
}

func catchSpawn(session *discordgo.Session, playerID, channelID string) (*CollectionEntry, *Character, error) {
	spawnState.Lock()
	spawn := spawnState.active
	if spawn == nil || spawn.messageID == "" {
		spawnState.Unlock()
		return nil, nil, errNoActiveSpawn
	}
	if spawn.channelID != channelID {
		spawnState.Unlock()
		return nil, nil, errSpawnWrongChannel
	}
	if spawn.claimed {
		spawnState.Unlock()
		return nil, nil, errSpawnAlreadyCaught
	}
	spawn.claimed = true
	if !spawn.despawnTimer.Stop() {
		select {
		case <-spawn.despawnTimer.C:
		default:
		}
	}
	spawnState.Unlock()

	entry, err := catchCharacter(playerID, spawn.character.ID, 0, 1)
	if err != nil {
		spawnState.Lock()
		if spawnState.active == spawn {
			spawn.claimed = false
			spawn.despawnTimer.Reset(spawn.despawnInterval)
		}
		spawnState.Unlock()
		return nil, nil, err
	}

	if err := session.ChannelMessageDelete(spawn.channelID, spawn.messageID); err != nil {
		log.Printf("failed to delete caught spawn message: %v", err)
	}

	spawnState.Lock()
	if spawnState.active == spawn {
		spawnState.active = nil
	}
	spawnState.Unlock()
	spawn.finish()

	return entry, spawn.character, nil
}

func loadSpawnConfig() (spawnConfig, error) {
	channelID := strings.TrimSpace(os.Getenv("SPAWN_CHANNEL_ID"))
	if channelID == "" {
		return spawnConfig{}, errors.New("SPAWN_CHANNEL_ID is not set")
	}

	minInterval, err := parseDurationEnv("SPAWN_MIN_INTERVAL", 1*time.Minute)
	if err != nil {
		return spawnConfig{}, err
	}
	maxInterval, err := parseDurationEnv("SPAWN_MAX_INTERVAL", 10*time.Minute)
	if err != nil {
		return spawnConfig{}, err
	}
	despawnInterval, err := parseDurationEnv("SPAWN_DESPAWN_INTERVAL", 5*time.Minute)
	if err != nil {
		return spawnConfig{}, err
	}

	if minInterval <= 0 {
		return spawnConfig{}, errors.New("SPAWN_MIN_INTERVAL must be greater than zero")
	}
	if maxInterval < minInterval {
		return spawnConfig{}, errors.New("SPAWN_MAX_INTERVAL must be greater than or equal to SPAWN_MIN_INTERVAL")
	}
	if despawnInterval <= 0 {
		return spawnConfig{}, errors.New("SPAWN_DESPAWN_INTERVAL must be greater than zero")
	}

	return spawnConfig{
		channelID:       channelID,
		minInterval:     minInterval,
		maxInterval:     maxInterval,
		despawnInterval: despawnInterval,
	}, nil
}

func parseDurationEnv(name string, defaultValue time.Duration) (time.Duration, error) {
	rawValue := strings.TrimSpace(os.Getenv(name))
	if rawValue == "" {
		return defaultValue, nil
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return value, nil
}

func randomDuration(minValue, maxValue time.Duration) time.Duration {
	if maxValue <= minValue {
		return minValue
	}

	delta := maxValue - minValue
	return minValue + time.Duration(rand.Int63n(int64(delta)+1))
}

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	Name        string
	Description string
	Handler     func(s *discordgo.Session, m *discordgo.MessageCreate)
}

type ResponseWaiter struct {
	Handler func(s *discordgo.Session, m *discordgo.MessageCreate)
}

func (rw *ResponseWaiter) WaitForResponse(s *discordgo.Session, m *discordgo.MessageCreate) {
	// this function will block until a response is received or a timeout occurs, it will call the Handler function with the session and message when a response is received
	// the key for activeResponseWaiters will be userID:channelID, so we can have multiple waiters for different users and channels without conflict
	key := fmt.Sprintf("%s:%s", m.Author.ID, m.ChannelID)
	responseWaitersMu.Lock()
	activeResponseWaiters[key] = rw
	responseWaitersMu.Unlock()

	// set up a timeout to remove the waiter after 60 seconds
	time.AfterFunc(60*time.Second, func() {
		responseWaitersMu.Lock()
		if activeResponseWaiters[key] == rw {
			delete(activeResponseWaiters, key)
		}
		responseWaitersMu.Unlock()
	})
}

func listenForResponses(s *discordgo.Session) {
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		key := fmt.Sprintf("%s:%s", m.Author.ID, m.ChannelID)
		responseWaitersMu.Lock()
		rw, exists := activeResponseWaiters[key]
		if exists {
			delete(activeResponseWaiters, key)
		}
		responseWaitersMu.Unlock()
		if exists {
			rw.Handler(s, m)
		}
	})
}

func listenForCommands(s *discordgo.Session) {
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		fields := strings.Fields(strings.TrimSpace(m.Content))
		if len(fields) == 0 || !strings.HasPrefix(fields[0], prefix) {
			return
		}
		commandName := strings.TrimPrefix(fields[0], prefix)

		for _, cmd := range allowedCommands {
			if strings.EqualFold(commandName, cmd.Name) {
				go cmd.Handler(s, m)
				return
			}
		}

		if _, err := s.ChannelMessageSend(m.ChannelID, "Unknown command. Type ?help for a list of commands."); err != nil {
			log.Printf("failed to send unknown command response: %v", err)
		}
	})
}

var (
	prefix                = "?"
	responseWaitersMu     sync.Mutex
	activeResponseWaiters = make(map[string]*ResponseWaiter) // key will be userID:channelID
	allowedCommands       = []Command{
		{
			Name:        "ping",
			Description: "Responds with 'abc!' to test bot responsiveness.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				if _, err := s.ChannelMessageSend(m.ChannelID, "abc!"); err != nil {
					log.Printf("failed to send ping response: %v", err)
				}
			},
		},
		{
			Name:        "help",
			Description: "Lists available commands.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				helpText := "You on your own homeboy 😂"

				if _, err := s.ChannelMessageSend(m.ChannelID, helpText); err != nil {
					log.Printf("failed to send help response: %v", err)
				}
			},
		},
		{
			Name:        "getcharacter",
			Description: "Fetches character information by ID. Usage: ?getcharacter <character_id>",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				parts := strings.Fields(m.Content)
				if len(parts) != 2 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Usage: ?getcharacter <character_id>"); err != nil {
						log.Printf("failed to send getcharacter usage response: %v", err)
					}
					return
				}

				characterID := parts[1]
				character, err := getCharacterByID(characterID)
				if err != nil {
					log.Printf("error fetching character by ID: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Character not found."); err != nil {
						log.Printf("failed to send character not found response: %v", err)
					}
					return

				}

				//returns a discord embed with the character information
				// based on rarity, the color will change gray common, green uncommon, blue rare, purple epic, orange legendary, pink mythic

				embed := &discordgo.MessageEmbed{
					Title: character.Name,
					Description: fmt.Sprintf("Type: %s\nRarity: %s\nToughness: %d\nPower: %d",
						character.Type, character.Rarity, character.Toughness, character.Power),
					Color: getRarityColor(character.Rarity),
					// Image: &discordgo.MessageEmbedImage{
					// URL: character.ImagePath,
					// },
				}
				if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
					log.Printf("failed to send character information response: %v", err)
				}
			},
		},

		{
			Name:        "getrandomcharacter",
			Description: "Fetches a random character.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				character, err := getRandomCharacter()
				if err != nil {
					log.Printf("error fetching random character: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch a random character."); err != nil {
						log.Printf("failed to send random character error response: %v", err)
					}
					return
				}

				embed := &discordgo.MessageEmbed{
					Title: character.Name,
					Description: fmt.Sprintf("Type: %s\nRarity: %s\nToughness: %d\nPower: %d",
						character.Type, character.Rarity, character.Toughness, character.Power),
					Color: getRarityColor(character.Rarity),
					// Image: &discordgo.MessageEmbedImage{
					// URL: character.ImagePath,
					// },
				}
				if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
					log.Printf("failed to send random character information response: %v", err)
				}
			},
		},
		{
			Name:        "characterpages",
			Description: "Fetches characters in pages of 10.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				characterPages, err := getCharacterPages()
				if err != nil {
					log.Printf("error fetching character pages: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch character pages."); err != nil {
						log.Printf("failed to send character pages error response: %v", err)
					}
					return
				}

				var embeds []*discordgo.MessageEmbed
				for _, page := range characterPages {
					description := ""
					for _, character := range page {
						description += fmt.Sprintf("**%s** (ID: %s)\nType: %s | Rarity: %s | Toughness: %d | Power: %d\n\n",
							character.Name, character.ID, character.Type, character.Rarity, character.Toughness, character.Power)
					}

					embed := &discordgo.MessageEmbed{
						Title:       "Characters",
						Description: description,
						Color:       0x00FF00,
					}
					embeds = append(embeds, embed)
				}

				createEmbedMenu(s, m.ChannelID, embeds)
			},
		},
		{
			Name:        "getrandomweighted",
			Description: "Fetches a random character with weighted probabilities based on rarity.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				superAdminID := os.Getenv("SUPER_ADMIN_ID")

				// check if super admin
				if m.Author.ID != superAdminID {
					log.Printf("user %s attempted to use getrandomweighted command without permission", m.Author.ID)
					if _, err := s.ChannelMessageSend(m.ChannelID, "You do not have permission to use this command."); err != nil {
						log.Printf("failed to send permission denied response: %v", err)
					}
					return
				}
				character, err := getRandomWeightedCharacter()
				if err != nil {
					log.Printf("error fetching random weighted character: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch a random weighted character."); err != nil {
						log.Printf("failed to send random weighted character error response: %v", err)
					}
					return
				}

				embed := &discordgo.MessageEmbed{
					Title: character.Name,
					Description: fmt.Sprintf("Type: %s\nRarity: %s\nToughness: %d\nPower: %d",
						character.Type, character.Rarity, character.Toughness, character.Power),
					Color: getRarityColor(character.Rarity),
					// Image: &discordgo.MessageEmbedImage{
					// URL: character.ImagePath,
					// },
				}
				if _, err := s.ChannelMessageSendEmbed(m.ChannelID, embed); err != nil {
					log.Printf("failed to send random weighted character information response: %v", err)
				}
			},
		},

		// PLAYER COMMANDS
		{
			Name:        "register",
			Description: "Registers the user as a player in the database.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				playerID := m.Author.ID
				_, err := ensurePlayer(playerID)
				if err != nil {
					log.Printf("error registering player: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to register player."); err != nil {
						log.Printf("failed to send player registration error response: %v", err)
					}
					return
				}

				if _, err := s.ChannelMessageSend(m.ChannelID, "You have been registered as a player!"); err != nil {
					log.Printf("failed to send player registration success response: %v", err)
				}
			},
		},
		{
			Name:        "catch",
			Description: "Catches the currently spawned character.",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				entry, character, err := catchSpawn(s, m.Author.ID, m.ChannelID)
				if err != nil {
					log.Printf("failed to catch spawned character: %v", err)

					message := "There is no active character to catch right now."
					switch {
					case errors.Is(err, errSpawnWrongChannel):
						message = "There is an active character, but it spawned in a different channel."
					case errors.Is(err, errSpawnAlreadyCaught):
						message = "That character was already caught."
					}

					if _, sendErr := s.ChannelMessageSend(m.ChannelID, message); sendErr != nil {
						log.Printf("failed to send catch failure response: %v", sendErr)
					}
					return
				}

				if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You caught %s! Collection entry UUID: %s", character.Name, entry.UUID)); err != nil {
					log.Printf("failed to send catch success response: %v", err)
				}
			},
		},
		{
			Name:        "mycollection",
			Description: "Displays the player's collection of characters.",
			//displays the player's collection of characters in pages of 5 displays it as an embed menu
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				collectionEntries, err := getPlayerCollection(m.Author.ID)
				if err != nil {
					log.Printf("error fetching player collection: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch your collection."); err != nil {
						log.Printf("failed to send collection fetch error response: %v", err)
					}
					return
				}

				if len(collectionEntries) == 0 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Your collection is empty. Catch some characters to fill it up!"); err != nil {
						log.Printf("failed to send empty collection response: %v", err)
					}
					return
				}

				var embeds []*discordgo.MessageEmbed
				for i := 0; i < len(collectionEntries); i += playerCollectionPageSize {
					end := i + playerCollectionPageSize
					if end > len(collectionEntries) {
						end = len(collectionEntries)
					}

					description := ""
					for _, entry := range collectionEntries[i:end] {
						character, err := getCharacterByID(entry.CharacterID)
						if err != nil {
							log.Printf("error fetching character for collection entry: %v", err)
							continue
						}

						// include rarity, toughness, power, level, xp, character id, and entry uuid in the description
						description += fmt.Sprintf("**%s** (ID: %s)\nRarity: %s | Toughness: %d | Power: %d | Level: %d | XP: %d\nEntry UUID: %s\n\n",
							character.Name, character.ID, character.Rarity, character.Toughness, character.Power, entry.Level, entry.XP, entry.UUID)
					}

					embed := &discordgo.MessageEmbed{
						Title:       "Your Collection",
						Description: description,
						Color:       0x00FF00,
					}
					embeds = append(embeds, embed)
				}

				createEmbedMenu(s, m.ChannelID, embeds)
			},
		},
		{
			Name:        "release",
			Description: "Releases a character from the player's collection. Usage: ?release <character_name>",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				parts := strings.Fields(m.Content)
				if len(parts) < 2 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Usage: ?release <character_name>"); err != nil {
						log.Printf("failed to send release usage response: %v", err)
					}
					return
				}

				characterName := strings.Join(parts[1:], " ")

				queryResult, err := queryPlayerCharacters(m.Author.ID, characterName)
				if err != nil {
					log.Printf("error querying player collection by name: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to query your collection."); err != nil {
						log.Printf("failed to send collection query error response: %v", err)
					}
					return
				}

				if len(queryResult.indivudalCharacters) == 0 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "No characters found in your collection matching that name."); err != nil {
						log.Printf("failed to send no characters found response: %v", err)
					}
					return
				}

				// if multiple characters, create embed menu to select which one, include index and uuid. they specify the index to remove
				if len(queryResult.indivudalCharacters) > 1 {
					var embeds []*discordgo.MessageEmbed
					description := ""
					for i, indivChar := range queryResult.indivudalCharacters {
						description += fmt.Sprintf("**%d. %s** (ID: %s)\nRarity: %s | Toughness: %d | Power: %d | Level: %d | XP: %d\nEntry UUID: %s\n\n",
							i+1, indivChar.CharacterInfo.Name, indivChar.CharacterInfo.ID, indivChar.CharacterInfo.Rarity, indivChar.CharacterInfo.Toughness, indivChar.CharacterInfo.Power, indivChar.Level, indivChar.Experience, indivChar.UUID)
					}

					embed := &discordgo.MessageEmbed{
						Title:       "Select a Character to Release",
						Description: description,
						Color:       0xFF0000,
					}
					embeds = append(embeds, embed)

					createEmbedMenu(s, m.ChannelID, embeds)

					responseWaiter := &ResponseWaiter{
						Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
							selection, err := strconv.Atoi(strings.TrimSpace(m.Content))
							if err != nil || selection < 1 || selection > len(queryResult.indivudalCharacters) {
								if _, err := s.ChannelMessageSend(m.ChannelID, "Invalid selection. Please enter the number corresponding to the character you want to release."); err != nil {
									log.Printf("failed to send invalid selection response: %v", err)
								}
								return
							}

							charToRelease := queryResult.indivudalCharacters[selection-1]
							_, err = releaseCharacter(m.Author.ID, charToRelease.UUID)
							if err != nil {
								log.Printf("error releasing character: %v", err)
								if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to release the character."); err != nil {
									log.Printf("failed to send character release error response: %v", err)
								}
								return
							}

							if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You have released %s from your collection.", charToRelease.CharacterInfo.Name)); err != nil {
								log.Printf("failed to send character release success response: %v", err)
							}
						},
					}
					responseWaiter.WaitForResponse(s, m)
					return
				}

				// if only one character found, release it
				charToRelease := queryResult.indivudalCharacters[0]
				_, err = releaseCharacter(m.Author.ID, charToRelease.UUID)
				if err != nil {
					log.Printf("error releasing character: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to release the character."); err != nil {
						log.Printf("failed to send character release error response: %v", err)
					}
					return
				}

				if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("You have released %s from your collection.", charToRelease.CharacterInfo.Name)); err != nil {
					log.Printf("failed to send character release success response: %v", err)
				}
			},
		},
		{
			Name:        "getcollection",
			Description: "Fetches a player's collection of characters by player ID. Usage: ?getcollection @<player_mention>",
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				if len(m.Mentions) != 1 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Usage: ?getcollection @<player_mention>"); err != nil {
						log.Printf("failed to send getcollection usage response: %v", err)
					}
					return
				}

				playerID := m.Mentions[0].ID
				collectionEntries, err := getPlayerCollection(playerID)
				if err != nil {
					log.Printf("error fetching player collection: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch the player's collection."); err != nil {
						log.Printf("failed to send getcollection error response: %v", err)
					}
					return
				}

				if len(collectionEntries) == 0 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "The player's collection is empty."); err != nil {
						log.Printf("failed to send empty collection response: %v", err)
					}
					return
				}

				var embeds []*discordgo.MessageEmbed
				for i := 0; i < len(collectionEntries); i += playerCollectionPageSize {
					end := i + playerCollectionPageSize
					if end > len(collectionEntries) {
						end = len(collectionEntries)
					}

					description := ""
					for _, entry := range collectionEntries[i:end] {
						character, err := getCharacterByID(entry.CharacterID)
						if err != nil {
							log.Printf("error fetching character for collection entry: %v", err)
							continue
						}

						description += fmt.Sprintf("**%s** (ID: %s)\nRarity: %s | Toughness: %d | Power: %d | Level: %d | XP: %d\nEntry UUID: %s\n\n",
							character.Name, character.ID, character.Rarity, character.Toughness, character.Power, entry.Level, entry.XP, entry.UUID)
					}

					embed := &discordgo.MessageEmbed{
						Title:       fmt.Sprintf("%s's Collection", m.Mentions[0].Username),
						Description: description,
						Color:       0x00FF00,
					}
					embeds = append(embeds, embed)
				}

				createEmbedMenu(s, m.ChannelID, embeds)
			},
		},
		{
			Name:        "battle",
			Description: "Initiates a battle between you and another player. Usage: ?battle @<opponent_mention>",
			Handler:     handleBattleChallenge,
		},
	}
)

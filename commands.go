package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	Name           string
	Description    string
	Handler        func(s *discordgo.Session, m *discordgo.MessageCreate)
	ResponseWaiter func(s *discordgo.Session, m *discordgo.MessageCreate)
}

type ResponseWaiter struct {
	Handler  func(s *discordgo.Session, m *discordgo.MessageCreate)
	Channels []bool // if empty, accept from any channel, otherwise only accept from channels at the specified indices in the command content split by spaces, for example if the command is "?example arg1 arg2 arg3" and Channels is [1, 3], then it will only accept responses from the same channel as the command (index 0) and the channel with ID equal to arg3 (index 3)
}

func (rw *ResponseWaiter) WaitForResponse(s *discordgo.Session, m *discordgo.MessageCreate) {
	// this function will block until a response is received or a timeout occurs, it will call the Handler function with the session and message when a response is received
	// the key for activeResponseWaiters will be userID:channelID, so we can have multiple waiters for different users and channels without conflict
	key := fmt.Sprintf("%s:%s", m.Author.ID, m.ChannelID)
	activeResponseWaiters[key] = rw

	// set up a timeout to remove the waiter after 60 seconds
	go func() {
		<-time.After(60 * time.Second)
		delete(activeResponseWaiters, key)
	}()
}

func (rw *ResponseWaiter) WaitForResponseFromUser(s *discordgo.Session, m *discordgo.MessageCreate, userID string) {
	key := fmt.Sprintf("%s:%s", userID, m.ChannelID)
	activeResponseWaiters[key] = rw

	// set up a timeout to remove the waiter after 60 seconds
	go func() {
		<-time.After(60 * time.Second)
		delete(activeResponseWaiters, key)
	}()
}

func listenForResponses(s *discordgo.Session) {
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		key := fmt.Sprintf("%s:%s", m.Author.ID, m.ChannelID)
		if rw, exists := activeResponseWaiters[key]; exists {
			rw.Handler(s, m)
			delete(activeResponseWaiters, key)
		}
	})
}

func listenForCommands(s *discordgo.Session) {
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		content := strings.TrimSpace(m.Content)

		if content == "" || !strings.HasPrefix(content, prefix) {
			return
		}

		// allow only the first word to be the command, the rest will be arguments
		for _, cmd := range allowedCommands {
			if strings.HasPrefix(content, prefix+cmd.Name) {
				go cmd.Handler(s, m)
				if cmd.ResponseWaiter != nil {
					rw := &ResponseWaiter{
						Handler: cmd.ResponseWaiter,
					}
					rw.WaitForResponse(s, m)
				}
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
			Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
				if len(m.Mentions) != 1 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Usage: ?battle @<opponent_mention>"); err != nil {
						log.Printf("failed to send battle usage response: %v", err)
					}
					return
				}

				opponentID := m.Mentions[0].ID
				if opponentID == m.Author.ID {
					if _, err := s.ChannelMessageSend(m.ChannelID, "You cannot battle yourself!"); err != nil {
						log.Printf("failed to send self battle response: %v", err)
					}
					return
				}

				// see if user has at least 1 character in collection
				collectionEntries, err := getPlayerCollection(m.Author.ID)
				if err != nil {
					log.Printf("error fetching player collection: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch your collection."); err != nil {
						log.Printf("failed to send collection fetch error response: %v", err)
					}
					return
				}

				if len(collectionEntries) == 0 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "You need at least one character in your collection to battle. Catch some characters and try again!"); err != nil {
						log.Printf("failed to send no characters for battle response: %v", err)
					}
					return
				}

				// see if opponent has at least 1 character in collection
				opponentCollectionEntries, err := getPlayerCollection(opponentID)
				if err != nil {
					log.Printf("error fetching opponent collection: %v", err)
					if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch your opponent's collection."); err != nil {
						log.Printf("failed to send opponent collection fetch error response: %v", err)
					}
					return
				}

				if len(opponentCollectionEntries) == 0 {
					if _, err := s.ChannelMessageSend(m.ChannelID, "Your opponent does not have any characters in their collection. They need to catch some characters before they can battle!"); err != nil {
						log.Printf("failed to send opponent no characters for battle response: %v", err)
					}
					return
				}

				// ask user if they accept the battle challenge
				if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s, do you accept the battle challenge from %s? Type 'yes' to accept.", m.Mentions[0].Username, m.Author.Username)); err != nil {
					log.Printf("failed to send battle challenge response: %v", err)
					return
				}

				responseWaiter := &ResponseWaiter{
					Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
						if strings.ToLower(strings.TrimSpace(m.Content)) != "yes" {
							if _, err := s.ChannelMessageSend(m.ChannelID, "Battle challenge declined."); err != nil {
								log.Printf("failed to send battle decline response: %v", err)
							}
							return
						}

						handleBattle(s, m, m.Author.ID, opponentID)
					},
					Channels: []bool{true, false}, // only accept response in the same channel as the command
				}
				responseWaiter.WaitForResponseFromUser(s, m, opponentID)

			},
		},
	}
)

func handleBattle(s *discordgo.Session, m *discordgo.MessageCreate, userID, opponentID string) {
	_, err := s.ChannelMessageSend(m.ChannelID, "Battle accepted! Preparing the battlefield...")
	if err != nil {
		log.Printf("failed to send battle preparation response: %v", err)
		return
	}

	player1Ready := make(chan bool)
	player2Ready := make(chan bool)

	player1Character := make(chan *IndividalCharacter)
	player2Character := make(chan *IndividalCharacter)

	go characterSelection(s, m, userID, player1Ready, player1Character)

	go characterSelection(s, m, opponentID, player2Ready, player2Character)

	startTime := time.Now()
	for {
		if time.Since(startTime) > 5*time.Minute {
			if _, err := s.ChannelMessageSend(m.ChannelID, "Battle timed out due to inactivity."); err != nil {
				log.Printf("failed to send battle timeout response: %v", err)
			}
			return
		}

		if <-player1Ready {
			if _, err := s.ChannelMessageSend(m.ChannelID, "Player 1 is ready!"); err != nil {
				log.Printf("failed to send player 1 ready response: %v", err)
			}
		}

		if <-player2Ready {
			if _, err := s.ChannelMessageSend(m.ChannelID, "Player 2 is ready!"); err != nil {
				log.Printf("failed to send player 2 ready response: %v", err)
			}
		}

		if <-player1Ready && <-player2Ready {
			break
		}
	}

	IndividualCharacter1 := <-player1Character
	IndividualCharacter2 := <-player2Character

	// for now, just compare power and bigger power = winner

	var winner string
	if IndividualCharacter1.CharacterInfo.Power > IndividualCharacter2.CharacterInfo.Power {
		winner = "Player 1"
	} else if IndividualCharacter2.CharacterInfo.Power > IndividualCharacter1.CharacterInfo.Power {
		winner = "Player 2"
	} else {
		winner = "It's a tie!"
	}

	if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The battle is over! %s wins!", winner)); err != nil {
		log.Printf("failed to send battle result response: %v", err)
	}

}

func characterSelection(s *discordgo.Session, m *discordgo.MessageCreate, userID string, readyChan chan<- bool, characterChan chan<- *IndividalCharacter) {
	collectionEntries, err := getPlayerCollection(userID)
	if err != nil {
		log.Printf("error fetching player collection for character selection: %v", err)
		if _, err := s.ChannelMessageSend(m.ChannelID, "Failed to fetch your collection for character selection."); err != nil {
			log.Printf("failed to send character selection collection fetch error response: %v", err)
		}
		return
	}

	s.ChannelMessageSend(m.ChannelID, "DEBUG: Fetched player collection for character selection")

	responseWaiter := &ResponseWaiter{
		Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {
			s.ChannelMessageSend(m.ChannelID, "DEBUG: Inside character selection response waiter handler")
			messageContent := strings.TrimSpace(m.Content)

			//check if their message is a name contained in their collection, if so, add the existing values to an array
			var matchingCharacters []IndividalCharacter
			for _, entry := range collectionEntries {
				character, err := getCharacterByID(entry.CharacterID)
				if err != nil {
					log.Printf("error fetching character for character selection: %v", err)
					continue
				}

				if containsIgnoreCase(character.Name, messageContent) {
					matchingCharacters = append(matchingCharacters, IndividalCharacter{
						CharacterInfo: *character,
						Level:         int(entry.Level),
						Experience:    int(entry.XP),
						UUID:          entry.UUID,
					})
				}
			}

			if len(matchingCharacters) == 0 {
				if _, err := s.ChannelMessageSend(m.ChannelID, "No characters found in your collection matching that name. Please enter the name of the character you want to use for the battle."); err != nil {
					log.Printf("failed to send no matching characters response: %v", err)
				}
			} else if len(matchingCharacters) == 1 {
				characterChan <- &matchingCharacters[0]
				readyChan <- true
			} else {
				var embeds []*discordgo.MessageEmbed
				description := ""
				for i, indivChar := range matchingCharacters {
					description += fmt.Sprintf("**%d. %s** (ID: %s)\nRarity: %s | Toughness: %d | Power: %d | Level: %d | XP: %d\nEntry UUID: %s\n\n",
						i+1, indivChar.CharacterInfo.Name, indivChar.CharacterInfo.ID, indivChar.CharacterInfo.Rarity, indivChar.CharacterInfo.Toughness, indivChar.CharacterInfo.Power, indivChar.Level, indivChar.Experience, indivChar.UUID)
				}

				embed := &discordgo.MessageEmbed{
					Title:       "Select a Character for Battle",
					Description: description,
					Color:       0x0000FF,
				}
				embeds = append(embeds, embed)

				createEmbedMenu(s, m.ChannelID, embeds)

				insideResponseWaiter := &ResponseWaiter{
					Handler: func(s *discordgo.Session, m *discordgo.MessageCreate) {

						s.ChannelMessageSend(m.ChannelID, "DEBUG: Inside character selection response waiter handler")

						selection, err := strconv.Atoi(strings.TrimSpace(m.Content))
						if err != nil || selection < 1 || selection > len(matchingCharacters) {
							if _, err := s.ChannelMessageSend(m.ChannelID, "Invalid selection. Please enter the number corresponding to the character you want to use for battle."); err != nil {
								log.Printf("failed to send invalid character selection response: %v", err)
							}
							return
						}

						characterChan <- &matchingCharacters[selection-1]
						readyChan <- true
					},
					Channels: []bool{true}, // only accept response in the same channel as the command
				}

				insideResponseWaiter.WaitForResponse(s, m)
			}
		},
		Channels: []bool{true}, // only accept response in the same channel as the command
	}

	responseWaiter.WaitForResponseFromUser(s, m, userID)
	//

}

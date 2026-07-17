package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: could not load .env file: %v", err)
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is not set")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("failed to create Discord session: %v", err)
	}
	defer dg.Close()

	dg.Identify.Intents = discordgo.IntentsGuilds
	dg.Identify.Intents |= discordgo.IntentsGuildMessages
	dg.Identify.Intents |= discordgo.IntentsGuildMessageReactions
	dg.Identify.Intents |= discordgo.IntentsMessageContent
	dg.Identify.Intents |= discordgo.IntentsDirectMessages

	listenForCommands(dg)
	listenForResponses(dg)

	if err := dg.Open(); err != nil {
		log.Fatalf("failed to connect to Discord gateway: %v", err)
	}

	startCharacterSpawner(dg)

	log.Println("Bot is connected. Press Ctrl+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down bot connection.")
}

// a menu that takes in a list of embeds and allows the user to navigate through them using reactions, the menu will automatically delete itself after 5 minutes of inactivity
func createEmbedMenu(s *discordgo.Session, channelID string, embeds []*discordgo.MessageEmbed) {
	if len(embeds) == 0 {
		return
	}

	currentIndex := 0
	timeout := time.NewTimer(5 * time.Minute)

	message, err := s.ChannelMessageSendEmbed(channelID, embeds[currentIndex])
	if err != nil {
		timeout.Stop()
		log.Printf("failed to send embed menu: %v", err)
		return
	}

	if len(embeds) > 1 {
		if err := s.MessageReactionAdd(channelID, message.ID, "⬅️"); err != nil {
			log.Printf("failed to add left reaction: %v", err)
		}
		if err := s.MessageReactionAdd(channelID, message.ID, "➡️"); err != nil {
			log.Printf("failed to add right reaction: %v", err)
		}
	}

	reactionChan := make(chan *discordgo.MessageReactionAdd, 8)
	removeHandler := func() {}

	if len(embeds) > 1 {
		removeHandler = s.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
			if r.ChannelID == channelID && r.MessageID == message.ID && r.UserID != s.State.User.ID && (r.Emoji.Name == "⬅️" || r.Emoji.Name == "➡️") {
				select {
				case reactionChan <- r:
				default:
				}
			}
		})
	}

	resetTimeout := func() {
		if !timeout.Stop() {
			select {
			case <-timeout.C:
			default:
			}
		}
		timeout.Reset(5 * time.Minute)
	}

	go func() {
		defer timeout.Stop()
		defer removeHandler()
		for {
			select {
			case reaction := <-reactionChan:
				resetTimeout()
				if reaction.Emoji.Name == "⬅️" {
					currentIndex = (currentIndex - 1 + len(embeds)) % len(embeds)
					if err := s.MessageReactionRemove(channelID, message.ID, "⬅️", reaction.UserID); err != nil {
						log.Printf("failed to remove left reaction: %v", err)
					}
				} else if reaction.Emoji.Name == "➡️" {
					currentIndex = (currentIndex + 1) % len(embeds)
					if err := s.MessageReactionRemove(channelID, message.ID, "➡️", reaction.UserID); err != nil {
						log.Printf("failed to remove right reaction: %v", err)
					}
				}

				if _, err := s.ChannelMessageEditEmbed(channelID, message.ID, embeds[currentIndex]); err != nil {
					log.Printf("failed to edit embed menu: %v", err)
				}
			case <-timeout.C:
				if err := s.ChannelMessageDelete(channelID, message.ID); err != nil {
					log.Printf("failed to delete embed menu after timeout: %v", err)
				}
				return
			}
		}
	}()
}

func getRarityColor(rarity string) int {
	switch strings.ToLower(strings.TrimSpace(rarity)) {
	case "common":
		return 0x808080 // gray
	case "uncommon":
		return 0x008000 // green
	case "rare":
		return 0x0000FF // blue
	case "epic":
		return 0x800080 // purple
	case "legendary":
		return 0xFFA500 // orange
	case "mythic":
		return 0xFFC0CB // pink
	default:
		return 0xFFFFFF // white for unknown rarity
	}
}

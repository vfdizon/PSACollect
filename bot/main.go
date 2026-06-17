package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

type Command struct {
	Name        string
	Description string
	Handler     func(s *discordgo.Session, m *discordgo.MessageCreate)
}

var (
	prefix          = "?"
	allowedCommands = []Command{
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
				if _, err := s.ChannelMessageSend(m.ChannelID, "Available commands: ?ping, ?help"); err != nil {
					log.Printf("failed to send help response: %v", err)
				}
			},
		},
	}
)

func listenForCommands(s *discordgo.Session) {
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		content := strings.TrimSpace(m.Content)

		if content == "" || !strings.HasPrefix(content, prefix) {
			return
		}

		for _, cmd := range allowedCommands {
			if content == prefix+cmd.Name {
				cmd.Handler(s, m)
				return
			}
		}

		if _, err := s.ChannelMessageSend(m.ChannelID, "Unknown command. Type ?help for a list of commands."); err != nil {
			log.Printf("failed to send unknown command response: %v", err)
		}
	})
}

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
	dg.Identify.Intents |= discordgo.IntentsMessageContent

	listenForCommands(dg)

	if err := dg.Open(); err != nil {
		log.Fatalf("failed to connect to Discord gateway: %v", err)
	}

	log.Println("Bot is connected. Press Ctrl+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down bot connection.")
}

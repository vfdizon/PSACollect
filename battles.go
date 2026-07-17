package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	battleChallengeTimeout = 60 * time.Second
	battleSelectionTimeout = 2 * time.Minute
	battleTurnTimeout      = 60 * time.Second
	battleWinnerBaseXP     = int64(25)
	battleLoserXP          = int64(10)
	levelDisadvantageXP    = int64(10)
	typeDisadvantageXP     = int64(15)
)

var (
	errBattleTimedOut = errors.New("battle timed out")
	activeBattlesMu   sync.Mutex
	activeBattleUsers = make(map[string]struct{})
)

type RarityMultiplier struct {
	Rarity     string
	Multiplier float64
}

type Effect struct {
	Name           string
	Description    string
	turnsRemaining int
	effectFunc     func(*FightingCharacter, *FightingCharacter)
}

type FightingCharacter struct {
	character        *Character
	currentPower     int
	currentToughness int
	maxToughness     int
	currentLevel     int
	uuid             string

	canHeal   bool
	canAttack bool

	effects []Effect
}

type currentBattle struct {
	player1 *FightingCharacter
	player2 *FightingCharacter
	turn    int
}

var rarityMultipliers = []RarityMultiplier{
	{Rarity: "Common", Multiplier: 1.0},
	{Rarity: "Uncommon", Multiplier: 1.5},
	{Rarity: "Rare", Multiplier: 2.0},
	{Rarity: "Epic", Multiplier: 3.0},
	{Rarity: "Legendary", Multiplier: 5.0},
	{Rarity: "Mythic", Multiplier: 10.0},
}

var Effects = []Effect{
	{
		Name:           "Weakened",
		Description:    "Reduces power by 20%% for 2 turns.",
		turnsRemaining: 2,
		effectFunc: func(attacker, defender *FightingCharacter) {
			attacker.currentPower = int(float64(attacker.currentPower) * 0.8)
		},
	},
	{
		Name:           "Vulnerable",
		Description:    "Reduces toughness by 20%% for 2 turns.",
		turnsRemaining: 2,
		effectFunc: func(attacker, defender *FightingCharacter) {
			defender.currentToughness = int(float64(defender.currentToughness) * 0.8)
		},
	},
	{
		Name:           "Injured",
		Description:    "Disallows healing for 3 turns.",
		turnsRemaining: 3,
		effectFunc: func(attacker, defender *FightingCharacter) {
			defender.canHeal = false
		},
	},
	{
		Name:           "Exhausted",
		Description:    "Disallows attacking for 1 turn.",
		turnsRemaining: 1,
		effectFunc: func(attacker, defender *FightingCharacter) {
			attacker.canAttack = false
		},
	},
}

func attack(attacker, defender *FightingCharacter) {
	if !attacker.canAttack {
		return
	}

	damage := attacker.currentPower
	if damage < 1 {
		damage = 1
	}
	defender.currentToughness -= damage
	if defender.currentToughness < 0 {
		defender.currentToughness = 0
	}

	for i := range attacker.effects {
		attacker.effects[i].effectFunc(attacker, defender)
	}

	for i := range defender.effects {
		defender.effects[i].effectFunc(attacker, defender)
	}

	attacker.canAttack = false
}

func heal(character *FightingCharacter) {
	if !character.canHeal {
		return
	}
	healAmount := character.maxToughness / 3
	if healAmount < 1 {
		healAmount = 1
	}
	character.currentToughness += healAmount
	if character.currentToughness > character.maxToughness {
		character.currentToughness = character.maxToughness
	}
	character.canHeal = false
}

func endTurn(battle *currentBattle) {
	battle.turn++

	for _, character := range []*FightingCharacter{battle.player1, battle.player2} {
		character.canAttack = true

		var remainingEffects []Effect
		for _, effect := range character.effects {
			effect.turnsRemaining--
			if effect.turnsRemaining > 0 {
				remainingEffects = append(remainingEffects, effect)
			}
		}
		character.effects = remainingEffects
	}
}

func applyEffect(character *FightingCharacter, effect Effect) {
	character.effects = append(character.effects, effect)
	effect.effectFunc(character, character)
}

func calculateRarityMultiplier(rarity string) float64 {
	for _, rm := range rarityMultipliers {
		if rm.Rarity == rarity {
			return rm.Multiplier
		}
	}
	return 1.0
}

func isBattleOver(battle *currentBattle) bool {
	return battle.player1.currentToughness <= 0 || battle.player2.currentToughness <= 0
}

func getBattleWinner(battle *currentBattle) string {
	if battle.player1.currentToughness <= 0 && battle.player2.currentToughness <= 0 {
		return "Draw"
	} else if battle.player1.currentToughness <= 0 {
		return "Player 2"
	} else if battle.player2.currentToughness <= 0 {
		return "Player 1"
	}
	return "None"
}

func startBattle(player1Char, player2Char *IndividalCharacter) *currentBattle {
	player1Power := calculateBattleStat(player1Char.CharacterInfo.Power, player1Char.CharacterInfo.Rarity)
	player1Toughness := calculateBattleStat(player1Char.CharacterInfo.Toughness, player1Char.CharacterInfo.Rarity)
	player1 := &FightingCharacter{
		character:        &player1Char.CharacterInfo,
		currentPower:     player1Power,
		currentToughness: player1Toughness,
		maxToughness:     player1Toughness,
		currentLevel:     player1Char.Level,
		uuid:             player1Char.UUID,
		canHeal:          true,
		canAttack:        true,
	}

	player2Power := calculateBattleStat(player2Char.CharacterInfo.Power, player2Char.CharacterInfo.Rarity)
	player2Toughness := calculateBattleStat(player2Char.CharacterInfo.Toughness, player2Char.CharacterInfo.Rarity)
	player2 := &FightingCharacter{
		character:        &player2Char.CharacterInfo,
		currentPower:     player2Power,
		currentToughness: player2Toughness,
		maxToughness:     player2Toughness,
		currentLevel:     player2Char.Level,
		uuid:             player2Char.UUID,
		canHeal:          true,
		canAttack:        true,
	}

	return &currentBattle{
		player1: player1,
		player2: player2,
		turn:    1,
	}
}

func calculateBattleStat(baseStat int, rarity string) int {
	if baseStat <= 0 {
		return 0
	}

	return max(1, int(float64(baseStat)*calculateRarityMultiplier(rarity)))
}

func hasTypeAdvantage(attackerType, defenderType string) bool {
	attackerType = canonicalCharacterType(attackerType)
	defenderType = canonicalCharacterType(defenderType)

	return (attackerType == CharacterTypeGoon && defenderType == CharacterTypeHarana) ||
		(attackerType == CharacterTypeHarana && defenderType == CharacterTypeBarkada) ||
		(attackerType == CharacterTypeBarkada && defenderType == CharacterTypeLockedIn) ||
		(attackerType == CharacterTypeLockedIn && defenderType == CharacterTypeGoon)
}

func calculateBattleWinnerXP(winner, loser *FightingCharacter) int64 {
	xp := battleWinnerBaseXP
	if winner.currentLevel < loser.currentLevel {
		xp += int64(loser.currentLevel-winner.currentLevel) * levelDisadvantageXP
	}
	if hasTypeAdvantage(loser.character.Type, winner.character.Type) {
		xp += typeDisadvantageXP
	}
	return xp
}

type battleSelectionResult struct {
	userID    string
	character *IndividalCharacter
	err       error
}

func handleBattleChallenge(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(m.Mentions) != 1 {
		sendBattleMessage(s, m.ChannelID, "Usage: ?battle @<opponent_mention>")
		return
	}

	opponent := m.Mentions[0]
	if opponent.ID == m.Author.ID {
		sendBattleMessage(s, m.ChannelID, "You cannot battle yourself!")
		return
	}
	if opponent.Bot {
		sendBattleMessage(s, m.ChannelID, "You cannot challenge a bot.")
		return
	}

	if !reserveBattleUsers(m.Author.ID, opponent.ID) {
		sendBattleMessage(s, m.ChannelID, "One of these players is already in a battle or selecting a character.")
		return
	}
	defer releaseBattleUsers(m.Author.ID, opponent.ID)

	challengerCharacters, err := getPlayerCharacters(m.Author.ID)
	if err != nil {
		log.Printf("failed to load challenger collection: %v", err)
		sendBattleMessage(s, m.ChannelID, "Failed to fetch your collection.")
		return
	}
	opponentCharacters, err := getPlayerCharacters(opponent.ID)
	if err != nil {
		log.Printf("failed to load opponent collection: %v", err)
		sendBattleMessage(s, m.ChannelID, "Failed to fetch your opponent's collection.")
		return
	}
	if len(challengerCharacters) == 0 {
		sendBattleMessage(s, m.ChannelID, "You need at least one character to battle.")
		return
	}
	if len(opponentCharacters) == 0 {
		sendBattleMessage(s, m.ChannelID, "Your opponent needs at least one character to battle.")
		return
	}

	sendBattleMessage(s, m.ChannelID, fmt.Sprintf("<@%s>, <@%s> challenged you. Reply `yes` to accept or `no` to decline.", opponent.ID, m.Author.ID))
	response, err := waitForUserMessage(s, opponent.ID, m.ChannelID, battleChallengeTimeout)
	if err != nil {
		sendBattleMessage(s, m.ChannelID, "The battle challenge expired.")
		return
	}
	if strings.ToLower(strings.TrimSpace(response.Content)) != "yes" {
		sendBattleMessage(s, m.ChannelID, "Battle challenge declined.")
		return
	}

	sendBattleMessage(s, m.ChannelID, "Battle accepted! Check your DMs and choose a character by number.")
	selections := make(chan battleSelectionResult, 2)
	go func() {
		character, err := selectBattleCharacter(s, m.Author.ID, challengerCharacters)
		selections <- battleSelectionResult{userID: m.Author.ID, character: character, err: err}
	}()
	go func() {
		character, err := selectBattleCharacter(s, opponent.ID, opponentCharacters)
		selections <- battleSelectionResult{userID: opponent.ID, character: character, err: err}
	}()

	first := <-selections
	second := <-selections
	if first.err != nil || second.err != nil {
		if first.err != nil {
			log.Printf("battle character selection failed: %v", first.err)
		}
		if second.err != nil {
			log.Printf("battle character selection failed: %v", second.err)
		}
		sendBattleMessage(s, m.ChannelID, "Battle cancelled because a character was not selected in time or DMs could not be delivered.")
		return
	}

	player1Character := first.character
	player2Character := second.character
	if first.userID != m.Author.ID {
		player1Character, player2Character = second.character, first.character
	}

	battle := startBattle(player1Character, player2Character)
	runBattle(s, m.ChannelID, m.Author.ID, opponent.ID, battle)
}

func reserveBattleUsers(userIDs ...string) bool {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()

	for _, userID := range userIDs {
		if _, exists := activeBattleUsers[userID]; exists {
			return false
		}
	}
	for _, userID := range userIDs {
		activeBattleUsers[userID] = struct{}{}
	}
	return true
}

func releaseBattleUsers(userIDs ...string) {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()
	for _, userID := range userIDs {
		delete(activeBattleUsers, userID)
	}
}

func selectBattleCharacter(s *discordgo.Session, userID string, characters []IndividalCharacter) (*IndividalCharacter, error) {
	dm, err := s.UserChannelCreate(userID)
	if err != nil {
		return nil, fmt.Errorf("open DM: %w", err)
	}

	messages := make(chan *discordgo.MessageCreate, 4)
	removeHandler := s.AddHandler(func(_ *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author != nil && !message.Author.Bot && message.Author.ID == userID && message.ChannelID == dm.ID {
			select {
			case messages <- message:
			default:
			}
		}
	})
	defer removeHandler()

	var choices strings.Builder
	choices.WriteString("Choose your battle character by replying with its number:\n")
	for i, character := range characters {
		fmt.Fprintf(&choices, "**%d. %s** — %s, Level %d, Power %d, Toughness %d\n",
			i+1, character.CharacterInfo.Name, character.CharacterInfo.Rarity, character.Level,
			character.CharacterInfo.Power, character.CharacterInfo.Toughness)
	}
	if err := sendChunkedMessage(s, dm.ID, choices.String()); err != nil {
		return nil, err
	}

	timer := time.NewTimer(battleSelectionTimeout)
	defer timer.Stop()
	for {
		select {
		case message := <-messages:
			selection, err := strconv.Atoi(strings.TrimSpace(message.Content))
			if err != nil || selection < 1 || selection > len(characters) {
				sendBattleMessage(s, dm.ID, fmt.Sprintf("Enter a number from 1 to %d.", len(characters)))
				continue
			}
			selected := characters[selection-1]
			sendBattleMessage(s, dm.ID, fmt.Sprintf("You selected **%s**.", selected.CharacterInfo.Name))
			return &selected, nil
		case <-timer.C:
			return nil, errBattleTimedOut
		}
	}
}

func runBattle(s *discordgo.Session, channelID, player1ID, player2ID string, battle *currentBattle) {
	sendBattleMessage(s, channelID, fmt.Sprintf("⚔️ Battle started: <@%s>'s **%s** vs <@%s>'s **%s**!",
		player1ID, battle.player1.character.Name, player2ID, battle.player2.character.Name))
	sendBattleStatus(s, channelID, player1ID, player2ID, battle)

	for !isBattleOver(battle) {
		attacker := battle.player1
		defender := battle.player2
		activePlayerID := player1ID
		if battle.turn%2 == 0 {
			attacker, defender = battle.player2, battle.player1
			activePlayerID = player2ID
		}

		sendBattleMessage(s, channelID, fmt.Sprintf("<@%s>, choose `attack`, `heal`, or `forfeit` (60 seconds).", activePlayerID))
		action, err := waitForBattleAction(s, activePlayerID, channelID, battleTurnTimeout)
		if err != nil {
			sendBattleMessage(s, channelID, fmt.Sprintf("<@%s> ran out of time and forfeited.", activePlayerID))
			finishBattle(s, channelID, battle, player1ID, player2ID, otherPlayer(activePlayerID, player1ID, player2ID), activePlayerID, false)
			return
		}

		switch action {
		case "forfeit":
			sendBattleMessage(s, channelID, fmt.Sprintf("<@%s> forfeited.", activePlayerID))
			finishBattle(s, channelID, battle, player1ID, player2ID, otherPlayer(activePlayerID, player1ID, player2ID), activePlayerID, false)
			return
		case "heal":
			if !attacker.canHeal {
				sendBattleMessage(s, channelID, "That character has already healed this battle. Choose another action.")
				continue
			}
			if attacker.currentToughness >= attacker.maxToughness {
				sendBattleMessage(s, channelID, "That character is already at full toughness. Choose another action.")
				continue
			}
			before := attacker.currentToughness
			heal(attacker)
			sendBattleMessage(s, channelID, fmt.Sprintf("**%s** healed for %d toughness.", attacker.character.Name, attacker.currentToughness-before))
		case "attack":
			before := defender.currentToughness
			attack(attacker, defender)
			sendBattleMessage(s, channelID, fmt.Sprintf("**%s** attacked **%s** for %d damage.", attacker.character.Name, defender.character.Name, before-defender.currentToughness))
		}

		if !isBattleOver(battle) {
			endTurn(battle)
			sendBattleStatus(s, channelID, player1ID, player2ID, battle)
		}
	}

	winnerID, loserID := player1ID, player2ID
	switch getBattleWinner(battle) {
	case "Draw":
		finishBattle(s, channelID, battle, player1ID, player2ID, "", "", true)
		return
	case "Player 2":
		winnerID, loserID = player2ID, player1ID
	}
	finishBattle(s, channelID, battle, player1ID, player2ID, winnerID, loserID, false)
}

func waitForBattleAction(s *discordgo.Session, userID, channelID string, timeout time.Duration) (string, error) {
	messages := make(chan *discordgo.MessageCreate, 4)
	removeHandler := s.AddHandler(func(_ *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author != nil && !message.Author.Bot && message.Author.ID == userID && message.ChannelID == channelID {
			select {
			case messages <- message:
			default:
			}
		}
	})
	defer removeHandler()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case message := <-messages:
			action := strings.ToLower(strings.TrimSpace(message.Content))
			action = strings.TrimPrefix(action, prefix)
			switch action {
			case "attack", "a":
				return "attack", nil
			case "heal", "h":
				return "heal", nil
			case "forfeit", "f":
				return "forfeit", nil
			default:
				sendBattleMessage(s, channelID, "Invalid action. Choose `attack`, `heal`, or `forfeit`.")
			}
		case <-timer.C:
			return "", errBattleTimedOut
		}
	}
}

func waitForUserMessage(s *discordgo.Session, userID, channelID string, timeout time.Duration) (*discordgo.MessageCreate, error) {
	messages := make(chan *discordgo.MessageCreate, 1)
	removeHandler := s.AddHandler(func(_ *discordgo.Session, message *discordgo.MessageCreate) {
		if message.Author != nil && !message.Author.Bot && message.Author.ID == userID && message.ChannelID == channelID {
			select {
			case messages <- message:
			default:
			}
		}
	})
	defer removeHandler()

	select {
	case message := <-messages:
		return message, nil
	case <-time.After(timeout):
		return nil, errBattleTimedOut
	}
}

func sendBattleStatus(s *discordgo.Session, channelID, player1ID, player2ID string, battle *currentBattle) {
	embed := &discordgo.MessageEmbed{
		Title: "Battle Status",
		Color: 0xE74C3C,
		Fields: []*discordgo.MessageEmbedField{
			{Name: battle.player1.character.Name, Value: fmt.Sprintf("Owner: <@%s>\nType: **%s** | Level: **%d**\nToughness: **%d/%d**\nPower: **%d**\nHeal: **%s**", player1ID, battle.player1.character.Type, battle.player1.currentLevel, battle.player1.currentToughness, battle.player1.maxToughness, battle.player1.currentPower, availability(battle.player1.canHeal)), Inline: true},
			{Name: battle.player2.character.Name, Value: fmt.Sprintf("Owner: <@%s>\nType: **%s** | Level: **%d**\nToughness: **%d/%d**\nPower: **%d**\nHeal: **%s**", player2ID, battle.player2.character.Type, battle.player2.currentLevel, battle.player2.currentToughness, battle.player2.maxToughness, battle.player2.currentPower, availability(battle.player2.canHeal)), Inline: true},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Turn %d", battle.turn)},
	}
	if _, err := s.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.Printf("failed to send battle status: %v", err)
	}
}

func finishBattle(s *discordgo.Session, channelID string, battle *currentBattle, player1ID, player2ID, winnerID, loserID string, draw bool) {
	if draw {
		sendBattleMessage(s, channelID, "The battle ended in a draw!")
		return
	}

	winner, loser := battle.player1, battle.player2
	if winnerID == player2ID {
		winner, loser = loser, winner
	}
	winnerXP := calculateBattleWinnerXP(winner, loser)
	sendBattleMessage(s, channelID, fmt.Sprintf("🏆 <@%s> wins the battle! (+%d XP; <@%s> receives +%d XP)", winnerID, winnerXP, loserID, battleLoserXP))

	if _, err := awardPlayerXP(winnerID, winnerXP); err != nil {
		log.Printf("failed to award winner XP: %v", err)
	}
	if _, err := awardPlayerXP(loserID, battleLoserXP); err != nil {
		log.Printf("failed to award participant XP: %v", err)
	}

	winnerProgress, err := awardCharacterXP(winnerID, winner.uuid, winnerXP)
	if err != nil {
		log.Printf("failed to award winner character XP: %v", err)
	} else if winnerProgress.LevelsGained > 0 {
		sendBattleMessage(s, channelID, fmt.Sprintf("⬆️ **%s** reached level %d!", winner.character.Name, winnerProgress.Entry.Level))
	}
	loserProgress, err := awardCharacterXP(loserID, loser.uuid, battleLoserXP)
	if err != nil {
		log.Printf("failed to award participant character XP: %v", err)
	} else if loserProgress.LevelsGained > 0 {
		sendBattleMessage(s, channelID, fmt.Sprintf("⬆️ **%s** reached level %d!", loser.character.Name, loserProgress.Entry.Level))
	}
}

func otherPlayer(activePlayerID, player1ID, player2ID string) string {
	if activePlayerID == player1ID {
		return player2ID
	}
	return player1ID
}

func availability(available bool) string {
	if available {
		return "Available"
	}
	return "Used"
}

func sendChunkedMessage(s *discordgo.Session, channelID, content string) error {
	const maxLength = 1900
	for len(content) > maxLength {
		split := strings.LastIndex(content[:maxLength], "\n")
		if split <= 0 {
			split = maxLength
		}
		if _, err := s.ChannelMessageSend(channelID, content[:split]); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		content = strings.TrimPrefix(content[split:], "\n")
	}
	if content != "" {
		if _, err := s.ChannelMessageSend(channelID, content); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}
	return nil
}

func sendBattleMessage(s *discordgo.Session, channelID, content string) {
	if _, err := s.ChannelMessageSend(channelID, content); err != nil {
		log.Printf("failed to send battle message: %v", err)
	}
}

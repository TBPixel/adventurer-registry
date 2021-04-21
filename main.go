package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/tbpixel/adventurer-registry/characters"

	"github.com/bwmarrin/discordgo"
	"github.com/tbpixel/adventurer-registry/config"
	"github.com/tbpixel/adventurer-registry/pkg/pq"
)

const (
	prefix = "!ar"
)

func main() {
	flag.Parse()

	_ = godotenv.Load()
	conf := config.Config{
		Host: os.Getenv("APP_HOST"),
		Port: os.Getenv("PORT"),
		Discord: config.Discord{
			Token: os.Getenv("DISCORD_TOKEN"),
		},
		Database: config.DB{
			URL: os.Getenv("DATABASE_URL"),
		},
	}
	db, err := pq.Connect(conf.Database)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := db.Disconnect(); err != nil {
			log.Println(err)
		}
	}()

	bot := Bot{db}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + conf.Discord.Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(bot.messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatalln(err)
		return
	}

	go func() {
		http.HandleFunc("/", health)
		log.Printf("server listening on port:%s", conf.Port)
		err = http.ListenAndServe(fmt.Sprintf(":%s", conf.Port), nil)
	}()

	// Ticker to ping service every 5 minutes, preventing idle
	ticker := time.NewTicker(5 * time.Minute)
	quitTicker := make(chan os.Signal, 1)
	signal.Notify(quitTicker, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	go func() {
		for {
			select {
			case <-ticker.C:
				_, _ = http.Get(conf.Host)
				// do stuff
			case <-quitTicker:
				ticker.Stop()
				return
			}
		}
	}()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	err = dg.Close()
	if err != nil {
		log.Fatalln(err)
	}
}

type Bot struct {
	db *pq.DB
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func (b Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bots
	if m.Author.ID == s.State.User.ID || m.Author.Bot {
		return
	}

	// ignore all messages that don't have our command prefix
	if !strings.HasPrefix(m.Content, prefix) {
		return
	}

	// no command provided, skip
	tokens := strings.Split(m.Content, " ")
	if len(tokens) <= 1 {
		return
	}

	command := tokens[1]
	var content string
	if len(tokens) >= 2 {
		content = strings.Join(tokens[2:], " ")
	}

	err := b.delegateCommands(command, content, s, m)
	if err != nil {
		write(err.Error(), s, m)
	}
}

func (b Bot) delegateCommands(command, content string, s *discordgo.Session, m *discordgo.MessageCreate) error {
	switch command {
	case "list":
		return b.handleList(s, m)
	case "register":
		return b.handleRegister(content, s, m)
	case "unregister":
		return b.handleUnregister(content, s, m)
	case "update":
		return b.handleUpdate(content, s, m)
	case "character":
		return b.handleCharacter(content, s, m)
	case "export":
		return b.handleExport(s, m)
	case "help":
		return b.handleHelp(s, m)
	default:
		return b.handleHelp(s, m)
	}
}

func (b Bot) handleList(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var chars []characters.Character
	var err error
	dm, err := isDM(s, m)
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	if dm {
		chars, err = b.db.Registry.CharactersByAuthor(m.Author.ID)
	} else {
		chars, err = b.db.Registry.Characters(m.GuildID)
	}
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	if len(chars) == 0 {
		write("No characters have been registered yet! Register the first with:\n `!ar register \"Character Name\" a full character description,\nnewlines included`", s, m)
		return nil
	}

	var names []string
	for _, c := range chars {
		names = append(names, c.Name)
	}

	list := strings.Join(names, "\n")

	writePrivate(fmt.Sprintf("All characters currently registered:\n%s", list), s, m)

	return nil
}

func (b Bot) handleRegister(content string, s *discordgo.Session, m *discordgo.MessageCreate) error {
	name, profile, err := extractNameAndProfile(content)
	if err != nil {
		return fmt.Errorf("A character must be registered with both a name and a content, in format: \n`!ar register \"Character Name\" Character description,\noptionally with newlines`")
	}

	attachments := ""
	if len(m.Attachments) > 0 {
		attachments = "\n"
	}
	for _, attachment := range m.Attachments {
		attachments = fmt.Sprintf("%s\n%s", attachments, attachment.ProxyURL)
	}

	c, err := b.db.Registry.Create(characters.Character{
		AuthorID: m.Author.ID,
		GuildID:  m.GuildID,
		Name:     name,
		Profile:  fmt.Sprintf("%s%s", profile, attachments),
	})
	if err != nil {
		if err == characters.ErrExists {
			return err
		}
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	write(fmt.Sprintf("%s has been registered!", c.Name), s, m)

	return nil
}

func (b Bot) handleUnregister(name string, s *discordgo.Session, m *discordgo.MessageCreate) error {
	err := b.db.Registry.Delete(name, m.GuildID)
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	write(fmt.Sprintf("'%s' has been unregistered!", name), s, m)

	return nil
}

func (b Bot) handleUpdate(content string, s *discordgo.Session, m *discordgo.MessageCreate) error {
	name, profile, err := extractNameAndProfile(content)
	if err != nil {
		return fmt.Errorf("A character must be updated with both a name and a content, in format: \n`!ar update \"Character Name\" Character description,\noptionally with newlines`")
	}

	attachments := ""
	if len(m.Attachments) > 0 {
		attachments = "\n"
	}
	for _, attachment := range m.Attachments {
		attachments = fmt.Sprintf("%s\n%s", attachments, attachment.ProxyURL)
	}

	_, err = b.db.Registry.Update(name, profile, m.GuildID)
	if err != nil {
		if err == characters.ErrNotFound {
			return err
		}

		log.Println(err)
		writeErr(s, m)
		return nil
	}

	write(fmt.Sprintf("'%s' has been updated!", name), s, m)
	return nil
}

func (b Bot) handleCharacter(name string, s *discordgo.Session, m *discordgo.MessageCreate) error {
	dm, err := isDM(s, m)
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	var c *characters.Character
	if dm {
		c, err = b.db.Registry.FindByAuthorID(name, m.Author.ID)
	} else {
		c, err = b.db.Registry.Find(name, m.GuildID)
	}
	if err != nil {
		if err == characters.ErrNotFound {
			write(fmt.Sprintf("No character by the name of '%s' exists in the Adventurer Registry!", name), s, m)
			return nil
		}
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	writePrivate(fmt.Sprintf("%s\n\n%s", c.Name, c.Profile), s, m)

	return nil
}

func (b Bot) handleExport(s *discordgo.Session, m *discordgo.MessageCreate) error {
	chars, err := b.db.Registry.CharactersByAuthor(m.Author.ID)
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	if len(chars) == 0 {
		writePrivate("You don't seem to have any characters registered yet.", s, m)
		return nil
	}

	channel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	w := ""
	for _, c := range chars {
		w += fmt.Sprintf("**%s**\n%s", c.Name, c.Profile)
		w += "\n\n\n" // generous breathing room for the next character
	}

	_, err = s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		File: &discordgo.File{
			Name:        fmt.Sprintf("%s character export", m.Author.Username),
			ContentType: "text/plain",
			Reader:      strings.NewReader(w),
		},
	})
	if err != nil {
		log.Println(err)
		writeErr(s, m)
		return nil
	}

	return nil
}

func (b Bot) handleHelp(s *discordgo.Session, m *discordgo.MessageCreate) error {
	help := "AdventureRegistry command help:"
	help += "\n**Characters registered with this bot are linked to you and the server you are in. Expect that characters you create in DMs with this bot will not be available in any servers, however all characters you create anywhere will be available to you in DMs.**\n"
	help += "\n`!ar` - is the bots command prefix. All commands will be prefixed with this"
	help += "\n`!ar list` - will list the names of all currently registered characters. Use this to confirm spelling when looking up a character"
	help += "\n`!ar register \"Character Name\" TypeFullDescriptionHere` - will allow you to add a character to the list"
	help += "\n`!ar unregister Character Name` - Removes a character from the registry, this is permanent"
	help += "\n`!ar update \"Character Name\" TypeFullDescriptionHere` - Updates a characters profile in the registry"
	help += "\n`!ar character Character Name` - Fetch a characters profile by name as a DM"
	help += "\n`!ar export` - Export all characters created by you"
	help += "\n`!ar help` shows this help screen"

	write(help, s, m)

	return nil
}

func extractNameAndProfile(content string) (string, string, error) {
	firstQuoteIdx := strings.Index(content, "\"")
	if firstQuoteIdx == -1 {
		return "", "", fmt.Errorf("character name must be passed in quotes")
	}
	lastQuoteIdx := strings.Index(content[1:], "\"")
	if lastQuoteIdx == -1 {
		return "", "", fmt.Errorf("character name must be passed in quotes")
	}

	name := content[firstQuoteIdx+1 : lastQuoteIdx+1]
	profile := content[lastQuoteIdx+2:]
	if name == "" || profile == "" {
		return "", "", fmt.Errorf("character name must be passed in quotes")
	}

	return name, profile, nil
}

func write(content string, s *discordgo.Session, m *discordgo.MessageCreate) {
	_, err := s.ChannelMessageSend(m.ChannelID, content)
	if err != nil {
		log.Println(err)
	}
}

func writePrivate(content string, s *discordgo.Session, m *discordgo.MessageCreate) {
	channel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = s.ChannelMessageSend(channel.ID, content)
	if err != nil {
		log.Println(err)
		return
	}
}

func writeErr(s *discordgo.Session, m *discordgo.MessageCreate) {
	write("Whoops! Something's gone wrong!\nWe're not sure what's happened but this issue has been logged and will be investigated in case a fix is needed.", s, m)
}

func isDM(s *discordgo.Session, m *discordgo.MessageCreate) (bool, error) {
	channel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		return false, err
	}

	return channel.ID == m.ChannelID, nil
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "online\n")
}

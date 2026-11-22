package modules

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	td "github.com/Kanha/Meow"
	"github.com/Kanha/Meow/filters/callbackquery"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"WasteMusic/waste/database"
)

// WordSeek word lists are sourced from the WordSeek project.
//
//go:embed wordseek/data/*.json
var wordSeekData embed.FS

var wordSeekLists = map[int][]string{}
var wordSeekValid = map[int]map[string]bool{}
var wordSeekGameColl, wordSeekGuessColl, wordSeekScoreColl *mongo.Collection

func initWordSeek(bot *td.Client) {
	if err := database.InitWordSeekCollections(); err != nil {
		return
	}
	wordSeekGameColl = database.WordSeekGames()
	wordSeekGuessColl = database.WordSeekGuesses()
	wordSeekScoreColl = database.WordSeekScores()
	for _, n := range []int{4, 5, 6} {
		common, _ := wordSeekData.ReadFile(fmt.Sprintf("wordseek/data/common-%d.json", n))
		all, _ := wordSeekData.ReadFile(fmt.Sprintf("wordseek/data/all-%d.json", n))
		_ = json.Unmarshal(common, &wordSeekLists[n])
		var words []string
		_ = json.Unmarshal(all, &words)
		wordSeekValid[n] = map[string]bool{}
		for _, w := range words {
			wordSeekValid[n][strings.ToLower(w)] = true
		}
	}
	bot.OnCommand("new4", wordSeekStart(4))
	bot.OnCommand("new5", wordSeekStart(5))
	bot.OnCommand("new6", wordSeekStart(6))
	bot.OnCommand("leaderboard", wordSeekLeaderboard)
	bot.OnUpdateNewMessage(wordSeekGuessHandler, nil)
	bot.OnUpdateNewCallbackQuery(wordSeekCallbackHandler, callbackquery.Regex("^ws:lb:"))
}

func wordSeekTopic(msg *td.Message) string {
	if t, ok := msg.TopicId.(*td.MessageTopicForum); ok {
		return fmt.Sprintf("%d", t.ForumTopicId)
	}
	return "general"
}
func wordSeekSender(msg *td.Message) int64 {
	if u, ok := msg.SenderId.(*td.MessageSenderUser); ok {
		return u.UserId
	}
	return 0
}
func wordSeekName(client *td.Client, id int64) (string, string) {
	u, err := client.GetUser(id)
	if err != nil || u == nil {
		return fmt.Sprintf("User %d", id), ""
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = fmt.Sprintf("User %d", id)
	}
	username := ""
	if u.Usernames != nil && len(u.Usernames.ActiveUsernames) > 0 {
		username = u.Usernames.ActiveUsernames[0]
	}
	return name, username
}
func wordSeekStart(length int) func(*td.Client, *td.Message) error {
	return func(c *td.Client, m *td.Message) error {
		chatID := m.ChatId
		topic := wordSeekTopic(m)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		var current bson.M
		err := wordSeekGameColl.FindOne(ctx, bson.M{"chatId": chatID, "topicId": topic}).Decode(&current)
		if err == nil {
			return sendWS(c, m, "There is already a game in progress here. Guess the word or wait for it to finish.")
		}
		if err != mongo.ErrNoDocuments {
			return sendWS(c, m, "Game database error. Please try again.")
		}
		words := wordSeekLists[length]
		if len(words) == 0 {
			return sendWS(c, m, "Word list is unavailable.")
		}
		solution := strings.ToLower(words[rand.Intn(len(words))])
		doc := bson.M{"chatId": chatID, "topicId": topic, "word": solution, "wordLength": length, "startedBy": wordSeekSender(m), "createdAt": time.Now()}
		_, err = wordSeekGameColl.InsertOne(ctx, doc)
		if err != nil {
			return sendWS(c, m, "A game is already running here.")
		}
		return sendWS(c, m, fmt.Sprintf("🎮 WordSeek started! Guess the %d-letter word.\nSend guesses as normal messages. Green = correct position, yellow = in the word, red = absent. You have 30 guesses.\nUse /leaderboard to view scores.", length))
	}
}
func sendWS(c *td.Client, m *td.Message, text string) error {
	_, err := m.ReplyText(c, text, nil)
	return err
}

func wordSeekGuessHandler(c *td.Client, u *td.UpdateNewMessage) error {
	if u.Message == nil || u.Message.IsOutgoing || u.Message.IsCommand() {
		return nil
	}
	guess := strings.ToLower(strings.TrimSpace(u.Message.GetText()))
	if !regexp.MustCompile(`^[a-z]{4,6}$`).MatchString(guess) {
		return nil
	}
	chatID := u.Message.ChatId
	topic := wordSeekTopic(u.Message)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var game struct {
		ID         bson.ObjectID `bson:"_id"`
		Word       string        `bson:"word"`
		WordLength int           `bson:"wordLength"`
		ChatID     int64         `bson:"chatId"`
		TopicID    string        `bson:"topicId"`
	}
	if err := wordSeekGameColl.FindOne(ctx, bson.M{"chatId": chatID, "topicId": topic}).Decode(&game); err != nil {
		return nil
	}
	if len(guess) != game.WordLength {
		return nil
	}
	if !wordSeekValid[game.WordLength][guess] {
		return sendWS(c, u.Message, fmt.Sprintf("%s is not a valid %d-letter word.", guess, game.WordLength))
	}
	var dup bson.M
	if err := wordSeekGuessColl.FindOne(ctx, bson.M{"gameId": game.ID, "guess": guess}).Decode(&dup); err == nil {
		return sendWS(c, u.Message, "Someone has already guessed this word. Try another one!")
	}
	uid := wordSeekSender(u.Message)
	name, username := wordSeekName(c, uid)
	if guess == game.Word {
		var count int64
		count, _ = wordSeekGuessColl.CountDocuments(ctx, bson.M{"gameId": game.ID})
		score := 30 - int(count)
		if score < 1 {
			score = 1
		}
		_, err := wordSeekScoreColl.InsertOne(ctx, bson.M{"chatId": chatID, "userId": uid, "name": name, "username": username, "wordLength": game.WordLength, "score": score, "createdAt": time.Now()})
		if err != nil {
			return sendWS(c, u.Message, "You guessed it! Score saving failed, but the game is complete.")
		}
		_, _ = wordSeekGameColl.DeleteOne(ctx, bson.M{"_id": game.ID})
		_, _ = wordSeekGuessColl.DeleteMany(ctx, bson.M{"gameId": game.ID})
		return sendWS(c, u.Message, fmt.Sprintf("🎉 Congrats! You guessed it correctly.\nCorrect word: %s\n🏆 Added %d points to the leaderboard.\nStart another game with /new%d", strings.ToUpper(guess), score, game.WordLength))
	}
	_, err := wordSeekGuessColl.InsertOne(ctx, bson.M{"gameId": game.ID, "chatId": chatID, "userId": uid, "guess": guess, "createdAt": time.Now()})
	if err != nil {
		return nil
	}
	var guesses []struct {
		Guess string `bson:"guess"`
	}
	cur, err := wordSeekGuessColl.Find(ctx, bson.M{"gameId": game.ID}, optionsWS())
	if err != nil {
		return nil
	}
	_ = cur.All(ctx, &guesses)
	if len(guesses) >= 30 {
		_, _ = wordSeekGameColl.DeleteOne(ctx, bson.M{"_id": game.ID})
		_, _ = wordSeekGuessColl.DeleteMany(ctx, bson.M{"gameId": game.ID})
		return sendWS(c, u.Message, fmt.Sprintf("Game over! The word was %s. Start a new game with /new%d", strings.ToUpper(game.Word), game.WordLength))
	}
	lines := make([]string, 0, len(guesses))
	for _, g := range guesses {
		lines = append(lines, wordSeekFeedback(g.Guess, game.Word))
	}
	return sendWS(c, u.Message, fmt.Sprintf("🔤 %d/%d guesses\n%s", len(guesses), 30, strings.Join(lines, "\n")))
}
func optionsWS() *options.FindOptions {
	return options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
}
func wordSeekFeedback(guess, solution string) string {
	g := []rune(strings.ToUpper(guess))
	s := []rune(strings.ToUpper(solution))
	states := make([]string, len(g))
	counts := map[rune]int{}
	for _, r := range s {
		counts[r]++
	}
	for i, r := range g {
		if r == s[i] {
			states[i] = "🟩"
			counts[r]--
		}
	}
	for i, r := range g {
		if states[i] == "" {
			if counts[r] > 0 {
				states[i] = "🟨"
				counts[r]--
			} else {
				states[i] = "🟥"
			}
		}
	}
	return strings.Join(states, "")
}

func wordSeekLeaderboard(c *td.Client, m *td.Message) error {
	args := strings.Fields(m.Args())
	scope, period, length := "global", "month", 5
	for _, a := range args {
		switch a {
		case "global", "group":
			scope = a
		case "today", "week", "month", "year", "all":
			period = a
		case "4", "5", "6":
			fmt.Sscanf(a, "%d", &length)
		}
	}
	return wordSeekSendLeaderboard(c, m, scope, period, length)
}
func wordSeekSendLeaderboard(c *td.Client, m *td.Message, scope, period string, length int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	match := bson.M{"wordLength": length}
	if scope == "group" {
		match["chatId"] = m.ChatId
	}
	if period != "all" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		switch period {
		case "week":
			start = start.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		case "month":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		case "year":
			start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		}
		match["createdAt"] = bson.M{"$gte": start}
	}
	pipeline := mongo.Pipeline{{{Key: "$match", Value: match}}, {{Key: "$group", Value: bson.M{"_id": "$userId", "name": bson.M{"$last": "$name"}, "username": bson.M{"$last": "$username"}, "score": bson.M{"$sum": "$score"}}}}, {{Key: "$sort", Value: bson.M{"score": -1}}}, {{Key: "$limit", Value: 20}}}
	cur, err := wordSeekScoreColl.Aggregate(ctx, pipeline)
	if err != nil {
		return sendWS(c, m, "Leaderboard unavailable right now.")
	}
	defer cur.Close(ctx)
	var rows []struct {
		Name     string `bson:"name"`
		Username string `bson:"username"`
		Score    int    `bson:"score"`
	}
	if err = cur.All(ctx, &rows); err != nil {
		return sendWS(c, m, "Leaderboard unavailable right now.")
	}
	title := "Global"
	if scope == "group" {
		title = "This chat"
	}
	if len(rows) == 0 {
		rows = []struct {
			Name     string `bson:"name"`
			Username string `bson:"username"`
			Score    int    `bson:"score"`
		}{}
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("🏆 %s Leaderboard 🏆\n\n", title))
	for i, r := range rows {
		rank := "🔅"
		if i == 0 {
			rank = "🥇"
		} else if i == 1 {
			rank = "🥈"
		} else if i == 2 {
			rank = "🥉"
		}
		name := r.Name
		if name == "" {
			name = "User"
		}
		out.WriteString(fmt.Sprintf("%s %s - %s pts\n", rank, name, formatWSNum(r.Score)))
	}
	if len(rows) == 0 {
		out.WriteString("No scores yet.")
	}
	kb := wordSeekLeaderboardKeyboard(scope, period, length)
	_, err = c.SendTextMessage(m.ChatId, out.String(), &td.SendTextMessageOpts{ReplyMarkup: kb})
	return err
}
func formatWSNum(n int) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
func wordSeekLeaderboardKeyboard(scope, period string, length int) td.ReplyMarkup {
	kb := &td.ReplyMarkupInlineKeyboard{}
	kb.Rows = [][]td.InlineKeyboardButton{}
	button := func(label, data string, active bool) td.InlineKeyboardButton {
		b := td.InlineKeyboardButton{Text: label, Type: td.InlineKeyboardButtonTypeCallback{Data: []byte(data)}}
		if active {
			b.Style = td.ButtonStylePrimary{}
		}
		return b
	}
	kb.Rows = append(kb.Rows, []td.InlineKeyboardButton{
		button("« Global »", fmt.Sprintf("ws:lb:global:%s:%d", period, length), scope == "global"),
		button("🔄", fmt.Sprintf("ws:lb:%s:%s:%d", scope, period, length), false),
		button("This chat", fmt.Sprintf("ws:lb:group:%s:%d", period, length), scope == "group"),
	})
	kb.Rows = append(kb.Rows, []td.InlineKeyboardButton{
		button("Today", fmt.Sprintf("ws:lb:%s:today:%d", scope, length), period == "today"),
		button("This week", fmt.Sprintf("ws:lb:%s:week:%d", scope, length), period == "week"),
		button("This month", fmt.Sprintf("ws:lb:%s:month:%d", scope, length), period == "month"),
	})
	kb.Rows = append(kb.Rows, []td.InlineKeyboardButton{
		button("This year", fmt.Sprintf("ws:lb:%s:year:%d", scope, length), period == "year"),
		button("All time", fmt.Sprintf("ws:lb:%s:all:%d", scope, length), period == "all"),
	})
	kb.Rows = append(kb.Rows, []td.InlineKeyboardButton{
		button("4 letters", fmt.Sprintf("ws:lb:%s:%s:4", scope, period), length == 4),
		button("5 letters", fmt.Sprintf("ws:lb:%s:%s:5", scope, period), length == 5),
		button("6 letters", fmt.Sprintf("ws:lb:%s:%s:6", scope, period), length == 6),
	})
	return kb
}

func wordSeekCallbackHandler(c *td.Client, u *td.UpdateNewCallbackQuery) error {
	data := u.DataString()
	parts := strings.Split(data, ":")
	if len(parts) != 5 {
		return nil
	}
	scope, period := parts[2], parts[3]
	length := 0
	fmt.Sscanf(parts[4], "%d", &length)
	if (scope != "global" && scope != "group") || !map[string]bool{"today": true, "week": true, "month": true, "year": true, "all": true}[period] || length < 4 || length > 6 {
		return nil
	}
	m, err := u.GetMessage(c)
	if err != nil || m == nil {
		return nil
	} // replace message with updated leaderboard
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	match := bson.M{"wordLength": length}
	if scope == "group" {
		match["chatId"] = u.ChatId
	}
	if period != "all" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		switch period {
		case "week":
			start = start.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
		case "month":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		case "year":
			start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		}
		match["createdAt"] = bson.M{"$gte": start}
	}
	pipeline := mongo.Pipeline{{{Key: "$match", Value: match}}, {{Key: "$group", Value: bson.M{"_id": "$userId", "name": bson.M{"$last": "$name"}, "username": bson.M{"$last": "$username"}, "score": bson.M{"$sum": "$score"}}}}, {{Key: "$sort", Value: bson.M{"score": -1}}}, {{Key: "$limit", Value: 20}}}
	cur, err := wordSeekScoreColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)
	var rows []struct {
		Name  string `bson:"name"`
		Score int    `bson:"score"`
	}
	_ = cur.All(ctx, &rows)
	title := "Global"
	if scope == "group" {
		title = "This chat"
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("🏆 %s Leaderboard 🏆\n\n", title))
	for i, r := range rows {
		rank := "🔅"
		if i == 0 {
			rank = "🥇"
		} else if i == 1 {
			rank = "🥈"
		} else if i == 2 {
			rank = "🥉"
		}
		out.WriteString(fmt.Sprintf("%s %s - %s pts\n", rank, r.Name, formatWSNum(r.Score)))
	}
	if len(rows) == 0 {
		out.WriteString("No scores yet.")
	}
	_, _ = u.EditMessageText(c, out.String(), &td.EditTextMessageOpts{ReplyMarkup: wordSeekLeaderboardKeyboard(scope, period, length)})
	_ = u.Answer(c, 0, false, "", "")
	return nil
}

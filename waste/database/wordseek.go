package database

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

var wordSeekGames, wordSeekGuesses, wordSeekScores *mongo.Collection

func InitWordSeekCollections() error {
	wordSeekGames = database.Collection("wordseek_games")
	wordSeekGuesses = database.Collection("wordseek_guesses")
	wordSeekScores = database.Collection("wordseek_scores")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := wordSeekGames.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: map[string]int{"chatId": 1, "topicId": 1}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = wordSeekGuesses.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: map[string]int{"gameId": 1, "guess": 1}, Options: options.Index().SetUnique(true)})
	if err != nil {
		return err
	}
	_, err = wordSeekScores.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: map[string]int{"userId": 1, "createdAt": -1}})
	return err
}
func WordSeekGames() *mongo.Collection   { return wordSeekGames }
func WordSeekGuesses() *mongo.Collection { return wordSeekGuesses }
func WordSeekScores() *mongo.Collection  { return wordSeekScores }

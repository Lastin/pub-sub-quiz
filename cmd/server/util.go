package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	pb "pubSubQuiz/proto"
)

func shuffleAnswers(incorrectAnswers []string, correctAnswer string) []string {
	answers := make([]string, len(incorrectAnswers)+1)
	answers[0] = correctAnswer
	for i, a := range incorrectAnswers {
		answers[i+1] = a
	}
	rand.Shuffle(len(answers), func(i, j int) {
		answers[i], answers[j] = answers[j], answers[i]
	})
	return answers
}

func mapQuestions(questions []Question) map[string]Question {
	m := make(map[string]Question)
	for i, q := range questions {
		m[fmt.Sprintf("question-%d", i)] = q
	}
	return m
}

func loadQuestions() ([]Question, error) {
	b, err := os.ReadFile(*questionFile)
	if err != nil {
		return nil, err
	}
	var questions []Question
	return questions, json.Unmarshal(b, &questions)
}

func printLeaderboard(leaderboard []*pb.PlayResponse_Score) {
	log.Printf("Leaderboard:")
	for _, score := range leaderboard {
		log.Printf("%s: %d", score.PlayerId, score.Score)
	}
}

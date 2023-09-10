package main

import (
	"flag"
	"log"
	"time"
)

var questionFile = flag.String("questionFile", "./questions_short.json", "Path to the question file")
var targetPlayers = flag.Int("targetPlayers", 1, "Number of players")
var questionTimeout = flag.Duration("questionTimeout", 10*time.Second, "Timeout for each question")
var port = flag.Int("port", 50052, "port number")
var startDelay = flag.Duration("startDelay", 10*time.Second, "Delay before starting the quiz")

func main() {
	flag.Parse()
	questions, err := loadQuestions()
	if err != nil {
		log.Fatal(err)
	}
	server := NewServer(questions, *targetPlayers, *questionTimeout, *startDelay)
	err = server.Start()
	if err != nil {
		panic(err)
	}
}

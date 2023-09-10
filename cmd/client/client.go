package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	pb "pubSubQuiz/proto"
	"time"
)

var (
	addr = flag.String("addr", "localhost:50052", "the address to connect to")
)

func init() {
	log.SetFlags(0)
}

func main() {
	flag.Parse()
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := pb.NewQuizClient(conn)
	stream, err := client.Play(context.Background())
	if err != nil {
		panic(err)
	}
	// connect to the server and try to join the game
	err = stream.Send(&pb.PlayRequest{
		Msg: &pb.PlayRequest_JoinRequest{
			JoinRequest: &pb.PlayRequest_Join{},
		},
	})
	// if there is an error, stop here
	if err != nil {
		panic(err)
	}

	// start listening for user inputs
	inputChan := make(chan string)
	go listenForInputs(inputChan)

	// start listening for server messages
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Server disconnected\n")
			return
		default:
			msg, err := stream.Recv()
			if err != nil {
				log.Printf("Error receiving message: %s\n", err.Error())
			} else {
				handleMessage(msg, stream, inputChan)
			}
		}
	}
}

// Go-routine that listens for user inputs into stdin
// and sends them to the inputChan
func listenForInputs(inputChan chan string) {
	go func() {
		for {
			var input string
			_, err := fmt.Scanln(&input)
			if err == nil {
				inputChan <- input
			}
		}
	}()
}

// handleMessage handles the messages received from the server
func handleMessage(msg *pb.PlayResponse, stream pb.Quiz_PlayClient, inputChan chan string) {
	var err error
	switch t := msg.Msg.(type) {
	case *pb.PlayResponse_JoinResponseMsg:
		handleJoinResponse(t.JoinResponseMsg)
	case *pb.PlayResponse_QuestionMsg:
		err = handleQuestion(t.QuestionMsg, stream, inputChan)
	case *pb.PlayResponse_GameStartsInMsg:
		handleGameStarts(t.GameStartsInMsg)
	case *pb.PlayResponse_StartInterruptedMsg:
		handleStartInterrupted(t.StartInterruptedMsg)
	case *pb.PlayResponse_GameEndsMsg:
		handleGameEnds(t.GameEndsMsg)
	case *pb.PlayResponse_AnswerRejectionMsg:
		handleAnswerRejected(t.AnswerRejectionMsg)
	}
	if err != nil {
		log.Printf("Failed to handle the message %s\n", err.Error())
	}
}

// handleAnswerRejected handles the message when the server rejects an answer
func handleAnswerRejected(msg *pb.PlayResponse_AnswerRejected) {
	log.Printf("Answer rejected due: %s\n", msg.Reason)
}

// handleGameEnds handles the message when the game ends
func handleGameEnds(msg *pb.PlayResponse_GameEnds) {
	log.Printf("Game finished!\n")
	log.Println("Leaderboard:")
	log.Println("=================")
	for _, score := range msg.Leaderboard {
		log.Printf("%s: %d\n", score.PlayerId, score.Score)
	}
	log.Println("=================")
}

// handleGameStarts handles the message when the game starts
func handleGameStarts(msg *pb.PlayResponse_GameStartsIn) {
	log.Printf("\rGame starts in %.f seconds", msg.SecondsUntilStart)
}

// handleJoinResponse handles the message when the server responds to a join request
func handleJoinResponse(r *pb.PlayResponse_JoinResponse) {
	if r.Success {
		log.Printf("Joined the quiz. Waiting for %d more players\n", r.TargetPlayers-r.CurrentPlayers)
	} else {
		log.Printf("Join rejected: %s\n", r.Reason)
		// in case of rejection, exit the client
		os.Exit(0)
	}
}

// handleQuestion handles the message when the server sends a question
func handleQuestion(question *pb.PlayResponse_QuizQuestion, stream pb.Quiz_PlayClient, inputChan chan string) error {
	log.Printf("Question: %s\n", question.Question)
	var err error
	m := printAnswerOptions(question.Answers)

	// create time-out context, cancelled when time is up
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(question.QuestionTimeout))
	defer cancel()
	// loop until either time is up, or user submits a valid answer
	for {
		select {
		case <-ctx.Done():
			log.Println("You ran out of time!")
			return nil
		case userInput := <-inputChan:
			// check if the user input is a valid answer
			if answer, ok := m[userInput]; ok {
				log.Printf("Submitting: %s\n", answer)
				err = stream.Send(&pb.PlayRequest{
					Msg: &pb.PlayRequest_AnswerRequest{
						AnswerRequest: &pb.PlayRequest_Answer{
							Answer:     answer,
							QuestionId: question.QuestionId,
						},
					},
				})
				return err
			} else {
				log.Printf("%s is not a valid answer", userInput)
			}
		}
	}
}

// printAnswerOptions prints available answer options and returns a map of the options for quick lookups
func printAnswerOptions(answers []string) map[string]string {
	m := make(map[string]string, len(answers))
	for i, v := range answers {
		id := i + 1
		log.Printf("%d: %s\n", id, v)
		m[fmt.Sprintf("%d", id)] = v
	}
	return m
}

// handleStartInterrupted handles the message when the game is interrupted
func handleStartInterrupted(msg *pb.PlayResponse_StartInterrupted) {
	log.Printf("Game interrupted due to: %s\n", msg.Reason)
}

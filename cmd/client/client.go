package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
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
	err = stream.Send(&pb.PlayRequest{
		Msg: &pb.PlayRequest_JoinRequest{
			JoinRequest: &pb.PlayRequest_Join{},
		},
	})
	if err != nil {
		panic(err)
	}
	inputChan := make(chan string)
	go listenForInputs(inputChan)
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

func handleAnswerRejected(msg *pb.PlayResponse_AnswerRejected) {
	log.Printf("Answer rejected due: %s\n", msg.Reason)
}

func handleGameEnds(msg *pb.PlayResponse_GameEnds) {
	log.Printf("Game finished!\n")
	log.Println("Leaderboard:")
	log.Println("=================")
	for _, score := range msg.Leaderboard {
		log.Printf("%s: %d\n", score.PlayerId, score.Score)
	}
	log.Println("=================")
}

func handleGameStarts(msg *pb.PlayResponse_GameStartsIn) {
	log.Printf("\rGame starts in %.f seconds", msg.SecondsUntilStart)
}

func handleJoinResponse(r *pb.PlayResponse_JoinResponse) {
	if r.Success {
		log.Printf("Joined the quiz. Waiting for %d more players\n", r.TargetPlayers-r.CurrentPlayers)
	} else {
		log.Printf("Join rejected: %s\n", r.Reason)
	}
}

func handleQuestion(question *pb.PlayResponse_QuizQuestion, stream pb.Quiz_PlayClient, inputChan chan string) error {
	log.Printf("Question: %s\n", question.Question)
	var err error
	m := printAnswerOptions(question.Answers)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(question.QuestionTimeout))
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			log.Println("You ran out of time!")
			return nil
		case userInput := <-inputChan:
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
			} else {
				log.Printf("%s is not a valid answer", userInput)
			}
			return err
		}
	}
}

func printAnswerOptions(answers []string) map[string]string {
	m := make(map[string]string, len(answers))
	for i, v := range answers {
		id := i + 1
		log.Printf("%d: %s\n", id, v)
		m[fmt.Sprintf("%d", id)] = v
	}
	return m
}

func handleStartInterrupted(msg *pb.PlayResponse_StartInterrupted) {
	log.Printf("Game interrupted due to: %s\n", msg.Reason)
}

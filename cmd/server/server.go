package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"log"
	"net"
	pb "pubSubQuiz/proto"
	"sort"
	"sync"
	"time"
)

// gRPC keepalive settings
var kaep = keepalive.EnforcementPolicy{
	MinTime:             1 * time.Second,
	PermitWithoutStream: true,
}

// gRPC keepalive settings
var kasp = keepalive.ServerParameters{
	MaxConnectionIdle:     1 * time.Second,
	MaxConnectionAgeGrace: 1 * time.Second,
	Time:                  1 * time.Second,
	Timeout:               1 * time.Second,
}

// Question representation from the JSON file
type Question struct {
	Question         string   `json:"question"`
	CorrectAnswer    string   `json:"correct_answer"`
	IncorrectAnswers []string `json:"incorrect_answers"`
}

// Player represents an entity that is connected to the server and plays the game
type Player struct {
	Uuid   string
	Score  int32
	stream pb.Quiz_PlayServer
}

type Server struct {
	pb.QuizServer
	mutex             sync.RWMutex
	questions         map[string]Question
	players           map[string]*Player
	questionTimeout   time.Duration
	startDelay        time.Duration
	state             QuizState
	currentQuestionId string
	targetPlayers     int
	disconnectChan    chan string
}

type QuizState string

var (
	QuizStateWaitingForPlayers QuizState = "waitingForPlayers"
	QuizStateStarting          QuizState = "starting"
	QuizStateQuestion          QuizState = "question"
	QuizStateFinished          QuizState = "finished"
)

func NewServer(questions []Question, targetPlayers int, timeout time.Duration, startDelay time.Duration) *Server {
	return &Server{
		state:           QuizStateWaitingForPlayers,
		questions:       mapQuestions(questions),
		players:         make(map[string]*Player, targetPlayers),
		questionTimeout: timeout,
		targetPlayers:   targetPlayers,
		startDelay:      startDelay,
		disconnectChan:  make(chan string), //unbuffered chan to watch for disconnecting users
	}
}

func (s *Server) Start() error {
	address := fmt.Sprintf(":%d", *port)
	log.Printf("Server listening on %s\n", address)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	rpcServer := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(kaep), grpc.KeepaliveParams(kasp))
	pb.RegisterQuizServer(rpcServer, s)
	log.Printf("Waiting for %d players\n", s.targetPlayers)
	return rpcServer.Serve(lis)
}

// Play is the gRPC method that handles the communication with the client
func (s *Server) Play(stream pb.Quiz_PlayServer) error {
	player := &Player{
		uuid.NewString(),
		0,
		stream,
	}
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Client disconnected\n")
			s.removePlayer(player.Uuid)
			s.disconnectChan <- player.Uuid
			return nil
		default:
			msg, err := stream.Recv()
			if err != nil {
				return err
			}
			s.handleMessage(msg, stream, player)
		}

	}
}

// handleMessage handles the incoming messages from the client
func (s *Server) handleMessage(msg *pb.PlayRequest, stream pb.Quiz_PlayServer, player *Player) {
	var err error
	switch v := msg.Msg.(type) {
	case *pb.PlayRequest_JoinRequest:
		err = s.HandleJoinMsg(stream, player)
	case *pb.PlayRequest_AnswerRequest:
		err = s.HandleAnswerMsg(player, stream, v.AnswerRequest)
	}
	if err != nil {
		log.Printf("Error handling message: %s\n", err.Error())
	}
}

// HandleJoinMsg handles the join request from the client
func (s *Server) HandleJoinMsg(stream pb.Quiz_PlayServer, player *Player) error {
	log.Printf("Received join request: %s\n", player.Uuid)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.state != QuizStateWaitingForPlayers { // check players can still join
		log.Printf("Game in state %s\n, rejecting join request", s.state)
		err := stream.Send(&pb.PlayResponse{
			Msg: &pb.PlayResponse_JoinResponseMsg{
				JoinResponseMsg: &pb.PlayResponse_JoinResponse{
					Success: false,
					Reason:  "Game already started",
				},
			},
		})
		if err != nil {
			log.Printf("Error sending message: %s\n", err.Error())
		}
		return err
	}
	if _, ok := s.players[player.Uuid]; !ok {
		s.players[player.Uuid] = player
	} else {
		log.Printf("Player %s already joined\n", player.Uuid)
	}
	err := stream.Send(&pb.PlayResponse{
		Msg: &pb.PlayResponse_JoinResponseMsg{
			JoinResponseMsg: &pb.PlayResponse_JoinResponse{
				Success:        true,
				TargetPlayers:  int32(s.targetPlayers),
				CurrentPlayers: int32(len(s.players)),
				PlayerId:       player.Uuid,
			},
		},
	})
	if err != nil {
		log.Printf("Error sending message: %s\n", err.Error())
	}
	if len(s.players) == s.targetPlayers {
		log.Printf("Current players: %d\n", len(s.players))
		go s.runQuizLoop()
	} else {
		log.Printf("Waiting for %d more players\n", s.targetPlayers-len(s.players))
	}
	return err
}

// HandleAnswerMsg handles the answer request from the client
func (s *Server) HandleAnswerMsg(player *Player, stream pb.Quiz_PlayServer, ar *pb.PlayRequest_Answer) error {
	log.Println("Received answer request")
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.state != QuizStateQuestion { // check players can still join
		err := stream.Send(&pb.PlayResponse{
			Msg: &pb.PlayResponse_AnswerRejectionMsg{
				AnswerRejectionMsg: &pb.PlayResponse_AnswerRejected{
					Reason: "Game not started",
				},
			},
		})
		if err != nil {
			log.Printf("Error sending message: %v\n", err)
		}
		return err

	} else if ar.QuestionId != s.currentQuestionId { // check if question id matches current one
		reason := fmt.Sprintf("Player %s submitted answer for wrong question id. Want: %s, got %s", player.Uuid, s.currentQuestionId, ar.QuestionId)
		log.Println(reason)
		err := stream.Send(&pb.PlayResponse{
			Msg: &pb.PlayResponse_AnswerRejectionMsg{
				AnswerRejectionMsg: &pb.PlayResponse_AnswerRejected{
					Reason: reason,
				},
			},
		})
		if err != nil {
			log.Printf("Error sending message: %v\n", err)
		}
		return err
	} else if ar.Answer == s.questions[ar.QuestionId].CorrectAnswer { // check if answer is correct
		s.players[player.Uuid].Score++
	}
	return nil
}

// resetState resets the server to the initial state
func (s *Server) resetState() {
	log.Printf("Resetting the server")
	s.mutex.Lock()
	s.players = make(map[string]*Player, s.targetPlayers)
	s.state = QuizStateWaitingForPlayers
	s.disconnectChan = make(chan string)
	s.mutex.Unlock()
	log.Printf("Server reset!")

}

// runQuizLoop main loop responsible for running the quiz
func (s *Server) runQuizLoop() {
	defer func() {
		log.Println("Game finished")
		s.resetState()
	}()

	// update the state to prevent other players from joining
	s.state = QuizStateStarting
	quizStartTime := time.Now().Add(s.startDelay)
	log.Printf("Starting quiz with %v players in %.f seconds\n", len(s.players), time.Until(quizStartTime).Seconds())
	ctx, cancel := context.WithTimeout(context.Background(), s.startDelay)
	// start a goroutine that pings client when the game is about to begin
	go s.pingUsersGetReady(ctx, quizStartTime)

	select {
	case <-ctx.Done():
		log.Println("Game starting")
		cancel()
	case playerId := <-s.disconnectChan: // if a player disconnects, abort the game
		log.Printf("Player %s disconnected", playerId)
		cancel()
		s.informAllInterrupt(fmt.Sprintf("Player %s has disconnected. Stopping the game", playerId))
		return
	}

	// update the state to enable receiving the answers
	s.mutex.Lock()
	s.state = QuizStateQuestion
	s.mutex.Unlock()
	for id, q := range s.questions {
		if len(s.players) == 0 {
			log.Printf("No players left")
			break
		}
		log.Printf("Sending question %v\n", id)

		go s.sendAllQuestion(id, q)
		log.Printf("Waiting for answers")

		s.mutex.Lock()
		s.currentQuestionId = id
		s.mutex.Unlock()

		// Wait to collect the answers
		time.Sleep(s.questionTimeout + time.Second)
		log.Printf("Finished waiting for answers for question %v\n", id)
	}
	s.state = QuizStateFinished
	leaderboard := s.getLeaderboard()
	printLeaderboard(leaderboard)
	s.mutex.Lock()
	s.state = QuizStateFinished
	s.mutex.Unlock()
	s.informAllEnd(leaderboard)
}

// Pings all players every second to inform them about the game start
// Stops when the context is cancelled
func (s *Server) pingUsersGetReady(ctx context.Context, startTime time.Time) {
	for {
		select {
		case <-time.Tick(time.Second):
			s.informAllStartsIn(startTime)
		case <-ctx.Done():
			return
		}
	}
}

// Inform all players that the game is about to begin
func (s *Server) informAllStartsIn(startTime time.Time) {
	msg := &pb.PlayResponse{
		Msg: &pb.PlayResponse_GameStartsInMsg{
			GameStartsInMsg: &pb.PlayResponse_GameStartsIn{
				SecondsUntilStart: time.Until(startTime).Seconds(),
			},
		},
	}
	wg := sync.WaitGroup{}
	for _, player := range s.players {
		player := player
		wg.Add(1)
		go func() {
			err := player.stream.Send(msg)
			if err != nil {
				log.Printf("Error sending message to player %s: %s\n", player.Uuid, err.Error())
				s.disconnectChan <- player.Uuid
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// Inform all players that the game has ended
func (s *Server) informAllEnd(leaaderboard []*pb.PlayResponse_Score) {
	msg := &pb.PlayResponse{
		Msg: &pb.PlayResponse_GameEndsMsg{
			GameEndsMsg: &pb.PlayResponse_GameEnds{
				Leaderboard: leaaderboard,
			},
		},
	}
	wg := sync.WaitGroup{}
	for _, player := range s.players {
		player := player
		wg.Add(1)
		go func() {
			err := player.stream.Send(msg)
			if err != nil {
				log.Printf("Error sending message to player %s: %s\n", player.Uuid, err.Error())
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// Inform all players that the game has been interrupted
func (s *Server) informAllInterrupt(reason string) {
	msg := &pb.PlayResponse{
		Msg: &pb.PlayResponse_StartInterruptedMsg{
			StartInterruptedMsg: &pb.PlayResponse_StartInterrupted{
				Reason: reason,
			},
		},
	}
	wg := sync.WaitGroup{}
	for _, player := range s.players {
		player := player
		wg.Add(1)
		go func() {
			err := player.stream.Send(msg)
			if err != nil {
				log.Printf("Error sending message: %s\n", err.Error())
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// Remove player from the server in a thread-safe way
func (s *Server) removePlayer(playerId string) {
	log.Printf("Removing player %s\n", playerId)
	s.mutex.Lock()
	delete(s.players, playerId)
	log.Printf("Remaining players: %d\n", len(s.players))
	s.mutex.Unlock()
}

// Sends question to all players
func (s *Server) sendAllQuestion(questionId string, question Question) {
	msg := &pb.PlayResponse{
		Msg: &pb.PlayResponse_QuestionMsg{
			QuestionMsg: &pb.PlayResponse_QuizQuestion{
				QuestionId:      questionId,
				Question:        question.Question,
				Answers:         shuffleAnswers(question.IncorrectAnswers, question.CorrectAnswer),
				QuestionTimeout: s.questionTimeout.Seconds(),
			},
		},
	}
	for _, player := range s.players {
		select {
		case <-player.stream.Context().Done(): // in case the player disconnects, remove them from the server
			log.Printf("Player %s disconnected\n", player.Uuid)
			s.removePlayer(player.Uuid)
			return
		default:
		}
		log.Printf("Sending question to %sn", player.Uuid)
		err := player.stream.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %s\n", err.Error())
		}
	}
}

// Returns sorted leaderboard
func (s *Server) getLeaderboard() []*pb.PlayResponse_Score {
	leaderboard := make([]*pb.PlayResponse_Score, 0, len(s.players))
	for _, p := range s.players {
		leaderboard = append(leaderboard, &pb.PlayResponse_Score{
			PlayerId: p.Uuid,
			Score:    p.Score,
		})
	}
	sort.Slice(leaderboard, func(i, j int) bool {
		return i < j
	})
	return leaderboard
}

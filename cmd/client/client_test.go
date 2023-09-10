package main

import (
	"bytes"
	"log"
	pb "pubSubQuiz/proto"
	"strings"
	"testing"
)

func Test_handleMessage(t *testing.T) {
	type args struct {
		msg    *pb.PlayResponse
		stream pb.Quiz_PlayClient
	}
	tests := []struct {
		name         string
		args         args
		wantMessages []string
	}{
		{
			name: "Test_handleMessage",
			args: args{
				msg: &pb.PlayResponse{
					Msg: &pb.PlayResponse_JoinResponseMsg{
						JoinResponseMsg: &pb.PlayResponse_JoinResponse{
							Success:        true,
							Reason:         "",
							CurrentPlayers: 50,
							TargetPlayers:  100,
							PlayerId:       "foo",
						},
					},
				},
			},
			wantMessages: []string{
				"Joined the quiz. Waiting for 50 more players",
				"",
			},
		},
		{
			name: "Test_handleMessage",
			args: args{
				msg: &pb.PlayResponse{
					Msg: &pb.PlayResponse_GameEndsMsg{
						GameEndsMsg: &pb.PlayResponse_GameEnds{
							Leaderboard: []*pb.PlayResponse_Score{
								{
									Score:    10,
									PlayerId: "foo",
								},
								{
									Score:    20,
									PlayerId: "bar",
								},
							},
						},
					},
				},
			},
			wantMessages: []string{
				"Game finished!",
				"Leaderboard:",
				"=================",
				"foo: 10",
				"bar: 20",
				"=================",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			handleMessage(tt.args.msg, tt.args.stream, nil)
			got := buf.String()
			lines := strings.Split(got, "\n")
			if len(lines) != len(tt.wantMessages) {
				t.Errorf("handleMessage() = %v, want %v", len(lines), len(tt.wantMessages))
				return
			}
			for i, line := range lines {
				if line != tt.wantMessages[i] {
					t.Errorf("handleMessage() = %v, want %v", lines[i], tt.wantMessages[i])
				}
			}
		})
	}
}

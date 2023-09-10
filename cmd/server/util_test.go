package main

import (
	"testing"
)

func Test_shuffleAnswers(t *testing.T) {
	type args struct {
		incorrectAnswers []string
		correctAnswer    string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test answers contain all available answers",
			args: args{
				incorrectAnswers: []string{"a", "b", "c"},
				correctAnswer:    "d",
			},
			want: []string{"d", "a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shuffleAnswers(tt.args.incorrectAnswers, tt.args.correctAnswer)
			if len(got) != len(tt.want) {
				t.Errorf("shuffleAnswers() = %v, want %v", got, tt.want)
				return
			}
			m := mapAvailableAnswers(tt.want)
			for _, v := range got {
				if _, ok := m[v]; !ok {
					t.Errorf("shuffleAnswers() = %v, want %v", got, tt.want)
					return
				} else {
					m[v]++
					if m[v] > 1 {
						t.Errorf("answer %s seen more than once", v)
					}
				}
			}
		})
	}
}

func mapAvailableAnswers(answers []string) map[string]int {
	m := make(map[string]int, len(answers))
	for _, v := range answers {
		m[v] = 0
	}
	return m
}

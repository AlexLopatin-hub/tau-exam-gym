package exam

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/AlexLopatin-hub/tau-exam-gym/internal/model"
)

//go:embed data/question_bank.json
var defaultQuestionBank []byte

func loadQuestions() ([]model.Question, error) {
	data := defaultQuestionBank
	if path := os.Getenv("QUESTION_BANK"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read question bank: %w", err)
		}
	}
	var questions []model.Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return nil, fmt.Errorf("parse question bank: %w", err)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("question bank is empty")
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if q.ID == "" || q.Text == "" || q.Answer == "" || q.Difficulty == "" {
			return nil, fmt.Errorf("question has an empty required field")
		}
		if seen[q.ID] {
			return nil, fmt.Errorf("duplicate question id %q", q.ID)
		}
		if difficultyRank(q.Difficulty) == 3 {
			return nil, fmt.Errorf("question %q has invalid difficulty %q", q.ID, q.Difficulty)
		}
		if q.Format != "" && q.Format != "latex" && q.Format != "mathml" {
			return nil, fmt.Errorf("question %q has invalid format %q", q.ID, q.Format)
		}
		seen[q.ID] = true
	}
	return questions, nil
}

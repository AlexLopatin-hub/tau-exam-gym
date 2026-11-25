package model

import "time"

const (
	EasyPerAttempt      = 7
	MediumPerAttempt    = 7
	HardPerAttempt      = 6
	QuestionsPerAttempt = EasyPerAttempt + MediumPerAttempt + HardPerAttempt
)

type Question struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Answer     string `json:"answer"`
	Difficulty string `json:"difficulty"`
	Format     string `json:"format,omitempty"`
}

type Attempt struct {
	ID          string
	Questions   []Question
	UserAnswers map[string]string
	Finished    bool
	FinishedAt  time.Time
	Created     time.Time
}

type Lecture struct {
	ID   string
	Date string
	HTML string
}

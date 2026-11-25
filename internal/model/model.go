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

// Difficulty — раздел каталога вопросов по уровню сложности.
type Difficulty struct {
	Slug  string
	Label string
	Value string
}

// Difficulties перечисляет разделы каталога в порядке отображения.
var Difficulties = []Difficulty{
	{Slug: "easy", Label: "Лёгкие", Value: "лёгкий"},
	{Slug: "medium", Label: "Средние", Value: "средний"},
	{Slug: "hard", Label: "Сложные", Value: "сложный"},
}

func DifficultyBySlug(slug string) (Difficulty, bool) {
	for _, difficulty := range Difficulties {
		if difficulty.Slug == slug {
			return difficulty, true
		}
	}
	return Difficulty{}, false
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

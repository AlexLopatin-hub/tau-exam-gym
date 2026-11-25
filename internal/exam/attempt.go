package exam

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"maps"
	randv2 "math/rand/v2"
	"net/url"
	"sync"
	"time"

	"github.com/AlexLopatin-hub/tau-exam-gym/internal/model"
)

var ErrInsufficientQuestions = errors.New("question bank has too few questions for a difficulty quota")
var ErrStoreFull = errors.New("attempt store is full")

const legacyQuestionsPerAttempt = 30
const maxAttempts = 2_000
const attemptLifetime = 24 * time.Hour

type AttemptStore struct {
	mu       sync.RWMutex
	attempts map[string]*model.Attempt
}

func NewAttemptStore() *AttemptStore { return &AttemptStore{attempts: make(map[string]*model.Attempt)} }

func (s *AttemptStore) Create(bank []model.Question) (*model.Attempt, error) {
	if len(bank) < model.QuestionsPerAttempt {
		return nil, ErrInsufficientQuestions
	}
	quotas := [...]int{model.EasyPerAttempt, model.MediumPerAttempt, model.HardPerAttempt}
	var groups [len(quotas)][]model.Question
	for _, index := range randv2.Perm(len(bank)) {
		question := bank[index]
		rank := difficultyRank(question.Difficulty)
		if rank < len(quotas) && len(groups[rank]) < quotas[rank] {
			groups[rank] = append(groups[rank], question)
		}
	}
	selected := make([]model.Question, 0, model.QuestionsPerAttempt)
	for rank, questions := range groups {
		if len(questions) < quotas[rank] {
			return nil, ErrInsufficientQuestions
		}
		selected = append(selected, questions...)
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.attempts) >= maxAttempts {
		for id, attempt := range s.attempts {
			if attemptExpired(attempt, now) {
				delete(s.attempts, id)
			}
		}
		if len(s.attempts) >= maxAttempts {
			return nil, ErrStoreFull
		}
	}
	a := &model.Attempt{ID: hex.EncodeToString(idBytes), Questions: selected, Created: now}
	s.attempts[a.ID] = a
	return copyAttempt(a), nil
}

func attemptExpired(a *model.Attempt, now time.Time) bool {
	return now.Sub(a.Created) >= attemptLifetime
}

func (s *AttemptStore) Get(id string) (*model.Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.attempts[id]
	if !ok || attemptExpired(a, time.Now()) {
		return nil, false
	}
	return copyAttempt(a), true
}

func (s *AttemptStore) Finish(id string, answers url.Values) (*model.Attempt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.attempts[id]
	if !ok || attemptExpired(a, time.Now()) {
		return nil, false
	}
	if !a.Finished {
		a.UserAnswers = make(map[string]string)
		for _, question := range a.Questions {
			if answer := answers.Get(question.ID); answer != "" {
				a.UserAnswers[question.ID] = answer
			}
		}
		a.Finished = true
		a.FinishedAt = time.Now()
	}
	return copyAttempt(a), true
}

func copyAttempt(a *model.Attempt) *model.Attempt {
	copy := *a
	copy.Questions = append([]model.Question(nil), a.Questions...)
	copy.UserAnswers = maps.Clone(a.UserAnswers)
	return &copy
}

func difficultyRank(difficulty string) int {
	switch difficulty {
	case "лёгкий":
		return 0
	case "средний":
		return 1
	case "сложный":
		return 2
	default:
		return 3
	}
}

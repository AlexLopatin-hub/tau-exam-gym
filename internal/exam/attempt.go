package exam

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"sync"
	"time"
)

var ErrInsufficientQuestions = errors.New("each topic needs at least 10 questions")

type Attempt struct {
	ID        string
	Questions []Question
	Finished  bool
	Created   time.Time
}

type AttemptStore struct {
	mu       sync.RWMutex
	attempts map[string]*Attempt
}

func NewAttemptStore() *AttemptStore { return &AttemptStore{attempts: make(map[string]*Attempt)} }

func (s *AttemptStore) Create(bank []Question) (*Attempt, error) {
	byTopic := map[string][]Question{}
	for _, q := range bank {
		byTopic[q.Topic] = append(byTopic[q.Topic], q)
	}
	if len(byTopic) != 3 {
		return nil, ErrInsufficientQuestions
	}
	selected := make([]Question, 0, 30)
	for _, topicQuestions := range byTopic {
		if len(topicQuestions) < 10 {
			return nil, ErrInsufficientQuestions
		}
		for i := range 10 {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(topicQuestions)-i)))
			if err != nil {
				return nil, err
			}
			j := i + int(n.Int64())
			topicQuestions[i], topicQuestions[j] = topicQuestions[j], topicQuestions[i]
			selected = append(selected, topicQuestions[i])
		}
	}
	for i := len(selected) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		j := int(n.Int64())
		selected[i], selected[j] = selected[j], selected[i]
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Topic != selected[j].Topic {
			return selected[i].Topic < selected[j].Topic
		}
		return difficultyRank(selected[i].Difficulty) < difficultyRank(selected[j].Difficulty)
	})
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	a := &Attempt{ID: hex.EncodeToString(idBytes), Questions: selected, Created: time.Now()}
	s.mu.Lock()
	s.attempts[a.ID] = a
	s.mu.Unlock()
	return copyAttempt(a), nil
}

func (s *AttemptStore) Get(id string) (*Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.attempts[id]
	if !ok {
		return nil, false
	}
	return copyAttempt(a), true
}

func (s *AttemptStore) Finish(id string) (*Attempt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.attempts[id]
	if ok {
		a.Finished = true
		return copyAttempt(a), true
	}
	return nil, false
}

func copyAttempt(a *Attempt) *Attempt {
	copy := *a
	copy.Questions = append([]Question(nil), a.Questions...)
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

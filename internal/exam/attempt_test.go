package exam

import (
	"slices"
	"testing"
	"time"
)

func TestDefaultBankAndStableAttempt(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	bank, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	if len(bank) != 60 {
		t.Fatalf("got %d questions in the imported bank, want 60", len(bank))
	}
	store := NewAttemptStore()
	a, err := store.Create(bank)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Questions) != 20 {
		t.Fatalf("got %d questions", len(a.Questions))
	}
	ids := map[string]bool{}
	var counts [3]int
	lastRank := -1
	for _, q := range a.Questions {
		if ids[q.ID] {
			t.Fatalf("duplicate question %q", q.ID)
		}
		ids[q.ID] = true
		rank := difficultyRank(q.Difficulty)
		counts[rank]++
		if rank < lastRank {
			t.Fatalf("question %q is out of difficulty order", q.ID)
		}
		lastRank = rank
	}
	if counts != [3]int{7, 7, 6} {
		t.Fatalf("got difficulty counts %v, want [7 7 6]", counts)
	}
	stored, ok := store.Get(a.ID)
	if !ok || stored.ID != a.ID || !slices.Equal(stored.Questions, a.Questions) {
		t.Fatal("attempt was not stable")
	}
	if _, ok := store.Finish(a.ID, nil); !ok {
		t.Fatal("finish failed")
	}
	finished, ok := store.Finish(a.ID, nil)
	if !ok || !finished.Finished {
		t.Fatal("finish not idempotent")
	}
}

func TestCreateRejectsBankMissingDifficultyQuota(t *testing.T) {
	store := NewAttemptStore()
	if _, err := store.Create(catalogTestBank(60)); err != ErrInsufficientQuestions {
		t.Fatalf("got %v, want insufficient questions", err)
	}
	if len(store.attempts) != 0 {
		t.Fatal("short bank created a partial attempt")
	}
}

func TestFullStoreReclaimsOnlyExpiredAttempts(t *testing.T) {
	store := NewAttemptStore()
	bank := catalogTestBank(questionsPerAttempt)
	for i := easyPerAttempt; i < easyPerAttempt+mediumPerAttempt; i++ {
		bank[i].Difficulty = "средний"
	}
	for i := easyPerAttempt + mediumPerAttempt; i < len(bank); i++ {
		bank[i].Difficulty = "сложный"
	}
	var id string
	for i := range maxAttempts {
		a, err := store.Create(bank)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		id = a.ID
	}
	if _, err := store.Create(bank); err != ErrStoreFull {
		t.Fatalf("full store: got %v", err)
	}
	store.attempts[id].Created = time.Now().Add(-attemptLifetime)
	if _, ok := store.Get(id); ok {
		t.Fatal("expired attempt is visible")
	}
	if _, ok := store.Finish(id, nil); ok {
		t.Fatal("expired attempt can be finished")
	}
	if _, err := store.Create(bank); err != nil {
		t.Fatal(err)
	}
	if len(store.attempts) != maxAttempts || store.attempts[id] != nil {
		t.Fatal("new attempt must take the expired attempt's place")
	}
}

package exam

import (
	"fmt"
	"testing"
	"time"
)

func TestDemoBankAndStableAttempt(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	bank, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	store := NewAttemptStore()
	a, err := store.Create(bank)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Questions) != 30 {
		t.Fatalf("got %d questions", len(a.Questions))
	}
	counts := map[string]int{}
	ids := map[string]bool{}
	for _, q := range a.Questions {
		counts[q.Topic]++
		if ids[q.ID] {
			t.Fatalf("duplicate question %q", q.ID)
		}
		ids[q.ID] = true
	}
	if len(counts) != 3 {
		t.Fatalf("got %d topics", len(counts))
	}
	for topic, count := range counts {
		if count != 10 {
			t.Fatalf("%s: got %d", topic, count)
		}
	}
	stored, ok := store.Get(a.ID)
	if !ok || stored.ID != a.ID || stored.Questions[0].ID != a.Questions[0].ID {
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

func TestCreateRejectsShortTopic(t *testing.T) {
	if _, err := NewAttemptStore().Create(demoQuestions[:29]); err == nil {
		t.Fatal("expected insufficient questions")
	}
}

func TestAttemptQuestionsAreGroupedAndDifficultySorted(t *testing.T) {
	a, err := NewAttemptStore().Create(demoQuestions)
	if err != nil {
		t.Fatal(err)
	}
	lastTopic := ""
	lastRank := -1
	for _, q := range a.Questions {
		if q.Topic != lastTopic {
			if lastTopic != "" && q.Topic < lastTopic {
				t.Fatalf("topics are not ordered: %q after %q", q.Topic, lastTopic)
			}
			lastTopic, lastRank = q.Topic, -1
		}
		rank := difficultyRank(q.Difficulty)
		if rank < lastRank {
			t.Fatalf("difficulty order changed in %q", q.Topic)
		}
		lastRank = rank
	}
}

func TestFullStoreReclaimsOnlyExpiredAttempts(t *testing.T) {
	store := NewAttemptStore()
	for i := range maxAttempts {
		id := fmt.Sprint(i)
		store.attempts[id] = &Attempt{ID: id, Created: time.Now()}
	}
	if _, err := store.Create(demoQuestions); err != ErrStoreFull {
		t.Fatalf("full store: got %v", err)
	}
	store.attempts["0"].Created = time.Now().Add(-attemptLifetime)
	if _, ok := store.Get("0"); ok {
		t.Fatal("expired attempt is visible")
	}
	if _, ok := store.Finish("0", nil); ok {
		t.Fatal("expired attempt can be finished")
	}
	if _, err := store.Create(demoQuestions); err != nil {
		t.Fatal(err)
	}
	if len(store.attempts) != maxAttempts || store.attempts["0"] != nil {
		t.Fatal("new attempt must take the expired attempt's place")
	}
}

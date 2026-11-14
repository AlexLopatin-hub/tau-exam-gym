package exam

import (
	"bytes"
	"context"
	"github.com/a-h/templ"
	"regexp"
	"strings"
	"testing"
)

func TestPageUsesSingleExactProductHeading(t *testing.T) {
	body := renderHTML(t, Page(PageView{Title: "Экзамен", Tab: "exam"}))
	if !strings.Contains(body, "<title>Вопросы по ТАУ</title>") {
		t.Fatal("document title is missing")
	}
	if strings.Count(body, "<h1>Вопросы по ТАУ</h1>") != 1 {
		t.Fatal("expected one exact h1")
	}
	if strings.Count(body, "Вопросы по ТАУ") != 2 {
		t.Fatal("product name should appear only in title and h1")
	}
}

func TestCatalogUsesCommonLocalDisclosureCard(t *testing.T) {
	bank, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	body := renderHTML(t, Page(PageView{Tab: "questions", Questions: bank[:1]}))
	if !strings.Contains(body, `<article class="card" id="question-`+bank[0].ID+`">`) {
		t.Fatal("catalog does not render the shared question card")
	}
	if !strings.Contains(body, `<summary class="answer-toggle">`) || !strings.Contains(body, "Показать ответ") || !strings.Contains(body, "Скрыть ответ") {
		t.Fatal("catalog card lacks native show/hide controls")
	}
	if !strings.Contains(body, bank[0].Answer) {
		t.Fatal("catalog answer was not preloaded")
	}
	if strings.Contains(body, "hx-post") || strings.Contains(body, "/reveal") || strings.Contains(body, "/hide") {
		t.Fatal("catalog disclosure should not make requests")
	}
}

func TestActiveAttemptAnswersArePreloadedAndStoredLocally(t *testing.T) {
	bank, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	store := NewAttemptStore()
	a, err := store.Create(bank)
	if err != nil {
		t.Fatal(err)
	}
	active := renderHTML(t, Page(PageView{Attempt: a, Blocks: groupQuestions(a)}))
	assertAttemptDetails(t, active, a)
	if !strings.Contains(active, "sessionStorage") || !strings.Contains(active, "tau-exam:active:${node.dataset.attemptId}:${node.dataset.questionNumber}") {
		t.Fatal("attempt disclosure state is not restored locally")
	}
	if strings.Contains(active, "hx-post") || strings.Contains(active, "/reveal") || strings.Contains(active, "/hide") {
		t.Fatal("attempt disclosure should not make requests")
	}

	finished, ok := store.Finish(a.ID)
	if !ok {
		t.Fatal("finish failed")
	}
	body := renderHTML(t, Page(PageView{Attempt: finished, Blocks: groupQuestions(finished)}))
	assertFinishedAttemptAnswers(t, body, finished)
}

func assertAttemptDetails(t *testing.T, body string, attempt *Attempt) {
	t.Helper()
	tags := regexp.MustCompile(`<details[^>]*>`).FindAllString(body, -1)
	if len(tags) != 30 {
		t.Fatalf("got %d attempt details, want 30", len(tags))
	}
	if !strings.Contains(body, attempt.Questions[0].Answer) {
		t.Fatal("answer should be present in the preloaded markup")
	}
	for _, tag := range tags {
		if !strings.Contains(tag, `data-attempt-id="`+attempt.ID+`"`) {
			t.Fatal("question is missing its attempt ID")
		}
		if strings.Contains(tag, " open") {
			t.Fatalf("wrong initial disclosure state in %s", tag)
		}
	}
}

func assertFinishedAttemptAnswers(t *testing.T, body string, attempt *Attempt) {
	t.Helper()
	if got := strings.Count(body, `<div class="answer revealed">`); got != len(attempt.Questions) {
		t.Fatalf("got %d static answers, want %d", got, len(attempt.Questions))
	}
	if strings.Contains(body, `<details class="catalog-answer"`) || strings.Contains(body, "Показать ответ") || strings.Contains(body, "Скрыть ответ") {
		t.Fatal("finished answer must not be collapsible")
	}
	for _, question := range attempt.Questions {
		if !strings.Contains(body, question.Answer) {
			t.Fatalf("finished answer %q is missing", question.ID)
		}
	}
}

func renderHTML(t *testing.T, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	if err := component.Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

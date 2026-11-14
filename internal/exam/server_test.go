package exam

import (
	"bytes"
	"context"
	"fmt"
	"github.com/a-h/templ"
	"net/http"
	"net/http/httptest"
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

func TestQuestionCatalogPagesTenInSourceOrder(t *testing.T) {
	withQuestionBank(t, catalogTestBank(21))
	first := requestQuestions(t, "/questions", false, false)
	if first.Code != http.StatusOK {
		t.Fatalf("default page status: %d", first.Code)
	}
	assertQuestionIDs(t, first.Body.String(), 1, 10)
	if !strings.Contains(first.Body.String(), `class="pagination"`) {
		t.Fatal("multi-page bank is missing navigation")
	}
	if strings.Index(first.Body.String(), `class="pagination"`) < strings.LastIndex(first.Body.String(), `id="question-q10"`) {
		t.Fatal("paginator should follow the questions list")
	}

	second := requestQuestions(t, "/questions?page=2", true, false)
	if second.Code != http.StatusOK {
		t.Fatalf("page 2 status: %d", second.Code)
	}
	if strings.Contains(second.Body.String(), "<html") {
		t.Fatal("HTMX request should return the main fragment")
	}
	assertQuestionIDs(t, second.Body.String(), 11, 20)
	for _, expected := range []string{`href="/questions?page=1"`, `hx-get="/questions?page=1"`, `hx-push-url="true"`, `Страница 2 из 3`, `hx-swap="outerHTML show:#question-results:top"`} {
		if !strings.Contains(second.Body.String(), expected) {
			t.Fatalf("page 2 is missing %q", expected)
		}
	}
	if strings.Index(second.Body.String(), `id="question-results"`) > strings.Index(second.Body.String(), `class="pagination"`) {
		t.Fatal("paginator should follow the questions list")
	}

	last := requestQuestions(t, "/questions?page=3", true, false)
	if last.Code != http.StatusOK {
		t.Fatalf("page 3 status: %d", last.Code)
	}
	assertQuestionIDs(t, last.Body.String(), 21, 21)
	if !strings.Contains(last.Body.String(), `href="/questions?page=2"`) || strings.Contains(last.Body.String(), `href="/questions?page=4"`) {
		t.Fatal("last page navigation has a dead link")
	}
}

func TestQuestionCatalogCanJumpFromFirstToFourthAndBack(t *testing.T) {
	withQuestionBank(t, catalogTestBank(40))
	first := requestQuestions(t, "/questions", true, false)
	if first.Code != http.StatusOK {
		t.Fatalf("page 1 status: %d", first.Code)
	}
	if !strings.Contains(first.Body.String(), `<a class="page-number" aria-label="Перейти на страницу 4" href="/questions?page=4"`) {
		t.Fatal("page 1 has no direct link to page 4")
	}
	if !strings.Contains(first.Body.String(), `<span class="page-number" aria-current="page" aria-label="Страница 1">1</span>`) {
		t.Fatal("current page should be a non-link with aria-current")
	}
	if strings.Contains(first.Body.String(), `href="/questions?page=1"`) {
		t.Fatal("current page should not be a link")
	}
	css := renderHTML(t, Page(PageView{Tab: "questions", Questions: bank[:10], CurrentPage: 1, TotalPages: 4}))
	if !strings.Contains(css, "ResizeObserver") || !strings.Contains(css, "window.addEventListener(\"resize\"") || !strings.Contains(css, "document.fonts.ready") || !strings.Contains(css, "htmx:afterSwap") || !strings.Contains(css, "htmx:historyRestore") {
		t.Fatal("pagination does not reevaluate after resize, font load, or history replacement")
	}
	if !strings.Contains(css, "flex-wrap: nowrap") || !strings.Contains(css, ".compact-hidden") || !strings.Contains(css, "overflow: hidden") {
		t.Fatal("pagination is missing its one-line compact fallback")
	}
	if !strings.Contains(first.Body.String(), `class="page-ellipsis" data-needed="true"`) {
		t.Fatal("compact pagination must show an ellipsis when pages are skipped")
	}
	if !strings.Contains(first.Body.String(), `aria-label="Назад"><span class="edge-label">Назад</span>`) || !strings.Contains(first.Body.String(), `aria-label="Далее" href="/questions?page=2"`) {
		t.Fatal("both edge controls must remain visible in compact mode")
	}
	if !strings.Contains(first.Body.String(), `class="page-edge" aria-disabled="true" aria-label="Назад"`) || strings.Contains(first.Body.String(), `href="/questions?page=1"`) {
		t.Fatal("first-page previous control must be disabled without a dead link")
	}
	if !strings.Contains(first.Body.String(), `class="edge-link" aria-label="Далее" href="/questions?page=2"`) {
		t.Fatal("next edge link is missing")
	}

	fourth := requestQuestions(t, "/questions?page=4", true, false)
	if fourth.Code != http.StatusOK {
		t.Fatalf("page 4 status: %d", fourth.Code)
	}
	assertQuestionIDs(t, fourth.Body.String(), 31, 40)
	if !strings.Contains(fourth.Body.String(), `href="/questions?page=1"`) || !strings.Contains(fourth.Body.String(), `href="/questions?page=3"`) {
		t.Fatal("page 4 does not link back to earlier pages")
	}
	if strings.Contains(fourth.Body.String(), `href="/questions?page=4"`) {
		t.Fatal("current page should not link to itself")
	}
	if !strings.Contains(fourth.Body.String(), `class="page-edge" aria-disabled="true" aria-label="Далее"`) {
		t.Fatal("last-page next control must be disabled without a dead link")
	}
}

func TestCompactPagerKeepsFirstLastCurrentNeighborsAndBothEllipses(t *testing.T) {
	withQuestionBank(t, catalogTestBank(300))
	recorder := requestQuestions(t, "/questions?page=15", true, false)
	body := recorder.Body.String()
	for _, page := range []string{"1", "14", "16", "30"} {
		if !strings.Contains(body, `href="/questions?page=`+page+`"`) {
			t.Errorf("compact pager is missing page %s", page)
		}
	}
	if !strings.Contains(body, `<span class="page-number" aria-current="page" aria-label="Страница 15">15</span>`) {
		t.Fatal("current page is missing")
	}
	if strings.Contains(body, `href="/questions?page=15"`) {
		t.Fatal("current page should not be a link")
	}
	if strings.Count(body, `class="page-ellipsis" data-needed="true"`) != 2 {
		t.Fatal("compact pager must show ellipses on both skipped ranges")
	}
	if !strings.Contains(body, `href="/questions?page=14"`) || !strings.Contains(body, `href="/questions?page=16"`) {
		t.Fatal("previous and next numeric links are missing")
	}
	css := renderHTML(t, Page(PageView{Tab: "questions", CurrentPage: 15, TotalPages: 30}))
	if !strings.Contains(css, `@media (max-width: 480px)`) || !strings.Contains(css, `.pagination:not(.show-edge-labels) .edge-label`) || !strings.Contains(css, `.pagination.compact-icons .edge-icon`) || !strings.Contains(css, `nav.scrollWidth > nav.clientWidth`) {
		t.Fatal("narrow-screen edge controls do not adapt to measured width")
	}
}

func TestQuestionCatalogRejectsInvalidPage(t *testing.T) {
	withQuestionBank(t, catalogTestBank(20))
	for _, value := range []string{"abc", "0", "-1", "3", ""} {
		recorder := requestQuestions(t, "/questions?page="+value, false, false)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("page %q: got status %d", value, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), "Страница каталога не найдена.") {
			t.Errorf("page %q: missing error message", value)
		}
	}
}

func TestQuestionCatalogOneOrZeroPagesHasNoNavigation(t *testing.T) {
	withQuestionBank(t, catalogTestBank(10))
	one := requestQuestions(t, "/questions", false, false)
	if one.Code != http.StatusOK {
		t.Fatalf("one page status: %d", one.Code)
	}
	if strings.Contains(one.Body.String(), `class="pagination"`) || strings.Contains(one.Body.String(), "Страница 1 из") {
		t.Fatal("single page should not show pagination")
	}
	assertQuestionIDs(t, one.Body.String(), 1, 10)

	withQuestionBank(t, nil)
	empty := requestQuestions(t, "/questions", false, false)
	if empty.Code != http.StatusOK {
		t.Fatalf("empty bank status: %d", empty.Code)
	}
	if !strings.Contains(empty.Body.String(), "В банке пока нет вопросов.") {
		t.Fatal("empty first page should be clear")
	}
	if strings.Contains(empty.Body.String(), `class="pagination"`) {
		t.Fatal("empty first page should not show pagination")
	}
	if ids := questionIDs(empty.Body.String()); len(ids) != 0 {
		t.Fatalf("empty page contains questions: %v", ids)
	}
}

func TestQuestionCatalogHistoryRestoreReturnsFullPage(t *testing.T) {
	withQuestionBank(t, catalogTestBank(15))
	recorder := requestQuestions(t, "/questions?page=2", true, true)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "<html") {
		t.Fatal("HTMX history restore should return a full document")
	}
	assertQuestionIDs(t, recorder.Body.String(), 11, 15)
}

func withQuestionBank(t *testing.T, questions []Question) {
	t.Helper()
	old := bank
	bank = questions
	t.Cleanup(func() { bank = old })
}

func catalogTestBank(count int) []Question {
	questions := make([]Question, count)
	for i := range count {
		questions[i] = Question{ID: fmt.Sprintf("q%02d", i+1), Text: fmt.Sprintf("Question %02d", i+1), Answer: fmt.Sprintf("Answer %02d", i+1), Topic: "Тема", Difficulty: "лёгкий"}
	}
	return questions
}

func requestQuestions(t *testing.T, target string, hx, historyRestore bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if historyRestore {
		req.Header.Set("HX-History-Restore-Request", "true")
	}
	recorder := httptest.NewRecorder()
	questions(recorder, req)
	return recorder
}

func questionIDs(body string) []string {
	matches := regexp.MustCompile(`<article class="card" id="question-(q\d+)">`).FindAllStringSubmatch(body, -1)
	ids := make([]string, len(matches))
	for i, match := range matches {
		ids[i] = match[1]
	}
	return ids
}

func assertQuestionIDs(t *testing.T, body string, first, last int) {
	t.Helper()
	ids := questionIDs(body)
	if len(ids) != last-first+1 {
		t.Fatalf("got %d cards, want %d: %v", len(ids), last-first+1, ids)
	}
	for i, id := range ids {
		if want := fmt.Sprintf("q%02d", first+i); id != want {
			t.Fatalf("position %d: got %s, want %s", i, id, want)
		}
	}
}

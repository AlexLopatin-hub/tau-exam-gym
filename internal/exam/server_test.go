package exam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/AlexLopatin-hub/tau-exam-gym/internal/model"
	"github.com/AlexLopatin-hub/tau-exam-gym/internal/view"
	"github.com/a-h/templ"
)

func TestPageUsesSingleExactProductHeading(t *testing.T) {
	body := renderHTML(t, views.Page(views.PageView{Title: "Экзамен", Tab: "exam"}))
	if !strings.Contains(body, "<title>Вопросы по ТАУ</title>") {
		t.Fatal("document title is missing")
	}
	if strings.Count(body, "<h1>Вопросы по ТАУ</h1>") != 1 {
		t.Fatal("expected one exact h1")
	}
	if strings.Count(body, "Вопросы по ТАУ") != 2 {
		t.Fatal("product name should appear only in title and h1")
	}
	if !strings.Contains(body, "/dist/katex.min.css") || !strings.Contains(body, "/dist/katex.min.js") ||
		!strings.Contains(body, "/dist/contrib/auto-render.min.js") {
		t.Fatal("page is missing the client-side KaTeX assets")
	}
}

func TestLectureRoutes(t *testing.T) {
	handler := routes()
	items, err := lectures()
	if err != nil || len(items) == 0 {
		t.Fatalf("load lecture menu: %v", err)
	}
	latest := items[len(items)-1]
	const scrollableTable = `<div class="lecture-table" role="region" aria-label="Таблица лекции" tabindex="0"><table>`
	for _, tc := range []struct {
		path     string
		fragment bool
		status   int
		want     []string
	}{
		{"/lectures", false, http.StatusOK, []string{`<html lang="ru">`, "<h2>Лекция " + latest.Date + "</h2>", `href="/lectures/10-17"`, `hx-get="/lectures/10-17"`, ">17 октября</a>", ">5 сентября</a>", `class="lecture-sidebar"`, `<details class="lecture-picker">`}},
		{"/lectures/09-26", true, http.StatusOK, []string{`<code class="language-mermaid">`, "flowchart TB", `&lt;b&gt;ВХОД-ВЫХОД&lt;/b&gt;&lt;br/&gt;`, `<div class="latex-source">$$\dot{x}=Ax+Bu$$</div>`, scrollableTable, "</table></div>"}},
		{"/lectures/10-24", true, http.StatusOK, []string{scrollableTable, "</table></div>", "Непрерывный случай", "Дискретный случай"}},
		{"/lectures/10-10", true, http.StatusOK, []string{`<main id="content"`, "Лекция 10 октября", "<strong>Вопросы к экзамену:</strong>", `$$\begin{equation*}`, `+\infty, &amp;\text{$t=0$}`}},
		{"/lectures/12-31", false, http.StatusNotFound, []string{"Лекция с такой датой не найдена.", `href="/lectures/10-17"`}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.fragment {
				req.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d", w.Code, tc.status)
			}
			body := w.Body.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("response missing %q", want)
				}
			}
			if tc.fragment && strings.Contains(body, "<html") {
				t.Error("htmx response contains the full HTML document")
			}
			if tc.path == "/lectures" && strings.Index(body, ">5 сентября</a>") >= strings.Index(body, ">17 октября</a>") {
				t.Error("lecture menu is not chronological")
			}
		})
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/lectures/10.17", nil))
	if w.Code != http.StatusPermanentRedirect || w.Header().Get("Location") != "/lectures/10-17" {
		t.Errorf("old lecture link: status %d, Location %q", w.Code, w.Header().Get("Location"))
	}
}

func TestEmbeddedLecturesContainHTML(t *testing.T) {
	items, err := lectures()
	if err != nil {
		t.Fatal(err)
	}
	for _, lecture := range items {
		t.Run(lecture.ID, func(t *testing.T) {
			if strings.TrimSpace(lecture.HTML) == "" {
				t.Error("lecture rendered empty HTML")
			}
			if strings.Contains(lecture.HTML, "<math") {
				t.Error("lecture must send LaTeX for client-side rendering")
			}
		})
	}
}

func TestExamRoutesRejectCrossOriginChanges(t *testing.T) {
	handler := routes()
	for _, path := range []string{"/exams", "/exams/example/finish"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", path, w.Code)
		}
	}
}

func TestAttemptPagesDisableAutomaticCaching(t *testing.T) {
	for _, view := range []views.PageView{
		{Attempt: &model.Attempt{}}, {Attempt: &model.Attempt{Finished: true}}, {ArchivedID: "example"},
	} {
		for _, fragment := range []string{"false", "true"} {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("HX-Request", fragment)
			w := httptest.NewRecorder()
			render(w, req, view, http.StatusOK)
			if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `hx-history="false"`) {
				t.Fatal("attempt page may be stored in HTTP or htmx navigation cache")
			}
		}
	}
}

func TestCatalogUsesCommonLocalDisclosureCard(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	bank, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	body := renderHTML(t, views.Page(views.PageView{Tab: "questions", Questions: bank[:1]}))
	if strings.Contains(body, "<textarea") {
		t.Fatal("catalog contains an exam answer field")
	}
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

func TestImportedQuestionContent(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	questions, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	withQuestionBank(t, questions)
	handler := routes()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/questions?page=2", nil))
	if body := page.Body.String(); page.Code != http.StatusOK ||
		!strings.Contains(body, `class="latex-source">`) || !strings.Contains(body, `\frac{`) ||
		!strings.Contains(body, `$$H(t)=\begin{cases}0, &amp; t&lt;0`) || strings.Contains(body, "<math") {
		t.Fatal("the imported catalog should send LaTeX for client-side rendering")
	}
	if got := renderHTML(t, views.AnswerBlock(model.Question{Answer: `<math><mi>x</mi><mo>&gt;</mo><mn>0</mn></math><img src="other.png" onerror="bad()"><mi onclick="bad()">y</mi>`, Format: "mathml"})); !strings.Contains(got, "<math><mi>x</mi><mo>&gt;</mo><mn>0</mn></math>") || strings.Contains(got, "<img") || strings.Contains(got, "<mi onclick") {
		t.Fatalf("unsafe or invalid formula output: %s", got)
	}
}

func TestCatalogQuestionFormulas(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	questions, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	withQuestionBank(t, questions)
	response := requestQuestions(t, "/questions?page=6", false, false)
	body := response.Body.String()
	for _, markup := range []string{
		`</h2><p class="question-givens latex-source">$`,
		`S^{-1}`,
		`x_{0}`,
		`$, $`,
		`при $t\ge0$`,
	} {
		if !strings.Contains(body, markup) {
			t.Fatalf("catalog question is missing formula markup %q", markup)
		}
	}
	fractions := requestQuestions(t, "/questions?page=5", false, false).Body.String()
	if !strings.Contains(fractions, `</h2><p class="question-givens latex-source">$$`) {
		t.Fatal("a displayed fraction should follow the question title in the givens")
	}
}

func TestArchiveRendersLegacyQuestionsWithoutTopicLabels(t *testing.T) {
	question := json.RawMessage(`{"id":"a01","text":"Первый вопрос","answer":"Правильный ответ","topic":"Основы ТАУ","difficulty":"лёгкий"}`)
	questions := make([]json.RawMessage, 30)
	for i := range questions {
		questions[i] = question
	}
	payload, err := json.Marshal(map[string]any{
		"Finished": true, "FinishedAt": "2026-01-01T12:00:00Z", "Questions": questions,
		"UserAnswers": map[string]string{"a01": "Сохранённый ответ"},
	})
	if err != nil {
		t.Fatal(err)
	}
	archive := httptest.NewRecorder()
	routes().ServeHTTP(archive, httptest.NewRequest(http.MethodPost, "/archive/render", bytes.NewReader(payload)))
	body := archive.Body.String()
	if archive.Code != http.StatusOK || strings.Contains(body, "Основы ТАУ") ||
		strings.Count(body, "Первый вопрос") != len(questions) || !strings.Contains(body, "Сохранённый ответ") ||
		!strings.Contains(body, "<p>Правильный ответ</p>") {
		t.Fatal("legacy archive should retain its questions and answers")
	}
}

func TestActiveAttemptAnswersArePreloadedAndStoredLocally(t *testing.T) {
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
	active := renderHTML(t, views.Page(views.PageView{Attempt: a}))
	assertAttemptDetails(t, active, a)
	for i := range a.Questions {
		key := fmt.Sprintf(`data-draft-key="tau-exam:active:%s:%d:draft"`, a.ID, i+1)
		if strings.Count(active, key) != 1 {
			t.Fatalf("expected one answer field for draft %s", key)
		}
	}
	if strings.Contains(active, "hx-post") || strings.Contains(active, "/reveal") || strings.Contains(active, "/hide") {
		t.Fatal("attempt disclosure should not make requests")
	}

	finished, ok := store.Finish(a.ID, nil)
	if !ok {
		t.Fatal("finish failed")
	}
	body := renderHTML(t, views.Page(views.PageView{Attempt: finished}))
	assertFinishedAttemptAnswers(t, body, finished)
}

func TestFinishSubmitsAndFreezesAnswers(t *testing.T) {
	t.Setenv("QUESTION_BANK", "")
	questions, err := loadQuestions()
	if err != nil {
		t.Fatal(err)
	}
	a, err := attempts.Create(questions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { delete(attempts.attempts, a.ID) })
	show := func() string {
		req := httptest.NewRequest(http.MethodGet, "/exams/"+a.ID, nil)
		req.SetPathValue("id", a.ID)
		w := httptest.NewRecorder()
		showAttempt(w, req)
		return w.Body.String()
	}
	active := show()
	oversized := httptest.NewRequest(http.MethodPost, "/exams/"+a.ID+"/finish", strings.NewReader("q="+strings.Repeat("x", maxAnswerFormBytes)))
	oversized.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oversized.SetPathValue("id", a.ID)
	rejected := httptest.NewRecorder()
	finishAttempt(rejected, oversized)
	if stored, ok := attempts.Get(a.ID); rejected.Code != http.StatusRequestEntityTooLarge || !ok || stored.Finished {
		t.Fatal("oversized submission must leave the attempt open for retry")
	}
	for _, q := range a.Questions {
		if !strings.Contains(active, `name="`+q.ID+`" form="finish-exam"`) {
			t.Fatalf("missing answer field for %s", q.ID)
		}
	}
	for _, answer := range []string{"Строка 1\n<пример>", "Замена"} {
		form := url.Values{a.Questions[0].ID: {answer}}
		req := httptest.NewRequest(http.MethodPost, "/exams/"+a.ID+"/finish", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetPathValue("id", a.ID)
		w := httptest.NewRecorder()
		finishAttempt(w, req)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("finish status %d", w.Code)
		}
		result := show()
		data := regexp.MustCompile(`(?s)<script id="completed-attempt" type="application/json">(.*?)</script>`).FindStringSubmatch(result)
		var snapshot model.Attempt
		if len(data) != 2 || json.Unmarshal([]byte(data[1]), &snapshot) != nil ||
			snapshot.ID != a.ID || snapshot.FinishedAt.IsZero() || !slices.Equal(snapshot.Questions, a.Questions) ||
			snapshot.UserAnswers[a.Questions[0].ID] != "Строка 1\n<пример>" {
			t.Fatal("archive snapshot must include the original ordered questions and submitted answers")
		}
		if !strings.Contains(result, "Строка 1\n&lt;пример&gt;") || strings.Contains(result, "<textarea") || strings.Count(result, "Ответ не указан.") != len(a.Questions)-1 {
			t.Fatal("finished page must retain the original submission and show unanswered questions")
		}
		// The archive must also work after the live attempt is gone.
		snapshot.ID = "archived-only"
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		archive := httptest.NewRecorder()
		routes().ServeHTTP(archive, httptest.NewRequest(http.MethodPost, "/archive/render", bytes.NewReader(payload)))
		if archive.Code != http.StatusOK || archive.Body.String() != renderHTML(t, views.AttemptQuestions(&snapshot)) ||
			archive.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("archived snapshot should render the original result as an uncached fragment")
		}
		if _, ok := attempts.Get(snapshot.ID); ok {
			t.Fatal("rendering an archived attempt must not store it")
		}
	}
}

func assertAttemptDetails(t *testing.T, body string, attempt *model.Attempt) {
	t.Helper()
	tags := regexp.MustCompile(`<details[^>]*>`).FindAllString(body, -1)
	if len(tags) != len(attempt.Questions) {
		t.Fatalf("got %d attempt details, want %d", len(tags), len(attempt.Questions))
	}
	first := attempt.Questions[0]
	if !strings.Contains(body, renderHTML(t, views.AnswerBlock(first))) {
		t.Fatal("answer should be present in the preloaded markup")
	}
	for _, tag := range tags {
		if strings.Contains(tag, " open") {
			t.Fatalf("wrong initial disclosure state in %s", tag)
		}
	}
}

func assertFinishedAttemptAnswers(t *testing.T, body string, attempt *model.Attempt) {
	t.Helper()
	if got := strings.Count(body, `<div class="answer revealed">`); got != len(attempt.Questions) {
		t.Fatalf("got %d static answers, want %d", got, len(attempt.Questions))
	}
	if strings.Contains(body, `<details class="catalog-answer"`) || strings.Contains(body, "Показать ответ") || strings.Contains(body, "Скрыть ответ") {
		t.Fatal("finished answer must not be collapsible")
	}
	for _, question := range attempt.Questions {
		if !strings.Contains(body, renderHTML(t, views.AnswerBlock(question))) {
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
	if !strings.Contains(second.Body.String(), `class="edge-link" aria-label="Назад" href="/questions?page=1"`) || !strings.Contains(second.Body.String(), `class="edge-link" aria-label="Далее" href="/questions?page=3"`) {
		t.Fatal("intermediate page should render both edge links")
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
	js := requestAsset(t, "/static/app.js", "text/javascript", "application/javascript")
	if !strings.Contains(js, "window.addEventListener(\"resize\"") || !strings.Contains(js, "document.fonts.ready") || !strings.Contains(js, "htmx:afterSwap") || !strings.Contains(js, "htmx:historyRestore") {
		t.Fatal("pagination does not reevaluate after resize, font load, or history replacement")
	}
	css := requestAsset(t, "/static/app.css", "text/css")
	if !strings.Contains(css, "flex-wrap: nowrap") || !strings.Contains(css, ".compact-hidden") || !strings.Contains(css, "overflow: hidden") {
		t.Fatal("pagination is missing its one-line compact fallback")
	}
	if !strings.Contains(first.Body.String(), `class="page-ellipsis" data-needed="true"`) {
		t.Fatal("compact pagination must show an ellipsis when pages are skipped")
	}
	if strings.Contains(first.Body.String(), `aria-label="Назад"`) || !strings.Contains(first.Body.String(), `aria-label="Далее" href="/questions?page=2"`) {
		t.Fatal("first page should omit Назад and retain Далее")
	}
	if strings.Contains(first.Body.String(), `href="/questions?page=1"`) {
		t.Fatal("first page should not include a dead link to itself")
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
	if strings.Contains(fourth.Body.String(), `aria-label="Далее"`) {
		t.Fatal("last page should omit Далее")
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
	css := requestAsset(t, "/static/app.css", "text/css")
	js := requestAsset(t, "/static/app.js", "text/javascript", "application/javascript")
	if !strings.Contains(css, `@media (max-width: 480px)`) || !strings.Contains(css, `.pagination.compact-icons .edge-icon`) || !strings.Contains(js, `nav.scrollWidth > nav.clientWidth`) {
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
	if !strings.Contains(recorder.Body.String(), `href="/static/app.css"`) || !strings.Contains(recorder.Body.String(), `src="/static/app.js"`) {
		t.Fatal("full document must link to the application stylesheet and script")
	}
	assertQuestionIDs(t, recorder.Body.String(), 11, 15)
	_, content, marked := strings.Cut(recorder.Body.String(), `<main id="content" hx-history-elt>`)
	main, _, closed := strings.Cut(content, "</main>")
	if !marked || !closed || strings.Contains(main, "<script") {
		t.Fatal("history should restore only main content, with initialization scripts outside it")
	}
}

func requestAsset(t *testing.T, path string, contentTypes ...string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d", path, recorder.Code)
	}
	contentType, _, _ := strings.Cut(recorder.Header().Get("Content-Type"), ";")
	if !slices.Contains(contentTypes, contentType) {
		t.Fatalf("GET %s returned unexpected Content-Type %q", path, contentType)
	}
	return recorder.Body.String()
}

func withQuestionBank(t *testing.T, questions []model.Question) {
	t.Helper()
	old := bank
	bank = questions
	t.Cleanup(func() { bank = old })
}

func catalogTestBank(count int) []model.Question {
	questions := make([]model.Question, count)
	for i := range count {
		questions[i] = model.Question{ID: fmt.Sprintf("q%02d", i+1), Text: fmt.Sprintf("Question %02d", i+1), Answer: fmt.Sprintf("Answer %02d", i+1), Difficulty: "лёгкий"}
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

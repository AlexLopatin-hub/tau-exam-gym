package exam

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
)

var bank []Question
var attempts = NewAttemptStore()
var demoBank bool

func Run() {
	var err error
	bank, err = loadQuestions()
	if err != nil {
		log.Fatal(err)
	}
	demoBank = os.Getenv("QUESTION_BANK") == ""
	log.Printf("loaded %d questions", len(bank))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Handler:           routes(),
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("GET /questions", questions)
	mux.HandleFunc("GET /lectures", showLecture)
	mux.HandleFunc("GET /lectures/{date}", showLecture)
	mux.HandleFunc("POST /archive/render", renderArchive)
	mux.HandleFunc("POST /exams", createAttempt)
	mux.HandleFunc("GET /exams/{id}", showAttempt)
	mux.HandleFunc("POST /exams/{id}/finish", finishAttempt)
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Изменять экзамен можно только со страницы сервиса.", http.StatusForbidden)
	}))
	return protection.Handler(mux)
}

func wantsFragment(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func render(w http.ResponseWriter, r *http.Request, view PageView, status int) {
	if view.Attempt != nil || view.ArchivedID != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	component := templ.Component(Page(view))
	if wantsFragment(r) && !strings.EqualFold(r.Header.Get("HX-History-Restore-Request"), "true") {
		component = Main(view)
	}
	renderComponent(w, r, component, status)
}

func renderComponent(w http.ResponseWriter, r *http.Request, component templ.Component, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("render page: %v", err)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	render(w, r, PageView{Title: "Экзамен", Tab: "exam", ArchivedID: r.URL.Query().Get("attempt")}, http.StatusOK)
}

func questions(w http.ResponseWriter, r *http.Request) {
	page := 1
	if values, ok := r.URL.Query()["page"]; ok {
		if len(values) != 1 {
			invalidCatalogPage(w, r)
			return
		}
		parsed, err := strconv.Atoi(values[0])
		if err != nil || parsed < 1 {
			invalidCatalogPage(w, r)
			return
		}
		page = parsed
	}
	totalPages := (len(bank) + 9) / 10
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		invalidCatalogPage(w, r)
		return
	}
	start := (page - 1) * 10
	end := min(start+10, len(bank))
	render(w, r, PageView{Title: "Все вопросы", Tab: "questions", Questions: bank[start:end], Demo: demoBank, CurrentPage: page, TotalPages: totalPages}, http.StatusOK)
}

func invalidCatalogPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, PageView{Title: "Не найдено", Tab: "questions", Error: "Страница каталога не найдена."}, http.StatusNotFound)
}

func pageLinkClass(page, currentPage, totalPages int) string {
	if page != 1 && page != totalPages && (page < currentPage-1 || page > currentPage+1) {
		return "page-number compact-hidden"
	}
	return "page-number"
}

func createAttempt(w http.ResponseWriter, r *http.Request) {
	a, err := attempts.Create(bank)
	if err != nil {
		if errors.Is(err, ErrStoreFull) {
			render(w, r, PageView{Tab: "exam", Error: "Сейчас все места для попыток заняты. Попробуйте начать экзамен позже."}, http.StatusServiceUnavailable)
			return
		}
		if err == ErrInsufficientQuestions {
			render(w, r, PageView{Title: "Экзамен", Tab: "exam", Error: "В одном из блоков пока меньше 10 вопросов. Экзамен создать нельзя."}, http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Не удалось создать экзамен", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/exams/"+a.ID, http.StatusSeeOther)
}

func showAttempt(w http.ResponseWriter, r *http.Request) {
	a, ok := attempts.Get(r.PathValue("id"))
	if !ok {
		render(w, r, PageView{Title: "Не найдено", Error: "Попытка не найдена."}, http.StatusNotFound)
		return
	}
	title := "Экзамен"
	if a.Finished {
		title = "Результат экзамена"
	}
	render(w, r, PageView{Title: title, Tab: "exam", Attempt: a, Blocks: groupQuestions(a)}, http.StatusOK)
}

const maxAnswerFormBytes = 256 << 10

func finishAttempt(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAnswerFormBytes)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Ответы слишком длинные. Сократите их и отправьте ещё раз. Экзамен не завершён.", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Не удалось прочитать ответы. Экзамен не завершён.", http.StatusBadRequest)
		return
	}
	_, ok := attempts.Finish(r.PathValue("id"), r.PostForm)
	if !ok {
		render(w, r, PageView{Title: "Не найдено", Error: "Попытка не найдена."}, http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/exams/"+r.PathValue("id"), http.StatusSeeOther)
}

func renderArchive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Архивный билет слишком большой.", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Не удалось прочитать архивный билет.", http.StatusBadRequest)
		return
	}
	var snapshot Attempt
	if json.Unmarshal(body, &snapshot) != nil || !snapshot.Finished || snapshot.FinishedAt.IsZero() || len(snapshot.Questions) != 30 {
		http.Error(w, "Некорректный архивный билет.", http.StatusBadRequest)
		return
	}
	for _, q := range snapshot.Questions {
		if q.ID == "" || q.Text == "" || q.Answer == "" || q.Topic == "" || q.Difficulty == "" {
			http.Error(w, "В архивном билете неполные данные вопроса.", http.StatusBadRequest)
			return
		}
	}
	renderComponent(w, r, QuestionBlocks(groupQuestions(&snapshot)), http.StatusOK)
}

func groupQuestions(a *Attempt) []AttemptBlock {
	blocks := make([]AttemptBlock, 0, 3)
	for i, question := range a.Questions {
		if len(blocks) == 0 || blocks[len(blocks)-1].Topic != question.Topic {
			blocks = append(blocks, AttemptBlock{Topic: question.Topic})
		}
		blocks[len(blocks)-1].Questions = append(blocks[len(blocks)-1].Questions, QuestionCardData{
			AnchorID: fmt.Sprintf("question-%d", i+1), Number: i + 1, Question: question, AttemptID: a.ID, Finished: a.Finished,
			UserAnswer: a.UserAnswers[question.ID],
		})
	}
	return blocks
}

func catalogCard(question Question) QuestionCardData {
	return QuestionCardData{AnchorID: "question-" + question.ID, Question: question}
}

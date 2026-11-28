package exam

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/AlexLopatin-hub/tau-exam-gym/frontend"
	"github.com/AlexLopatin-hub/tau-exam-gym/internal/model"
	"github.com/AlexLopatin-hub/tau-exam-gym/internal/view"
)

var bank []model.Question
var attempts = NewAttemptStore()

func Run() {
	var err error
	bank, err = loadQuestions()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d questions", len(bank))
	if items, err := lectures(); err != nil {
		log.Printf("load lectures: %v", err)
	} else {
		log.Printf("loaded %d lectures", len(items))
	}
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
	static := http.FileServerFS(frontend.Assets)
	mux.Handle("GET /static/app.css", static)
	mux.Handle("GET /static/app.js", static)
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

func render(w http.ResponseWriter, r *http.Request, view views.PageView, status int) {
	if view.Attempt != nil || view.ArchivedID != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	component := views.Page(view)
	if wantsFragment(r) && !strings.EqualFold(r.Header.Get("HX-History-Restore-Request"), "true") {
		component = views.Main(view)
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
	render(w, r, views.PageView{Title: "Экзамен", Tab: "exam", ArchivedID: r.URL.Query().Get("attempt")}, http.StatusOK)
}

func questions(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("difficulty")
	if slug == "" {
		slug = model.Difficulties[0].Slug
	}
	difficulty, ok := model.DifficultyBySlug(slug)
	if !ok {
		render(w, r, views.PageView{Title: "Не найдено", Tab: "questions", Error: "Раздел вопросов не найден."}, http.StatusNotFound)
		return
	}
	selected := make([]model.Question, 0, len(bank))
	for _, question := range bank {
		if question.Difficulty == difficulty.Value {
			selected = append(selected, question)
		}
	}
	render(w, r, views.PageView{Title: "Все вопросы", Tab: "questions", Questions: selected, Difficulty: slug}, http.StatusOK)
}

func createAttempt(w http.ResponseWriter, r *http.Request) {
	a, err := attempts.Create(bank)
	if err != nil {
		if errors.Is(err, ErrStoreFull) {
			render(w, r, views.PageView{Tab: "exam", Error: "Сейчас все места для попыток заняты. Попробуйте начать экзамен позже."}, http.StatusServiceUnavailable)
			return
		}
		if err == ErrInsufficientQuestions {
			render(w, r, views.PageView{Title: "Экзамен", Tab: "exam", Error: "Для экзамена нужно как минимум 7 лёгких, 7 средних и 6 сложных вопросов."}, http.StatusServiceUnavailable)
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
		render(w, r, views.PageView{Title: "Не найдено", Error: "Попытка не найдена."}, http.StatusNotFound)
		return
	}
	title := "Экзамен"
	if a.Finished {
		title = "Результат экзамена"
	}
	render(w, r, views.PageView{Title: title, Tab: "exam", Attempt: a}, http.StatusOK)
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
		render(w, r, views.PageView{Title: "Не найдено", Error: "Попытка не найдена."}, http.StatusNotFound)
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
	var snapshot model.Attempt
	if json.Unmarshal(body, &snapshot) != nil || !snapshot.Finished || snapshot.FinishedAt.IsZero() ||
		(len(snapshot.Questions) != model.QuestionsPerAttempt && len(snapshot.Questions) != legacyQuestionsPerAttempt) {
		http.Error(w, "Некорректный архивный билет.", http.StatusBadRequest)
		return
	}
	for _, q := range snapshot.Questions {
		if q.ID == "" || q.Text == "" || q.Answer == "" || q.Difficulty == "" {
			http.Error(w, "В архивном билете неполные данные вопроса.", http.StatusBadRequest)
			return
		}
	}
	renderComponent(w, r, views.AttemptQuestions(&snapshot), http.StatusOK)
}

package exam

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AlexLopatin-hub/tau-exam-gym/internal/model"
	views "github.com/AlexLopatin-hub/tau-exam-gym/internal/view"
)

//go:generate go run ./cmd/lecturegen

//go:embed data/lecture_pages.json
var lecturePages []byte

var lectures = sync.OnceValues(loadLectures)

func loadLectures() ([]model.Lecture, error) {
	var pages map[string]string
	if err := json.Unmarshal(lecturePages, &pages); err != nil {
		return nil, fmt.Errorf("read generated lectures: %w", err)
	}

	lectures := make([]model.Lecture, 0, len(pages))
	months := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	// The fixed-width MM.DD names sort chronologically.
	for _, name := range slices.Sorted(maps.Keys(pages)) {
		date, err := time.Parse("01.02", name)
		if err != nil {
			return nil, fmt.Errorf("lecture %s: %w", name, err)
		}
		lectures = append(lectures, model.Lecture{
			ID:   date.Format("01-02"),
			Date: fmt.Sprintf("%d %s", date.Day(), months[date.Month()-1]),
			HTML: pages[name],
		})
	}
	return lectures, nil
}

func showLecture(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "HX-Request, HX-History-Restore-Request")
	view := views.PageView{Tab: "lectures"}
	var err error
	view.Lectures, err = lectures()
	if err != nil {
		view.Error = "Не удалось открыть конспекты."
		render(w, r, view, http.StatusInternalServerError)
		return
	}

	id := r.PathValue("date")
	if id == "" && len(view.Lectures) > 0 {
		id = view.Lectures[len(view.Lectures)-1].ID
	}
	canonicalID := strings.Replace(id, ".", "-", 1)
	for i := range view.Lectures {
		if view.Lectures[i].ID == canonicalID {
			if id != canonicalID {
				http.Redirect(w, r, "/lectures/"+canonicalID, http.StatusPermanentRedirect)
				return
			}
			view.Lecture = &view.Lectures[i]
			render(w, r, view, http.StatusOK)
			return
		}
	}
	view.Error = "Лекция с такой датой не найдена."
	render(w, r, view, http.StatusNotFound)
}

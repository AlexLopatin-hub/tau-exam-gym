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
)

//go:generate go run ./cmd/lecturegen

//go:embed lecture_pages.json
var lecturePages []byte

type Lecture struct {
	ID   string
	Date string
	html string
}

var lectures = sync.OnceValues(loadLectures)

func loadLectures() ([]Lecture, error) {
	var pages map[string]string
	if err := json.Unmarshal(lecturePages, &pages); err != nil {
		return nil, fmt.Errorf("read generated lectures: %w", err)
	}

	lectures := make([]Lecture, 0, len(pages))
	months := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	// The fixed-width MM.DD names sort chronologically.
	for _, name := range slices.Sorted(maps.Keys(pages)) {
		date, err := time.Parse("01.02", name)
		if err != nil {
			return nil, fmt.Errorf("lecture %s: %w", name, err)
		}
		lectures = append(lectures, Lecture{
			ID:   date.Format("01-02"),
			Date: fmt.Sprintf("%d %s", date.Day(), months[date.Month()-1]),
			html: pages[name],
		})
	}
	return lectures, nil
}

func showLecture(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "HX-Request, HX-History-Restore-Request")
	view := PageView{Tab: "lectures"}
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
			view.LectureHTML = view.Lecture.html
			render(w, r, view, http.StatusOK)
			return
		}
	}
	view.Error = "Лекция с такой датой не найдена."
	render(w, r, view, http.StatusNotFound)
}

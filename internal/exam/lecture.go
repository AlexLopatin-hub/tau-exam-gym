package exam

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FurqanSoftware/goldmark-katex"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed lectures/*.md
var lectureFiles embed.FS

type Lecture struct {
	ID     string
	Date   string
	render func() (string, error)
}

var lectures = sync.OnceValues(loadLectures)

func loadLectures() ([]Lecture, error) {
	files, err := fs.Glob(lectureFiles, "lectures/*.md")
	if err != nil {
		return nil, err
	}

	lectures := make([]Lecture, 0, len(files))
	months := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	// The fixed-width MM.DD names returned by Glob sort chronologically.
	for _, filename := range files {
		basename := strings.TrimSuffix(strings.TrimPrefix(filename, "lectures/"), ".md")
		date, err := time.Parse("01.02", basename)
		if err != nil {
			return nil, fmt.Errorf("lecture %s: %w", filename, err)
		}
		lectures = append(lectures, Lecture{
			ID:   date.Format("01-02"),
			Date: fmt.Sprintf("%d %s", date.Day(), months[date.Month()-1]),
			render: sync.OnceValues(func() (string, error) {
				return renderLectureFile(filename)
			}),
		})
	}
	return lectures, nil
}

func renderLectureFile(filename string) (string, error) {
	source, err := lectureFiles.ReadFile(filename)
	if err != nil {
		return "", err
	}
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM, &katex.Extender{ThrowOnError: true}))
	var body bytes.Buffer
	if err := markdown.Convert(source, &body); err != nil {
		return "", fmt.Errorf("lecture %s: %w", filename, err)
	}
	return body.String(), nil
}

func showLecture(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "HX-Request, HX-History-Restore-Request")
	view := PageView{Tab: "lectures"}
	var err error
	view.Lectures, err = lectures()
	if err != nil {
		log.Printf("load lectures: %v", err)
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
			view.LectureHTML, err = view.Lecture.render()
			if err != nil {
				log.Printf("render lecture: %v", err)
				view.Error = "Не удалось отобразить эту лекцию."
				render(w, r, view, http.StatusInternalServerError)
				return
			}
			render(w, r, view, http.StatusOK)
			return
		}
	}
	view.Error = "Лекция с такой датой не найдена."
	render(w, r, view, http.StatusNotFound)
}

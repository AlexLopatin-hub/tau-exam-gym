package exam

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
)

type Question struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Answer     string `json:"answer"`
	Difficulty string `json:"difficulty"`
	Format     string `json:"format,omitempty"`
}

//go:embed data/question_bank.json
var defaultQuestionBank []byte

func loadQuestions() ([]Question, error) {
	data := defaultQuestionBank
	if path := os.Getenv("QUESTION_BANK"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read question bank: %w", err)
		}
	}
	var questions []Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return nil, fmt.Errorf("parse question bank: %w", err)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("question bank is empty")
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if q.ID == "" || q.Text == "" || q.Answer == "" || q.Difficulty == "" {
			return nil, fmt.Errorf("question has an empty required field")
		}
		if seen[q.ID] {
			return nil, fmt.Errorf("duplicate question id %q", q.ID)
		}
		if difficultyRank(q.Difficulty) == 3 {
			return nil, fmt.Errorf("question %q has invalid difficulty %q", q.ID, q.Difficulty)
		}
		if q.Format != "" && q.Format != "mathml" {
			return nil, fmt.Errorf("question %q has invalid format %q", q.ID, q.Format)
		}
		seen[q.ID] = true
	}
	return questions, nil
}

// Archive snapshots also carry this markup, so only fixed presentation tags may pass through.
var questionMarkup = regexp.MustCompile(`</?(?:math|mfrac|mi|mn|mo|mrow|msqrt|msub|msubsup|msup|mtable|mtd|mtr)>|<math display="block">|<mo stretchy="false">`)

const legacyHeavisideImage = `<img src="/static/heaviside.png" alt="H(t) = 0 при отрицательном t, H(t) = 1 при неотрицательном t">`
const heavisideMath = `<math display="block"><mi>H</mi><mo>(</mo><mi>t</mi><mo>)</mo><mo>=</mo><mrow><mo>{</mo><mtable><mtr><mtd><mn>0</mn><mo>,</mo></mtd><mtd><mi>t</mi><mo>&lt;</mo><mn>0</mn></mtd></mtr><mtr><mtd><mn>1</mn><mo>,</mo></mtd><mtd><mi>t</mi><mo>≥</mo><mn>0</mn></mtd></mtr></mtable></mrow></math>`

func renderQuestionContent(content, format string) string {
	if format != "mathml" {
		return html.EscapeString(content)
	}
	content = strings.ReplaceAll(content, legacyHeavisideImage, heavisideMath)
	var result strings.Builder
	start := 0
	for _, tag := range questionMarkup.FindAllStringIndex(content, -1) {
		result.WriteString(html.EscapeString(html.UnescapeString(content[start:tag[0]])))
		result.WriteString(content[tag[0]:tag[1]])
		start = tag[1]
	}
	result.WriteString(html.EscapeString(html.UnescapeString(content[start:])))
	return result.String()
}

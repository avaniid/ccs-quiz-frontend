package admin

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

// Importing quiz questions from a CSV, instead of typing each one into the
// add-question form. One row per question:
//
//	shift, question, option_a, option_b, option_c, option_d, correct, image_url
//
// Any number of option columns (header starting with "option"); empty ones are
// skipped. "correct" is the option's letter (A, B, ...) or number (1, 2, ...).
// A question already in the bank, or twice in the file, is skipped, so the same
// question can never land in two shifts.

type questionIssue struct {
	Row      int    `json:"row"`
	Question string `json:"question,omitempty"`
	Reason   string `json:"reason"`
}

type questionImportResult struct {
	DryRun     bool            `json:"dry_run"`
	Rows       int             `json:"rows"`
	Valid      int             `json:"valid"`
	PerShift   map[int]int     `json:"per_shift"`  // questions being added, by shift
	BankShift  map[int]int     `json:"bank_shift"` // questions in the bank after the import, by shift
	Invalid    []questionIssue `json:"invalid"`    // cannot be used as written
	Duplicates []questionIssue `json:"duplicates"` // same question earlier in the file or already in the bank
	Sample     []questionRow   `json:"sample"`     // first few parsed rows, to eyeball the columns
}

type parsedQuestion struct {
	row     int
	shift   int
	text    string
	options []string
	correct int
	image   string
}

// questionKey normalises question text for duplicate detection: case, spacing
// and line breaks do not make a question new.
func questionKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func shortText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 90 {
		return string(r[:90]) + "..."
	}
	return s
}

// parseCorrect reads "B", "b", "2", "Option B" or "(b)" as an option index.
func parseCorrect(v string, n int) (int, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "option")
	v = strings.Trim(v, " ().:")
	if v == "" {
		return 0, false
	}
	if k, err := strconv.Atoi(v); err == nil {
		return k - 1, k >= 1 && k <= n
	}
	if len(v) == 1 && v[0] >= 'a' && v[0] <= 'z' {
		k := int(v[0] - 'a')
		return k, k < n
	}
	return 0, false
}

func parseQuestions(r io.Reader, existing map[string]bool, res *questionImportResult) ([]parsedQuestion, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, errors.New("could not read the CSV header row")
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	shiftCol := headerIndex(header, "shift", "shift number", "shift no")
	textCol := headerIndex(header, "question", "questions", "question text")
	correctCol := headerIndex(header, "correct", "answer", "correct answer", "correct option", "ans")
	imageCol := headerIndex(header, "image", "image_url", "image url", "image link")
	var optionCols []int
	for i, h := range header {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(h)), "option") {
			optionCols = append(optionCols, i)
		}
	}
	switch {
	case shiftCol < 0:
		return nil, errors.New(`no "shift" column in the header row`)
	case textCol < 0:
		return nil, errors.New(`no "question" column in the header row`)
	case correctCol < 0:
		return nil, errors.New(`no "correct" column in the header row`)
	case len(optionCols) < 2:
		return nil, errors.New(`need at least two option columns (option_a, option_b, ...)`)
	}

	var out []parsedQuestion
	seen := map[string]int{}
	rowNo := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNo++
		if err != nil {
			res.Invalid = append(res.Invalid, questionIssue{Row: rowNo, Reason: "unreadable row"})
			continue
		}
		if strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		res.Rows++

		q := parsedQuestion{row: rowNo, text: strings.TrimSpace(cell(rec, textCol)), image: cell(rec, imageCol)}
		bad := func(reason string) {
			res.Invalid = append(res.Invalid, questionIssue{Row: rowNo, Question: shortText(q.text), Reason: reason})
		}
		if q.text == "" {
			bad("question is empty")
			continue
		}
		n, err := strconv.Atoi(cell(rec, shiftCol))
		if err != nil || n < 1 {
			bad("shift must be a whole number 1 or more")
			continue
		}
		q.shift = n

		// Options keep their column letter for "correct", so an empty option C
		// does not shift D into C's place.
		letters := map[int]int{} // column position -> index in q.options
		optSeen := map[string]bool{}
		dupOption := false
		for pos, col := range optionCols {
			v := cell(rec, col)
			if v == "" {
				continue
			}
			k := questionKey(v)
			if optSeen[k] {
				dupOption = true
			}
			optSeen[k] = true
			letters[pos] = len(q.options)
			q.options = append(q.options, v)
		}
		if len(q.options) < 2 {
			bad("needs at least two options")
			continue
		}
		if dupOption {
			bad("two options have the same text")
			continue
		}
		pos, ok := parseCorrect(cell(rec, correctCol), len(optionCols))
		idx, filled := letters[pos]
		if !ok || !filled {
			bad(fmt.Sprintf("correct answer %q does not point to a filled option", cell(rec, correctCol)))
			continue
		}
		q.correct = idx
		if q.image != "" && !strings.HasPrefix(q.image, "https://") && !strings.HasPrefix(q.image, "http://") {
			bad("image must be a web link (https://...)")
			continue
		}

		key := questionKey(q.text)
		if existing[key] {
			res.Duplicates = append(res.Duplicates, questionIssue{Row: rowNo, Question: shortText(q.text), Reason: "already in the question bank"})
			continue
		}
		if first, dup := seen[key]; dup {
			res.Duplicates = append(res.Duplicates, questionIssue{Row: rowNo, Question: shortText(q.text), Reason: fmt.Sprintf("same question as row %d", first)})
			continue
		}
		seen[key] = rowNo
		out = append(out, q)
	}
	return out, nil
}

// ImportQuestions adds every valid row of an uploaded CSV to the question bank.
// POST /admin/import-questions, multipart form: file, dry_run ("true" to only
// preview).
func ImportQuestions(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, importMaxBytes)
	dryRun := c.PostForm("dry_run") == "true"

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose a CSV file to upload"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not open the uploaded file"})
		return
	}
	defer f.Close()

	bank, err := loadQuestionBank()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load the question bank"})
		return
	}
	existing := map[string]bool{}
	res := questionImportResult{DryRun: dryRun, PerShift: map[int]int{}, BankShift: map[int]int{}}
	for _, q := range bank {
		existing[questionKey(q.Question)] = true
		res.BankShift[q.Shift]++
	}

	parsed, err := parseQuestions(f, existing, &res)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res.Valid = len(parsed)

	var questions, answers []any
	for _, p := range parsed {
		res.PerShift[p.shift]++
		res.BankShift[p.shift]++
		q := models.Quiz_Questions{QuestionID: GenerateQID(), Question: p.text, Image: p.image, Shift: p.shift}
		var correct models.OID
		for i, v := range p.options {
			o := models.Option{Value: v, Id: GenerateOID()}
			if i == p.correct {
				correct = o.Id
			}
			q.Options = append(q.Options, o)
		}
		questions = append(questions, q)
		answers = append(answers, models.Quiz_Answer{QuestionID: q.QuestionID, Answer: correct})
		if len(res.Sample) < 5 {
			row := questionRow{Question: p.text, Correct: p.correct, Image: p.image, Shift: p.shift, Options: p.options}
			res.Sample = append(res.Sample, row)
		}
	}
	sort.SliceStable(res.Invalid, func(i, j int) bool { return res.Invalid[i].Row < res.Invalid[j].Row })

	if dryRun || len(questions) == 0 {
		c.JSON(http.StatusOK, res)
		return
	}

	// Answers first: a question without its answer would be handed out and
	// could never score, while an answer without its question is harmless.
	if _, err := db.Quiz_Answers.Coll.InsertMany(db.Quiz_Answers.Context, answers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save the answers, nothing was added"})
		return
	}
	if _, err := db.Quiz_Questions.Coll.InsertMany(db.Quiz_Questions.Context, questions); err != nil {
		ids := make([]models.QID, 0, len(questions))
		for _, q := range questions {
			ids = append(ids, q.(models.Quiz_Questions).QuestionID)
		}
		// Roll back whatever part of the batch did land, so a retry of the same
		// file does not report those rows as duplicates.
		db.Quiz_Questions.Coll.DeleteMany(db.Quiz_Questions.Context, bson.M{"questionID": bson.M{"$in": ids}})
		db.Quiz_Answers.Coll.DeleteMany(db.Quiz_Answers.Context, bson.M{"questionID": bson.M{"$in": ids}})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save the questions, nothing was added"})
		return
	}
	c.JSON(http.StatusOK, res)
}

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
	"time"

	login "ccs.quizportal/Login"
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Importing candidates from the registration CSV.
//
// The registration form lives outside this system (a Google Form export), so
// reg_responses is empty and "Assign Shifts & Questions" has nobody to assign.
// But a candidate's user id is derived from their email alone, so a paper can
// be created for an address before that person ever signs in: when they do, the
// id their login produces is the one the paper was filed under.
//
// The sheet needs an "email" column, and either a "shift" column or a shift
// size to fill shifts in file order. "name" and "roll" are kept for the
// results page when present.

const (
	importMaxBytes = 5 << 20 // a few thousand rows is well under 1MB
	importMaxList  = 300     // cap each problem list in the response
)

type importIssue struct {
	Row    int    `json:"row"` // 1-based, counting the header as row 1
	Email  string `json:"email,omitempty"`
	Reason string `json:"reason"`
}

type importResult struct {
	DryRun          bool          `json:"dry_run"`
	Rows            int           `json:"rows"`
	Valid           int           `json:"valid"`
	PerShift        map[int]int   `json:"per_shift"`
	Created         int           `json:"created"`
	Replaced        int           `json:"replaced"`
	QuestionsPerSet int           `json:"questions_per_set"`
	FallbackShifts  []int         `json:"fallback_shifts"`
	SmallShifts     map[int]int   `json:"small_shifts"` // shift -> questions actually available, when < requested
	AlreadyStarted  []importIssue `json:"already_started"`
	Invalid         []importIssue `json:"invalid"`
	Duplicates      []importIssue `json:"duplicates"`
	NotAllowed      []importIssue `json:"not_allowed"`
}

type candidateRow struct {
	row   int
	email string
	name  string
	roll  string
	shift int
	uid   models.UID
}

// Candidate is the stored record of an imported candidate.
type Candidate struct {
	UserID     models.UID `bson:"userID"`
	Email      string     `bson:"email"`
	Name       string     `bson:"name"`
	Roll       string     `bson:"roll"`
	Shift      int        `bson:"shift"`
	ImportedAt time.Time  `bson:"imported_at"`
}

func ShowImportPage(c *gin.Context) {
	c.HTML(http.StatusOK, "import.html", nil)
}

// headerIndex finds the first column whose header is one of names.
func headerIndex(header []string, names ...string) int {
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		for _, n := range names {
			if h == n {
				return i
			}
		}
	}
	return -1
}

func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func appendIssue(list []importIssue, issue importIssue) []importIssue {
	if len(list) < importMaxList {
		list = append(list, issue)
	}
	return list
}

// parseCandidates reads the sheet and resolves each row's shift. Rows that
// cannot be used are reported, not fatal.
func parseCandidates(r io.Reader, perShift, numShifts, startShift int, res *importResult) ([]candidateRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows from spreadsheet exports
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, errors.New("could not read the CSV header row")
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff") // Excel's UTF-8 BOM
	}
	emailCol := headerIndex(header, "email", "e-mail", "email address", "email id", "mail")
	if emailCol < 0 {
		return nil, errors.New(`no "email" column found in the header row`)
	}
	shiftCol := headerIndex(header, "shift", "shift number", "shift no", "slot")
	nameCol := headerIndex(header, "name", "full name", "candidate name")
	rollCol := headerIndex(header, "roll", "roll number", "roll no", "roll no.", "rollno")

	if shiftCol < 0 && perShift < 1 && numShifts < 1 {
		return nil, errors.New(`the sheet has no "shift" column; set "number of shifts" or "candidates per shift" to fill shifts automatically`)
	}

	var out []candidateRow
	seen := map[string]int{} // email -> row it first appeared on
	rowNo := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNo++
		if err != nil {
			res.Invalid = appendIssue(res.Invalid, importIssue{Row: rowNo, Reason: "unreadable row"})
			continue
		}
		if strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue // blank line
		}
		res.Rows++

		email := login.NormalizeEmail(cell(rec, emailCol))
		if at := strings.LastIndex(email, "@"); at < 1 || at == len(email)-1 || strings.ContainsAny(email, " ,;<>") {
			res.Invalid = appendIssue(res.Invalid, importIssue{Row: rowNo, Email: email, Reason: "not a valid email"})
			continue
		}
		if first, dup := seen[email]; dup {
			res.Duplicates = appendIssue(res.Duplicates, importIssue{
				Row: rowNo, Email: email, Reason: fmt.Sprintf("same email as row %d, kept row %d", first, first),
			})
			continue
		}
		if !login.EmailAllowed(email) {
			res.NotAllowed = appendIssue(res.NotAllowed, importIssue{
				Row: rowNo, Email: email, Reason: "this domain is not allowed to sign in (ALLOWED_EMAIL_DOMAINS)",
			})
			continue
		}

		shift := 0
		if shiftCol >= 0 {
			n, err := strconv.Atoi(cell(rec, shiftCol))
			if err != nil || n < 1 {
				res.Invalid = appendIssue(res.Invalid, importIssue{Row: rowNo, Email: email, Reason: "shift must be a whole number 1 or more"})
				continue
			}
			shift = n
		}

		seen[email] = rowNo
		out = append(out, candidateRow{
			row:   rowNo,
			email: email,
			name:  cell(rec, nameCol),
			roll:  cell(rec, rollCol),
			shift: shift,
			uid:   login.GenerateUserUUID(email),
		})
	}

	// No shift column: fill shifts in file order, perShift at a time. A number
	// of shifts splits the valid rows evenly (568 over 2 is 284 each).
	if shiftCol < 0 && numShifts > 0 && len(out) > 0 {
		perShift = (len(out) + numShifts - 1) / numShifts
	}
	if shiftCol < 0 {
		for i := range out {
			out[i].shift = startShift + i/perShift
		}
	}
	return out, nil
}

// ImportCandidates creates (or replaces) a paper for every row of an uploaded
// CSV. POST /admin/import-candidates, multipart form:
//
//	file               the CSV
//	questions_per_set  questions on each paper
//	shifts             number of shifts to split evenly into, used only without a shift column
//	per_shift          candidates per shift, used only without a shift column and without shifts
//	start_shift        first shift number for automatic filling (default 1)
//	dry_run            "true" to validate and preview without writing anything
//
// Anyone who has already opened their paper is left alone: replacing a paper
// mid-attempt would invalidate the answers they are giving.
func ImportCandidates(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, importMaxBytes)

	perSet, _ := strconv.Atoi(c.PostForm("questions_per_set"))
	if perSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "questions per paper must be 1 or more"})
		return
	}
	perShift, _ := strconv.Atoi(c.PostForm("per_shift"))
	numShifts, _ := strconv.Atoi(c.PostForm("shifts"))
	startShift, _ := strconv.Atoi(c.PostForm("start_shift"))
	if startShift < 1 {
		startShift = 1
	}
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

	res := importResult{
		DryRun:          dryRun,
		PerShift:        map[int]int{},
		SmallShifts:     map[int]int{},
		QuestionsPerSet: perSet,
	}
	rows, err := parseCandidates(f, perShift, numShifts, startShift, &res)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bank, err := loadQuestionBank()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load the question bank"})
		return
	}
	if len(bank) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the question bank is empty; add quiz questions first"})
		return
	}

	// Who already has a paper, and who has already opened it.
	hasPaper := map[string]bool{}
	loadByUser(db.User_Questions, bson.M{"userID": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID models.UID `bson:"userID"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		hasPaper[uidKey(d.UserID)] = true
		return nil
	})
	started := map[string]bool{}
	loadByUser(db.Updates, bson.M{"userID": 1, "quiz_started": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID  models.UID `bson:"userID"`
			Started bool       `bson:"quiz_started"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		if d.Started {
			started[uidKey(d.UserID)] = true
		}
		return nil
	})

	// One question set per shift (everyone in a shift gets the same questions,
	// each in their own order), drawn from that shift's tagged questions.
	sets := map[int][]models.Quiz_Questions{}
	fallback := map[int]bool{}
	setFor := func(shift int) []models.Quiz_Questions {
		if s, ok := sets[shift]; ok {
			return s
		}
		s, fb := pickSet(bank, shift, perSet)
		sets[shift], fallback[shift] = s, fb
		if len(s) < perSet {
			res.SmallShifts[shift] = len(s)
		}
		return s
	}

	var paperWrites, candidateWrites []mongo.WriteModel
	now := time.Now()
	for _, r := range rows {
		key := uidKey(r.uid)
		if started[key] {
			res.AlreadyStarted = appendIssue(res.AlreadyStarted, importIssue{
				Row: r.row, Email: r.email, Reason: "already opened their paper, left unchanged",
			})
			continue
		}
		res.Valid++
		res.PerShift[r.shift]++
		if hasPaper[key] {
			res.Replaced++
		} else {
			res.Created++
		}

		paper := models.UserQuestions{
			UserID:    r.uid,
			Shift:     r.shift,
			Questions: personalize(setFor(r.shift)),
		}
		paperWrites = append(paperWrites, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"userID": r.uid}).SetReplacement(paper).SetUpsert(true))
		candidateWrites = append(candidateWrites, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"userID": r.uid}).
			SetReplacement(Candidate{
				UserID: r.uid, Email: r.email, Name: r.name, Roll: r.roll, Shift: r.shift, ImportedAt: now,
			}).SetUpsert(true))
	}

	for shift, fb := range fallback {
		if fb {
			res.FallbackShifts = append(res.FallbackShifts, shift)
		}
	}
	sort.Ints(res.FallbackShifts)

	if dryRun || len(paperWrites) == 0 {
		c.JSON(http.StatusOK, res)
		return
	}

	unordered := options.BulkWrite().SetOrdered(false)
	if _, err := db.User_Questions.Coll.BulkWrite(db.User_Questions.Context, paperWrites, unordered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "saving papers failed part-way: " + err.Error() +
			". Running the import again is safe: it replaces rather than duplicates."})
		return
	}
	if _, err := db.Candidates.Coll.BulkWrite(db.Candidates.Context, candidateWrites, unordered); err != nil {
		// Papers are in place, which is what matters for the exam; names and
		// roll numbers only feed the results page.
		c.JSON(http.StatusOK, gin.H{"warning": "papers saved, but names/roll numbers could not be stored: " + err.Error(), "result": res})
		return
	}
	c.JSON(http.StatusOK, res)
}

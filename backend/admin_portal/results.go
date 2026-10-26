package admin

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	models "ccs.quizportal/Models"
	quiz "ccs.quizportal/Quiz"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// The results page: one row per candidate with when they sat the quiz, how it
// went and what the proctoring saw.

// maxWarnings mirrors MAX_WARNINGS in the frontend (Quiz.jsx): reaching it,
// counting warnings of every type, auto-submits the attempt.
const maxWarnings = 20

// flagTypes is the column order for per-violation counts in the CSV.
var flagTypes = []string{
	"tab-switch", "window-blur", "fullscreen-exit", "face-missing", "multiple-faces",
	"device-detected", "camera-blocked", "mic-blocked", "camera-lost", "voice-mismatch",
}

var ist = time.FixedZone("IST", 5*3600+1800)

type resultRow struct {
	UID            string         `json:"uid"`
	Email          string         `json:"email"`
	Name           string         `json:"name"`
	Roll           string         `json:"roll"`
	Shift          int            `json:"shift"`
	ImportedShift  int            `json:"imported_shift"` // shift from the CSV; differs from Shift once moved for a re-attempt
	PriorAttempts  int            `json:"prior_attempts"` // attempts an admin reset (kept in attempt_archive)
	ShiftStart     *time.Time     `json:"shift_start"`
	Status         string         `json:"status"` // no_paper | not_started | in_progress | submitted | not_submitted
	StartedAt      *time.Time     `json:"started_at"`
	SubmittedAt    *time.Time     `json:"submitted_at"`
	TimeTakenSec   *int           `json:"time_taken_sec"`
	SubmitKind     string         `json:"submit_kind"` // manual | timeout | warnings (inferred)
	TotalQuestions int            `json:"total_questions"`
	Answered       int            `json:"answered"`
	Correct        int            `json:"correct"`
	Wrong          int            `json:"wrong"`
	Unanswered     int            `json:"unanswered"`
	Marks          *int           `json:"marks"`
	Flags          int            `json:"flags"`
	Strikes        int            `json:"strikes"`
	FlagCounts     map[string]int `json:"flag_counts"`
	HasSnapshot    bool           `json:"has_snapshot"`
}

func ShowResultsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "results.html", nil)
}

// buildResults reads each collection once and joins them in memory, so the
// page costs a fixed number of round trips however many candidates there are.
func buildResults() ([]*resultRow, error) {
	rows := map[string]*resultRow{}
	get := func(id models.UID) *resultRow {
		k := uidKey(id)
		r, ok := rows[k]
		if !ok {
			r = &resultRow{UID: hex.EncodeToString(id[:]), FlagCounts: map[string]int{}}
			rows[k] = r
		}
		return r
	}

	// Imported candidates: name, roll, intended shift.
	loadByUser(db.Candidates, bson.M{"userID": 1, "email": 1, "name": 1, "roll": 1, "shift": 1}, func(cur *mongo.Cursor) error {
		var d Candidate
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		r := get(d.UserID)
		r.Email, r.Name, r.Roll, r.Shift, r.ImportedShift = d.Email, d.Name, d.Roll, d.Shift, d.Shift
		return nil
	})

	// Sign-ins: the email for anyone who was not imported. Accounts that only
	// signed in are kept separately and added only if they appear elsewhere or
	// as "no paper" rows below.
	emails := map[string]string{}
	if err := loadByUser(db.AUTH, bson.M{"userID": 1, "userEmail": 1}, func(cur *mongo.Cursor) error {
		var u models.AuthUser
		if err := cur.Decode(&u); err != nil || u.UserId == nil {
			return errSkip
		}
		emails[uidKey(u.UserId)] = u.Email
		get(u.UserId)
		return nil
	}); err != nil {
		return nil, err
	}

	// Papers: shift and how many questions. Only ids are projected; the full
	// papers are large.
	hasPaper := map[string]bool{}
	loadByUser(db.User_Questions, bson.M{"userID": 1, "shift": 1, "questions.questionID": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID    models.UID `bson:"userID"`
			Shift     int        `bson:"shift"`
			Questions []struct {
				QuestionID models.QID `bson:"questionID"`
			} `bson:"questions"`
		}
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		r := get(d.UserID)
		r.Shift = d.Shift
		r.TotalQuestions = len(d.Questions)
		hasPaper[uidKey(d.UserID)] = true
		return nil
	})

	// Attempts: timing and proctoring.
	type attemptDoc struct {
		UserID      models.UID `bson:"userID"`
		Started     bool       `bson:"quiz_started"`
		Submitted   bool       `bson:"quiz_submitted"`
		StartedAt   time.Time  `bson:"started_at"`
		SubmittedAt time.Time  `bson:"submitted_at"`
		Flags       int        `bson:"flags"`
		Strikes     int        `bson:"strikes"`
		FlagLog     []struct {
			Type string `bson:"type"`
		} `bson:"flag_log"`
	}
	attempts := map[string]attemptDoc{}
	loadByUser(db.Updates, bson.M{
		"userID": 1, "quiz_started": 1, "quiz_submitted": 1, "started_at": 1, "submitted_at": 1,
		"flags": 1, "strikes": 1, "flag_log.type": 1,
	}, func(cur *mongo.Cursor) error {
		var d attemptDoc
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		attempts[uidKey(d.UserID)] = d
		get(d.UserID)
		return nil
	})

	// Marks and whether a snapshot exists. The snapshot itself (~100KB each) is
	// not fetched here; the page loads it on demand.
	type trackDoc struct {
		UserID      models.UID `bson:"userID"`
		Marks       int        `bson:"marks"`
		FlagsRaised int        `bson:"flagsraised"`
		HasSnapshot bool       `bson:"has_snapshot"`
	}
	tracks := map[string]trackDoc{}
	loadByUser(db.Quiz_Track, bson.M{
		"userID": 1, "marks": 1, "flagsraised": 1,
		"has_snapshot": bson.M{"$gt": bson.A{bson.M{"$strLenBytes": bson.M{"$ifNull": bson.A{"$SnapshotLink", ""}}}, 0}},
	}, func(cur *mongo.Cursor) error {
		var d trackDoc
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		tracks[uidKey(d.UserID)] = d
		return nil
	})

	// Answers and the key, for correct / wrong counts.
	key := map[[7]byte][7]byte{}
	loadByUser(db.Quiz_Answers, bson.M{}, func(cur *mongo.Cursor) error {
		var a models.Quiz_Answer
		if err := cur.Decode(&a); err != nil || a.QuestionID == nil || a.Answer == nil {
			return errSkip
		}
		key[*a.QuestionID] = *a.Answer
		return nil
	})
	loadByUser(db.Quiz_Responses, bson.M{}, func(cur *mongo.Cursor) error {
		var d models.Quiz_Responses
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		r := get(d.UserID)
		counted := map[[7]byte]bool{}
		for _, a := range d.Responses {
			if a.QuestionID == nil || a.Answer == nil || counted[*a.QuestionID] {
				continue
			}
			counted[*a.QuestionID] = true
			r.Answered++
			if correct, ok := key[*a.QuestionID]; ok && correct == *a.Answer {
				r.Correct++
			} else {
				r.Wrong++
			}
		}
		return nil
	})

	// Resets: how many earlier attempts each candidate had cleared.
	loadByUser(db.Attempt_Archive, bson.M{"userID": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID models.UID `bson:"userID"`
		}
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		get(d.UserID).PriorAttempts++
		return nil
	})

	shiftStart := map[int]time.Time{}
	loadByUser(db.Shifts, bson.M{"shift": 1, "start_time": 1}, func(cur *mongo.Cursor) error {
		var s models.Shift
		if err := cur.Decode(&s); err != nil {
			return err
		}
		shiftStart[s.Shift] = s.Start
		return nil
	})

	duration := quiz.TestDuration()
	cutoff := duration + quiz.SubmitGrace()
	now := time.Now()

	out := make([]*resultRow, 0, len(rows))
	for k, r := range rows {
		if r.Email == "" {
			r.Email = emails[k]
		}
		if st, ok := shiftStart[r.Shift]; ok && r.Shift > 0 {
			st := st
			r.ShiftStart = &st
		}
		if r.TotalQuestions > 0 {
			r.Unanswered = r.TotalQuestions - r.Answered
			if r.Unanswered < 0 {
				r.Unanswered = 0
			}
		}

		a, attempted := attempts[k]
		t, tracked := tracks[k]
		switch {
		case attempted && a.Submitted:
			r.Status = "submitted"
		case attempted && a.Started && !a.StartedAt.IsZero() && now.After(a.StartedAt.Add(cutoff)):
			r.Status = "not_submitted" // opened the paper, time ran out, nothing reached the server
		case attempted && a.Started:
			r.Status = "in_progress"
		case hasPaper[k]:
			r.Status = "not_started"
		default:
			r.Status = "no_paper"
		}

		if attempted {
			r.Flags, r.Strikes = a.Flags, a.Strikes
			for _, f := range a.FlagLog {
				r.FlagCounts[f.Type]++
			}
			if !a.StartedAt.IsZero() {
				s := a.StartedAt
				r.StartedAt = &s
			}
			if a.Submitted && !a.SubmittedAt.IsZero() {
				s := a.SubmittedAt
				r.SubmittedAt = &s
				if r.StartedAt != nil {
					secs := int(s.Sub(*r.StartedAt).Seconds())
					if secs < 0 {
						secs = 0
					}
					r.TimeTakenSec = &secs
				}
			}
		}
		if tracked {
			if t.FlagsRaised > r.Flags {
				r.Flags = t.FlagsRaised
			}
			r.HasSnapshot = t.HasSnapshot
		}
		if r.Status == "submitted" {
			m := 4*r.Correct - r.Wrong
			if tracked {
				m = t.Marks
			}
			r.Marks = &m
			switch {
			case r.Flags >= maxWarnings:
				r.SubmitKind = "warnings"
			case r.TimeTakenSec != nil && time.Duration(*r.TimeTakenSec)*time.Second >= duration-5*time.Second:
				r.SubmitKind = "timeout"
			default:
				r.SubmitKind = "manual"
			}
		}

		// Someone who only signed in, was never imported and has no paper is
		// noise on a results page (wrong account, curious visitor) - keep them
		// so a missing candidate can be spotted, but only if they have an email.
		if r.Email == "" && r.Status == "no_paper" {
			continue
		}
		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Shift != out[j].Shift {
			return out[i].Shift < out[j].Shift
		}
		return out[i].Email < out[j].Email
	})
	return out, nil
}

// GET /admin/api/results
func GetResults(c *gin.Context) {
	rows, err := buildResults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load results"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"generated_at":     time.Now(),
		"duration_minutes": int(quiz.TestDuration().Minutes()),
		"max_warnings":     maxWarnings,
		"rows":             rows,
	})
}

func istString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(ist).Format("02 Jan 2006 15:04:05")
}

// GET /admin/api/results.csv
func DownloadResultsCSV(c *gin.Context) {
	rows, err := buildResults()
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load results")
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\ufeff") // so Excel reads it as UTF-8
	w := csv.NewWriter(&buf)
	header := []string{
		"Email", "Name", "Roll", "Shift", "Shift start (IST)", "Status",
		"Started (IST)", "Submitted (IST)", "Time taken (mm:ss)", "Submit type",
		"Questions", "Answered", "Correct", "Wrong", "Unanswered", "Marks",
		"Warnings", "Strikes", "Snapshot", "Imported shift", "Re-attempt", "Earlier attempts (reset)",
	}
	header = append(header, flagTypes...)
	w.Write(header)
	for _, r := range rows {
		taken := ""
		if r.TimeTakenSec != nil {
			taken = fmt.Sprintf("%d:%02d", *r.TimeTakenSec/60, *r.TimeTakenSec%60)
		}
		marks := ""
		if r.Marks != nil {
			marks = strconv.Itoa(*r.Marks)
		}
		snap := "no"
		if r.HasSnapshot {
			snap = "yes"
		}
		rec := []string{
			r.Email, r.Name, r.Roll, strconv.Itoa(r.Shift), istString(r.ShiftStart), r.Status,
			istString(r.StartedAt), istString(r.SubmittedAt), taken, r.SubmitKind,
			strconv.Itoa(r.TotalQuestions), strconv.Itoa(r.Answered), strconv.Itoa(r.Correct),
			strconv.Itoa(r.Wrong), strconv.Itoa(r.Unanswered), marks,
			strconv.Itoa(r.Flags), strconv.Itoa(r.Strikes), snap,
			importedShift(r), yesNo(isReattempt(r)), strconv.Itoa(r.PriorAttempts),
		}
		for _, t := range flagTypes {
			rec = append(rec, strconv.Itoa(r.FlagCounts[t]))
		}
		w.Write(rec)
	}
	w.Flush()
	name := "quiz-results-" + time.Now().In(ist).Format("2006-01-02-1504") + ".csv"
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// GET /admin/api/snapshot/:uid - the webcam frame taken at submit, as an image.
func GetSnapshot(c *gin.Context) {
	raw, err := hex.DecodeString(c.Param("uid"))
	if err != nil || len(raw) != 7 {
		c.String(http.StatusBadRequest, "bad id")
		return
	}
	var id [7]byte
	copy(id[:], raw)
	var t models.QuizTrack
	if err := db.Quiz_Track.Coll.FindOne(db.Quiz_Track.Context, bson.M{"userID": models.UID(&id)}).Decode(&t); err != nil {
		c.String(http.StatusNotFound, "no snapshot")
		return
	}
	data := t.SnapShot
	comma := strings.IndexByte(data, ',')
	if !strings.HasPrefix(data, "data:image/") || comma < 0 {
		c.String(http.StatusNotFound, "no snapshot")
		return
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(data[:comma], "data:"), ";base64")
	img, err := base64.StdEncoding.DecodeString(data[comma+1:])
	if err != nil {
		c.String(http.StatusNotFound, "unreadable snapshot")
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, mime, img)
}

// isReattempt: moved off the shift the CSV gave them (absentee moved, or reset
// and moved), or had an earlier attempt reset.
func isReattempt(r *resultRow) bool {
	return (r.ImportedShift > 0 && r.Shift > 0 && r.Shift != r.ImportedShift) || r.PriorAttempts > 0
}

func importedShift(r *resultRow) string {
	if r.ImportedShift == 0 {
		return ""
	}
	return strconv.Itoa(r.ImportedShift)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

package quiz

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	login "ccs.quizportal/Login"
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Conditions that are the candidate's situation, not a server failure. Handlers
// map these to 4xx so a finished or too-early candidate is not reported as a
// backend error (and does not show up as one in monitoring).
var (
	ErrAlreadySubmitted = errors.New("you have already submitted this quiz")
	ErrNoPaperAssigned  = errors.New("no quiz has been assigned to your account yet")
	ErrQuizNotStarted   = errors.New("quiz not started for your shift yet")
	ErrShiftOver        = errors.New("entry for your shift has closed")
	ErrTimeUp           = errors.New("your time for this quiz is over")
)

// submitGrace is how long after the deadline a submission is still accepted.
// It covers the auto-submit's network round trip and its retries.
const submitGrace = 2 * time.Minute

// SubmitGrace is submitGrace, for code outside this package that needs to know
// when an unsubmitted attempt can no longer be handed in.
func SubmitGrace() time.Duration { return submitGrace }

// TestDuration is the exam length (TEST_DURATION, in minutes, default 80).
func TestDuration() time.Duration {
	minutes := 80
	if v := os.Getenv("TEST_DURATION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minutes = n
		}
	}
	return time.Duration(minutes) * time.Minute
}

// EntryWindow is how long after a shift starts its candidates may still open
// the paper (ENTRY_WINDOW, in minutes). Everyone who gets in still has the
// full TestDuration from their own start, so a short window ends the shift
// early without cutting anyone's time. Unset, it equals TestDuration, which
// was the only behaviour before it existed.
func EntryWindow() time.Duration {
	if v := os.Getenv("ENTRY_WINDOW"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return TestDuration()
}

// shiftCacheTTL bounds how stale a shift timing can be. Admins can re-time a
// shift while the server runs; SetShiftTiming also clears the cache directly,
// the TTL covers anything that edits shift_data behind the server's back.
const shiftCacheTTL = 30 * time.Second

var (
	shiftsMu       sync.RWMutex
	shiftsMap      map[int]models.Shift
	shiftsLoadedAt time.Time
)

// InvalidateShiftCache forces the next lookup to reload timings from Mongo.
func InvalidateShiftCache() {
	shiftsMu.Lock()
	shiftsMap = nil
	shiftsMu.Unlock()
}

// getShift returns a shift's timing from a cache that is refreshed on a miss
// or once it is older than shiftCacheTTL.
//
// The mutex matters because every candidate hits this at once when a shift
// opens, and concurrent read/write of a plain map panics the process.
func getShift(shift int) (models.Shift, error) {
	shiftsMu.RLock()
	s, ok := shiftsMap[shift]
	fresh := time.Since(shiftsLoadedAt) < shiftCacheTTL
	shiftsMu.RUnlock()
	if ok && fresh {
		return s, nil
	}

	shiftsMu.Lock()
	defer shiftsMu.Unlock()
	if s, ok := shiftsMap[shift]; ok && time.Since(shiftsLoadedAt) < shiftCacheTTL {
		return s, nil // another request may have just reloaded
	}

	loaded, err := LoadAllShifts()
	if err != nil {
		log.Printf("failed to load shift timings: %v", err)
		return models.Shift{}, errors.New("could not load shift timings")
	}
	shiftsMap = loaded
	shiftsLoadedAt = time.Now()

	s, ok = shiftsMap[shift]
	if !ok {
		return models.Shift{}, fmt.Errorf("no timing has been set for shift %d yet", shift)
	}
	return s, nil
}

// attempt is the candidate's row in the updates collection.
type attempt struct {
	Started   bool      `bson:"quiz_started"`
	Submitted bool      `bson:"quiz_submitted"`
	StartedAt time.Time `bson:"started_at"`
	Flags     int       `bson:"flags"`
	Strikes   int       `bson:"strikes"`
}

// loadAttempt returns the candidate's attempt row; found is false when the
// candidate has never opened the paper.
func loadAttempt(userID models.UID) (attempt, bool, error) {
	var a attempt
	err := db.Updates.Coll.FindOne(db.Updates.Context, bson.M{"userID": userID}).Decode(&a)
	if err == mongo.ErrNoDocuments {
		return a, false, nil
	}
	return a, err == nil, err
}

// QuizPaper is what /quiz/get hands the candidate: the questions plus a
// server-decided deadline, so reloading the page cannot restart the clock.
type QuizPaper struct {
	Questions []models.Quiz_Questions
	Deadline  time.Time
	Flags     int
	Strikes   int
}

func GetQuizQuestions(c *gin.Context) (*QuizPaper, error) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		return nil, err
	}

	prev, started, err := loadAttempt(userID)
	if err != nil {
		log.Printf("failed to load attempt for user %v: %v", userID, err)
		return nil, errors.New("could not load your quiz, please try again")
	}
	testMode := TestMode()
	// Refuse to hand the paper back out once the attempt is over. In test mode
	// the finished attempt is cleared instead, so the same login can retake it.
	if prev.Submitted {
		if !testMode {
			return nil, ErrAlreadySubmitted
		}
		if err := resetTestAttempt(userID); err != nil {
			log.Printf("test mode: reset failed for user %v: %v", userID, err)
			return nil, errors.New("could not start a new attempt, please try again")
		}
		prev, started = attempt{}, false
	}

	var uq models.UserQuestions
	if testMode {
		// No slot request or admin assignment while testing: a paper is drawn
		// on the spot from the whole bank.
		uq, err = ensureTestPaper(userID)
		if errors.Is(err, ErrNoQuestionsInBank) {
			return nil, err
		}
	} else {
		err = db.User_Questions.Coll.FindOne(db.User_Questions.Context, bson.M{"userID": userID}).Decode(&uq)
		if err == mongo.ErrNoDocuments {
			// Authenticated, but the admin has not given this account a paper yet.
			return nil, ErrNoPaperAssigned
		}
	}
	if err != nil {
		log.Printf("failed to load questions for user %v: %v", userID, err)
		return nil, errors.New("could not load your quiz, please try again")
	}

	now := time.Now()
	duration := TestDuration()
	// Shift timings are not used in test mode: the paper opens whenever the
	// tester signs in, and only their personal deadline applies.
	if !testMode {
		shiftInfo, err := getShift(uq.Shift)
		if err != nil {
			return nil, err
		}
		if now.Before(shiftInfo.Start) {
			return nil, ErrQuizNotStarted
		}
		// Entry closes once the shift's own window is over; someone from shift 1
		// must not be able to open the paper during shift 3. A candidate who had
		// already started is handled by their personal deadline below.
		if !started && now.After(shiftInfo.Start.Add(EntryWindow())) {
			return nil, ErrShiftOver
		}
	}

	// First open stamps started_at; later opens (a refresh, a crash) keep it,
	// which is what pins the deadline.
	var cur attempt
	err = db.Updates.Coll.FindOneAndUpdate(
		db.Updates.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": bson.M{
			"userID":         userID,
			"quiz_started":   true,
			"quiz_submitted": false,
			"started_at":     now,
			"flags":          0,
			"strikes":        0,
		}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&cur)
	if err != nil {
		log.Printf("failed to record start for user %v: %v", userID, err)
		return nil, errors.New("could not start your quiz, please try again")
	}
	if cur.StartedAt.IsZero() {
		// Row from before started_at existed: start the clock now.
		cur.StartedAt = now
		db.Updates.Coll.UpdateOne(db.Updates.Context,
			bson.M{"userID": userID, "started_at": bson.M{"$exists": false}},
			bson.M{"$set": bson.M{"started_at": now, "quiz_started": true}})
	}

	deadline := cur.StartedAt.Add(duration)
	if now.After(deadline.Add(submitGrace)) {
		return nil, ErrTimeUp
	}

	return &QuizPaper{
		Questions: uq.Questions,
		Deadline:  deadline,
		Flags:     cur.Flags,
		Strikes:   cur.Strikes,
	}, nil
}

func qidKey(id *[7]byte) string { return hex.EncodeToString(id[:]) }

func RecieveResponse(c *gin.Context) {
	var resp models.Form_Responses
	if err := c.ShouldBindJSON(&resp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// An attempt only exists once /quiz/get handed the paper out, which is also
	// where the shift start was checked. Without this a page that was never
	// shown a paper (a "quiz not started" screen) could post an empty attempt.
	prev, started, err := loadAttempt(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load your attempt"})
		return
	}
	if prev.Submitted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already submitted this quiz", "code": "already_submitted"})
		return
	}
	if !started || !prev.Started {
		c.JSON(http.StatusForbidden, gin.H{"error": "You have not started this quiz"})
		return
	}
	if !prev.StartedAt.IsZero() && time.Now().After(prev.StartedAt.Add(TestDuration()+submitGrace)) {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrTimeUp.Error(), "code": "time_up"})
		return
	}

	var uq models.UserQuestions
	err = db.User_Questions.Coll.FindOne(db.User_Questions.Context, bson.M{"userID": userID}).Decode(&uq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user questions"})
		return
	}

	// questionID -> the option ids that belong to it.
	assigned := make(map[string]map[string]bool)
	for _, q := range uq.Questions {
		if q.QuestionID == nil {
			continue
		}
		opts := make(map[string]bool)
		for _, o := range q.Options {
			if o.Id != nil {
				opts[qidKey(o.Id)] = true
			}
		}
		assigned[qidKey(q.QuestionID)] = opts
	}

	// One answer per assigned question, and the answer must be one of that
	// question's own options. Repeating a question used to score every copy.
	seen := make(map[string]bool)
	for _, ans := range resp.Responses {
		if ans.QuestionID == nil || ans.Answer == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid answer"})
			return
		}
		qk := qidKey(ans.QuestionID)
		opts, ok := assigned[qk]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid answer for unassigned question"})
			return
		}
		if seen[qk] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Duplicate answer for a question"})
			return
		}
		seen[qk] = true
		if !opts[qidKey(ans.Answer)] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid option for question"})
			return
		}
	}

	dbResponses := models.Quiz_Responses{
		UserID:    userID,
		Responses: resp.Responses,
	}

	res, err := db.Quiz_Responses.Coll.UpdateOne(
		db.Quiz_Responses.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": dbResponses},
		options.Update().SetUpsert(true),
	)
	if writeErr, ok := err.(mongo.WriteException); ok && writeErr.HasErrorCode(11000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already submitted this quiz", "code": "already_submitted"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store responses"})
		return
	}
	// $setOnInsert leaves an existing attempt untouched and reports no error;
	// UpsertedCount is 0 exactly when an attempt was already on record.
	if res.UpsertedCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already submitted this quiz", "code": "already_submitted"})
		return
	}

	// The browser's warning count can be edited before it is sent; the server
	// has been counting every reported violation itself, so keep the larger.
	flags := resp.FlagsRaised
	if prev.Flags > flags {
		flags = prev.Flags
	}

	dbEntry := models.QuizTrack{
		UserID:      userID,
		SnapShot:    resp.Image,
		FlagsRaised: flags,
		// Scored on the spot so the admin portal shows real marks right away
		// instead of 0 until someone runs "Calculate Scores" (which still
		// recomputes everything, e.g. after an answer key is corrected).
		Marks: scoreResponses(resp.Responses),
	}

	_, err = db.Quiz_Track.Coll.UpdateOne(
		db.Quiz_Track.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": dbEntry},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		// The answers are safe; a missing track row only loses the snapshot.
		log.Printf("failed to store quiz track for user %v: %v", userID, err)
	}

	// Mark submitted before answering, and upsert, so the flag cannot be lost
	// when the attempt row is missing (it used to be a fire-and-forget update
	// filtered on quiz_started, which silently matched nothing).
	if _, err := db.Updates.Coll.UpdateOne(
		db.Updates.Context,
		bson.M{"userID": userID},
		bson.M{"$set": bson.M{"quiz_started": true, "quiz_submitted": true, "submitted_at": time.Now()}},
		options.Update().SetUpsert(true),
	); err != nil {
		log.Printf("failed to update submitted flag for user %v: %v", userID, err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Submission Successful"})
}

// FlagViolation records one proctoring violation as it happens, so the count
// survives a page reload and does not depend on what the browser reports at
// submit time. "strike" violations are the ones that count toward the
// auto-submit limit. POST /quiz/flag
func FlagViolation(c *gin.Context) {
	var req struct {
		Type   string `json:"type"`
		Strike bool   `json:"strike"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Type == "" || len(req.Type) > 40 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type required"})
		return
	}
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	inc := bson.M{"flags": 1}
	if req.Strike {
		inc["strikes"] = 1
	}
	_, err = db.Updates.Coll.UpdateOne(
		db.Updates.Context,
		bson.M{"userID": userID, "quiz_started": true, "quiz_submitted": bson.M{"$ne": true}},
		bson.M{
			"$inc": inc,
			"$push": bson.M{"flag_log": bson.M{
				"$each":  []bson.M{{"type": req.Type, "at": time.Now()}},
				"$slice": -200,
			}},
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record flag"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func IsSubmitted(c *gin.Context) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var result struct {
		Submitted bool `bson:"quiz_submitted"`
	}
	err = db.Updates.Coll.FindOne(
		db.Updates.Context,
		bson.M{"userID": userID},
		options.FindOne().SetProjection(bson.M{"quiz_submitted": 1, "_id": 0}),
	).Decode(&result)
	if err == mongo.ErrNoDocuments {
		// No tracking row yet simply means the user has not started the quiz.
		// That is not an auth failure, so don't answer 401 — the frontend reads
		// a 401 here as an expired session and sends the user back to sign in.
		c.JSON(http.StatusNotFound, gin.H{"submitted": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read submission status"})
		return
	}
	// In test mode a finished attempt is cleared when the paper is opened
	// again, so do not send the tester to the "submitted" page on sign-in.
	if result.Submitted && !TestMode() {
		c.JSON(http.StatusOK, gin.H{"submitted": true})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"submitted": false})
	}
}

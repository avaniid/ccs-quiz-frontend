package admin

import (
	"errors"
	"net/http"
	"sort"
	"time"

	models "ccs.quizportal/Models"
	quiz "ccs.quizportal/Quiz"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Direct slot assignment, per user, from the admin portal.
//
// The normal path is "Assign Shifts & Questions", which sweeps everyone in
// reg_responses and splits them across shifts round-robin. That needs the
// registration app to be live. These two handlers let an admin hand a slot to a
// single signed-in account instead, which is what testing needs before
// registration exists. They only ever write user_questions, so the registration
// flow and the bulk assignment are untouched.

type pendingUser struct {
	Email       string `json:"email"`
	Shift       int    `json:"shift"` // 0 when no paper assigned yet
	Assigned    bool   `json:"assigned"`
	Requested   bool   `json:"requested"`    // raised a slot request and is waiting
	RequestedAt string `json:"requested_at"` // local time, "" when never requested
	Submitted   bool   `json:"submitted"`    // finished; needs a reset to retake
	Started     bool   `json:"started"`      // opened the paper
	Flags       int    `json:"flags"`        // proctoring warnings recorded so far
	Marks       *int   `json:"marks"`        // nil until submitted; 0 until scores are calculated
}

// ResetAttempt clears one account's attempt so the same login can take the quiz
// again. Testing aid: a finished attempt is deliberately permanent, and there
// are only a handful of real @thapar.edu logins to test with.
// POST /admin/reset-attempt
func ResetAttempt(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email required"})
		return
	}

	var user models.AuthUser
	if err := db.AUTH.Coll.FindOne(
		db.AUTH.Context, bson.M{"userEmail": req.Email},
	).Decode(&user); err != nil || user.UserId == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "that account has not signed in yet"})
		return
	}

	filter := bson.M{"userID": user.UserId}
	// Answers, marks/flags/snapshot, and the started/submitted flags. The
	// assigned paper is kept, so the candidate can simply retake it.
	r1, err1 := db.Quiz_Responses.Coll.DeleteOne(db.Quiz_Responses.Context, filter)
	r2, err2 := db.Quiz_Track.Coll.DeleteOne(db.Quiz_Track.Context, filter)
	r3, err3 := db.Updates.Coll.DeleteOne(db.Updates.Context, filter)
	if err1 != nil || err2 != nil || err3 != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not clear the attempt"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "attempt cleared, this account can take the quiz again",
		"email":     req.Email,
		"responses": r1.DeletedCount,
		"track":     r2.DeletedCount,
		"updates":   r3.DeletedCount,
	})
}

// uidKey turns a stored 7-byte user id into a map key.
func uidKey(id models.UID) string {
	if id == nil {
		return ""
	}
	return string(id[:])
}

// loadByUser runs one query over a whole collection and hands each row to fn,
// so building the list costs a fixed handful of round trips instead of three
// per account.
func loadByUser(coll db.Collection, projection bson.M, fn func(cur *mongo.Cursor) error) error {
	cur, err := coll.Coll.Find(coll.Context, bson.M{}, options.Find().SetProjection(projection))
	if err != nil {
		return err
	}
	defer cur.Close(coll.Context)
	for cur.Next(coll.Context) {
		if err := fn(cur); err != nil {
			continue // one unreadable row must not hide everyone else
		}
	}
	return cur.Err()
}

// ListSignedInUsers reports every account that has signed in, with its paper,
// attempt and score. GET /admin/api/signed-in-users
//
// This used to run three queries per account. With the database in another
// region each round trip is ~0.2 s, so a hundred accounts took over a minute
// and the admin page looked frozen. It now reads each collection once.
func ListSignedInUsers(c *gin.Context) {
	rows := map[string]*pendingUser{}
	var order []string

	err := loadByUser(db.AUTH, bson.M{"userID": 1, "userEmail": 1}, func(cur *mongo.Cursor) error {
		var u models.AuthUser
		if err := cur.Decode(&u); err != nil || u.UserId == nil {
			return errSkip
		}
		k := uidKey(u.UserId)
		if _, seen := rows[k]; !seen {
			order = append(order, k)
		}
		rows[k] = &pendingUser{Email: u.Email}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load accounts"})
		return
	}

	// The papers themselves are large; only the owner and shift are needed.
	loadByUser(db.User_Questions, bson.M{"userID": 1, "shift": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID models.UID `bson:"userID"`
			Shift  int        `bson:"shift"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		if r := rows[uidKey(d.UserID)]; r != nil {
			r.Assigned, r.Shift = true, d.Shift
		}
		return nil
	})

	loadByUser(db.Updates, bson.M{"userID": 1, "quiz_started": 1, "quiz_submitted": 1, "flags": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID    models.UID `bson:"userID"`
			Started   bool       `bson:"quiz_started"`
			Submitted bool       `bson:"quiz_submitted"`
			Flags     int        `bson:"flags"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		if r := rows[uidKey(d.UserID)]; r != nil {
			r.Started, r.Submitted, r.Flags = d.Started, d.Submitted, d.Flags
		}
		return nil
	})

	// The snapshot is excluded: it is the bulk of every track row.
	loadByUser(db.Quiz_Track, bson.M{"userID": 1, "marks": 1, "flagsraised": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID models.UID `bson:"userID"`
			Marks  int        `bson:"marks"`
			Flags  int        `bson:"flagsraised"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		if r := rows[uidKey(d.UserID)]; r != nil {
			m := d.Marks
			r.Marks = &m
			if d.Flags > r.Flags {
				r.Flags = d.Flags
			}
		}
		return nil
	})

	loadByUser(db.Slot_Requests, bson.M{"userID": 1, "requested_at": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID      models.UID `bson:"userID"`
			RequestedAt time.Time  `bson:"requested_at"`
		}
		if err := cur.Decode(&d); err != nil {
			return err
		}
		if r := rows[uidKey(d.UserID)]; r != nil {
			r.Requested = true
			r.RequestedAt = d.RequestedAt.Local().Format("02 Jan, 15:04")
		}
		return nil
	})

	users := make([]pendingUser, 0, len(order))
	for _, k := range order {
		users = append(users, *rows[k])
	}

	// Waiting-and-unassigned first: that is the queue the admin is here to clear.
	sort.SliceStable(users, func(i, j int) bool {
		wi := users[i].Requested && !users[i].Assigned
		wj := users[j].Requested && !users[j].Assigned
		if wi != wj {
			return wi
		}
		return users[i].Email < users[j].Email
	})

	c.JSON(http.StatusOK, gin.H{"users": users, "test_mode": quiz.TestMode()})
}

var errSkip = errors.New("skip row")

// AssignSlotToUser gives one account a shift and a freshly shuffled question
// set, replacing any paper it already had. POST /admin/assign-slot
func AssignSlotToUser(c *gin.Context) {
	var req struct {
		Email           string `json:"email"`
		Shift           int    `json:"shift"`
		QuestionsPerSet int    `json:"questions_per_set"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Shift < 1 || req.QuestionsPerSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, shift and questions_per_set required"})
		return
	}

	var user models.AuthUser
	if err := db.AUTH.Coll.FindOne(
		db.AUTH.Context, bson.M{"userEmail": req.Email},
	).Decode(&user); err != nil || user.UserId == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "that account has not signed in yet"})
		return
	}

	bank, err := loadQuestionBank()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load quiz questions"})
		return
	}
	if len(bank) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no quiz questions in the bank yet"})
		return
	}
	set, _ := pickSet(bank, req.Shift, req.QuestionsPerSet)

	entry := models.UserQuestions{
		UserID:    user.UserId,
		Shift:     req.Shift,
		Questions: personalize(set),
	}
	// Replace rather than insert, so re-assigning a slot during testing does not
	// leave the account holding two papers.
	if _, err := db.User_Questions.Coll.DeleteOne(
		db.User_Questions.Context, bson.M{"userID": user.UserId},
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear previous paper"})
		return
	}
	if _, err := db.User_Questions.Coll.InsertOne(db.User_Questions.Context, entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign paper"})
		return
	}

	// The request has been answered, so drop it from the waiting queue.
	db.Slot_Requests.Coll.DeleteOne(db.Slot_Requests.Context, bson.M{"userID": user.UserId})

	c.JSON(http.StatusOK, gin.H{
		"message":   "slot assigned",
		"email":     req.Email,
		"shift":     req.Shift,
		"questions": len(entry.Questions),
	})
}

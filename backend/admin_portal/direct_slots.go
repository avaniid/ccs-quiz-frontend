package admin

import (
	"net/http"
	"sort"
	"time"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
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

// ListSignedInUsers reports every account that has signed in, and whether it
// already holds a paper. GET /admin/api/signed-in-users
func ListSignedInUsers(c *gin.Context) {
	cur, err := db.AUTH.Coll.Find(db.AUTH.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load accounts"})
		return
	}
	defer cur.Close(db.AUTH.Context)

	users := []pendingUser{}
	for cur.Next(db.AUTH.Context) {
		var u models.AuthUser
		if err := cur.Decode(&u); err != nil || u.UserId == nil {
			continue
		}
		row := pendingUser{Email: u.Email}
		var uq models.UserQuestions
		if err := db.User_Questions.Coll.FindOne(
			db.User_Questions.Context, bson.M{"userID": u.UserId},
		).Decode(&uq); err == nil {
			row.Assigned = true
			row.Shift = uq.Shift
		}

		var done struct {
			Submitted bool `bson:"quiz_submitted"`
		}
		if err := db.Updates.Coll.FindOne(
			db.Updates.Context, bson.M{"userID": u.UserId},
		).Decode(&done); err == nil {
			row.Submitted = done.Submitted
		}

		var reqDoc struct {
			RequestedAt time.Time `bson:"requested_at"`
		}
		if err := db.Slot_Requests.Coll.FindOne(
			db.Slot_Requests.Context, bson.M{"userID": u.UserId},
		).Decode(&reqDoc); err == nil {
			row.Requested = true
			row.RequestedAt = reqDoc.RequestedAt.Local().Format("02 Jan, 15:04")
		}
		users = append(users, row)
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

	c.JSON(http.StatusOK, gin.H{"users": users})
}

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

	var questions []models.Quiz_Questions
	qCur, err := db.Quiz_Questions.Coll.Find(db.Quiz_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load quiz questions"})
		return
	}
	defer qCur.Close(db.Quiz_Questions.Context)
	for qCur.Next(db.Quiz_Questions.Context) {
		var q models.Quiz_Questions
		if qCur.Decode(&q) == nil {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no quiz questions in the bank yet"})
		return
	}
	if req.QuestionsPerSet > len(questions) {
		req.QuestionsPerSet = len(questions)
	}
	shuffleQuestions(questions)

	entry := models.UserQuestions{
		UserID:    user.UserId,
		Shift:     req.Shift,
		Questions: questions[:req.QuestionsPerSet],
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

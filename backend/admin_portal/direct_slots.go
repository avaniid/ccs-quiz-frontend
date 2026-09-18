package admin

import (
	"net/http"

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
	Email    string `json:"email"`
	Shift    int    `json:"shift"`    // 0 when no paper assigned yet
	Assigned bool   `json:"assigned"`
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
		users = append(users, row)
	}
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

	c.JSON(http.StatusOK, gin.H{
		"message":   "slot assigned",
		"email":     req.Email,
		"shift":     req.Shift,
		"questions": len(entry.Questions),
	})
}

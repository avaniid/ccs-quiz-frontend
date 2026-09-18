package quiz

import (
	"net/http"
	"time"

	login "ccs.quizportal/Login"
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Slot requests are a testing aid: they let a signed-in candidate raise their
// hand for a shift before the registration app exists. The real flow assigns
// papers in bulk from reg_responses, and this collection plays no part in it.

// RequestSlot records that the signed-in user is waiting for a shift.
// POST /quiz/request-slot
func RequestSlot(c *gin.Context) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Already has a paper: nothing to request.
	var uq models.UserQuestions
	if err := db.User_Questions.Coll.FindOne(
		db.User_Questions.Context, bson.M{"userID": userID},
	).Decode(&uq); err == nil {
		c.JSON(http.StatusOK, gin.H{"status": "assigned", "shift": uq.Shift})
		return
	}

	// Upsert so repeated clicks keep one row and don't reset the original time.
	_, err = db.Slot_Requests.Coll.UpdateOne(
		db.Slot_Requests.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": bson.M{
			"userID":       userID,
			"requested_at": time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		if we, ok := err.(mongo.WriteException); !ok || !we.HasErrorCode(11000) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record your request"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "pending"})
}

// SlotStatus tells the frontend where the user stands, so it can poll after
// raising a request instead of making them guess. GET /quiz/slot-status
func SlotStatus(c *gin.Context) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var uq models.UserQuestions
	if err := db.User_Questions.Coll.FindOne(
		db.User_Questions.Context, bson.M{"userID": userID},
	).Decode(&uq); err == nil {
		c.JSON(http.StatusOK, gin.H{"status": "assigned", "shift": uq.Shift})
		return
	}

	n, _ := db.Slot_Requests.Coll.CountDocuments(db.Slot_Requests.Context, bson.M{"userID": userID})
	if n > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "none"})
}

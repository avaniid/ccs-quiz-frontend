package quiz

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
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

// const shift int = 1 //later will make it central , time mapped

var (
	shiftsMu  sync.RWMutex
	shiftsMap map[int]models.Shift
)

// getShift returns a shift's timing, loading the cache on first use and
// reloading it whenever the requested shift is missing.
//
// Reloading on a miss matters because admins set shift timings from the portal
// while the server is running: LoadAllShifts always returns a non-nil map, so a
// cache first populated before any timing existed would never refresh itself and
// every candidate would be told their shift does not exist until a restart.
//
// The mutex matters because every candidate hits this at once when a shift
// opens, and concurrent read/write of a plain map panics the process.
func getShift(shift int) (models.Shift, error) {
	shiftsMu.RLock()
	s, ok := shiftsMap[shift]
	shiftsMu.RUnlock()
	if ok {
		return s, nil
	}

	shiftsMu.Lock()
	defer shiftsMu.Unlock()
	if s, ok := shiftsMap[shift]; ok { // another request may have just reloaded
		return s, nil
	}

	loaded, err := LoadAllShifts()
	if err != nil {
		log.Printf("failed to load shift timings: %v", err)
		return models.Shift{}, errors.New("could not load shift timings")
	}
	shiftsMap = loaded

	s, ok = shiftsMap[shift]
	if !ok {
		return models.Shift{}, fmt.Errorf("no timing has been set for shift %d yet", shift)
	}
	return s, nil
}

func GetQuizQuestions(c *gin.Context) ([]models.Quiz_Questions, error) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		return nil, err
	}

	var uq models.UserQuestions
	filter := bson.M{"userID": userID}
	err = db.User_Questions.Coll.FindOne(db.User_Questions.Context, filter).Decode(&uq)
	if err == mongo.ErrNoDocuments {
		// The candidate is authenticated but no paper has been assigned to them
		// yet (admin has not run "Assign Shifts"). Say so instead of surfacing
		// the raw driver error to them.
		return nil, errors.New("no quiz has been assigned to your account yet")
	}
	if err != nil {
		log.Printf("failed to load questions for user %v: %v", userID, err)
		return nil, errors.New("could not load your quiz, please try again")
	}

	shiftInfo, err := getShift(uq.Shift)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(shiftInfo.Start.Location())
	if now.Before(shiftInfo.Start) {
		return nil, errors.New("quiz not started for your shift yet")
	}

	dbEntry := models.Updates{
		UserID:    userID,
		Started:   true,
		Submitted: false,
	}

	_, err = db.Updates.Coll.UpdateOne(
		db.Updates.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": dbEntry},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Failed to insert/update started flag for user %v: %v", userID, err)
	}

	return uq.Questions, nil
}

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

	var uq models.UserQuestions
	filter := bson.M{"userID": userID}
	err = db.User_Questions.Coll.FindOne(db.User_Questions.Context, filter).Decode(&uq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user questions"})
		return
	}

	assignedQIDs := make(map[string]bool)
	for _, q := range uq.Questions {
		if q.QuestionID != nil {
			assignedQIDs[hex.EncodeToString(q.QuestionID[:])] = true
		}
	}

	for _, ans := range resp.Responses {
		if ans.QuestionID == nil || !assignedQIDs[hex.EncodeToString(ans.QuestionID[:])] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid answer for unassigned question"})
			return
		}
	}

	dbResponses := models.Quiz_Responses{
		UserID:    userID,
		Responses: resp.Responses,
	}

	_, err = db.Quiz_Responses.Coll.UpdateOne(
		db.Quiz_Responses.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": dbResponses},
		options.Update().SetUpsert(true),
	)
	if writeErr, ok := err.(mongo.WriteException); ok && writeErr.HasErrorCode(11000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User has already submitted"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store responses"})
		return
	}

	dbEntry := models.QuizTrack{
		UserID:      userID,
		SnapShot:    resp.Image,
		FlagsRaised: resp.FlagsRaised,
		Marks:       0,
	}

	_, err = db.Quiz_Track.Coll.UpdateOne(
		db.Quiz_Track.Context,
		bson.M{"userID": userID},
		bson.M{"$setOnInsert": dbEntry},
		options.Update().SetUpsert(true),
	)
	if writeErr, ok := err.(mongo.WriteException); ok && writeErr.HasErrorCode(11000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User has already submitted"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store quiz track"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Submission Successful"})

	go func(userID models.UID) {
		filter := bson.M{"userID": userID, "quiz_started": true}
		update := bson.M{"$set": bson.M{"quiz_submitted": true}}

		_, err := db.Updates.Coll.UpdateOne(db.Updates.Context, filter, update)
		if err != nil {
			log.Printf("failed to update submitted flag for user %v: %v", userID, err)
		}
	}(userID)
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
	if result.Submitted {
		c.JSON(http.StatusOK, gin.H{"submitted": true})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"submitted": false})
	}
}

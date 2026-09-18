package registerationform

import (
	"log"
	"net/http"

	login "ccs.quizportal/Login"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SubmitForm(c *gin.Context) {
	StoreRegResponse(c)
}

func GetAllQues(c *gin.Context) {
	// get ques from mongo and send to frontend
	questions, err := GetRegQuestions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch registeration questions , sad :/"})
		log.Fatal("reg ques nhi mile")
	}
	c.JSON(http.StatusOK, gin.H{"regQuestions": questions})
}

func Check_Registered(c *gin.Context) {
	userID, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"registered": false, "error": "unauthorized"})
		return
	}

	filter := map[string]interface{}{"userID": userID}
	err = db.Registeration_Responses.Coll.FindOne(
		db.Registeration_Responses.Context,
		filter,
		options.FindOne().SetProjection(map[string]interface{}{"_id": 1}),
	).Err()

	if err == nil {
		c.JSON(http.StatusOK, gin.H{"registered": true})
	} else if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusOK, gin.H{"registered": false})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"registered": false, "error": "db error"})
	}
}

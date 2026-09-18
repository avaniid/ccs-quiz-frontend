package registerationform

import (
	"net/http"

	login "ccs.quizportal/Login"
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func GetRegQuestions() ([]models.Questions, error) {

	var questions []models.Questions
	cursor, err := db.Registeration_Questions.Coll.Find(db.Registeration_Questions.Context, bson.M{})

	if err != nil {
		return nil, err
	}
	defer cursor.Close(db.Registeration_Questions.Context)

	for cursor.Next(db.Registeration_Questions.Context) {
		var q models.Questions
		if err := cursor.Decode(&q); err != nil {
			return nil, err
		}

		questions = append(questions, q)

	}
	return questions, cursor.Err()
}

func StoreRegResponse(c *gin.Context) {
	var resp models.RespUser
	if err := c.ShouldBindJSON(&resp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var questions []models.Questions
	cursor, err := db.Registeration_Questions.Coll.Find(db.Registeration_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch questions"})
		return
	}
	if err := cursor.All(db.Registeration_Questions.Context, &questions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse questions"})
		return
	}

	// Validate answers
	if err := ValidateAnswers(resp.RegAnswers, questions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, err := login.GetUIDFromSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	dbEntry := models.RegResponseDb{
		UserID:    uid,
		Responses: resp,
	}

	_, err = db.Registeration_Responses.Coll.InsertOne(db.Registeration_Responses.Context, dbEntry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Response stored"})
}

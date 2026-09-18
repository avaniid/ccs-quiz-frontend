package quiz

import (
	"net/http"
	"os"
	"strconv"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

// @Summary      Submit quiz response
// @Description  User submits answers
// @Tags         quiz
// @Accept       json
// @Produce      json
// @Param        request  body      models.Form_Responses  true  "Quiz Response"
// @Success      200      {object}  map[string]string "Success message"
// @Failure      400      {object}  map[string]string "Invalid request"
// @Failure      500      {object}  map[string]string "Internal server error"
// @Router       /quiz/submit [post]
func SubmitQuiz(c *gin.Context) {
	// var Resp models.Form_Responses
	// err := c.ShouldBindJSON(&Resp)

	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid Request"})
	// 	return
	// }

	RecieveResponse(c)
}

// @Summary      Get quiz Questions
// @Description  Send questions for each user to the frontend
// @Tags         quiz
// @Produce      json
// @Success      200      {object}  map[string][]models.Quiz_Questions
// @Failure      500      {object}  map[string]string
// @Router       /quiz/get [get]
func GetQuizQues(c *gin.Context) {

	questions, err := GetQuizQuestions(c)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"questions": questions})

}

func GetScores(c *gin.Context) {
	err := CalcScore()
	if(err!=nil){
		c.JSON(http.StatusInternalServerError,gin.H{"error":"nhi hua score calc :/ "})
		return
	}
	c.JSON(http.StatusAccepted,gin.H{"success":"Scores have been updated in the db yayayay"})
}

func GetAppConfig(c *gin.Context) {
	cur, err := db.Shifts.Coll.Find(db.Shifts.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query shifts"})
		return
	}
	defer cur.Close(db.Shifts.Context)

	var shifts []models.Shift
	for cur.Next(db.Shifts.Context) {
		var s models.Shift
		if err := cur.Decode(&s); err == nil {
			shifts = append(shifts, s)
		}
	}

	testDuration := 80
	if v := os.Getenv("TEST_DURATION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			testDuration = n
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"shifts":        shifts,
		"test_duration": testDuration,
	})
}

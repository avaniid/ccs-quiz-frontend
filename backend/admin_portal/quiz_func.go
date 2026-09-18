package admin

import (
	"crypto/rand"
	"net/http"
	"strconv"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
)

func ShowAddQuizQuestion(c *gin.Context) {
	c.HTML(http.StatusOK, "add_quiz_question.html", gin.H{})
}
func GenerateOID() models.OID {
	var id [7]byte
	_, err := rand.Read(id[:])
	if err != nil {
		panic("failed to generate OID")
	}
	return &id
}

func AddQuizQuestion(c *gin.Context) {
	question := c.PostForm("question")
	shiftStr := c.PostForm("shift")
	imageURL := c.PostForm("image_url")
	optionValues := c.PostFormArray("option_value")
	correctOptionStr := c.PostForm("correct_option")

	shift, err := strconv.Atoi(shiftStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid shift value")
		return
	}

	var options []models.Option
	var correctOID models.OID
	for idx, val := range optionValues {
		oid := GenerateOID()
		options = append(options, models.Option{
			Value: val,
			Id:    oid,
		})
		if strconv.Itoa(idx) == correctOptionStr {
			correctOID = oid
		}
	}
	qID := GenerateQID()
	quizQ := models.Quiz_Questions{
		QuestionID: qID,
		Question:   question,
		Options:    options,
		Image:      imageURL,
		Shift:      shift,
	}

	quizAnswer := models.Quiz_Answer{
		QuestionID: qID,
		Answer:     correctOID,
	}
	_, err = db.Quiz_Questions.Coll.InsertOne(db.Quiz_Questions.Context, quizQ)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to add question: %v", err)
		return
	}

	_, err = db.Quiz_Answers.Coll.InsertOne(db.Quiz_Answers.Context, quizAnswer)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to add answer: %v", err)
		return
	}

	c.String(http.StatusOK, "Quiz question added successfully!")
}

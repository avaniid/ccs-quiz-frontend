package models

type Questions struct {
	QuestionID   QID    `bson:"questionID"`
	QuestionType bool   `bson:"questionType"`
	Question     string `bson:"regQuestions"`
	Validation string `bson:"regex"`
} // export to frontend

type Answer struct {
	QuestionID   QID    `bson:"questionID"`
	QuestionType bool   `bson:"questionType"`
	Answer       string `bson:"regAnswers"`
} // return from frontend and put in mongo

type RegResponseDb struct {
	UserID    UID      `bson:"userID"`
	Responses RespUser `bson:"response_reg"`
}

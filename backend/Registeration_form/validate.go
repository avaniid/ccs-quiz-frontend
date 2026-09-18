package registerationform

import (
	"encoding/hex"
	"errors"
	"regexp"

	models "ccs.quizportal/Models"
)

func qidToString(qid models.QID) string {
	if qid == nil {
		return ""
	}
	return hex.EncodeToString(qid[:])
}

func qidEqual(a, b models.QID) bool {
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < 7; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ValidateAnswers(answers []models.Answer, questions []models.Questions) error {
	for _, ans := range answers {
		found := false
		var q models.Questions
		for _, ques := range questions {
			if qidEqual(ans.QuestionID, ques.QuestionID) {
				found = true
				q = ques
				break
			}
		}
		if !found {
			return errors.New("invalid question")
		}
		// Only validate if the question is compulsory
		if q.QuestionType && q.Validation != "" {
			matched, _ := regexp.MatchString(q.Validation, ans.Answer)
			if !matched {
				return errors.New("answer for question '" + q.Question + "' does not match validation")
			}
		}
	}
	return nil
}

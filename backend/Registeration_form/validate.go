package registerationform

import (
	"encoding/hex"
	"errors"
	"regexp"
	"strings"

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
	// Every compulsory question needs a non-blank answer. Only the answers that
	// were sent used to be checked, so leaving a compulsory one out passed.
	for _, q := range questions {
		if !q.QuestionType {
			continue
		}
		answered := false
		for _, ans := range answers {
			if qidEqual(ans.QuestionID, q.QuestionID) && strings.TrimSpace(ans.Answer) != "" {
				answered = true
				break
			}
		}
		if !answered {
			return errors.New("question '" + q.Question + "' is compulsory")
		}
	}

	seen := make(map[string]bool)
	for _, ans := range answers {
		key := qidToString(ans.QuestionID)
		if seen[key] {
			return errors.New("duplicate answer")
		}
		seen[key] = true

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
		// Optional questions may be left blank, but anything typed into one
		// still has to match its format.
		if q.Validation != "" && (q.QuestionType || strings.TrimSpace(ans.Answer) != "") {
			matched, _ := regexp.MatchString(q.Validation, ans.Answer)
			if !matched {
				return errors.New("answer for question '" + q.Question + "' does not match validation")
			}
		}
	}
	return nil
}

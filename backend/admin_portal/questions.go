package admin

import (
	"encoding/hex"
	"net/http"
	"sort"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// The question bank page: every quiz question with its correct option and how
// many papers carry it, and a delete that removes the question and its answer
// together.
//
// Papers hold their own copy of each question, so deleting from the bank does
// not change papers already handed out. Re-importing replaces the unopened
// ones; an opened paper keeps the question, and without its answer it can no
// longer score marks. The delete refuses in that case unless forced.

type questionRow struct {
	ID       string   `json:"id"` // hex of the 7-byte question id
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Correct  int      `json:"correct"` // index into Options, -1 when no answer is stored
	Image    string   `json:"image"`
	Shift    int      `json:"shift"`
	Papers   int      `json:"papers"`        // papers that include it
	Opened   int      `json:"opened_papers"` // of those, ones the candidate has opened
}

func ShowQuestionsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "questions.html", nil)
}

func qidKey(id models.QID) string {
	if id == nil {
		return ""
	}
	return string(id[:])
}

// paperUsage counts, per question, the papers that include it and how many of
// those have been opened.
func paperUsage() (papers, opened map[string]int, err error) {
	started := map[string]bool{}
	if err := loadByUser(db.Updates, bson.M{"userID": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID models.UID `bson:"userID"`
		}
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		started[uidKey(d.UserID)] = true
		return nil
	}); err != nil {
		return nil, nil, err
	}

	papers, opened = map[string]int{}, map[string]int{}
	err = loadByUser(db.User_Questions, bson.M{"userID": 1, "questions.questionID": 1}, func(cur *mongo.Cursor) error {
		var d struct {
			UserID    models.UID `bson:"userID"`
			Questions []struct {
				QuestionID models.QID `bson:"questionID"`
			} `bson:"questions"`
		}
		if err := cur.Decode(&d); err != nil || d.UserID == nil {
			return errSkip
		}
		isOpen := started[uidKey(d.UserID)]
		for _, q := range d.Questions {
			k := qidKey(q.QuestionID)
			papers[k]++
			if isOpen {
				opened[k]++
			}
		}
		return nil
	})
	return papers, opened, err
}

// GET /admin/api/questions
func ListQuestions(c *gin.Context) {
	bank, err := loadQuestionBank()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load questions"})
		return
	}
	answers := map[string]string{}
	if err := loadByUser(db.Quiz_Answers, bson.M{}, func(cur *mongo.Cursor) error {
		var a models.Quiz_Answer
		if err := cur.Decode(&a); err != nil || a.QuestionID == nil || a.Answer == nil {
			return errSkip
		}
		answers[qidKey(a.QuestionID)] = string(a.Answer[:])
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load answers"})
		return
	}
	papers, opened, err := paperUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load papers"})
		return
	}

	out := make([]questionRow, 0, len(bank))
	for _, q := range bank {
		if q.QuestionID == nil {
			continue
		}
		k := qidKey(q.QuestionID)
		r := questionRow{
			ID:       hex.EncodeToString(q.QuestionID[:]),
			Question: q.Question,
			Options:  make([]string, 0, len(q.Options)),
			Correct:  -1,
			Image:    q.Image,
			Shift:    q.Shift,
			Papers:   papers[k],
			Opened:   opened[k],
		}
		for i, o := range q.Options {
			r.Options = append(r.Options, o.Value)
			if o.Id != nil && answers[k] == string(o.Id[:]) {
				r.Correct = i
			}
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Shift < out[j].Shift })
	c.JSON(http.StatusOK, gin.H{"questions": out})
}

// POST /admin/api/delete-question {"id": "<hex>", "force": false}
func DeleteQuestion(c *gin.Context) {
	var req struct {
		ID    string `json:"id"`
		Force bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	raw, err := hex.DecodeString(req.ID)
	if err != nil || len(raw) != 7 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid question id"})
		return
	}
	var id [7]byte
	copy(id[:], raw)
	qid := models.QID(&id)

	papers, opened, err := paperUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check papers"})
		return
	}
	if n := opened[qidKey(qid)]; n > 0 && !req.Force {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "candidates have already opened papers with this question; deleting it means nobody can score on it",
			"papers":        papers[qidKey(qid)],
			"opened_papers": n,
		})
		return
	}

	filter := bson.M{"questionID": qid}
	rq, err := db.Quiz_Questions.Coll.DeleteOne(db.Quiz_Questions.Context, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete the question"})
		return
	}
	ra, err := db.Quiz_Answers.Coll.DeleteOne(db.Quiz_Answers.Context, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "question deleted, but its answer could not be"})
		return
	}
	if rq.DeletedCount == 0 && ra.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "question not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":        "question deleted",
		"papers_with_it": papers[qidKey(qid)],
	})
}

package quiz

import (
	"errors"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TEST_MODE=true is for the testing phase: anyone who signs in gets a paper
// straight away (no slot request, no admin assignment, no shift timing), and a
// finished attempt can be taken again. Leave it unset for the real exam.

var ErrNoQuestionsInBank = errors.New("no quiz questions have been added yet")

func TestMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TEST_MODE")), "true")
}

// testQuestionCount is how many questions a test paper gets (TEST_QUESTIONS,
// default 10, capped at the size of the bank).
func testQuestionCount() int {
	if n, err := strconv.Atoi(os.Getenv("TEST_QUESTIONS")); err == nil && n > 0 {
		return n
	}
	return 10
}

// ensureTestPaper hands the candidate a paper drawn from the whole question
// bank if they do not have one yet, and returns it.
func ensureTestPaper(userID models.UID) (models.UserQuestions, error) {
	var uq models.UserQuestions
	err := db.User_Questions.Coll.FindOne(db.User_Questions.Context, bson.M{"userID": userID}).Decode(&uq)
	if err == nil {
		return uq, nil
	}
	if err != mongo.ErrNoDocuments {
		return uq, err
	}

	var bank []models.Quiz_Questions
	cur, err := db.Quiz_Questions.Coll.Find(db.Quiz_Questions.Context, bson.M{})
	if err != nil {
		return uq, err
	}
	if err := cur.All(db.Quiz_Questions.Context, &bank); err != nil {
		return uq, err
	}
	if len(bank) == 0 {
		return uq, ErrNoQuestionsInBank
	}

	rand.Shuffle(len(bank), func(i, j int) { bank[i], bank[j] = bank[j], bank[i] })
	if n := testQuestionCount(); n < len(bank) {
		bank = bank[:n]
	}
	for i := range bank {
		opts := append([]models.Option(nil), bank[i].Options...)
		rand.Shuffle(len(opts), func(a, b int) { opts[a], opts[b] = opts[b], opts[a] })
		bank[i].Options = opts
	}

	uq = models.UserQuestions{UserID: userID, Shift: 0, Questions: bank}
	if _, err := db.User_Questions.Coll.InsertOne(db.User_Questions.Context, uq); err != nil {
		// Two tabs opening at once: the other one won, use its paper.
		if we, ok := err.(mongo.WriteException); ok && we.HasErrorCode(11000) {
			err = db.User_Questions.Coll.FindOne(db.User_Questions.Context, bson.M{"userID": userID}).Decode(&uq)
		}
		return uq, err
	}
	return uq, nil
}

// resetTestAttempt clears a finished attempt so the same login can take the
// paper again, exactly like the admin portal's "Reset attempt".
func resetTestAttempt(userID models.UID) error {
	filter := bson.M{"userID": userID}
	for _, c := range []db.Collection{db.Quiz_Responses, db.Quiz_Track, db.Updates} {
		if _, err := c.Coll.DeleteOne(c.Context, filter); err != nil {
			return err
		}
	}
	log.Printf("test mode: cleared finished attempt for user %v", userID)
	return nil
}

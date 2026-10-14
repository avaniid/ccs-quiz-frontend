package quiz

import (
	"log"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CalcScore() error {
	ctx := db.Quiz_Responses.Context

	respCursor, err := db.Quiz_Responses.Coll.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer respCursor.Close(ctx)
	ansCursor, err := db.Quiz_Answers.Coll.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer ansCursor.Close(ctx)

	answerMap := make(map[[7]byte][7]byte)

	for ansCursor.Next(ctx) {
		var ans models.Quiz_Answer
		if err := ansCursor.Decode(&ans); err != nil {
			log.Printf("calcScore: skipping unreadable answer key: %v", err)
			continue
		}
		if ans.QuestionID == nil || ans.Answer == nil {
			continue
		}
		answerMap[*ans.QuestionID] = *ans.Answer
	}

	for respCursor.Next(ctx) {
		var response models.Quiz_Responses

		if err := respCursor.Decode(&response); err != nil {
			log.Printf("calcScore: skipping unreadable response: %v", err)
			continue
		}
		if response.UserID == nil {
			continue
		}

		score := 0
		// Only the first answer per question counts, and malformed entries are
		// skipped instead of dereferenced (a nil id used to panic the handler
		// halfway through, leaving some candidates scored and others not).
		counted := make(map[[7]byte]bool)
		for _, resp := range response.Responses {
			if resp.QuestionID == nil || resp.Answer == nil || counted[*resp.QuestionID] {
				continue
			}
			counted[*resp.QuestionID] = true
			if correct, ok := answerMap[*resp.QuestionID]; ok && correct == *resp.Answer {
				score = score + 4
			} else {
				score = score - 1
			}
		}
		// fmt.Println("id: ", *response.UserID)
		// fmt.Println("score: ", score)

		filter := bson.M{"userID": *response.UserID}
		update := bson.M{"$set": bson.M{"marks": score}}

		opts := options.Update().SetUpsert(true)
		_, err := db.Quiz_Track.Coll.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Printf("failed to update marks for user %v: %v\n", *response.UserID, err)
			continue
		}
	}
	return respCursor.Err()
}

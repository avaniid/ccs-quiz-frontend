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
		err := ansCursor.Decode(&ans)
		if err != nil {
			log.Fatal(err)
		}
		answerMap[*ans.QuestionID] = *ans.Answer
	}

	for respCursor.Next(ctx) {
		var response models.Quiz_Responses

		if err := respCursor.Decode(&response); err != nil {
			log.Fatal(err)
		}

		score := 0

		for _, resp := range response.Responses {
			if answerMap[*resp.QuestionID] == *resp.Answer {
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
		
		if err := respCursor.Err(); err != nil {
		return err
		}
	}
	return nil
}

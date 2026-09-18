package admin

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func RemoveDuplicateRegistrations(c *gin.Context) {
	coll := db.Registeration_Responses.Coll
	authColl := db.AUTH.Coll

	type Answer struct {
		QuestionID   any    `bson:"questionID"`
		QuestionType bool   `bson:"questionType"`
		RegAnswers   string `bson:"regAnswers"`
	}

	type ResponseDoc struct {
		ID          any `bson:"_id"`
		UserID      any `bson:"userID"`
		ResponseReg struct {
			Answers []Answer `bson:"answers"`
		} `bson:"response_reg"`
	}

	type AuthDoc struct {
		UserID    any    `bson:"userID"`
		UserEmail string `bson:"userEmail"`
	}

	cursor, err := coll.Find(db.Registeration_Responses.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
		return
	}
	defer cursor.Close(db.Registeration_Responses.Context)

	userMap := make(map[string][]ResponseDoc)
	for cursor.Next(db.Registeration_Responses.Context) {
		var doc ResponseDoc
		if err := cursor.Decode(&doc); err == nil {
			uid := ""
			switch v := doc.UserID.(type) {
			case []byte:
				uid = fmt.Sprintf("%x", v)
			case string:
				uid = v
			default:
				uid = fmt.Sprintf("%v", v)
			}
			userMap[uid] = append(userMap[uid], doc)
		}
	}

	removedUsers := []gin.H{}

	for _, docs := range userMap {
		if len(docs) > 1 {
			sort.Slice(docs, func(i, j int) bool {
				oid1, ok1 := docs[i].ID.(primitive.ObjectID)
				oid2, ok2 := docs[j].ID.(primitive.ObjectID)
				if ok1 && ok2 {
					return oid1.Timestamp().Before(oid2.Timestamp())
				}
				return i < j
			})
			for _, dup := range docs[1:] {
				_, err := coll.DeleteOne(db.Registeration_Responses.Context, bson.M{"_id": dup.ID})
				if err != nil {
					continue
				}
				name := ""
				for _, ans := range dup.ResponseReg.Answers {
					if ans.RegAnswers != "" {
						name = ans.RegAnswers
						break
					}
				}

				var authDoc AuthDoc
				err = authColl.FindOne(db.Registeration_Responses.Context, bson.M{"userID": dup.UserID}).Decode(&authDoc)
				email := ""
				if err == nil {
					email = authDoc.UserEmail
				}
				removedUsers = append(removedUsers, gin.H{
					"name":  name,
					"email": email,
				})

				log.Printf("Removed duplicate registration: %v (%v)", name, email)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"removed": removedUsers,
		"count":   len(removedUsers),
	})
}

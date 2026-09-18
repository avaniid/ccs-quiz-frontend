package admin

// A TOTALLY VIBE CODED FUNCTION....DIDNT HAVE THE CAPACITY and BANDWIDTH TO RIGHT IT BY OWN
// CAN BE MADE REALLY BETTER
// I MISS PANDASSSSS ...Go ahchi hai..but Python has its rizz.

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ShowRegResponses(c *gin.Context) {
	c.HTML(http.StatusOK, "reg_responses.html", nil)
}

// Convert various Mongo ID types into hex string
func toHexID(v interface{}) string {
	switch t := v.(type) {
	case primitive.Binary:
		return hex.EncodeToString(t.Data)
	case primitive.ObjectID:
		return t.Hex()
	case []byte:
		return hex.EncodeToString(t)
	case string:
		return t
	default:
		return ""
	}
}

// GetRegResponsesData builds table data for reg_responses.html
func GetRegResponsesData(c *gin.Context) {
	ctx := db.Registeration_Questions.Context

	// -------------------
	// 1) Load registration questions
	// -------------------
	qMap := make(map[string]string) // qidHex -> question text
	var qOrder []string             // preserve order

	qCur, err := db.Registeration_Questions.Coll.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load registration questions"})
		return
	}
	defer qCur.Close(ctx)

	for qCur.Next(ctx) {
		var doc bson.M
		if err := qCur.Decode(&doc); err != nil {
			continue
		}

		var qidHex string
		if v, ok := doc["questionID"]; ok {
			qidHex = toHexID(v)
		} else if v, ok := doc["QuestionID"]; ok {
			qidHex = toHexID(v)
		} else if v, ok := doc["_id"]; ok {
			qidHex = toHexID(v)
		}
		if qidHex == "" {
			continue
		}

		qtext := ""
		if v, ok := doc["regQuestions"].(string); ok && v != "" {
			qtext = v
		} else if v, ok := doc["question"].(string); ok && v != "" {
			qtext = v
		} else if v, ok := doc["Question"].(string); ok && v != "" {
			qtext = v
		}
		if qtext == "" {
			qtext = qidHex
		}

		if _, exists := qMap[qidHex]; !exists {
			qMap[qidHex] = qtext
			qOrder = append(qOrder, qidHex)
		}
	}

	// -------------------
	// 2) Load auth users for name/email/roll
	// -------------------
	userMap := make(map[string]map[string]string)
	authCur, err := db.AUTH.Coll.Find(ctx, bson.M{})

	if err == nil {
		defer authCur.Close(ctx)
		for authCur.Next(ctx) {
			var doc bson.M
			if err := authCur.Decode(&doc); err != nil {
				continue
			}
			var uidHex string
			if v, ok := doc["userID"]; ok {
				uidHex = toHexID(v)
			} else if v, ok := doc["_id"]; ok {
				uidHex = toHexID(v)
			}
			if uidHex == "" {
				continue
			}

			info := make(map[string]string)

			// email fallback checks
			if v, ok := doc["userEmail"].(string); ok && v != "" {
				info["email"] = v
			} else if v, ok := doc["email"].(string); ok && v != "" {
				info["email"] = v
			} else if v, ok := doc["Email"].(string); ok && v != "" {
				info["email"] = v
			}

			if v, ok := doc["name"].(string); ok {
				info["name"] = v
			}
			if v, ok := doc["username"].(string); ok && info["name"] == "" {
				info["name"] = v
			}
			if v, ok := doc["roll"].(string); ok {
				info["roll"] = v
			}

			// Prevent overwriting a valid email with empty
			if existing, exists := userMap[uidHex]; exists {
				if existing["email"] == "" && info["email"] != "" {
					existing["email"] = info["email"]
				}
				if existing["name"] == "" && info["name"] != "" {
					existing["name"] = info["name"]
				}
				if existing["roll"] == "" && info["roll"] != "" {
					existing["roll"] = info["roll"]
				}
				userMap[uidHex] = existing
			} else {
				userMap[uidHex] = info
			}
		}
	}

	// -------------------
	// 3) Load registration responses
	// -------------------
	rows := make([]map[string]interface{}, 0)
	respCur, err := db.Registeration_Responses.Coll.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load registration responses"})
		return
	}
	defer respCur.Close(ctx)

	for respCur.Next(ctx) {
		var doc bson.M
		if err := respCur.Decode(&doc); err != nil {
			continue
		}

		// user id
		uidHex := ""
		if v, ok := doc["userID"]; ok {
			uidHex = toHexID(v)
		} else if v, ok := doc["_id"]; ok {
			uidHex = toHexID(v)
		}

		row := make(map[string]interface{})
		if info, ok := userMap[uidHex]; ok {
			row["email"] = info["email"]
			if row["email"] == "" {
				fmt.Printf("⚠️ Email empty for UID %s (AUTH entry exists: %+v)\n", uidHex, info)
			}
		} else {
			fmt.Printf("❌ No AUTH entry found for UID %s\n", uidHex)
			row["email"] = ""
		}
		row["userID"] = uidHex

		// initialize all question columns to ""
		for _, qid := range qOrder {
			colLabel := qMap[qid]
			row[colLabel] = ""
		}

		// responses field may vary
		var answersArr []interface{}
		if a, ok := doc["response_reg"].(primitive.A); ok {
			answersArr = a
		} else if obj, ok := doc["response_reg"].(bson.M); ok {
			if inner, ok := obj["answers"].(primitive.A); ok {
				answersArr = inner
			}
		} else if a, ok := doc["Responses"].(primitive.A); ok {
			answersArr = a
		} else if a, ok := doc["responses"].(primitive.A); ok {
			answersArr = a
		} else if a, ok := doc["regAnswers"].(primitive.A); ok {
			answersArr = a
		}

		// iterate answers
		for _, ai := range answersArr {
			var ansDoc bson.M
			switch v := ai.(type) {
			case bson.M:
				ansDoc = v
			case map[string]interface{}:
				ansDoc = bson.M(v)
			default:
				continue
			}

			// question ID
			var aidHex string
			if v, ok := ansDoc["questionID"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["QuestionID"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["QuestionId"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["Question"].(string); ok {
				aidHex = v
			}

			if aidHex == "" {
				continue
			}

			// answer value
			ansStr := ""
			if v, ok := ansDoc["regAnswers"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["Answer"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["answer"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["response"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["value"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["regAnswers"].(primitive.Binary); ok {
				ansStr = string(v.Data)
			}

			// map to known question label
			if qlabel, ok := qMap[aidHex]; ok {
				row[qlabel] = ansStr
			} else {
				// unknown question — add dynamically
				row[aidHex] = ansStr
				if _, present := qMap[aidHex]; !present {
					qMap[aidHex] = aidHex
					qOrder = append(qOrder, aidHex)
				}
			}
		}

		rows = append(rows, row)
	}

	// -------------------
	// 4) Build final columns list
	// -------------------
	columns := []string{"email"}
	for _, qid := range qOrder {
		columns = append(columns, qMap[qid])
	}

	c.JSON(http.StatusOK, gin.H{
		"columns": columns,
		"rows":    rows,
	})
}

func GetRegResponsesDataSecond(c *gin.Context) {
	ctx := db.Registeration_Questions.Context

	// -------------------
	// 1) Load registration questions
	// -------------------
	qMap := make(map[string]string) // qidHex -> question text
	var qOrder []string             // preserve order

	qCur, err := db.Registeration_Questions.Coll.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load registration questions"})
		return
	}
	defer qCur.Close(ctx)

	for qCur.Next(ctx) {
		var doc bson.M
		if err := qCur.Decode(&doc); err != nil {
			continue
		}

		var qidHex string
		if v, ok := doc["questionID"]; ok {
			qidHex = toHexID(v)
		} else if v, ok := doc["QuestionID"]; ok {
			qidHex = toHexID(v)
		} else if v, ok := doc["_id"]; ok {
			qidHex = toHexID(v)
		}
		if qidHex == "" {
			continue
		}

		qtext := ""
		if v, ok := doc["regQuestions"].(string); ok && v != "" {
			qtext = v
		} else if v, ok := doc["question"].(string); ok && v != "" {
			qtext = v
		} else if v, ok := doc["Question"].(string); ok && v != "" {
			qtext = v
		}
		if qtext == "" {
			qtext = qidHex
		}

		if _, exists := qMap[qidHex]; !exists {
			qMap[qidHex] = qtext
			qOrder = append(qOrder, qidHex)
		}
	}

	// -------------------
	// 2) Load auth users for name/email/roll
	// -------------------
	userMap := make(map[string]map[string]string)
	authCur, err := db.AUTH.Coll.Find(ctx, bson.M{})

	if err == nil {
		defer authCur.Close(ctx)
		for authCur.Next(ctx) {
			var doc bson.M
			if err := authCur.Decode(&doc); err != nil {
				continue
			}
			var uidHex string
			if v, ok := doc["userID"]; ok {
				uidHex = toHexID(v)
			} else if v, ok := doc["_id"]; ok {
				uidHex = toHexID(v)
			}
			if uidHex == "" {
				continue
			}

			info := make(map[string]string)

			// email fallback checks
			if v, ok := doc["userEmail"].(string); ok && v != "" {
				info["email"] = v
			} else if v, ok := doc["email"].(string); ok && v != "" {
				info["email"] = v
			} else if v, ok := doc["Email"].(string); ok && v != "" {
				info["email"] = v
			}

			if v, ok := doc["name"].(string); ok {
				info["name"] = v
			}
			if v, ok := doc["username"].(string); ok && info["name"] == "" {
				info["name"] = v
			}
			if v, ok := doc["roll"].(string); ok {
				info["roll"] = v
			}

			// Prevent overwriting a valid email with empty
			if existing, exists := userMap[uidHex]; exists {
				if existing["email"] == "" && info["email"] != "" {
					existing["email"] = info["email"]
				}
				if existing["name"] == "" && info["name"] != "" {
					existing["name"] = info["name"]
				}
				if existing["roll"] == "" && info["roll"] != "" {
					existing["roll"] = info["roll"]
				}
				userMap[uidHex] = existing
			} else {
				userMap[uidHex] = info
			}
		}
	}

	// -------------------
	// 3) Load registration responses
	// -------------------
	rows := make([]map[string]interface{}, 0)
	respCur, err := db.Registeration_Responses.Coll.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load registration responses"})
		return
	}
	defer respCur.Close(ctx)

	for respCur.Next(ctx) {
		var doc bson.M
		if err := respCur.Decode(&doc); err != nil {
			continue
		}

		// user id
		uidHex := ""
		if v, ok := doc["userID"]; ok {
			uidHex = toHexID(v)
		} else if v, ok := doc["_id"]; ok {
			uidHex = toHexID(v)
		}

		row := make(map[string]interface{})
		if info, ok := userMap[uidHex]; ok {
			row["email"] = info["email"]
			if row["email"] == "" {
				fmt.Printf("⚠️ Email empty for UID %s (AUTH entry exists: %+v)\n", uidHex, info)
			}
		} else {
			fmt.Printf("❌ No AUTH entry found for UID %s\n", uidHex)
			row["email"] = ""
		}
		row["userID"] = uidHex

		// initialize all question columns to ""
		for _, qid := range qOrder {
			colLabel := qMap[qid]
			row[colLabel] = ""
		}

		// responses field may vary
		var answersArr []interface{}
		if a, ok := doc["response_reg"].(primitive.A); ok {
			answersArr = a
		} else if obj, ok := doc["response_reg"].(bson.M); ok {
			if inner, ok := obj["answers"].(primitive.A); ok {
				answersArr = inner
			}
		} else if a, ok := doc["Responses"].(primitive.A); ok {
			answersArr = a
		} else if a, ok := doc["responses"].(primitive.A); ok {
			answersArr = a
		} else if a, ok := doc["regAnswers"].(primitive.A); ok {
			answersArr = a
		}

		// iterate answers
		for _, ai := range answersArr {
			var ansDoc bson.M
			switch v := ai.(type) {
			case bson.M:
				ansDoc = v
			case map[string]interface{}:
				ansDoc = bson.M(v)
			default:
				continue
			}

			// question ID
			var aidHex string
			if v, ok := ansDoc["questionID"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["QuestionID"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["QuestionId"]; ok {
				aidHex = toHexID(v)
			} else if v, ok := ansDoc["Question"].(string); ok {
				aidHex = v
			}

			if aidHex == "" {
				continue
			}

			// answer value
			ansStr := ""
			if v, ok := ansDoc["regAnswers"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["Answer"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["answer"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["response"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["value"].(string); ok {
				ansStr = v
			} else if v, ok := ansDoc["regAnswers"].(primitive.Binary); ok {
				ansStr = string(v.Data)
			}

			// map to known question label
			if qlabel, ok := qMap[aidHex]; ok {
				row[qlabel] = ansStr
			} else {
				// unknown question — add dynamically
				row[aidHex] = ansStr
				if _, present := qMap[aidHex]; !present {
					qMap[aidHex] = aidHex
					qOrder = append(qOrder, aidHex)
				}
			}
		}

		rows = append(rows, row)
	}

	// -------------------
	// 4) Build final columns list
	// -------------------
	columns := []string{"email"}
	for _, qid := range qOrder {
		columns = append(columns, qMap[qid])
	}

	c.JSON(http.StatusOK, gin.H{
		"columns": columns,
		"rows":    rows,
	})
}

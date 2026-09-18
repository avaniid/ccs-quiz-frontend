package admin

import (
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func shuffleQuestions(qs []models.Quiz_Questions) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })
}

func StringToUID(s string) models.UID {
	var arr [7]byte
	copy(arr[:], []byte(s))
	return &arr
}

func AssignShiftsAndQuestions(c *gin.Context) {

	var req struct {
		NumShifts       int `json:"num_shifts"`
		QuestionsPerSet int `json:"questions_per_set"`
	}

	// body, _ := io.ReadAll(c.Request.Body)
	// log.Println("Received body:", string(body))
	// c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	if err := c.ShouldBindJSON(&req); err != nil || req.NumShifts < 1 || req.QuestionsPerSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "num_shifts and questions_per_set required"})
		return
	}

	fmt.Printf("num_shifts=%d, questions_per_set=%d\n", req.NumShifts, req.QuestionsPerSet)
	// log.Println(1)

	cursor, err := db.Registeration_Responses.Coll.Find(db.Registeration_Responses.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load registrations"})
		return
	}
	defer cursor.Close(db.Registeration_Responses.Context)

	// log.Println(2)
	type RegDoc struct {
		UserID any `bson:"userID"`
	}
	var userIDs []string
	for cursor.Next(db.Registeration_Responses.Context) {
		var doc RegDoc
		if err := cursor.Decode(&doc); err == nil {
			switch v := doc.UserID.(type) {
			case []byte:
				userIDs = append(userIDs, string(v))
			case string:
				userIDs = append(userIDs, v)
			case primitive.Binary:
				userIDs = append(userIDs, string(v.Data))
			default:
				fmt.Printf("Unknown userID type: %T\n", v)
			}
		}
	}
	// log.Println(3)
	if len(userIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No registered users found"})
		return
	}
	sort.Strings(userIDs)

	var questions []models.Quiz_Questions
	qCursor, err := db.Quiz_Questions.Coll.Find(db.Quiz_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load quiz questions"})
		return
	}
	defer qCursor.Close(db.Quiz_Questions.Context)
	for qCursor.Next(db.Quiz_Questions.Context) {
		var q models.Quiz_Questions
		if err := qCursor.Decode(&q); err == nil {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No quiz questions found"})
		return
	}

	existingCursor, err := db.User_Questions.Coll.Find(db.User_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load existing user questions"})
		return
	}
	defer existingCursor.Close(db.User_Questions.Context)

	existingUserIDs := make(map[string]struct{})
	for existingCursor.Next(db.User_Questions.Context) {
		var doc struct {
			UserID any `bson:"userID"`
		}
		if err := existingCursor.Decode(&doc); err == nil {
			switch v := doc.UserID.(type) {
			case []byte:
				existingUserIDs[string(v)] = struct{}{}
			case string:
				existingUserIDs[v] = struct{}{}
			case primitive.Binary:
				existingUserIDs[string(v.Data)] = struct{}{}
			}
		}
	}
	// log.Printf("Loaded %d existing user entries\n", len(existingUserIDs)) // BHAK SAALA GPT...Debugging sikhara hai.

	if req.QuestionsPerSet > len(questions) {
		req.QuestionsPerSet = len(questions)
	}
	preShuffledSets := make([][]models.Quiz_Questions, req.NumShifts)
	for i := 0; i < req.NumShifts; i++ {
		qcopy := make([]models.Quiz_Questions, len(questions))
		copy(qcopy, questions)
		shuffleQuestions(qcopy)
		preShuffledSets[i] = qcopy[:req.QuestionsPerSet]
	}

	var newEntries []interface{}
	created, skipped := 0, 0

	for idx, uid := range userIDs {
		if _, exists := existingUserIDs[uid]; exists {
			skipped++
			continue
		}

		shift := (idx % req.NumShifts) + 1
		entry := models.UserQuestions{
			UserID:    StringToUID(uid),
			Shift:     shift,
			Questions: preShuffledSets[shift-1],
		}
		newEntries = append(newEntries, entry)
		created++
	}

	if len(newEntries) > 0 {
		_, err := db.User_Questions.Coll.InsertMany(db.User_Questions.Context, newEntries)
		if err != nil {
			fmt.Println("InsertMany error:", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Shifts and questions assigned",
		"users_processed":  len(userIDs),
		"created":          created,
		"skipped_existing": skipped,
		"num_shifts":       req.NumShifts,
	})
}

func ReassignShiftAndQuestions(c *gin.Context) {
	var req struct {
		Shift           int `json:"shift"`
		QuestionsPerSet int `json:"questions_per_set"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Shift < 1 || req.QuestionsPerSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shift and questions_per_set required"})
		return
	}

	cursor, err := db.Updates.Coll.Find(db.Updates.Context, bson.M{
		"$or": []bson.M{
			{"quiz_started": false},
			{"quiz_submitted": false},
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updates"})
		return
	}
	defer cursor.Close(db.Updates.Context)

	type UpdateDoc struct {
		UserID any `bson:"userID"`
	}
	var userIDs []string
	for cursor.Next(db.Updates.Context) {
		var doc UpdateDoc
		if err := cursor.Decode(&doc); err == nil {
			switch v := doc.UserID.(type) {
			case []byte:
				userIDs = append(userIDs, string(v))
			case string:
				userIDs = append(userIDs, v)
			case primitive.Binary:
				userIDs = append(userIDs, string(v.Data))
			}
		}
	}
	if len(userIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No users found for reassignment"})
		return
	}

	var questions []models.Quiz_Questions
	qCursor, err := db.Quiz_Questions.Coll.Find(db.Quiz_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load quiz questions"})
		return
	}
	defer qCursor.Close(db.Quiz_Questions.Context)
	for qCursor.Next(db.Quiz_Questions.Context) {
		var q models.Quiz_Questions
		if err := qCursor.Decode(&q); err == nil {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No quiz questions found"})
		return
	}
	if req.QuestionsPerSet > len(questions) {
		req.QuestionsPerSet = len(questions)
	}
	created := 0
	for _, uid := range userIDs {

		_, _ = db.User_Questions.Coll.DeleteMany(db.User_Questions.Context, bson.M{"userID": StringToUID(uid)})

		qcopy := make([]models.Quiz_Questions, len(questions))
		copy(qcopy, questions)
		shuffleQuestions(qcopy)
		userQs := qcopy[:req.QuestionsPerSet]

		entry := models.UserQuestions{
			UserID:    StringToUID(uid),
			Shift:     req.Shift,
			Questions: userQs,
		}
		_, err := db.User_Questions.Coll.InsertOne(db.User_Questions.Context, entry)
		if err == nil {
			created++
		}

		_, _ = db.Updates.Coll.DeleteMany(db.Updates.Context, bson.M{"userID": StringToUID(uid)})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Reassigned shifts and questions",
		"users_processed": len(userIDs),
		"created":         created,
		"shift":           req.Shift,
	})
}

func SetShiftTiming(c *gin.Context) {
	var req struct {
		Shift     int    `json:"shift"`
		Date      string `json:"date"`
		StartTime string `json:"start_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Shift < 1 || req.Date == "" || req.StartTime == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shift, date, start_time required"})
		return
	}

	dateParsed, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
		return
	}
	startParsed, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}
	ist := time.FixedZone("IST", 5*3600+1800)
	startDateTime := time.Date(
		dateParsed.Year(), dateParsed.Month(), dateParsed.Day(),
		startParsed.Hour(), startParsed.Minute(), 0, 0, ist,
	)

	shiftEntry := models.Shift{
		Shift: req.Shift,
		Date:  dateParsed,
		Start: startDateTime,
	}

	_, err = db.Shifts.Coll.InsertOne(db.Shifts.Context, shiftEntry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert shift"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Shift timing set", "shift": req.Shift})
}

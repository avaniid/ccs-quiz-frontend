package admin

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"time"

	models "ccs.quizportal/Models"
	quiz "ccs.quizportal/Quiz"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func shuffleQuestions(qs []models.Quiz_Questions) {
	rand.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })
}

func StringToUID(s string) models.UID {
	var arr [7]byte
	copy(arr[:], []byte(s))
	return &arr
}

func uidString(v any) (string, bool) {
	switch t := v.(type) {
	case []byte:
		return string(t), true
	case string:
		return t, true
	case primitive.Binary:
		return string(t.Data), true
	}
	return "", false
}

func loadQuestionBank() ([]models.Quiz_Questions, error) {
	var questions []models.Quiz_Questions
	cur, err := db.Quiz_Questions.Coll.Find(db.Quiz_Questions.Context, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(db.Quiz_Questions.Context)
	for cur.Next(db.Quiz_Questions.Context) {
		var q models.Quiz_Questions
		if err := cur.Decode(&q); err == nil {
			questions = append(questions, q)
		}
	}
	return questions, cur.Err()
}

// poolForShift returns the questions tagged for a shift. Questions are tagged
// with a shift when added, but assignment used to ignore that and draw every
// shift from the whole bank, so one shift could pass questions to the next.
// When nothing is tagged for the shift it falls back to the whole bank and
// says so, rather than handing out empty papers.
func poolForShift(bank []models.Quiz_Questions, shift int) ([]models.Quiz_Questions, bool) {
	var pool []models.Quiz_Questions
	for _, q := range bank {
		if q.Shift == shift {
			pool = append(pool, q)
		}
	}
	if len(pool) == 0 {
		return append([]models.Quiz_Questions(nil), bank...), true
	}
	return pool, false
}

// personalize gives one candidate their own order of questions and options
// over a shared set, so neighbours in the same shift do not see the same
// sequence. The option slices are copied: shuffling them in place would
// reorder every other paper that shares the backing array.
func personalize(set []models.Quiz_Questions) []models.Quiz_Questions {
	out := make([]models.Quiz_Questions, len(set))
	for i, q := range set {
		q.Options = append([]models.Option(nil), q.Options...)
		rand.Shuffle(len(q.Options), func(a, b int) { q.Options[a], q.Options[b] = q.Options[b], q.Options[a] })
		out[i] = q
	}
	shuffleQuestions(out)
	return out
}

// pickSet draws k questions for a shift; every candidate in the shift gets the
// same set (in their own order), so the paper is equally hard for all of them.
func pickSet(bank []models.Quiz_Questions, shift, k int) ([]models.Quiz_Questions, bool) {
	pool, fallback := poolForShift(bank, shift)
	shuffleQuestions(pool)
	if k > len(pool) {
		k = len(pool)
	}
	return pool[:k], fallback
}

func AssignShiftsAndQuestions(c *gin.Context) {
	var req struct {
		NumShifts       int `json:"num_shifts"`
		QuestionsPerSet int `json:"questions_per_set"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NumShifts < 1 || req.QuestionsPerSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "num_shifts and questions_per_set required"})
		return
	}

	cursor, err := db.Registeration_Responses.Coll.Find(db.Registeration_Responses.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load registrations"})
		return
	}
	defer cursor.Close(db.Registeration_Responses.Context)

	// A set, because older builds stored one registration row per submit and
	// the same candidate must not be given several papers.
	uniq := make(map[string]struct{})
	for cursor.Next(db.Registeration_Responses.Context) {
		var doc struct {
			UserID any `bson:"userID"`
		}
		if err := cursor.Decode(&doc); err == nil {
			if id, ok := uidString(doc.UserID); ok {
				uniq[id] = struct{}{}
			} else {
				log.Printf("assign-shifts: unknown userID type %T", doc.UserID)
			}
		}
	}
	if len(uniq) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No registered users found"})
		return
	}
	userIDs := make([]string, 0, len(uniq))
	for id := range uniq {
		userIDs = append(userIDs, id)
	}
	sort.Strings(userIDs)

	bank, err := loadQuestionBank()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load quiz questions"})
		return
	}
	if len(bank) == 0 {
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
			if id, ok := uidString(doc.UserID); ok {
				existingUserIDs[id] = struct{}{}
			}
		}
	}

	sets := make([][]models.Quiz_Questions, req.NumShifts)
	var fallbackShifts []int
	for i := range sets {
		set, fallback := pickSet(bank, i+1, req.QuestionsPerSet)
		sets[i] = set
		if fallback {
			fallbackShifts = append(fallbackShifts, i+1)
		}
	}

	var newEntries []interface{}
	skipped := 0
	for idx, uid := range userIDs {
		if _, exists := existingUserIDs[uid]; exists {
			skipped++
			continue
		}
		shift := (idx % req.NumShifts) + 1
		newEntries = append(newEntries, models.UserQuestions{
			UserID:    StringToUID(uid),
			Shift:     shift,
			Questions: personalize(sets[shift-1]),
		})
	}

	created := 0
	if len(newEntries) > 0 {
		res, err := db.User_Questions.Coll.InsertMany(db.User_Questions.Context, newEntries,
			options.InsertMany().SetOrdered(false))
		if res != nil {
			created = len(res.InsertedIDs)
		}
		if err != nil {
			// This used to be printed and then reported as success, so an
			// admin only found out when candidates saw "no quiz assigned".
			log.Printf("assign-shifts: InsertMany error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":            "Some papers could not be saved: " + err.Error(),
				"created":          created,
				"failed":           len(newEntries) - created,
				"skipped_existing": skipped,
			})
			return
		}
	}

	resp := gin.H{
		"message":          "Shifts and questions assigned",
		"users_processed":  len(userIDs),
		"created":          created,
		"skipped_existing": skipped,
		"num_shifts":       req.NumShifts,
	}
	if len(fallbackShifts) > 0 {
		resp["warning"] = fmt.Sprintf("no questions are tagged for shift(s) %v, so they drew from the whole bank", fallbackShifts)
	}
	c.JSON(http.StatusOK, resp)
}

// ReassignShiftAndQuestions moves absentees into a new shift: candidates who
// hold a paper, whose own shift is already over, and who never opened it.
//
// It used to select rows in `updates` with started=false or submitted=false.
// Those rows only exist once someone has opened the paper, so that caught the
// candidates who were in the middle of the exam (swapping their paper and
// making their submit fail) and never the actual absentees.
func ReassignShiftAndQuestions(c *gin.Context) {
	var req struct {
		Shift           int `json:"shift"`
		QuestionsPerSet int `json:"questions_per_set"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Shift < 1 || req.QuestionsPerSet < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shift and questions_per_set required"})
		return
	}

	shifts, err := quiz.LoadAllShifts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load shift timings"})
		return
	}

	startedCur, err := db.Updates.Coll.Find(db.Updates.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updates"})
		return
	}
	defer startedCur.Close(db.Updates.Context)
	opened := make(map[string]bool)
	for startedCur.Next(db.Updates.Context) {
		var doc struct {
			UserID any `bson:"userID"`
		}
		if startedCur.Decode(&doc) == nil {
			if id, ok := uidString(doc.UserID); ok {
				opened[id] = true
			}
		}
	}

	paperCur, err := db.User_Questions.Coll.Find(db.User_Questions.Context, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load papers"})
		return
	}
	defer paperCur.Close(db.User_Questions.Context)

	now := time.Now()
	duration := quiz.TestDuration()
	var absentees []string
	for paperCur.Next(db.User_Questions.Context) {
		var doc struct {
			UserID any `bson:"userID"`
			Shift  int `bson:"shift"`
		}
		if paperCur.Decode(&doc) != nil {
			continue
		}
		id, ok := uidString(doc.UserID)
		if !ok || opened[id] || doc.Shift == req.Shift {
			continue
		}
		s, ok := shifts[doc.Shift]
		if !ok || now.Before(s.Start.Add(duration)) {
			continue // their shift has not happened yet, or is still running
		}
		absentees = append(absentees, id)
	}
	if len(absentees) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No absentees from finished shifts to reassign", "shift": req.Shift})
		return
	}

	bank, err := loadQuestionBank()
	if err != nil || len(bank) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No quiz questions found"})
		return
	}
	set, fallback := pickSet(bank, req.Shift, req.QuestionsPerSet)

	moved := 0
	for _, uid := range absentees {
		_, err := db.User_Questions.Coll.UpdateOne(
			db.User_Questions.Context,
			bson.M{"userID": StringToUID(uid)},
			bson.M{"$set": bson.M{"shift": req.Shift, "questions": personalize(set)}},
		)
		if err == nil {
			moved++
		} else {
			log.Printf("reassign: failed for a user: %v", err)
		}
	}

	resp := gin.H{
		"message":         "Reassigned absentees",
		"users_processed": len(absentees),
		"created":         moved,
		"shift":           req.Shift,
	}
	if fallback {
		resp["warning"] = fmt.Sprintf("no questions are tagged for shift %d, so it drew from the whole bank", req.Shift)
	}
	if _, ok := shifts[req.Shift]; !ok {
		resp["warning_timing"] = fmt.Sprintf("shift %d has no timing yet - set it or these candidates cannot start", req.Shift)
	}
	c.JSON(http.StatusOK, resp)
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

	// Replace, not add: every edit used to insert another row for the shift,
	// and which one the quiz then used depended on Mongo's return order.
	if _, err := db.Shifts.Coll.DeleteMany(db.Shifts.Context, bson.M{"shift": req.Shift}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update shift"})
		return
	}
	if _, err := db.Shifts.Coll.InsertOne(db.Shifts.Context, shiftEntry); err != nil {
		if we, ok := err.(mongo.WriteException); ok && we.HasErrorCode(11000) {
			c.JSON(http.StatusConflict, gin.H{"error": "Shift was being edited at the same time, try again"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert shift"})
		return
	}
	// Takes effect immediately instead of after the next server restart.
	quiz.InvalidateShiftCache()
	c.JSON(http.StatusOK, gin.H{"message": "Shift timing set", "shift": req.Shift, "start": startDateTime})
}

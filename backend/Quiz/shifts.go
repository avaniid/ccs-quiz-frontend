package quiz

import (
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func LoadAllShifts() (map[int]models.Shift, error) {
	shiftsMap := make(map[int]models.Shift)
	// Oldest first, so if duplicate rows for a shift exist (older builds
	// inserted a new row on every edit) the most recent one wins.
	cursor, err := db.Shifts.Coll.Find(db.Shifts.Context, bson.M{},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(db.Shifts.Context)

	for cursor.Next(db.Shifts.Context) {
		var shift models.Shift
		if err := cursor.Decode(&shift); err == nil {
			shiftsMap[shift.Shift] = shift
		}
	}
	return shiftsMap, nil
}

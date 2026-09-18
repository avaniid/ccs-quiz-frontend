package quiz

import (
	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"go.mongodb.org/mongo-driver/bson"
)

func LoadAllShifts() (map[int]models.Shift, error) {
	shiftsMap := make(map[int]models.Shift)
	cursor, err := db.Shifts.Coll.Find(db.Shifts.Context, bson.M{})
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

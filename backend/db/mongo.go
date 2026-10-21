package db

import (
	"context"
	"errors"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Collection with Context for operations. One Stop Solution hai ye.
type Collection struct {
	Coll    *mongo.Collection
	Context context.Context
}

var (
	// Declaring DB
	DATABASE *mongo.Database

	//Declaring Collection to used in our portal
	AUTH                    Collection
	Registeration_Questions Collection
	Registeration_Responses Collection
	Quiz_Questions          Collection
	Quiz_Responses          Collection
	Quiz_Answers            Collection
	Quiz_Track              Collection
	User_Questions          Collection
	Updates                 Collection //and now the db architecture has been fucked. UPdates collection honi hi nhi chaiye , but I am in no mood of updating the QuizTrack and its validation all over the code.
	Shifts                  Collection
	Slot_Requests           Collection
	// One row per candidate imported from the registration CSV: their email,
	// name, roll number and shift, for the results page.
	Candidates Collection
	// A copy of every attempt an admin reset, taken before it is cleared, so a
	// re-attempt never erases the record of the first one.
	Attempt_Archive Collection
)

func Init() error {
	log.Println("MONGO INIT ON HUA")
	ctx := context.Background()
	uri := os.Getenv("MONGOURI")

	if uri == "" {
		return errors.New("URI Set Nhi Hai")
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return errors.New("DB.Init : mongo.Connect: " + err.Error())
	}

	if err = client.Ping(ctx, nil); err != nil {
		return errors.New("DB.Init: client.Ping\n" + err.Error())
	}

	log.Println("Successfully Conected to Mongo....")

	// The database name is configurable so a new recruitment cycle can start on
	// a clean namespace without touching code (and without inheriting an older
	// cycle's registrations, papers and responses from the same cluster).
	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "Recruitments_2025"
	}
	log.Printf("Using database: %s", dbName)
	DATABASE = client.Database(dbName)

	Registeration_Questions = Collection{DATABASE.Collection("reg_ques"), ctx}
	AUTH = Collection{DATABASE.Collection("auth"), ctx}

	Registeration_Responses = Collection{DATABASE.Collection("reg_responses"), ctx}
	Quiz_Questions = Collection{DATABASE.Collection("quiz_ques"), ctx}
	Quiz_Responses = Collection{DATABASE.Collection("quiz_responses"), ctx}
	Quiz_Answers = Collection{DATABASE.Collection("quiz_answers"), ctx}
	Quiz_Track = Collection{DATABASE.Collection("quiz_track"), ctx}
	User_Questions = Collection{DATABASE.Collection("user_questions"), ctx}
	Updates = Collection{DATABASE.Collection("updates"), ctx}
	Shifts = Collection{DATABASE.Collection("shift_data"), ctx}
	Slot_Requests = Collection{DATABASE.Collection("slot_requests"), ctx}
	Candidates = Collection{DATABASE.Collection("candidates"), ctx}
	Attempt_Archive = Collection{DATABASE.Collection("attempt_archive"), ctx}

	ensureIndexes(ctx)
	return nil

}

// ensureIndexes makes every per-candidate lookup an index hit (they were all
// collection scans, which hurts when a whole shift opens the paper at once)
// and lets Mongo itself refuse a second row for the same candidate, which is
// what actually closes the double-submit race that $setOnInsert alone leaves.
//
// A failure is logged, not fatal: a database that already holds duplicates
// (older builds allowed them) cannot take a unique index until they are
// cleaned up, and that must not stop the server from starting.
func ensureIndexes(ctx context.Context) {
	unique := func(field string) mongo.IndexModel {
		return mongo.IndexModel{
			Keys:    bson.D{{Key: field, Value: 1}},
			Options: options.Index().SetUnique(true),
		}
	}
	plan := []struct {
		coll  Collection
		index mongo.IndexModel
	}{
		{AUTH, unique("userEmail")},
		{AUTH, unique("userID")},
		{Registeration_Responses, unique("userID")},
		{User_Questions, unique("userID")},
		{Updates, unique("userID")},
		{Quiz_Responses, unique("userID")},
		{Quiz_Track, unique("userID")},
		{Slot_Requests, unique("userID")},
		{Candidates, unique("userID")},
		{Shifts, unique("shift")},
		{Quiz_Answers, unique("questionID")},
	}
	for _, p := range plan {
		if _, err := p.coll.Coll.Indexes().CreateOne(ctx, p.index); err != nil {
			log.Printf("WARNING: could not create unique index on %s: %v (remove duplicate rows, then restart)",
				p.coll.Coll.Name(), err)
		}
	}
}

func (db *Collection) Sync(bsonM bson.M, entry any) error {
	result, err := db.Coll.ReplaceOne(db.Context, bsonM, entry)
	if err != nil {
		return errors.New("Error: DB.syncBson error\n" + err.Error())
	}

	if result.MatchedCount == 0 {
		return errors.New("Error: DB.syncBson failed, No document synced")
	}

	return nil
}

func (db *Collection) SyncTryHard(bsonM bson.M, entry any, maxTries byte) error {
	var tries byte = 0

sync:
	if err := db.Sync(bsonM, entry); err != nil {
		if tries > maxTries {
			return errors.New("DB.syncBsonTryHard: Error in DB.syncBson, Max Tries reached\n" + err.Error())
		}
		tries += 1
		goto sync
	}

	return nil
}

func (db *Collection) GetExists(bsonM bson.M, out any) (bool, error) {
	result := db.Coll.FindOne(db.Context, bsonM)
	err := result.Err()

	if err == mongo.ErrNoDocuments {
		return false, nil
	} else if err != nil {
		return false, errors.New("getExistsBson: DB.FindOne error\n" + err.Error())
	}

	if out != nil {
		if err := result.Decode(out); err != nil {
			return false, errors.New("getExistsBson: result.Decode error\n" + err.Error())
		}
	}

	return true, nil
}

func (db *Collection) Get(bsonM bson.M, out any) error {
	exists, err := db.GetExists(bsonM, out)
	if !exists {
		return errors.New("DB.get: document does not exist")
	}
	return err
}

func (db *Collection) exists(bsonM bson.M) (bool, error) {
	return db.GetExists(bsonM, nil)
}

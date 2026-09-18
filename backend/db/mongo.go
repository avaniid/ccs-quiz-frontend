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

	DATABASE = client.Database("Recruitments_2025")

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

	return nil

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

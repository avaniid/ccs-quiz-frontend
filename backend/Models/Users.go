package models

type (
	UID *[7]byte
	QID *[7]byte
	OID *[7]byte
)

type AuthUser struct {
	UserId       UID    `bson:"userID"`
	Email        string `bson:"userEmail"`
	SessionToken string `bson:"sessionToken"`
}

type RespUser struct {
	// UserID     UID      `bson:"userID"`
	RegAnswers []Answer `bson:"answers"`
}

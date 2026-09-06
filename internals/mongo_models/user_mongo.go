package mongomodels

import "go.mongodb.org/mongo-driver/v2/bson"

type UserMongo struct {
	Id       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string        `json:"username"`
	Email    string        `json:"email"`
	Password string        `json:"password"`
}

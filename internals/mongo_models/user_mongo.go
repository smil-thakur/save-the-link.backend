package mongomodels

import "go.mongodb.org/mongo-driver/v2/bson"

type UserMongo struct {
	Id       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string        `json:"username"`
	Email    string        `json:"email"`
	Password string        `json:"password"`
	// BookmarkedPageIds are public pages owned by someone else that this user
	// has bookmarked for quick access from the sidebar. Bookmarking your own
	// page is rejected at the service layer, so this never contains a page
	// this user owns.
	BookmarkedPageIds []bson.ObjectID `bson:"bookmarkedPageIds,omitempty" json:"-"`
}

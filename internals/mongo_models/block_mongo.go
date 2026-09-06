package mongomodels

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BlockMongo struct {
	Id          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	PageId      bson.ObjectID `bson:"pageId" json:"pageId"`
	URL         string        `bson:"url" json:"url"`
	Title       string        `bson:"title" json:"title"`
	Description string        `bson:"description" json:"description"`
	CoverImage  string        `bson:"coverImage,omitempty" json:"coverImage,omitempty"`
	Favicon     string        `bson:"favicon,omitempty" json:"favicon,omitempty"`
	SiteName    string        `bson:"siteName,omitempty" json:"siteName,omitempty"`
	FetchStatus string        `bson:"fetchStatus" json:"fetchStatus"`
	Tags        []string      `bson:"tags" json:"tags"`
	Order       float64       `bson:"order" json:"order"`
	CreatedAt   time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt" json:"updatedAt"`
}

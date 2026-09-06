package mongomodels

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type PageMongo struct {
	Id            bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	OwnerId       bson.ObjectID  `bson:"ownerId" json:"ownerId"`
	ParentPageId  *bson.ObjectID `bson:"parentPageId,omitempty" json:"parentPageId,omitempty"`
	Title         string         `bson:"title" json:"title"`
	Icon          *string        `bson:"icon,omitempty" json:"icon,omitempty"`
	Layout        string         `bson:"layout" json:"layout"`
	Visibility    string         `bson:"visibility" json:"visibility"`
	Collaboration string         `bson:"collaboration" json:"collaboration"`
	Slug          *string        `bson:"slug,omitempty" json:"slug,omitempty"`
	// CollaboratorIds/CollaboratorEmails are only consulted when Collaboration
	// is "invite" — edit access restricted to these specific registered users,
	// as opposed to "edit" (anyone with the link). Emails are a denormalized
	// copy of the resolved users' emails, kept in sync by SetCollaborators, so
	// the owner can see who's invited without an extra join on every read.
	CollaboratorIds    []bson.ObjectID `bson:"collaboratorIds,omitempty" json:"collaboratorIds,omitempty"`
	CollaboratorEmails []string        `bson:"collaboratorEmails,omitempty" json:"collaboratorEmails,omitempty"`
	Order              float64         `bson:"order" json:"order"`
	DeletedAt          *time.Time      `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	CreatedAt          time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time       `bson:"updatedAt" json:"updatedAt"`
}

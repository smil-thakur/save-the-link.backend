package constants

type token_type string

const (
	AccessToken  token_type = "access_token"
	RefreshToken token_type = "refresh_token"
)

const (
	GridLayout     string = "grid"
	FreeformLayout string = "freeform"
)

const (
	PrivatePage string = "private"
	PublicPage  string = "public"
)

const (
	NoCollaboration   string = "none"
	ViewCollaboration string = "view"
	EditCollaboration string = "edit"
)

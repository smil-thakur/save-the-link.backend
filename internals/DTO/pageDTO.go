package dto

type CreatePageDTO struct {
	Title        string  `json:"title" binding:"required"`
	ParentPageId *string `json:"parentPageId"`
}

type UpdatePageDTO struct {
	Title        *string  `json:"title"`
	Icon         *string  `json:"icon"`
	ParentPageId *string  `json:"parentPageId"`
	Order        *float64 `json:"order"`
}

type PublishPageDTO struct {
	Collaboration string `json:"collaboration" binding:"required"`
}

type SetCollaboratorsDTO struct {
	Emails []string `json:"emails"`
}

type PageResponseDTO struct {
	Id                 string   `json:"id"`
	ParentPageId       *string  `json:"parentPageId,omitempty"`
	Title              string   `json:"title"`
	Icon               *string  `json:"icon,omitempty"`
	Layout             string   `json:"layout"`
	Visibility         string   `json:"visibility"`
	Collaboration      string   `json:"collaboration"`
	Slug               *string  `json:"slug,omitempty"`
	CollaboratorEmails []string `json:"collaboratorEmails,omitempty"`
	Order              float64  `json:"order"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
	DeletedAt          *string  `json:"deletedAt,omitempty"`
}

type SetCollaboratorsResponseDTO struct {
	Page           PageResponseDTO `json:"page"`
	NotFoundEmails []string        `json:"notFoundEmails,omitempty"`
}

type PublicPageSummaryDTO struct {
	Id        string  `json:"id"`
	Title     string  `json:"title"`
	Icon      *string `json:"icon,omitempty"`
	Slug      string  `json:"slug"`
	LinkCount int     `json:"linkCount"`
}

type UserSummaryDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

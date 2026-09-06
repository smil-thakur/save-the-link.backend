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

type PageResponseDTO struct {
	Id            string  `json:"id"`
	ParentPageId  *string `json:"parentPageId,omitempty"`
	Title         string  `json:"title"`
	Icon          *string `json:"icon,omitempty"`
	Layout        string  `json:"layout"`
	Visibility    string  `json:"visibility"`
	Collaboration string  `json:"collaboration"`
	Slug          *string `json:"slug,omitempty"`
	Order         float64 `json:"order"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	DeletedAt     *string `json:"deletedAt,omitempty"`
}

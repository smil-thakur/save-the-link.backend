package dto

type SearchResultsDTO struct {
	Pages  []PageResponseDTO  `json:"pages"`
	Blocks []BlockResponseDTO `json:"blocks"`
}

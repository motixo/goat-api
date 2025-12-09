package helper

type PaginationInput struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

func (p *PaginationInput) Validate() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 10
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
}

func (p *PaginationInput) Offset() int {
	return (p.Page - 1) * p.Limit
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewPaginationMeta(total int64, input PaginationInput) PaginationMeta {
	totalPages := int((total + int64(input.Limit) - 1) / int64(input.Limit))
	return PaginationMeta{
		Page:       input.Page,
		Limit:      input.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

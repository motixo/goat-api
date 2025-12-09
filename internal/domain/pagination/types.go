package pagination

type Input struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func (p *Input) Validate() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 10
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewMeta(total int64, input Input) Meta {
	totalPages := int((total + int64(input.PageSize) - 1) / int64(input.PageSize))
	return Meta{
		Page:       input.Page,
		PageSize:   input.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

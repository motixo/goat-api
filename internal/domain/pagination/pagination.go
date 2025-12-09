package pagination

func CalculateOffset(page, pageSize int) int {
	return (page - 1) * pageSize
}

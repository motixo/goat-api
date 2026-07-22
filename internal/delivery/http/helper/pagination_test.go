package helper

import "testing"

func TestPaginationInputValidateCapsPageSize(t *testing.T) {
	input := PaginationInput{Page: 2, Limit: 101}

	input.Validate()

	if input.Page != 2 || input.Limit != 100 {
		t.Fatalf("pagination = page %d, limit %d; want page 2, limit 100", input.Page, input.Limit)
	}
	if offset := input.Offset(); offset != 100 {
		t.Fatalf("offset = %d, want 100", offset)
	}
}

func TestPaginationInputOffsetDoesNotOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	input := PaginationInput{Page: maxInt, Limit: 100}

	if offset := input.Offset(); offset != maxInt {
		t.Fatalf("offset = %d, want clamped max int %d", offset, maxInt)
	}
}

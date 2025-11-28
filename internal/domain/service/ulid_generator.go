package service

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

type ULIDGenerator struct {
}

func NewULIDGenerator() *ULIDGenerator {
	return &ULIDGenerator{}
}

func (u *ULIDGenerator) New() string {
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader).String()
}

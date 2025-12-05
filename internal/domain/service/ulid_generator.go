package service

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

type IDGenerator interface {
	Generate() string
}

type ULIDGenerator struct {
}

func NewULIDGenerator() IDGenerator {
	return &ULIDGenerator{}
}

func (u *ULIDGenerator) Generate() string {
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader).String()
}

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPasswordServiceRejectsCanceledCallerBeforeAdmission(t *testing.T) {
	cancellationCause := errors.New("request canceled before password work")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)
	var deriveCalls atomic.Int32
	passwordService := mustNewControlledPasswordService(
		t,
		2,
		nil,
		func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
			deriveCalls.Add(1)
			return fixedDerivedKey(argonKeyLen)
		},
	)

	if _, err := passwordService.Hash(ctx, testPlainPassword("Password1!")); !errors.Is(err, context.Canceled) || !errors.Is(err, cancellationCause) {
		t.Fatalf("Hash() error = %v, want caller cancellation and cause", err)
	}
	if _, err := passwordService.Verify(ctx, testPlainPassword("Password1!"), testPasswordDigest("invalid")); !errors.Is(err, context.Canceled) || !errors.Is(err, cancellationCause) {
		t.Fatalf("Verify() error = %v, want caller cancellation and cause", err)
	}
	if got := deriveCalls.Load(); got != 0 {
		t.Fatalf("Argon2 derivations = %d, want 0", got)
	}
}

func TestPasswordServiceCancellationWhileWaitingForCapacity(t *testing.T) {
	var deriveCalls atomic.Int32
	passwordService := mustNewControlledPasswordService(
		t,
		1,
		nil,
		func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
			deriveCalls.Add(1)
			return fixedDerivedKey(argonKeyLen)
		},
	)
	passwordService.capacity <- struct{}{}
	released := false
	t.Cleanup(func() {
		if !released {
			passwordService.release()
		}
	})

	cancellationCause := errors.New("request canceled while awaiting password capacity")
	parent, cancel := context.WithCancelCause(context.Background())
	ctx := newAdmissionObservedContext(parent)
	result := make(chan error, 1)
	go func() {
		_, err := passwordService.Hash(ctx, testPlainPassword("Password1!"))
		result <- err
	}()

	waitForSignal(t, ctx.beforeAdmissionWait, "Hash() to begin waiting for capacity")
	cancel(cancellationCause)
	err := waitForResult(t, result, "canceled Hash()")
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cancellationCause) {
		t.Fatalf("Hash() error = %v, want caller cancellation and cause", err)
	}
	if got := deriveCalls.Load(); got != 0 {
		t.Fatalf("Argon2 derivations = %d, want 0", got)
	}
	passwordService.release()
	released = true
}

func TestPasswordServiceExecutesAfterCapacityBecomesAvailable(t *testing.T) {
	passwordService := mustNewControlledPasswordService(t, 1, nil, nil)
	passwordService.capacity <- struct{}{}
	released := false
	t.Cleanup(func() {
		if !released {
			passwordService.release()
		}
	})

	ctx := newAdmissionObservedContext(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := passwordService.Hash(ctx, testPlainPassword("Password1!"))
		result <- err
	}()

	waitForSignal(t, ctx.beforeAdmissionWait, "Hash() to begin waiting for capacity")
	passwordService.release()
	released = true
	if err := waitForResult(t, result, "admitted Hash()"); err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
}

func TestPasswordServiceBoundsConcurrentArgonOperations(t *testing.T) {
	const (
		maxConcurrency = 2
		operations     = 8
	)
	var inFlight atomic.Int32
	var maximum atomic.Int32
	entered := make(chan struct{}, operations)
	release := make(chan struct{})
	passwordService := mustNewControlledPasswordService(
		t,
		maxConcurrency,
		nil,
		func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
			current := inFlight.Add(1)
			for observed := maximum.Load(); current > observed; observed = maximum.Load() {
				if maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			entered <- struct{}{}
			<-release
			inFlight.Add(-1)
			return fixedDerivedKey(argonKeyLen)
		},
	)

	results := make(chan error, operations)
	for range operations {
		go func() {
			_, err := passwordService.Hash(context.Background(), testPlainPassword("Password1!"))
			results <- err
		}()
	}
	for range maxConcurrency {
		waitForSignal(t, entered, "bounded Argon2 operation to start")
	}
	if got := inFlight.Load(); got != maxConcurrency {
		t.Fatalf("Argon2 operations in flight = %d, want %d", got, maxConcurrency)
	}
	if got := len(passwordService.capacity); got != maxConcurrency {
		t.Fatalf("occupied password slots = %d, want %d", got, maxConcurrency)
	}
	close(release)
	for range operations {
		if err := waitForResult(t, results, "bounded Hash()"); err != nil {
			t.Fatalf("Hash() error = %v", err)
		}
	}
	if got := maximum.Load(); got != maxConcurrency {
		t.Fatalf("maximum concurrent Argon2 operations = %d, want %d", got, maxConcurrency)
	}
	if got := len(passwordService.capacity); got != 0 {
		t.Fatalf("occupied password slots after completion = %d, want 0", got)
	}
}

func TestPasswordServiceHashAndVerifyShareCapacity(t *testing.T) {
	var deriveCalls atomic.Int32
	entered := make(chan struct{}, 1)
	releaseHash := make(chan struct{})
	derived := fixedDerivedKey(argonKeyLen)
	passwordService := mustNewControlledPasswordService(
		t,
		1,
		nil,
		func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
			call := deriveCalls.Add(1)
			if call == 1 {
				entered <- struct{}{}
				<-releaseHash
			}
			return derived
		},
	)
	hashResult := make(chan error, 1)
	go func() {
		_, err := passwordService.Hash(context.Background(), testPlainPassword("Password1!"))
		hashResult <- err
	}()
	waitForSignal(t, entered, "Hash() derivation to occupy capacity")

	parent, cancel := context.WithCancel(context.Background())
	verifyCtx := newAdmissionObservedContext(parent)
	verifyResult := make(chan error, 1)
	go func() {
		_, err := passwordService.Verify(
			verifyCtx,
			testPlainPassword("Password1!"),
			encodedPassword(derived),
		)
		verifyResult <- err
	}()
	waitForSignal(t, verifyCtx.beforeAdmissionWait, "Verify() to wait behind Hash()")
	cancel()
	if err := waitForResult(t, verifyResult, "canceled Verify()"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Verify() error = %v, want context.Canceled", err)
	}
	if got := deriveCalls.Load(); got != 1 {
		t.Fatalf("Argon2 derivations before releasing Hash() = %d, want 1", got)
	}
	close(releaseHash)
	if err := waitForResult(t, hashResult, "Hash() occupying shared capacity"); err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
}

func TestPasswordServiceReleasesCapacityAfterFailureAndPanic(t *testing.T) {
	saltErr := errors.New("entropy unavailable")
	passwordService := mustNewControlledPasswordService(
		t,
		1,
		func([]byte) (int, error) { return 0, saltErr },
		nil,
	)
	if _, err := passwordService.Hash(context.Background(), testPlainPassword("Password1!")); !errors.Is(err, domainErrors.ErrPasswordHashingFailed) {
		t.Fatalf("Hash() salt error = %v, want ErrPasswordHashingFailed", err)
	}
	if got := len(passwordService.capacity); got != 0 {
		t.Fatalf("occupied password slots after salt failure = %d, want 0", got)
	}

	passwordService.readSalt = deterministicSalt
	passwordService.derive = func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
		panic("controlled Argon2 panic")
	}
	panicked := false
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		_, _ = passwordService.Hash(context.Background(), testPlainPassword("Password1!"))
	}()
	if !panicked {
		t.Fatal("Hash() did not propagate controlled derivation panic")
	}
	if got := len(passwordService.capacity); got != 0 {
		t.Fatalf("occupied password slots after panic = %d, want 0", got)
	}

	passwordService.derive = deterministicDerive
	if _, err := passwordService.Hash(context.Background(), testPlainPassword("Password1!")); err != nil {
		t.Fatalf("Hash() after released failure capacity error = %v", err)
	}
}

func TestPasswordServiceDoesNotReportCancellationAfterDerivationStarts(t *testing.T) {
	hashCtx, cancelHash := context.WithCancel(context.Background())
	passwordService := mustNewControlledPasswordService(
		t,
		1,
		nil,
		func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
			cancelHash()
			return fixedDerivedKey(argonKeyLen)
		},
	)
	if _, err := passwordService.Hash(hashCtx, testPlainPassword("Password1!")); err != nil {
		t.Fatalf("Hash() error after derivation began = %v, want completed result", err)
	}
	if !errors.Is(hashCtx.Err(), context.Canceled) {
		t.Fatal("controlled Hash() context was not canceled during derivation")
	}

	verifyCtx, cancelVerify := context.WithCancel(context.Background())
	passwordService.derive = func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
		cancelVerify()
		return fixedDerivedKey(argonKeyLen)
	}
	verified, err := passwordService.Verify(
		verifyCtx,
		testPlainPassword("Password1!"),
		encodedPassword(fixedDerivedKey(argonKeyLen)),
	)
	if err != nil {
		t.Fatalf("Verify() error after derivation began = %v, want completed result", err)
	}
	if !verified {
		t.Fatal("Verify() discarded the completed derivation result after cancellation")
	}
	if !errors.Is(verifyCtx.Err(), context.Canceled) {
		t.Fatal("controlled Verify() context was not canceled during derivation")
	}
}

func TestPasswordServiceProductionAdapterStartsNoGoroutines(t *testing.T) {
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"password_service_impl.go",
		nil,
		0,
	)
	if err != nil {
		t.Fatalf("parse password_service_impl.go: %v", err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.GoStmt); ok {
			t.Error("password adapter launches a goroutine; Argon2id must execute synchronously")
		}
		return true
	})
}

type admissionObservedContext struct {
	context.Context
	checks              atomic.Int32
	beforeAdmissionWait chan struct{}
	once                sync.Once
}

func newAdmissionObservedContext(parent context.Context) *admissionObservedContext {
	return &admissionObservedContext{
		Context:             parent,
		beforeAdmissionWait: make(chan struct{}),
	}
}

func (c *admissionObservedContext) Err() error {
	if c.checks.Add(1) == 2 {
		c.once.Do(func() { close(c.beforeAdmissionWait) })
	}
	return c.Context.Err()
}

func mustNewControlledPasswordService(
	t testing.TB,
	maxConcurrency int,
	readSalt saltReaderFunc,
	derive deriveKeyFunc,
) *PasswordService {
	t.Helper()
	if readSalt == nil {
		readSalt = deterministicSalt
	}
	if derive == nil {
		derive = deterministicDerive
	}
	passwordService, err := newPasswordService(
		PasswordHasherConfig{
			Pepper:         "controlled-test-pepper",
			MaxConcurrency: maxConcurrency,
		},
		readSalt,
		derive,
	)
	if err != nil {
		t.Fatalf("newPasswordService() error = %v", err)
	}
	return passwordService
}

func deterministicSalt(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = byte(index + 1)
	}
	return len(destination), nil
}

func deterministicDerive(
	_ []byte,
	_ []byte,
	_ uint32,
	_ uint32,
	_ uint8,
	keyLen uint32,
) []byte {
	return fixedDerivedKey(keyLen)
}

func fixedDerivedKey(keyLen uint32) []byte {
	derived := make([]byte, keyLen)
	for index := range derived {
		derived[index] = 0x5a
	}
	return derived
}

func encodedPassword(derived []byte) valueobject.PasswordDigest {
	salt := make([]byte, saltLen)
	return testPasswordDigest(fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(derived),
	))
}

func waitForSignal(t testing.TB, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForResult[T any](t testing.TB, result <-chan T, description string) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		var zero T
		return zero
	}
}

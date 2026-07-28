package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRuntimeReadinessCheckerChecksEachDependencyOnce(t *testing.T) {
	t.Parallel()

	var calls []string
	checker := runtimeReadinessChecker{
		postgres: func(context.Context) error {
			calls = append(calls, "postgres")
			return nil
		},
		redis: func(context.Context) error {
			calls = append(calls, "redis")
			return nil
		},
	}

	if err := checker.CheckReadiness(context.Background()); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}
	if want := []string{"postgres", "redis"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("readiness calls = %v, want %v", calls, want)
	}
}

func TestRuntimeReadinessCheckerPreservesDependencyErrors(t *testing.T) {
	t.Parallel()

	postgresErr := errors.New("postgres unavailable")
	redisErr := errors.New("redis unavailable")
	tests := []struct {
		name              string
		postgresResult    error
		redisResult       error
		want              error
		wantPostgresCalls int
		wantRedisCalls    int
	}{
		{
			name:              "PostgreSQL failure stops the check",
			postgresResult:    postgresErr,
			want:              postgresErr,
			wantPostgresCalls: 1,
		},
		{
			name:              "Redis failure is preserved",
			redisResult:       redisErr,
			want:              redisErr,
			wantPostgresCalls: 1,
			wantRedisCalls:    1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			postgresCalls := 0
			redisCalls := 0
			checker := runtimeReadinessChecker{
				postgres: func(context.Context) error {
					postgresCalls++
					return test.postgresResult
				},
				redis: func(context.Context) error {
					redisCalls++
					return test.redisResult
				},
			}

			err := checker.CheckReadiness(context.Background())
			if !errors.Is(err, test.want) {
				t.Fatalf("CheckReadiness() error = %v, want wrapped %v", err, test.want)
			}
			if postgresCalls != test.wantPostgresCalls || redisCalls != test.wantRedisCalls {
				t.Fatalf(
					"readiness calls = (postgres %d, redis %d), want (%d, %d)",
					postgresCalls,
					redisCalls,
					test.wantPostgresCalls,
					test.wantRedisCalls,
				)
			}
		})
	}
}

func TestRuntimeReadinessCheckerRejectsCanceledContextBeforeProbing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checker := runtimeReadinessChecker{
		postgres: func(context.Context) error {
			t.Fatal("PostgreSQL probe ran after caller cancellation")
			return nil
		},
		redis: func(context.Context) error {
			t.Fatal("Redis probe ran after caller cancellation")
			return nil
		},
	}

	if err := checker.CheckReadiness(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckReadiness() error = %v, want wrapped context.Canceled", err)
	}
}

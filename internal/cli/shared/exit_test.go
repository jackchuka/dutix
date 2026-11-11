package shared

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackchuka/dutix/internal/domain"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitOK},
		{name: "other", err: errors.New("boom"), want: ExitError},
		{name: "usage", err: Usagef("bad flag"), want: ExitUsage},
		{name: "wrapped usage", err: fmt.Errorf("context: %w", Usagef("bad flag")), want: ExitUsage},
		{name: "not found", err: fmt.Errorf("%w: Foo", domain.ErrAppNotFound), want: ExitNotFound},
		{name: "ambiguous", err: fmt.Errorf("%w matching: Code", domain.ErrAmbiguousApp), want: ExitAmbiguous},
		{name: "apply failed", err: CheckResults([]domain.ApplyResult{{Item: domain.PlanItem{Status: domain.StatusFailed}}}), want: ExitApplyFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestCheckResults(t *testing.T) {
	results := []domain.ApplyResult{
		{Item: domain.PlanItem{Status: domain.StatusSuccess}},
		{Item: domain.PlanItem{Status: domain.StatusFailed}},
		{Item: domain.PlanItem{Status: domain.StatusSkipped}, Declined: true},
	}
	err := CheckResults(results)
	if !errors.Is(err, ErrApplyFailed) || err.Error() != "failed to apply 1 of 3 item(s)" {
		t.Errorf("CheckResults() = %v", err)
	}
	if err := CheckResults(results[:1]); err != nil {
		t.Errorf("CheckResults() = %v, want nil", err)
	}
	if err := CheckResults(results[2:]); err != nil {
		t.Errorf("CheckResults() for a declined item = %v, want nil", err)
	}
}

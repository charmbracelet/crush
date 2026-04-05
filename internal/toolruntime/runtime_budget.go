package toolruntime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Governance struct {
	RuntimeBudget time.Duration
	FailureBudget int
	FailureDomain string
}

type runtimeBudgetContextKey struct{}
type failureDomainContextKey struct{}

type runtimeBudgetController struct {
	cancel        context.CancelCauseFunc
	failureBudget int

	mu           sync.Mutex
	failureCount map[string]int
}

func WithGovernance(ctx context.Context, governance Governance) (context.Context, context.CancelFunc) {
	controller := controllerFromContext(ctx)
	if controller == nil && governance.FailureBudget > 0 {
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(ctx)
		controller = &runtimeBudgetController{
			cancel:        cancel,
			failureBudget: governance.FailureBudget,
			failureCount:  make(map[string]int),
		}
		ctx = context.WithValue(ctx, runtimeBudgetContextKey{}, controller)
	}

	if domain := strings.TrimSpace(governance.FailureDomain); domain != "" {
		ctx = context.WithValue(ctx, failureDomainContextKey{}, domain)
	}

	if controller != nil && controller.failureBudget > 0 && FailureDomainFromContext(ctx) == "" {
		ctx = context.WithValue(ctx, failureDomainContextKey{}, "default")
	}

	if controller != nil && controller.failureBudget > 0 {
		ctx = context.WithValue(ctx, runtimeBudgetContextKey{}, controller)
	}

	var cancel context.CancelFunc = func() {}
	if governance.RuntimeBudget > 0 {
		var budgetCtx context.Context
		budgetCtx, cancel = context.WithTimeout(ctx, governance.RuntimeBudget)
		ctx = budgetCtx
		if controller != nil {
			ctx = context.WithValue(ctx, runtimeBudgetContextKey{}, controller)
		}
	}

	return ctx, cancel
}

func FailureDomainFromContext(ctx context.Context) string {
	domain, _ := ctx.Value(failureDomainContextKey{}).(string)
	return strings.TrimSpace(domain)
}

func ReportFailure(ctx context.Context, component string, err error) bool {
	controller := controllerFromContext(ctx)
	if controller == nil || err == nil || controller.failureBudget <= 0 {
		return false
	}

	domain := FailureDomainFromContext(ctx)
	if domain == "" {
		domain = "default"
	}

	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.failureCount[domain]++
	if controller.failureCount[domain] < controller.failureBudget {
		return false
	}
	if controller.cancel != nil {
		controller.cancel(fmt.Errorf("failure budget exhausted in domain %q after %d failure(s): %s: %w", domain, controller.failureCount[domain], strings.TrimSpace(component), err))
	}
	return true
}

func controllerFromContext(ctx context.Context) *runtimeBudgetController {
	controller, _ := ctx.Value(runtimeBudgetContextKey{}).(*runtimeBudgetController)
	return controller
}

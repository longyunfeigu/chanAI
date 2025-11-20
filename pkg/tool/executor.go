package tool

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExecutorConfig controls how tools are executed.
type ExecutorConfig struct {
	MaxConcurrency int
	DefaultTimeout time.Duration
}

// Executor runs tools with concurrency limits, timeouts, and retries.
type Executor struct {
	config    ExecutorConfig
	semaphore chan struct{}
}

// NewExecutor builds an Executor with sane defaults.
func NewExecutor(cfg ExecutorConfig) *Executor {
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 5
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 60 * time.Second
	}
	return &Executor{
		config:    cfg,
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
	}
}

// ExecuteRequest describes a single tool invocation.
type ExecuteRequest struct {
	Tool    Tool
	Input   map[string]any
	Context *ToolContext
	// Overrides tool's default timeout if set > 0
	TimeoutOverride time.Duration
}

// ExecuteResult captures the output of a tool invocation.
type ExecuteResult struct {
	Success     bool
	Output      any
	Error       error
	Duration    time.Duration
	StartedAt   time.Time
	FinishedAt  time.Time
	Attempts    int
	LongRunning bool
}

// Execute runs one tool with observability, timeout, and retry logic.
func (e *Executor) Execute(ctx context.Context, req *ExecuteRequest) *ExecuteResult {
	start := time.Now()

	// 1. Acquire concurrency slot
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return &ExecuteResult{Success: false, Error: ctx.Err(), StartedAt: start, FinishedAt: time.Now()}
	}

	// 2. Input Validation
	if err := ValidateInput(req.Tool, req.Input); err != nil {
		return &ExecuteResult{Success: false, Error: err, StartedAt: start, FinishedAt: time.Now()}
	}

	// 3. Determine config (Timeout, Retry)
	var (
		timeout          = e.config.DefaultTimeout
		retryPolicy      *RetryPolicy
		longRunning      bool
		requiresApproval bool
	)

	if et, ok := req.Tool.(EnhancedTool); ok {
		if t := et.Timeout(); t > 0 {
			timeout = t
		}
		retryPolicy = et.RetryPolicy()
		longRunning = et.IsLongRunning()
		requiresApproval = et.RequiresApproval()
		// Long running tools often manage their own lifecycle; relax timeout if unset.
		if longRunning && et.Timeout() == 0 && req.TimeoutOverride == 0 {
			timeout = 0
		}
	}

	// Request override takes precedence
	if req.TimeoutOverride > 0 {
		timeout = req.TimeoutOverride
	}

	if requiresApproval && !approved(req.Context) {
		err := fmt.Errorf("tool %s requires approval before execution", req.Tool.Name())
		end := time.Now()
		return &ExecuteResult{
			Success:    false,
			Error:      err,
			StartedAt:  start,
			FinishedAt: end,
			Duration:   end.Sub(start),
			Attempts:   0,
		}
	}

	// 4. Execution Loop
	var (
		output   any
		execErr  error
		attempts int
	)

	maxAttempts := 1
	if retryPolicy != nil {
		maxAttempts += retryPolicy.MaxRetries
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		attempts = attempt + 1

		// Create attempt context
		execCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		output, execErr = req.Tool.Execute(execCtx, req.Input, req.Context)
		if cancel != nil {
			cancel()
		}

		if execErr == nil {
			break // Success
		}

		// Check if we should retry
		if attempt < maxAttempts-1 {
			if !isRetryable(execErr, retryPolicy) {
				break
			}

			// Backoff
			delay := calculateBackoff(attempt, retryPolicy)

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				execErr = ctx.Err()
				goto Finish
			}
		}
	}

Finish:
	end := time.Now()
	return &ExecuteResult{
		Success:     execErr == nil,
		Output:      output,
		Error:       execErr,
		StartedAt:   start,
		FinishedAt:  end,
		Duration:    end.Sub(start),
		Attempts:    attempts,
		LongRunning: longRunning,
	}
}

// ExecuteBatch runs a batch of requests concurrently.
func (e *Executor) ExecuteBatch(ctx context.Context, requests []*ExecuteRequest) []*ExecuteResult {
	results := make([]*ExecuteResult, len(requests))
	if len(requests) == 0 {
		return results
	}
	type prioritized struct {
		idx      int
		req      *ExecuteRequest
		priority int
	}

	items := make([]prioritized, 0, len(requests))
	for i, req := range requests {
		p := 0
		if et, ok := req.Tool.(EnhancedTool); ok {
			p = et.Priority()
		}
		items = append(items, prioritized{idx: i, req: req, priority: p})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].priority > items[j].priority
	})

	var wg sync.WaitGroup
	wg.Add(len(items))

	for _, item := range items {
		item := item
		go func() {
			defer wg.Done()
			results[item.idx] = e.Execute(ctx, item.req)
		}()
	}

	wg.Wait()
	return results
}

func isRetryable(err error, policy *RetryPolicy) bool {
	if policy == nil || len(policy.RetryableErrors) == 0 {
		return true // Default to retry all if policy exists but specifies no filters
	}
	errStr := err.Error()
	for _, pattern := range policy.RetryableErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

func calculateBackoff(attempt int, policy *RetryPolicy) time.Duration {
	if policy == nil {
		return 0
	}
	backoff := float64(policy.InitialBackoff) * math.Pow(policy.BackoffMultiplier, float64(attempt))
	if backoff > float64(policy.MaxBackoff) {
		backoff = float64(policy.MaxBackoff)
	}
	return time.Duration(backoff)
}

func approved(tc *ToolContext) bool {
	if tc == nil {
		return false
	}
	if v, ok := tc.Metadata["approved"].(bool); ok {
		return v
	}
	return false
}

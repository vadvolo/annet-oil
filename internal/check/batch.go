package check

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"annet-oil/internal/inventory"
)

// BatchReport aggregates the results of checking many devices.
type BatchReport struct {
	GeneratedAt  time.Time `json:"generated_at"`
	DurationMs   int64     `json:"duration_ms"`
	Concurrency  int       `json:"concurrency"`
	Total        int       `json:"total"`
	Reachable    int       `json:"reachable"`
	Unreachable  int       `json:"unreachable"`
	Canceled     int       `json:"canceled"`
	LoginOK      int       `json:"login_ok"`
	LoginFailed  int       `json:"login_failed"`
	LoginSkipped int       `json:"login_skipped"`
	CommandsOK      int    `json:"commands_ok"`
	CommandsFailed  int    `json:"commands_failed"`
	CommandsSkipped int    `json:"commands_skipped"`
	Results      []*Result `json:"results"`
}

// Devices checks a set of devices concurrently in batches of `concurrency`
// workers and returns an aggregated report. Results are sorted by hostname for
// stable output. A concurrency <= 0 defaults to 50.
func Devices(ctx context.Context, devices []inventory.Device, opts Options, concurrency int) *BatchReport {
	if concurrency <= 0 {
		concurrency = 50
	}
	start := time.Now()

	report := &BatchReport{
		GeneratedAt: start,
		Concurrency: concurrency,
		Total:       len(devices),
		Results:     make([]*Result, len(devices)),
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := range devices {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, dev inventory.Device) {
			defer wg.Done()
			defer func() { <-sem }()
			report.Results[idx] = Device(ctx, &dev, opts)
		}(i, devices[i])
	}

	wg.Wait()

	// Drop any nil slots left by early context cancellation, then aggregate.
	results := report.Results[:0]
	for _, r := range report.Results {
		if r == nil {
			continue
		}
		results = append(results, r)
		switch {
		case r.Reachable:
			report.Reachable++
		case r.Error != nil && r.Error.Type == ErrCanceled:
			report.Canceled++
		default:
			report.Unreachable++
		}
		switch r.Login {
		case LoginOK:
			report.LoginOK++
		case LoginFailed:
			report.LoginFailed++
		case LoginSkipped:
			report.LoginSkipped++
		}
		if r.Commands != nil {
			switch r.Commands.Status {
			case CommandsOK:
				report.CommandsOK++
			case CommandsFailed:
				report.CommandsFailed++
			case CommandsSkipped:
				report.CommandsSkipped++
			}
		}
	}
	report.Results = results

	sort.Slice(report.Results, func(i, j int) bool {
		return report.Results[i].Hostname < report.Results[j].Hostname
	})

	report.DurationMs = time.Since(start).Milliseconds()
	return report
}

// classifyLoginError maps an SSH login error to a Result error type:
// timeout, auth_err, or login_err (transport/other).
func classifyLoginError(err error) string {
	if err == nil {
		return ""
	}
	if isTimeoutError(err) {
		return ErrTimeout
	}
	if isAuthError(err) {
		return ErrAuth
	}
	return ErrLogin
}

// isTimeoutError reports whether the error is a dial/handshake timeout,
// including a deadline that closed the connection mid-handshake.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "use of closed network connection")
}

// isAuthError reports whether an SSH error is an authentication rejection as
// opposed to a transport/connectivity problem.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "permission denied")
}

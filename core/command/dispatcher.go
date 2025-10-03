package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful dispatcher shutdown.
	DefaultShutdownTimeout = 30 * time.Second

	// DefaultStaleThreshold is the default time after which a dispatcher is considered stale.
	DefaultStaleThreshold = 5 * time.Minute

	// DefaultStuckThreshold is the default number of active commands that indicates a stuck dispatcher.
	DefaultStuckThreshold = 1000
)

// Dispatcher manages command handlers and coordinates command processing.
type Dispatcher struct {
	handlers        map[string]Handler
	commandBus      commandSource
	fallbackHandler func(context.Context, Command) error
	mu              sync.RWMutex

	shutdownTimeout       time.Duration
	maxConcurrentHandlers int
	handlerSemaphore      chan struct{}
	staleThreshold        time.Duration
	stuckThreshold        int32
	logger                *slog.Logger

	running    atomic.Bool
	cancelFunc atomic.Pointer[context.CancelFunc]
	done       atomic.Pointer[chan struct{}]
	wg         sync.WaitGroup

	commandsProcessed atomic.Int64
	commandsFailed    atomic.Int64
	activeCommands    atomic.Int32
	lastActivityAt    atomic.Int64
}

type commandSource interface {
	Commands() <-chan []byte
}

// DispatcherStats provides observability metrics for monitoring and debugging.
type DispatcherStats struct {
	CommandsProcessed int64
	CommandsFailed    int64
	ActiveCommands    int32
	IsRunning         bool
	LastActivityAt    time.Time
}

// NewDispatcher creates a new command dispatcher with the given options.
//
// Example:
//
//	dispatcher := command.NewDispatcher(
//	    command.WithCommandSource(bus),
//	    command.WithHandler(handler1),
//	)
func NewDispatcher(opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		handlers:              make(map[string]Handler),
		shutdownTimeout:       DefaultShutdownTimeout,
		maxConcurrentHandlers: 0, // 0 means unlimited
		staleThreshold:        DefaultStaleThreshold,
		stuckThreshold:        DefaultStuckThreshold,
		logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.maxConcurrentHandlers > 0 {
		d.handlerSemaphore = make(chan struct{}, d.maxConcurrentHandlers)
	}

	return d
}

// Start begins processing commands from the command source. This is a blocking operation
// that runs until the context is cancelled. Use Run() for errgroup pattern or call this in a goroutine.
func (d *Dispatcher) Start(ctx context.Context) error {
	if !d.running.CompareAndSwap(false, true) {
		return ErrDispatcherAlreadyStarted
	}
	defer d.running.Store(false)

	d.mu.RLock()
	hasHandlers := len(d.handlers) > 0 || d.fallbackHandler != nil
	d.mu.RUnlock()

	if d.commandBus == nil {
		return ErrCommandSourceNil
	}

	if !hasHandlers {
		return ErrNoHandler
	}

	dispCtx, cancel := context.WithCancel(ctx)
	d.cancelFunc.Store(&cancel)

	done := make(chan struct{})
	d.done.Store(&done)
	defer close(done)

	d.logger.InfoContext(dispCtx, "command dispatcher started",
		slog.Int("handler_count", len(d.handlers)))

	commands := d.commandBus.Commands()

	for {
		select {
		case <-dispCtx.Done():
			d.logger.Info("command dispatcher stopping")
			return dispCtx.Err()
		case data, ok := <-commands:
			if !ok {
				d.logger.Info("command source closed")
				return nil
			}

			var command Command
			if err := json.Unmarshal(data, &command); err != nil {
				d.logger.ErrorContext(dispCtx, "failed to unmarshal command",
					slog.String("error", err.Error()))
				continue
			}

			if err := d.processHandler(dispCtx, command); err != nil {
				if !errors.Is(err, ErrNoHandler) {
					d.logger.ErrorContext(dispCtx, "failed to process command",
						slog.String("command_id", command.ID),
						slog.String("command_name", command.Name),
						slog.String("error", err.Error()))
				}
			}
		}
	}
}

// Stop gracefully shuts down the dispatcher with a timeout.
// Returns an error if the shutdown timeout is exceeded.
func (d *Dispatcher) Stop() error {
	if !d.running.Load() {
		return ErrDispatcherNotStarted
	}

	if cancel := d.cancelFunc.Load(); cancel != nil {
		(*cancel)()
	}

	d.logger.Info("command dispatcher stopping, waiting for active handlers to complete",
		slog.Duration("timeout", d.shutdownTimeout))

	if done := d.done.Load(); done != nil {
		<-*done
	}

	ctx, ctxCancel := context.WithTimeout(context.Background(), d.shutdownTimeout)
	defer ctxCancel()

	waitDone := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		d.logger.Info("command dispatcher stopped cleanly")
		return nil
	case <-ctx.Done():
		d.logger.Warn("command dispatcher shutdown timeout exceeded - some handlers may be abandoned",
			slog.Duration("timeout", d.shutdownTimeout))
		return fmt.Errorf("shutdown timeout exceeded after %s", d.shutdownTimeout)
	}
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the dispatcher, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (d *Dispatcher) Run(ctx context.Context) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- d.Start(ctx)
		}()

		// Wait for either context cancellation (external shutdown signal)
		// or Start() completion (internal error or command source closed).
		select {
		case <-ctx.Done():
			if stopErr := d.Stop(); stopErr != nil {
				d.logger.Error("graceful shutdown failed", slog.String("error", stopErr.Error()))
			}
			<-errCh
			return nil
		case err := <-errCh:
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
	}
}

func (d *Dispatcher) acquireSemaphore(ctx context.Context) bool {
	if d.handlerSemaphore == nil {
		return true
	}
	select {
	case d.handlerSemaphore <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (d *Dispatcher) releaseSemaphore() {
	if d.handlerSemaphore != nil {
		<-d.handlerSemaphore
	}
}

func (d *Dispatcher) processHandler(ctx context.Context, command Command) error {
	d.mu.RLock()
	handler, exists := d.handlers[command.Name]
	fallback := d.fallbackHandler
	d.mu.RUnlock()

	if !exists {
		if fallback != nil {
			d.wg.Add(1)
			d.activeCommands.Add(1)

			go func() {
				defer d.wg.Done()
				defer d.activeCommands.Add(-1)

				handlerCtx := WithStartProcessingTime(WithCommandMeta(ctx, command), time.Now())

				if !d.acquireSemaphore(handlerCtx) {
					return
				}
				defer d.releaseSemaphore()

				defer func() {
					if r := recover(); r != nil {
						d.commandsFailed.Add(1)
						d.logger.ErrorContext(handlerCtx, "fallback handler panicked",
							slog.String("command_id", command.ID),
							slog.String("command_name", command.Name),
							slog.Any("panic", r))
					}
				}()

				start := time.Now()

				if err := fallback(handlerCtx, command); err != nil {
					d.commandsFailed.Add(1)
					d.logger.ErrorContext(handlerCtx, "fallback handler failed",
						slog.String("command_id", command.ID),
						slog.String("command_name", command.Name),
						slog.Duration("duration", time.Since(start)),
						slog.String("error", err.Error()))
				} else {
					d.commandsProcessed.Add(1)
					d.logger.DebugContext(handlerCtx, "fallback handler completed",
						slog.String("command_id", command.ID),
						slog.String("command_name", command.Name),
						slog.Duration("duration", time.Since(start)))
				}

				d.lastActivityAt.Store(time.Now().UnixNano())
			}()

			return nil
		}
		return ErrNoHandler
	}

	d.wg.Add(1)
	d.activeCommands.Add(1)

	go func() {
		defer d.wg.Done()
		defer d.activeCommands.Add(-1)

		handlerCtx := WithStartProcessingTime(WithCommandMeta(ctx, command), time.Now())

		if !d.acquireSemaphore(handlerCtx) {
			return
		}
		defer d.releaseSemaphore()

		defer func() {
			if r := recover(); r != nil {
				d.commandsFailed.Add(1)
				d.logger.ErrorContext(handlerCtx, "command handler panicked",
					slog.String("command_id", command.ID),
					slog.String("command_name", command.Name),
					slog.String("handler", handler.CommandName()),
					slog.Any("panic", r))
			}
		}()

		start := time.Now()

		if err := handler.Handle(handlerCtx, command.Payload); err != nil {
			d.commandsFailed.Add(1)
			d.logger.ErrorContext(handlerCtx, "command handler failed",
				slog.String("command_id", command.ID),
				slog.String("command_name", command.Name),
				slog.String("handler", handler.CommandName()),
				slog.Duration("duration", time.Since(start)),
				slog.String("error", err.Error()))
		} else {
			d.commandsProcessed.Add(1)
			d.logger.DebugContext(handlerCtx, "command handler completed",
				slog.String("command_id", command.ID),
				slog.String("command_name", command.Name),
				slog.String("handler", handler.CommandName()),
				slog.Duration("duration", time.Since(start)))
		}

		d.lastActivityAt.Store(time.Now().UnixNano())
	}()

	return nil
}

// Stats returns current dispatcher statistics for observability and monitoring.
func (d *Dispatcher) Stats() DispatcherStats {
	lastActivity := d.lastActivityAt.Load()
	var lastActivityTime time.Time
	if lastActivity > 0 {
		lastActivityTime = time.Unix(0, lastActivity)
	}

	return DispatcherStats{
		CommandsProcessed: d.commandsProcessed.Load(),
		CommandsFailed:    d.commandsFailed.Load(),
		ActiveCommands:    d.activeCommands.Load(),
		IsRunning:         d.running.Load(),
		LastActivityAt:    lastActivityTime,
	}
}

// Healthcheck validates that the dispatcher is operational.
// Returns nil if healthy, or an error describing the health issue.
// Checks for:
// - Dispatcher running status
// - Stale dispatcher (no recent activity)
// - Stuck dispatcher (too many active commands)
func (d *Dispatcher) Healthcheck(ctx context.Context) error {
	stats := d.Stats()

	if !stats.IsRunning {
		return errors.Join(ErrHealthcheckFailed, ErrDispatcherNotRunning)
	}

	var healthErrors []error

	if !stats.LastActivityAt.IsZero() {
		timeSinceActivity := time.Since(stats.LastActivityAt)
		if timeSinceActivity > d.staleThreshold {
			healthErrors = append(healthErrors, fmt.Errorf("%w: last activity %s ago (threshold: %s)",
				ErrDispatcherStale, timeSinceActivity.Round(time.Second), d.staleThreshold))
		}
	}

	if stats.ActiveCommands > d.stuckThreshold {
		healthErrors = append(healthErrors, fmt.Errorf("%w: %d active commands (threshold: %d)",
			ErrDispatcherStuck, stats.ActiveCommands, d.stuckThreshold))
	}

	if len(healthErrors) > 0 {
		return errors.Join(append([]error{ErrHealthcheckFailed}, healthErrors...)...)
	}

	return nil
}

package queue

// Storage is a unified interface that combines all repository interfaces
// required for queue operations. Implementations of this interface can
// serve as the complete storage backend for Worker, Scheduler, and Enqueuer.
//
// This interface is designed to simplify queue service initialization by
// requiring only a single storage dependency that satisfies all component needs.
type Storage interface {
	// EnqueuerRepository provides task creation capabilities
	EnqueuerRepository

	// WorkerRepository provides task claiming and processing capabilities
	WorkerRepository

	// SchedulerRepository provides scheduled task management capabilities
	SchedulerRepository
}

package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"YT-GO/internal/platform"
)

const (
	defaultMaxConcurrentDownloads = 3
	maxDownloadWorkers            = 10
	maxQueuedDownloads            = 100
)

var (
	ErrDownloadQueueFull = errors.New("download queue is full")
	ErrServiceClosed     = errors.New("download service is shutting down")
)

type downloadJob struct {
	taskID    string
	request   DownloadRequest
	ytdlpPath string
	control   *downloadControl
	ready     chan struct{}
}

type downloadControl struct {
	ctx    context.Context
	cancel context.CancelFunc

	eventMu       sync.Mutex
	processMu     sync.Mutex
	pid           int
	processCancel context.CancelFunc
	terminate     func(int) error
	finish        sync.Once
	workerExited  atomic.Bool
}

func newDownloadControl(parent context.Context) *downloadControl {
	ctx, cancel := context.WithCancel(parent)
	return &downloadControl{ctx: ctx, cancel: cancel, terminate: platform.TerminateProcessTree}
}

func (c *downloadControl) attachProcess(pid int, processCancel context.CancelFunc) error {
	c.processMu.Lock()
	defer c.processMu.Unlock()
	c.pid = pid
	c.processCancel = processCancel
	cancelled := c.ctx.Err() != nil
	if cancelled {
		err := c.terminateProcessTree(pid)
		if err == nil {
			processCancel()
		}
		return err
	}
	return nil
}

func (c *downloadControl) detachProcess(pid int) {
	c.processMu.Lock()
	var processCancel context.CancelFunc
	if c.pid == pid {
		c.pid = 0
		processCancel = c.processCancel
		c.processCancel = nil
	}
	c.processMu.Unlock()
	if processCancel != nil {
		processCancel()
	}
}

func (c *downloadControl) cancelAndTerminate() error {
	c.cancel()
	c.processMu.Lock()
	defer c.processMu.Unlock()
	pid := c.pid
	err := c.terminateProcessTree(pid)
	if err == nil && c.processCancel != nil {
		c.processCancel()
	}
	return err
}

func (c *downloadControl) terminateProcessTree(pid int) error {
	if pid <= 0 || c.terminate == nil {
		return nil
	}
	return c.terminate(pid)
}

type downloadResult struct {
	outputPath string
	size       string
	warning    string
}

type downloadOutcome uint8

const (
	downloadCompleted downloadOutcome = iota
	downloadFailed
	downloadCancelled
)

func (s *Service) ensureRuntimeLocked() {
	if s.runtimeStarted {
		return
	}
	s.jobs = make(chan downloadJob, maxQueuedDownloads)
	s.startPersistenceWriterLocked()
	for i := 0; i < maxDownloadWorkers; i++ {
		s.workerWG.Add(1)
		go s.downloadWorker()
	}
	s.runtimeStarted = true
}

func (s *Service) ensureRuntime() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.accepting {
		return ErrServiceClosed
	}
	s.ensureRuntimeLocked()
	return nil
}

func (s *Service) downloadWorker() {
	defer s.workerWG.Done()
	for job := range s.jobs {
		select {
		case <-job.ready:
		case <-job.control.ctx.Done():
			continue
		}
		if err := s.limiter.Acquire(job.control.ctx); err != nil {
			s.finishDownload(job.taskID, job.control, downloadCancelled, downloadResult{}, err)
			continue
		}
		if !s.markDownloadStarted(job.taskID, job.control) {
			s.limiter.Release()
			continue
		}
		s.runDownloadJob(job)
	}
}

func (s *Service) runDownloadJob(job downloadJob) {
	defer s.limiter.Release()
	defer func() {
		job.control.workerExited.Store(true)
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("download worker panic: %v", recovered)
			if terminateErr := job.control.cancelAndTerminate(); terminateErr != nil {
				s.emitLog("failed to terminate process tree for task %s: %v", job.taskID, terminateErr)
				return
			}
			s.emitDownloadLog(job.taskID, "[YT-GO] "+err.Error())
			if job.control.ctx.Err() != nil {
				s.finishDownload(job.taskID, job.control, downloadCancelled, downloadResult{}, context.Canceled)
			} else {
				s.finishDownload(job.taskID, job.control, downloadFailed, downloadResult{}, err)
			}
		}
	}()
	s.executeDownload(job)
}

func (s *Service) markDownloadStarted(taskID string, control *downloadControl) bool {
	if control.ctx.Err() != nil {
		return false
	}
	var snapshot *DownloadTask
	s.mu.Lock()
	if active, ok := s.activeDownloads[taskID]; ok && active == control {
		if task, exists := s.downloads[taskID]; exists && task.Status == "pending" {
			task.Status = "downloading"
			snapshot = cloneDownloadTask(task)
			s.enqueueTaskSaveLocked(snapshot)
		}
	}
	s.mu.Unlock()
	if snapshot != nil {
		s.emitActiveDownloadUpdate(taskID, snapshot)
		return true
	}
	return false
}

func (s *Service) finishDownload(taskID string, control *downloadControl, outcome downloadOutcome, result downloadResult, runErr error) {
	control.finish.Do(func() {
		control.cancel()
		control.eventMu.Lock()
		defer control.eventMu.Unlock()
		var snapshot *DownloadTask
		removed := false

		s.mu.Lock()
		if active, ok := s.activeDownloads[taskID]; ok && active == control {
			delete(s.activeDownloads, taskID)
		}
		if task, ok := s.downloads[taskID]; ok {
			switch outcome {
			case downloadCancelled:
				delete(s.downloads, taskID)
				s.enqueueTaskDeleteLocked([]string{taskID})
				removed = true
			case downloadFailed:
				task.Status = "error"
				if runErr != nil {
					task.Error = runErr.Error()
				}
				snapshot = cloneDownloadTask(task)
				s.enqueueTaskSaveLocked(snapshot)
			case downloadCompleted:
				task.Status = "completed"
				task.Progress = 100
				task.ETA = ""
				task.Error = ""
				if result.outputPath != "" {
					task.OutputPath = result.outputPath
				}
				if result.size != "" {
					task.Size = result.size
				}
				snapshot = cloneDownloadTask(task)
				s.enqueueTaskSaveLocked(snapshot)
			}
		}
		s.mu.Unlock()

		if removed {
			s.emitDownloadRemove(taskID)
		} else if snapshot != nil {
			s.emitDownloadUpdate(snapshot)
		}
	})
}

func (s *Service) emitActiveDownloadUpdate(taskID string, snapshot *DownloadTask) {
	s.mu.RLock()
	control := s.activeDownloads[taskID]
	s.mu.RUnlock()
	if control == nil {
		return
	}
	control.eventMu.Lock()
	defer control.eventMu.Unlock()
	s.mu.RLock()
	active := s.activeDownloads[taskID] == control
	s.mu.RUnlock()
	if active {
		s.emitDownloadUpdate(snapshot)
	}
}

// CancelDownload removes queued work immediately. Running work remains visible
// until its worker confirms that the process or request has exited.
func (s *Service) CancelDownload(taskID string) error {
	s.mu.RLock()
	control, ok := s.activeDownloads[taskID]
	status := ""
	if task := s.downloads[taskID]; task != nil {
		status = task.Status
	}
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task not found or not active")
	}
	if err := control.cancelAndTerminate(); err != nil {
		s.emitLog("failed to terminate process tree for task %s: %v", taskID, err)
		return fmt.Errorf("cancel task %s: %w", taskID, err)
	}
	if status == "pending" || control.workerExited.Load() {
		s.finishDownload(taskID, control, downloadCancelled, downloadResult{}, context.Canceled)
	}
	return nil
}

// Shutdown stops accepting work, cancels active downloads, drains persistence,
// and closes the database. The shutdown continues safely if ctx expires.
func (s *Service) Shutdown(ctx context.Context) error {
	s.shutdownOnce.Do(func() {
		go s.shutdown()
	})
	select {
	case <-s.shutdownDone:
		s.shutdownErrMu.Lock()
		defer s.shutdownErrMu.Unlock()
		return s.shutdownErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) shutdown() {
	s.lifecycleMu.Lock()
	s.accepting = false
	started := s.runtimeStarted
	if started {
		close(s.jobs)
	}
	s.lifecycleMu.Unlock()

	type activeTask struct {
		id      string
		control *downloadControl
	}
	s.mu.RLock()
	active := make([]activeTask, 0, len(s.activeDownloads))
	for id, control := range s.activeDownloads {
		active = append(active, activeTask{id: id, control: control})
	}
	s.mu.RUnlock()
	var terminationErrors []error
	for _, task := range active {
		s.mu.RLock()
		status := ""
		if current := s.downloads[task.id]; current != nil {
			status = current.Status
		}
		s.mu.RUnlock()
		if err := task.control.cancelAndTerminate(); err != nil {
			s.emitLog("failed to terminate process tree for task %s: %v", task.id, err)
			terminationErrors = append(terminationErrors, fmt.Errorf("cancel task %s: %w", task.id, err))
			continue
		}
		if status == "pending" || task.control.workerExited.Load() {
			s.finishDownload(task.id, task.control, downloadCancelled, downloadResult{}, context.Canceled)
		}
	}

	if started {
		s.workerWG.Wait()
	}
	s.closePersistenceWriter()

	err := errors.Join(terminationErrors...)
	err = errors.Join(err, s.closeDatabase())
	if persistenceErr := s.PersistenceError(); persistenceErr != nil {
		err = errors.Join(err, persistenceErr)
	}
	s.shutdownErrMu.Lock()
	s.shutdownErr = err
	s.shutdownErrMu.Unlock()
	close(s.shutdownDone)
}

// Close performs an unbounded graceful shutdown.
func (s *Service) Close() error {
	return s.Shutdown(context.Background())
}

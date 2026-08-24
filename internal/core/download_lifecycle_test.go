package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"

	"YT-GO/internal/platform"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPendingDownloadCanBeCancelledImmediately(t *testing.T) {
	service := NewService("test")
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer service.limiter.Release()
	t.Cleanup(func() { _ = service.Close() })

	removed := make(chan string, 1)
	service.SetHooks(Hooks{DownloadRemove: func(taskID string) { removed <- taskID }})
	id, err := service.enqueueDownload(DownloadRequest{URL: "https://example.com/video", OutputDir: t.TempDir()}, "unused")
	if err != nil {
		t.Fatalf("enqueue download: %v", err)
	}
	if err := service.CancelDownload(id); err != nil {
		t.Fatalf("cancel pending download: %v", err)
	}
	if _, err := service.GetDownload(id); err == nil {
		t.Fatal("cancelled pending download should be removed immediately")
	}
	select {
	case got := <-removed:
		if got != id {
			t.Fatalf("removed task %q, want %q", got, id)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for removal hook")
	}
}

func TestRunningCancellationFailureKeepsTaskVisibleAndRetryable(t *testing.T) {
	service := NewService("test")
	if err := service.ensureRuntime(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	control := newDownloadControl(context.Background())
	processCtx, processCancel := context.WithCancel(context.Background())
	defer processCancel()
	terminateErr := errors.New("tree termination failed")
	control.processMu.Lock()
	control.pid = 4242
	control.processCancel = processCancel
	control.terminate = func(int) error { return terminateErr }
	control.processMu.Unlock()
	task := &DownloadTask{ID: "running-task", Status: "downloading", CreatedAt: time.Now().Format(time.RFC3339)}
	service.mu.Lock()
	service.downloads[task.ID] = task
	service.activeDownloads[task.ID] = control
	service.mu.Unlock()

	if err := service.CancelDownload(task.ID); !errors.Is(err, terminateErr) {
		t.Fatalf("cancel error=%v, want %v", err, terminateErr)
	}
	if _, err := service.GetDownload(task.ID); err != nil {
		t.Fatalf("task disappeared after failed termination: %v", err)
	}
	service.mu.RLock()
	active := service.activeDownloads[task.ID] == control
	service.mu.RUnlock()
	if !active {
		t.Fatal("failed cancellation removed active control")
	}
	select {
	case <-control.ctx.Done():
	default:
		t.Fatal("task context was not cancelled promptly")
	}
	select {
	case <-processCtx.Done():
		t.Fatal("process context was cancelled after tree termination failed")
	default:
	}

	control.processMu.Lock()
	control.terminate = func(int) error { return nil }
	control.processMu.Unlock()
	control.workerExited.Store(true)
	if err := service.CancelDownload(task.ID); err != nil {
		t.Fatalf("retry cancellation: %v", err)
	}
	if _, err := service.GetDownload(task.ID); err == nil {
		t.Fatal("task should be removed after successful retry and confirmed worker exit")
	}
	select {
	case <-processCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("successful retry did not cancel process context")
	}
}

func TestRunningCancellationWaitsForWorkerExitBeforeRemoval(t *testing.T) {
	service := NewService("test")
	if err := service.ensureRuntime(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	processCtx, processCancel := context.WithCancel(context.Background())
	defer processCancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(processCtx, "cmd.exe", "/c", "ping", "-n", "30", "127.0.0.1")
	} else {
		cmd = exec.CommandContext(processCtx, "sh", "-c", "sleep 30")
	}
	platform.ConfigureCmdWindow(cmd, true)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	control := newDownloadControl(context.Background())
	if err := control.attachProcess(cmd.Process.Pid, processCancel); err != nil {
		t.Fatal(err)
	}
	task := &DownloadTask{ID: "running-process", Status: "downloading", CreatedAt: time.Now().Format(time.RFC3339)}
	service.mu.Lock()
	service.downloads[task.ID] = task
	service.activeDownloads[task.ID] = control
	service.mu.Unlock()

	processExited := make(chan struct{})
	allowFinish := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		control.detachProcess(cmd.Process.Pid)
		close(processExited)
		<-allowFinish
		service.finishDownload(task.ID, control, downloadCancelled, downloadResult{}, context.Canceled)
		close(finished)
	}()

	if err := service.CancelDownload(task.ID); err != nil {
		t.Fatalf("cancel running process: %v", err)
	}
	select {
	case <-processExited:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not terminate the running process")
	}
	if _, err := service.GetDownload(task.ID); err != nil {
		t.Fatalf("task was removed before worker confirmed exit: %v", err)
	}
	close(allowFinish)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("worker finalization did not complete")
	}
	if _, err := service.GetDownload(task.ID); err == nil {
		t.Fatal("task remained after worker confirmed exit")
	}
}

func TestDownloadQueueIsBoundedWithoutPerRequestGoroutines(t *testing.T) {
	service := NewService("test")
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		service.limiter.Release()
		_ = service.Close()
	})

	before := runtime.NumGoroutine()
	ids := make([]string, 0, maxQueuedDownloads+maxDownloadWorkers)
	for i := 0; i < maxQueuedDownloads+maxDownloadWorkers+1; i++ {
		id, err := service.enqueueDownload(DownloadRequest{
			URL:       fmt.Sprintf("https://example.com/video/%d", i),
			OutputDir: t.TempDir(),
		}, "unused")
		if errors.Is(err, ErrDownloadQueueFull) {
			break
		}
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 || len(ids) > maxQueuedDownloads+maxDownloadWorkers {
		t.Fatalf("accepted %d downloads for a queue bounded at %d plus %d workers", len(ids), maxQueuedDownloads, maxDownloadWorkers)
	}
	if delta := runtime.NumGoroutine() - before; delta > maxDownloadWorkers+4 {
		t.Fatalf("enqueue created too many goroutines: delta=%d", delta)
	}
	for _, id := range ids {
		if err := service.CancelDownload(id); err != nil {
			t.Fatalf("cancel %s: %v", id, err)
		}
	}
}

func TestHookReceivesTaskSnapshot(t *testing.T) {
	service := NewService("test")
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer service.limiter.Release()
	t.Cleanup(func() { _ = service.Close() })

	service.SetHooks(Hooks{DownloadUpdate: func(task *DownloadTask) {
		task.Status = "corrupted-by-hook"
		task.Title = "corrupted-by-hook"
	}})
	id, err := service.enqueueDownload(DownloadRequest{
		URL:       "https://example.com/video",
		OutputDir: t.TempDir(),
		VideoInfo: &VideoInfo{Title: "original"},
	}, "unused")
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.GetDownload(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "pending" || task.Title != "original" {
		t.Fatalf("hook mutated internal task: %+v", task)
	}
	_ = service.CancelDownload(id)
}

func TestClearCompletedEmitsRemovalForEveryDeletedTask(t *testing.T) {
	service := NewService("test")
	if err := service.ensureRuntime(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	removed := make(map[string]bool)
	service.SetHooks(Hooks{DownloadRemove: func(taskID string) { removed[taskID] = true }})
	service.mu.Lock()
	service.downloads["completed"] = &DownloadTask{ID: "completed", Status: "completed"}
	service.downloads["failed"] = &DownloadTask{ID: "failed", Status: "error"}
	service.downloads["active"] = &DownloadTask{ID: "active", Status: "downloading"}
	service.mu.Unlock()

	service.ClearCompleted()

	if !removed["completed"] || !removed["failed"] || len(removed) != 2 {
		t.Fatalf("removal hooks = %v, want completed and failed", removed)
	}
	if _, err := service.GetDownload("active"); err != nil {
		t.Fatalf("active task was removed: %v", err)
	}
}

func TestPersistenceWriterUsesSnapshotsAndPreservesDeleteOrder(t *testing.T) {
	service := newPersistenceTestService(t)
	task := &DownloadTask{
		ID:        "task-1",
		URL:       "https://example.com/video",
		Status:    "pending",
		OutputDir: t.TempDir(),
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	service.mu.Lock()
	service.enqueueTaskSaveLocked(task)
	task.Status = "completed"
	service.enqueueTaskSaveLocked(task)
	service.enqueueTaskDeleteLocked([]string{task.ID})
	service.mu.Unlock()
	if err := service.flushPersistence(); err != nil {
		t.Fatalf("flush persistence: %v", err)
	}

	var count int64
	if err := service.db.Model(&DownloadRecord{}).Where("id = ?", task.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale save resurrected deleted task; count=%d", count)
	}
}

func TestCancellingPendingDownloadCannotResurrectDatabaseRecord(t *testing.T) {
	service := newPersistenceTestService(t)
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer service.limiter.Release()

	id, err := service.enqueueDownload(DownloadRequest{
		URL:       "https://example.com/video",
		OutputDir: t.TempDir(),
	}, "unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CancelDownload(id); err != nil {
		t.Fatal(err)
	}
	if err := service.flushPersistence(); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := service.db.Model(&DownloadRecord{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cancelled task was resurrected in the database; count=%d", count)
	}
}

func TestPersistenceSaveCopiesTaskBeforeWriterUsesIt(t *testing.T) {
	service := newPersistenceTestService(t)
	task := &DownloadTask{
		ID:        "task-snapshot",
		URL:       "https://example.com/video",
		Status:    "pending",
		OutputDir: t.TempDir(),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	service.mu.Lock()
	service.enqueueTaskSaveLocked(task)
	service.mu.Unlock()
	task.Status = "mutated-after-enqueue"
	if err := service.flushPersistence(); err != nil {
		t.Fatal(err)
	}
	var record DownloadRecord
	if err := service.db.First(&record, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if record.Status != "pending" {
		t.Fatalf("database observed mutable task pointer: %q", record.Status)
	}
}

func TestPersistenceErrorsAreObservable(t *testing.T) {
	service := newPersistenceTestService(t)
	database, err := service.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	task := &DownloadTask{
		ID:        "task-error",
		URL:       "https://example.com/video",
		Status:    "pending",
		OutputDir: t.TempDir(),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	service.mu.Lock()
	service.enqueueTaskSaveLocked(task)
	service.mu.Unlock()
	if err := service.flushPersistence(); err != nil {
		t.Fatal(err)
	}
	if service.PersistenceError() == nil {
		t.Fatal("expected asynchronous database failure to be observable")
	}
}

func TestSaveSettingsUpdatesConcurrencyLimitWithoutRestartingWorkers(t *testing.T) {
	service := newPersistenceTestServiceWithoutRuntime(t)
	service.lifecycleMu.Lock()
	startedBeforeSave := service.runtimeStarted
	service.lifecycleMu.Unlock()
	if startedBeforeSave {
		t.Fatal("test service unexpectedly started its runtime")
	}

	settings := service.GetSettings()
	settings.MaxConcurrent = 7
	if err := service.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	service.limiter.mu.Lock()
	limit := service.limiter.limit
	service.limiter.mu.Unlock()
	if limit != 7 {
		t.Fatalf("concurrency limit=%d, want 7", limit)
	}
	service.lifecycleMu.Lock()
	jobs := service.jobs
	startedAfterSave := service.runtimeStarted
	service.lifecycleMu.Unlock()
	if !startedAfterSave || jobs == nil {
		t.Fatal("saving settings did not start the persistence runtime")
	}
	settings.MaxConcurrent = 5
	if err := service.SaveSettings(settings); err != nil {
		t.Fatalf("save settings again: %v", err)
	}
	service.lifecycleMu.Lock()
	if service.jobs != jobs {
		service.lifecycleMu.Unlock()
		t.Fatal("saving concurrency rebuilt the queue")
	}
	service.lifecycleMu.Unlock()
	if err := service.ResetSettings(); err != nil {
		t.Fatalf("reset settings: %v", err)
	}
	service.limiter.mu.Lock()
	limit = service.limiter.limit
	service.limiter.mu.Unlock()
	if limit != defaultMaxConcurrentDownloads {
		t.Fatalf("concurrency limit after reset=%d, want %d", limit, defaultMaxConcurrentDownloads)
	}
}

func TestShutdownRejectsNewDownloads(t *testing.T) {
	service := NewService("test")
	if err := service.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}
	_, err := service.enqueueDownload(DownloadRequest{URL: "https://example.com/video", OutputDir: t.TempDir()}, "unused")
	if !errors.Is(err, ErrServiceClosed) {
		t.Fatalf("enqueue after shutdown error=%v, want %v", err, ErrServiceClosed)
	}
}

func TestResizableLimiterAppliesNewLimitToWaitingWork(t *testing.T) {
	limiter := newDownloadLimiter(1)
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	acquired := make(chan struct{})
	go func() {
		if limiter.Acquire(context.Background()) == nil {
			close(acquired)
		}
	}()
	select {
	case <-acquired:
		t.Fatal("second acquisition should initially wait")
	case <-time.After(20 * time.Millisecond):
	}
	limiter.SetLimit(2)
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("waiting acquisition did not observe increased limit")
	}
	limiter.Release()
	limiter.Release()
}

func newPersistenceTestService(t *testing.T) *Service {
	service := newPersistenceTestServiceWithoutRuntime(t)
	if err := service.ensureRuntime(); err != nil {
		t.Fatal(err)
	}
	return service
}

func newPersistenceTestServiceWithoutRuntime(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&DownloadRecord{}, &SettingsRecord{}); err != nil {
		t.Fatal(err)
	}
	service := NewService("test")
	service.db = db
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { _ = service.Close() }) })
	return service
}

package core

import (
	"errors"
	"fmt"
	"log"
)

const persistenceQueueCapacity = maxQueuedDownloads*3 + 32

type persistenceOperationKind uint8

const (
	persistenceSaveTask persistenceOperationKind = iota
	persistenceDeleteTasks
	persistenceSaveSettings
	persistenceDeleteSettings
	persistenceBarrier
)

type persistenceOperation struct {
	kind     persistenceOperationKind
	task     DownloadTask
	ids      []string
	settings SettingsRecord
	done     chan error
}

func (s *Service) startPersistenceWriterLocked() {
	s.persistCh = make(chan persistenceOperation, persistenceQueueCapacity)
	s.persistWG.Add(1)
	go func() {
		defer s.persistWG.Done()
		for operation := range s.persistCh {
			err := s.applyPersistenceOperation(operation)
			if err != nil {
				s.recordPersistenceError(err)
			}
			if operation.done != nil {
				operation.done <- err
				close(operation.done)
			}
		}
	}()
}

func (s *Service) applyPersistenceOperation(operation persistenceOperation) error {
	if operation.kind == persistenceBarrier {
		return nil
	}
	db := s.database()
	if db == nil {
		if operation.kind == persistenceSaveSettings || operation.kind == persistenceDeleteSettings {
			return errors.New("database not initialized")
		}
		return nil
	}

	switch operation.kind {
	case persistenceSaveTask:
		record := taskToRecord(&operation.task)
		return db.Save(&record).Error
	case persistenceDeleteTasks:
		if len(operation.ids) == 0 {
			return nil
		}
		return db.Delete(&DownloadRecord{}, operation.ids).Error
	case persistenceSaveSettings:
		record := operation.settings
		return db.Save(&record).Error
	case persistenceDeleteSettings:
		return db.Where("id = ?", 1).Delete(&SettingsRecord{}).Error
	default:
		return fmt.Errorf("unknown persistence operation: %d", operation.kind)
	}
}

func (s *Service) submitPersistence(operation persistenceOperation) error {
	s.persistSubmitMu.Lock()
	if s.persistClosed || s.persistCh == nil {
		s.persistSubmitMu.Unlock()
		return errors.New("persistence writer is closed")
	}
	s.persistCh <- operation
	s.persistSubmitMu.Unlock()
	if operation.done == nil {
		return nil
	}
	return <-operation.done
}

func (s *Service) enqueueTaskSaveLocked(task *DownloadTask) {
	if task == nil {
		return
	}
	snapshot := *cloneDownloadTask(task)
	if err := s.submitPersistence(persistenceOperation{kind: persistenceSaveTask, task: snapshot}); err != nil {
		s.recordPersistenceError(err)
	}
}

func (s *Service) enqueueTaskDeleteLocked(ids []string) {
	if len(ids) == 0 {
		return
	}
	snapshot := append([]string(nil), ids...)
	if err := s.submitPersistence(persistenceOperation{kind: persistenceDeleteTasks, ids: snapshot}); err != nil {
		s.recordPersistenceError(err)
	}
}

func (s *Service) flushPersistence() error {
	done := make(chan error, 1)
	return s.submitPersistence(persistenceOperation{kind: persistenceBarrier, done: done})
}

func (s *Service) recordPersistenceError(err error) {
	if err == nil {
		return
	}
	s.persistErrMu.Lock()
	s.persistErr = err
	s.persistErrMu.Unlock()
	log.Printf("YT-GO persistence error: %v", err)
}

// PersistenceError returns the most recent asynchronous database error.
func (s *Service) PersistenceError() error {
	s.persistErrMu.Lock()
	defer s.persistErrMu.Unlock()
	return s.persistErr
}

func (s *Service) closePersistenceWriter() {
	s.persistSubmitMu.Lock()
	if !s.persistClosed {
		s.persistClosed = true
		if s.persistCh != nil {
			close(s.persistCh)
		}
	}
	s.persistSubmitMu.Unlock()
	s.persistWG.Wait()
}

func (s *Service) closeDatabase() error {
	db := s.database()
	if db == nil {
		return nil
	}
	database, err := db.DB()
	if err != nil {
		return err
	}
	return database.Close()
}

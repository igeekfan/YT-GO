package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"
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
	settings Settings
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
		return persistSettingValues(db, operation.settings)
	case persistenceDeleteSettings:
		return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SettingValueRecord{}).Error
	default:
		return fmt.Errorf("unknown persistence operation: %d", operation.kind)
	}
}

func persistSettingValues(db *gorm.DB, settings Settings) error {
	payload, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}
	rows := make([]SettingValueRecord, 0, len(settingDefinitions))
	keys := make([]string, 0, len(settingDefinitions))
	for _, definition := range settingDefinitions {
		value, ok := values[definition.Key]
		if !ok {
			continue
		}
		rows = append(rows, SettingValueRecord{Key: definition.Key, Value: string(value)})
		keys = append(keys, definition.Key)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("key IN ?", keys).Delete(&SettingValueRecord{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
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

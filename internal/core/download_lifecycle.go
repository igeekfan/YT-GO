package core

import (
	"context"
	"os/exec"
)

func (s *Service) startDownloadTask(taskID string, cancel context.CancelFunc) (*DownloadTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.downloads[taskID]
	if !ok {
		return nil, false
	}
	s.cancelFns[taskID] = cancel
	task.Status = "downloading"
	copy := *task
	return &copy, true
}

func (s *Service) storeDownloadCommand(taskID string, cmd *exec.Cmd) {
	s.mu.Lock()
	s.cmds[taskID] = cmd
	s.mu.Unlock()
}

func (s *Service) clearActiveDownload(taskID string) {
	s.mu.Lock()
	delete(s.cancelFns, taskID)
	delete(s.cmds, taskID)
	s.mu.Unlock()
}

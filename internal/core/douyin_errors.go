package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// translateDouyinError translates i18n key errors from internal douyin functions
// into user-facing localized messages.
func (s *Service) translateDouyinError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Handle parameterized keys like "douyin.status_code:404"
	if idx := strings.Index(msg, ":"); idx > 0 && strings.HasPrefix(msg, "douyin.") {
		prefix := msg[:idx]
		suffix := msg[idx+1:]
		translated := s.i18n.T(prefix)
		if translated != prefix {
			if strings.Contains(translated, "%d") {
				if val, parseErr := strconv.Atoi(suffix); parseErr == nil {
					return errors.New(fmt.Sprintf(translated, val))
				}
			}
			if strings.Contains(translated, "%s") {
				return errors.New(fmt.Sprintf(translated, suffix))
			}
			return errors.New(translated + ": " + suffix)
		}
	}
	// Handle simple keys like "douyin.link_not_found"
	if strings.HasPrefix(msg, "douyin.") {
		translated := s.i18n.T(msg)
		if translated != msg {
			return errors.New(translated)
		}
	}
	return err
}

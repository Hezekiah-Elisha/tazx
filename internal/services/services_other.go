//go:build !linux

package services

import "context"

// IsSystemdAvailable always returns false on non-Linux operating systems.
func IsSystemdAvailable() bool {
	return false
}

// ListServices returns ErrSystemdUnavailable on non-Linux operating systems.
func ListServices(_ context.Context, _ bool) ([]ServiceInfo, error) {
	return nil, ErrSystemdUnavailable
}

// GetService returns ErrSystemdUnavailable on non-Linux operating systems.
func GetService(_ context.Context, _ string) (*ServiceInfo, error) {
	return nil, ErrSystemdUnavailable
}

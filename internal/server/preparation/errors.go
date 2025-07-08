package preparation

import (
	"errors"
	"fmt"
)

var (
	ErrPackageManagerNotSupported = errors.New("package manager not supported")
	ErrPrerequisiteCheckFailed    = errors.New("prerequisite check failed")
	ErrPackageInstallationFailed  = errors.New("package installation failed")
	ErrDirectoryCreationFailed    = errors.New("directory creation failed")
	ErrVerificationFailed         = errors.New("verification failed")
)

type InstallationError struct {
	Tool       string
	Underlying error
}

func (ie *InstallationError) Error() string {
	return fmt.Sprintf("installation of %s failed: %v", ie.Tool, ie.Underlying)
}

func (ie *InstallationError) Unwrap() error {
	return ie.Underlying
}

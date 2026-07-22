//go:build !windows

package shared

func lockingProcesses(path string) ([]FileLockingProcess, error) {
	_ = path
	return nil, nil
}

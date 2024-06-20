package procutil

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func ReleaseFileHandles(file string) error {
	pid := int32(os.Getpid())
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to get running processes: %s", err)
	}
	var wg sync.WaitGroup
	for _, ps := range processes {
		if ps.Pid == pid {
			continue
		}
		wg.Add(1)
		go func(ps *process.Process) {
			defer wg.Done()
			var kill = false
			var openedFiles []process.OpenFilesStat
			openedFiles, err = ps.OpenFiles()
			if err != nil {
				return
			}
			for _, openedFile := range openedFiles {
				if strings.HasPrefix(openedFile.Path, file) {
					kill = true
					break
				}
			}
			var exePath string
			exePath, err = ps.Exe()
			if err == nil {
				if strings.HasPrefix(exePath, file) {
					kill = true
				}
			}
			if !kill {
				return
			}
			err = ps.Kill()
			if err != nil {
				err = fmt.Errorf("failed to kill process: %s", err)
			}
		}(ps)
	}
	wg.Wait()
	if err != nil {
		return fmt.Errorf("failed to release everything: %s", err)
	}
	time.Sleep(time.Second)
	return nil
}

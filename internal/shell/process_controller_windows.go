//go:build windows

package shell

import (
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processController interface {
	Terminate(killTimeout time.Duration) error
	Close() error
}

type basicProcessController struct {
	cmd *exec.Cmd
}

func newProcessController(cmd *exec.Cmd, detached bool) processController {
	basic := &basicProcessController{cmd: cmd}
	if !detached || cmd == nil || cmd.Process == nil {
		return basic
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return basic
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		return basic
	}

	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		windows.CloseHandle(job)
		return basic
	}
	defer windows.CloseHandle(processHandle)

	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		windows.CloseHandle(job)
		return basic
	}

	return &windowsJobController{
		basicProcessController: basic,
		job:                    job,
	}
}

func (c *basicProcessController) Terminate(_ time.Duration) error {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}

func (c *basicProcessController) Close() error {
	return nil
}

type windowsJobController struct {
	*basicProcessController
	job windows.Handle
}

func (c *windowsJobController) Terminate(_ time.Duration) error {
	if c == nil {
		return nil
	}
	if c.job != 0 {
		return windows.TerminateJobObject(c.job, 1)
	}
	return c.basicProcessController.Terminate(0)
}

func (c *windowsJobController) Close() error {
	if c == nil || c.job == 0 {
		return nil
	}
	err := windows.CloseHandle(c.job)
	c.job = 0
	return err
}

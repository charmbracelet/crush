//go:build windows

package pty

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCreatePseudoConsole    = kernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole    = kernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole     = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAL = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttr   = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAL     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procCreateProcessW         = kernel32.NewProc("CreateProcessW")
)

const (
	_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x00020016
	_EXTENDED_STARTUPINFO_PRESENT        = 0x00080000
)

// coord matches Win32 COORD: X in low 16 bits, Y in high 16 bits (little-endian x86/ARM).
type coord struct {
	x, y int16
}

func coordToDWORD(c coord) uintptr {
	return uintptr(*(*uint32)(unsafe.Pointer(&c)))
}

// startupInfoEx matches Win32 STARTUPINFOEXW.
type startupInfoEx struct {
	startupInfo   syscall.StartupInfo
	lpAttributeList uintptr
}

func newPlatformPTY(opts Options) (*PTY, error) {
	// Create pipes for PTY I/O.
	ptyInR, ptyInW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create input pipe: %w", err)
	}
	ptyOutR, ptyOutW, err := os.Pipe()
	if err != nil {
		ptyInR.Close()
		ptyInW.Close()
		return nil, fmt.Errorf("create output pipe: %w", err)
	}

	// Create the pseudo console.
	size := coord{x: int16(opts.Cols), y: int16(opts.Rows)}
	var hPC syscall.Handle

	r, _, err := procCreatePseudoConsole.Call(
		coordToDWORD(size),
		ptyInR.Fd(),
		ptyOutW.Fd(),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if r != 0 {
		ptyInR.Close()
		ptyInW.Close()
		ptyOutR.Close()
		ptyOutW.Close()
		return nil, fmt.Errorf("CreatePseudoConsole failed: HRESULT 0x%x: %w", r, err)
	}

	// Close the pipe ends that the ConPTY now owns.
	ptyInR.Close()
	ptyOutW.Close()

	// Initialize process thread attribute list for ConPTY.
	// First call: query required size.
	var attrListSize uintptr
	procInitializeProcThreadAL.Call(0, 1, 0, 0, uintptr(unsafe.Pointer(&attrListSize)))

	attrListBuf := make([]byte, attrListSize)
	attrListPtr := uintptr(unsafe.Pointer(&attrListBuf[0]))

	r, _, err = procInitializeProcThreadAL.Call(attrListPtr, 1, 0, 0, uintptr(unsafe.Pointer(&attrListSize)))
	if r == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		ptyInW.Close()
		ptyOutR.Close()
		return nil, fmt.Errorf("InitializeProcThreadAttributeList failed: %w", err)
	}

	// Bind the ConPTY handle to the attribute list.
	r, _, err = procUpdateProcThreadAttr.Call(
		attrListPtr,
		0,
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(hPC),
		unsafe.Sizeof(hPC),
		0, 0,
	)
	if r == 0 {
		procDeleteProcThreadAL.Call(attrListPtr)
		procClosePseudoConsole.Call(uintptr(hPC))
		ptyInW.Close()
		ptyOutR.Close()
		return nil, fmt.Errorf("UpdateProcThreadAttribute failed: %w", err)
	}

	// Build command line string.
	cmdLine := opts.Command
	if len(opts.Args) > 0 {
		cmdLine += " " + strings.Join(opts.Args, " ")
	}
	cmdLineUTF16, _ := syscall.UTF16PtrFromString(cmdLine)

	// Build environment block.
	env := os.Environ()
	env = append(env, opts.Env...)
	envBlock := buildEnvBlock(env)

	// Build working directory.
	var cwdPtr *uint16
	if opts.Dir != "" {
		cwdPtr, _ = syscall.UTF16PtrFromString(opts.Dir)
	}

	// Create STARTUPINFOEXW with the attribute list.
	// This is the critical part — exec.Command cannot do this.
	si := startupInfoEx{}
	si.startupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.lpAttributeList = attrListPtr

	var pi syscall.ProcessInformation

	r, _, err = procCreateProcessW.Call(
		0, // lpApplicationName — use command line
		uintptr(unsafe.Pointer(cmdLineUTF16)),
		0, // lpProcessAttributes
		0, // lpThreadAttributes
		0, // bInheritHandles — ConPTY manages the handles
		_EXTENDED_STARTUPINFO_PRESENT|syscall.CREATE_UNICODE_ENVIRONMENT,
		uintptr(unsafe.Pointer(&envBlock[0])),
		uintptr(unsafe.Pointer(cwdPtr)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		procDeleteProcThreadAL.Call(attrListPtr)
		procClosePseudoConsole.Call(uintptr(hPC))
		ptyInW.Close()
		ptyOutR.Close()
		return nil, fmt.Errorf("CreateProcessW failed: %w", err)
	}

	// Clean up attribute list and thread handle — we only need the process handle.
	procDeleteProcThreadAL.Call(attrListPtr)
	syscall.CloseHandle(pi.Thread)

	return &PTY{
		cmd:    nil, // not using exec.Cmd — we manage the process directly
		stdin:  ptyInW,
		stdout: ptyOutR,
		handle: hPC,
		cols:   opts.Cols,
		rows:   opts.Rows,
		procHandle: uintptr(pi.Process),
	}, nil
}

func (p *PTY) resizePlatform(cols, rows int) error {
	hPC, ok := p.handle.(syscall.Handle)
	if !ok {
		return fmt.Errorf("invalid ConPTY handle")
	}
	size := coord{x: int16(cols), y: int16(rows)}
	r, _, err := procResizePseudoConsole.Call(uintptr(hPC), coordToDWORD(size))
	if r != 0 {
		return fmt.Errorf("ResizePseudoConsole failed: HRESULT 0x%x: %w", r, err)
	}
	return nil
}

func (p *PTY) waitPlatform() (int, error) {
	h := syscall.Handle(p.procHandle)
	s, err := syscall.WaitForSingleObject(h, syscall.INFINITE)
	if err != nil {
		return -1, fmt.Errorf("WaitForSingleObject: %w", err)
	}
	if s != syscall.WAIT_OBJECT_0 {
		return -1, fmt.Errorf("unexpected wait result: %d", s)
	}
	var exitCode uint32
	if err := syscall.GetExitCodeProcess(h, &exitCode); err != nil {
		return -1, fmt.Errorf("GetExitCodeProcess: %w", err)
	}
	return int(exitCode), nil
}

func (p *PTY) closePlatform() error {
	if hPC, ok := p.handle.(syscall.Handle); ok {
		procClosePseudoConsole.Call(uintptr(hPC))
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.procHandle != 0 {
		h := syscall.Handle(p.procHandle)
		syscall.TerminateProcess(h, 1)
		syscall.CloseHandle(h)
	}
	return nil
}

// buildEnvBlock creates a Windows environment block (null-separated, double-null-terminated UTF-16).
func buildEnvBlock(env []string) []uint16 {
	var block []uint16
	for _, e := range env {
		block = append(block, utf16.Encode([]rune(e))...)
		block = append(block, 0)
	}
	block = append(block, 0) // double null terminator
	return block
}

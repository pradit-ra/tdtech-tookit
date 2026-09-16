//go:build unix

package gcpx

import "syscall"

func syscallSig0() syscall.Signal {
	return syscall.Signal(0)
}

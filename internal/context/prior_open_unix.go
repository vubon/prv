//go:build unix

package context

import "syscall"

// Opening a FIFO must not block before the file type can be checked.
const priorNonblock = syscall.O_NONBLOCK

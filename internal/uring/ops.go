package uring

import "unsafe"

// io_uring opcodes from linux/io_uring.h.
const (
	OpOpenat = 18
	OpClose  = 19
	OpStatx  = 21
	OpRead   = 22
)

// Constants for openat/statx.
const (
	atFdCwd    = -100   // AT_FDCWD
	atEmptyPath = 0x1000 // AT_EMPTY_PATH
	statxSize  = 0x200   // STATX_SIZE
)

// PrepOpenat sets up an SQE for IORING_OP_OPENAT.
// pathPtr must be a pointer to a null-terminated C string that stays alive
// until the CQE is reaped.
func (sqe *SQE) PrepOpenat(dirfd int32, pathPtr *byte, flags uint32, mode uint32) {
	*sqe = SQE{} // zero out
	sqe.Opcode = OpOpenat
	sqe.Fd = dirfd
	sqe.Addr = uint64(uintptr(unsafe.Pointer(pathPtr)))
	sqe.Len = mode
	sqe.OpcodeFlags = flags
}

// PrepStatx sets up an SQE for IORING_OP_STATX.
// When using AT_EMPTY_PATH, pathPtr should point to an empty C string (""),
// and fd should be the file descriptor to stat.
func (sqe *SQE) PrepStatx(fd int32, pathPtr *byte, statxFlags uint32, mask uint32, buf *Statx) {
	*sqe = SQE{} // zero out
	sqe.Opcode = OpStatx
	sqe.Fd = fd
	sqe.Addr = uint64(uintptr(unsafe.Pointer(pathPtr)))
	sqe.Len = mask
	sqe.Off = uint64(uintptr(unsafe.Pointer(buf)))
	sqe.OpcodeFlags = statxFlags
}

// PrepRead sets up an SQE for IORING_OP_READ.
func (sqe *SQE) PrepRead(fd int32, buf *byte, nbytes uint32, offset uint64) {
	*sqe = SQE{} // zero out
	sqe.Opcode = OpRead
	sqe.Fd = fd
	sqe.Addr = uint64(uintptr(unsafe.Pointer(buf)))
	sqe.Len = nbytes
	sqe.Off = offset
}

// PrepClose sets up an SQE for IORING_OP_CLOSE.
func (sqe *SQE) PrepClose(fd int32) {
	*sqe = SQE{} // zero out
	sqe.Opcode = OpClose
	sqe.Fd = fd
}

// ATFdCwd returns AT_FDCWD for use with PrepOpenat.
func ATFdCwd() int32 { return atFdCwd }

// ATEmptyPath returns AT_EMPTY_PATH for use with PrepStatx.
func ATEmptyPath() uint32 { return atEmptyPath }

// StatxSizeMask returns STATX_SIZE for use with PrepStatx.
func StatxSizeMask() uint32 { return statxSize }

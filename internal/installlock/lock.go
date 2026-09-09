// Package installlock coordinates installers without leaving stale locks after a crash.
package installlock

/*
#include <stdlib.h>
#ifdef _WIN32
#include <windows.h>
#include <stdint.h>
static intptr_t open_lock(const char *path) {
 int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
 if (!n) return -1;
 wchar_t *wide = calloc(n, sizeof(wchar_t)); if (!wide) return -1;
 MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, n);
 HANDLE h = CreateFileW(wide, GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
 free(wide); return (intptr_t)h;
}
static int try_lock(intptr_t h) { OVERLAPPED o = {0}; return LockFileEx((HANDLE)h, LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &o) ? 0 : -1; }
static void close_lock(intptr_t h) { CloseHandle((HANDLE)h); }
#else
#include <stdint.h>
#include <sys/file.h>
#include <fcntl.h>
#include <unistd.h>
static intptr_t open_lock(const char *path) { return open(path, O_RDWR|O_CREAT, 0600); }
static int try_lock(intptr_t h) { return flock((int)h, LOCK_EX|LOCK_NB); }
static void close_lock(intptr_t h) { close((int)h); }
#endif
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

func Acquire(path string) (func(), error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	handle := C.open_lock(p)
	if handle == -1 {
		return nil, fmt.Errorf("oneocr: cannot open install lock %q", path)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for C.try_lock(handle) != 0 {
		if time.Now().After(deadline) {
			C.close_lock(handle)
			return nil, fmt.Errorf("oneocr: another installer is still using %q; retry after it finishes", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return func() { C.close_lock(handle) }, nil
}

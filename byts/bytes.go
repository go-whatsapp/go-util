//go:build go1.20

package byts

import (
	"unsafe"
)

// UnsafeBytes returns a byte pointer without allocation.
func UnsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

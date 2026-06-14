package util

import (
	"encoding/binary"
	"errors"
	"io"
)

var ErrOverflow = errors.New("binary: varint overflows a 64-bit integer")

func ReadUvarint(r io.Reader) (uint64, int, error) {
	var x uint64
	var s uint
	b := make([]byte, 1)
	for i := 0; i < binary.MaxVarintLen64; i++ {
		_, err := r.Read(b)
		if err != nil {
			return x, 0, err
		}
		if b[0] < 0x80 {
			if i == binary.MaxVarintLen64-1 && b[0] > 1 {
				return x, 0, ErrOverflow
			}
			return x | uint64(b[0])<<s, i + 1, nil
		}
		x |= uint64(b[0]&0x7f) << s
		s += 7
	}
	return x, 0, ErrOverflow
}

package sealedexec

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// readControllerReply refuses a line as soon as it reaches the exclusive
// ceiling, including a peer that never sends LF or EOF. ReadSlice retains at
// most one bufio buffer beyond the accumulated prefix.
func readControllerReply(reader *bufio.Reader) ([]byte, error) {
	var frame []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(part) >= OwnerFrameCeiling-len(frame) {
			return nil, fmt.Errorf("controller reply reaches frame ceiling")
		}
		frame = append(frame, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, err
		}
		return frame, nil
	}
}

func decodeControllerFrame(reader io.Reader, target any) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, OwnerFrameCeiling))
	if err != nil {
		return nil, err
	}
	if len(raw) >= OwnerFrameCeiling {
		return nil, fmt.Errorf("controller frame reaches frame ceiling")
	}
	return decodeStrict(bytes.NewReader(raw), target)
}

func boundedControllerFrame(frame []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if len(frame) >= OwnerFrameCeiling {
		return nil, fmt.Errorf("controller frame reaches frame ceiling")
	}
	return frame, nil
}

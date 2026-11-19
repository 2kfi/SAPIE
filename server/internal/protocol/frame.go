package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

// MaxFrameSize caps a single decoded protobuf Frame at 1 MiB.
const MaxFrameSize = 1 << 20

var ErrFrameTooLarge = errors.New("frame exceeds 1 MiB")

func ReadFrame(r io.Reader) (*Frame, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > MaxFrameSize {
		return nil, fmt.Errorf("%w (length %d)", ErrFrameTooLarge, n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var frame Frame
	if err := proto.Unmarshal(buf, &frame); err != nil {
		return nil, fmt.Errorf("decode frame: %w", err)
	}
	return &frame, nil
}

func WriteFrame(w io.Writer, frame *Frame) error {
	buf, err := proto.Marshal(frame)
	if err != nil {
		return fmt.Errorf("encode frame: %w", err)
	}
	if len(buf) > MaxFrameSize {
		return ErrFrameTooLarge
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(buf)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

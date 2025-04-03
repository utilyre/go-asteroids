package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var errShortMessage = errors.New("message not long enough")

func readAckIndex(r io.Reader) (index uint32, err error) {
	data := make([]byte, 4)
	n, err := r.Read(data)
	if err != nil {
		return 0, fmt.Errorf("reading ack: %w", err)
	}
	if l := len(data); n < l {
		return 0, fmt.Errorf("reading ack: %w", errShortMessage)
	}

	n, err = binary.Decode(data, binary.BigEndian, &index)
	if err != nil {
		panic("data should have been big enough")
	}
	if l := len(data); n < l {
		panic("message should have been big enough")
	}

	return index, nil
}

/* func (g *Game) inputBufferFlusher() {
	for {
		index, err := readAckIndex(g.conn)
		if errors.Is(err, net.ErrClosed) {
			slog.Info("connection closed", "remote", g.conn.RemoteAddr())
			return
		}
		if err != nil {
			slog.Warn("failed to read ack index",
				"remote", g.conn.RemoteAddr(), "error", err)
			continue
		}

		err = g.inputBuffer.FlushUntil(index)
		if err != nil {
			slog.Error("failed to flush input buffer",
				"until_index", index, "error", err)
			continue
		}
	}
} */

func writeInputBuffer(w io.Writer, buf *InputBuffer) error {
	bufData, err := buf.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling input buffer: %w", err)
	}
	_, err = w.Write(bufData)
	if err != nil {
		return fmt.Errorf("writing input buffer: %w", err)
	}
	return nil
}

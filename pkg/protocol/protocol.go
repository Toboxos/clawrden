// Package protocol defines the shared types and constants for communication
// between the Clawrden shim (Prisoner-side) and the Warden (Supervisor-side)
// over a Unix Domain Socket.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// DefaultSocketPath is the canonical path for the Warden's Unix Domain Socket.
const DefaultSocketPath = "/var/run/clawrden/warden.sock"

// Stream type markers for the framing protocol.
const (
	StreamStdout byte = 1
	StreamStderr byte = 2
	StreamExit   byte = 3
	StreamCancel byte = 4
)

// Ack bytes sent by the Warden after evaluating a request.
const (
	AckAllowed     byte = 0
	AckDenied      byte = 1
	AckPendingHITL byte = 2
)

// Identity holds the UID/GID of the process that invoked the shim.
type Identity struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

// Request is the JSON payload sent from the Shim to the Warden.
type Request struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
	Env     []string `json:"env"`
	Identity Identity `json:"identity"`
}

// Frame represents a single chunk of streamed output or control data.
type Frame struct {
	Type    byte   // StreamStdout, StreamStderr, StreamExit, or StreamCancel
	Payload []byte // For StreamExit, payload is a single byte (exit code)
}

// WriteRequest serializes a Request as a length-prefixed JSON message.
// Wire format: [4-byte big-endian length][JSON payload]
func WriteRequest(w io.Writer, req *Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Write 4-byte length header
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write JSON payload
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadRequest reads a length-prefixed JSON Request from the reader.
func ReadRequest(r io.Reader) (*Request, error) {
	// Read 4-byte length header
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	// Sanity check: reject absurdly large payloads (> 10MB)
	if length > 10*1024*1024 {
		return nil, fmt.Errorf("request too large: %d bytes", length)
	}

	// Read JSON payload
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	return &req, nil
}

// WriteFrame writes a single frame to the writer.
// Wire format: [1-byte type][4-byte big-endian length][payload]
func WriteFrame(w io.Writer, f Frame) error {
	// Write stream type
	if _, err := w.Write([]byte{f.Type}); err != nil {
		return fmt.Errorf("write frame type: %w", err)
	}

	// Write 4-byte payload length
	length := uint32(len(f.Payload))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write frame length: %w", err)
	}

	// Write payload
	if len(f.Payload) > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return fmt.Errorf("write frame payload: %w", err)
		}
	}

	return nil
}

// ReadFrame reads a single frame from the reader.
func ReadFrame(r io.Reader) (Frame, error) {
	var f Frame

	// Read 1-byte stream type
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, typeBuf); err != nil {
		return f, fmt.Errorf("read frame type: %w", err)
	}
	f.Type = typeBuf[0]

	// Read 4-byte payload length
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return f, fmt.Errorf("read frame length: %w", err)
	}

	// Sanity check
	if length > 10*1024*1024 {
		return f, fmt.Errorf("frame too large: %d bytes", length)
	}

	// Read payload
	if length > 0 {
		f.Payload = make([]byte, length)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return f, fmt.Errorf("read frame payload: %w", err)
		}
	}

	return f, nil
}

// WriteAck sends a single ack byte to the writer.
func WriteAck(w io.Writer, ack byte) error {
	_, err := w.Write([]byte{ack})
	return err
}

// ReadAck reads a single ack byte from the reader.
func ReadAck(r io.Reader) (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(r, buf)
	return buf[0], err
}

// WriteExitCode sends an exit code frame.
func WriteExitCode(w io.Writer, code int) error {
	return WriteFrame(w, Frame{
		Type:    StreamExit,
		Payload: []byte{byte(code)},
	})
}

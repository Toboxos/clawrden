package warden

import (
	"fmt"
	"net"
	"syscall"
)

// PeerCredentials holds the kernel-enforced identity of a Unix socket peer.
// Extracted via SO_PEERCRED — the prisoner cannot fake these values.
type PeerCredentials struct {
	PID         int32
	UID         uint32
	GID         uint32
	ContainerID string // resolved via cgroup (empty if host process)
}

// extractPeerCreds retrieves the peer credentials from a Unix domain socket connection.
// Uses SO_PEERCRED which is kernel-enforced and cannot be spoofed.
func extractPeerCreds(conn net.Conn) (*PeerCredentials, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a Unix socket")
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("get raw connection: %w", err)
	}

	var cred *syscall.Ucred
	var credErr error

	err = raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, fmt.Errorf("raw control: %w", err)
	}
	if credErr != nil {
		return nil, fmt.Errorf("getsockopt SO_PEERCRED: %w", credErr)
	}

	return &PeerCredentials{
		PID: cred.Pid,
		UID: cred.Uid,
		GID: cred.Gid,
	}, nil
}

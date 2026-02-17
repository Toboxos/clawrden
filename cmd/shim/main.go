// Command shim is the universal Clawrden shim binary.
// It determines which tool it's impersonating via os.Args[0],
// captures the execution context, and forwards it to the Warden
// over a Unix Domain Socket.
package main

import (
	"clawrden/internal/shim"
	"os"
)

func main() {
	os.Exit(shim.Run())
}

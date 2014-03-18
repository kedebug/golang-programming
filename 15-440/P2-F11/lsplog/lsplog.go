// Useful support functions in LSP implementation

package lsplog

import (
	"log"
	"strings"
)

// The Vlog function provides a means to include print statements
// that are activated according to different levels of verbosity.
// That way, you can set the verbosity level to 0 to eliminate any
// logging, but set to higher levels for debugging.

var verbosity int = 0

func SetVerbose(level int) {
	verbosity = level
}

// NOTE: Here are the levels of verbosity we built into our LSP implementation
// General Rules for Verbosity
// 0: Silent
// 1: New connections by clients and servers
// 2: Application-level jobs
// 3: Low-level tasks
// 4: Data communications
// 5: All communications
// 6: Everything

// Log result if verbosity level high enough
func Vlogf(level int, format string, v ...interface{}) {
	if level <= verbosity {
		log.Printf(format, v...)
	}
}

// Handle errors
func CheckReport(level int, err error) bool {
	if err == nil {
		return false
	}
	Vlogf(level, "Error: %s", err.Error())
	return true
}

func CheckFatal(err error) {
	if err == nil {
		return
	}
	log.Fatalf("Fatal: %s", err.Error())
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
// Error handling

type LspErr struct {
	msg string
}

func (e LspErr) Error() string {
	return e.msg
}

func MakeErr(msg string) LspErr {
	return LspErr{msg}
}

// Common error types
func NotImplemented(name string) LspErr {
	return MakeErr("Not implemented: " + name)
}

func ConnectionClosed() LspErr {
	return MakeErr("Connection closed")
}

func ErrClosed(err error) bool {
	return err != nil && strings.EqualFold(err.Error(), "Connection closed")
}

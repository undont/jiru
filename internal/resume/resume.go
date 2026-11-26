// Package resume reports jiru's resume command to the terminal over OSC 7577,
// so terminals supporting the protocol can restore the process.
// https://www.superlogical.com/rex/docs/build/program-resume
package resume

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func Report(osc7577 string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("no args passed to resume report")
	}

	return emit(osc7577, "argv="+encodeArgv(argv), os.Stdout)
}

// Clear is for a user quit only, never a hangup or kill: the terminal ending
// is when the record is needed.
func Clear(osc7577 string) error {
	return emit(osc7577, "clear=1", os.Stdout)
}

func emit(osc7577, fields string, w io.Writer) error {
	return send(os.Getpid(), time.Now().Unix(), osc7577, fields, w)
}

func send(pid int, reportTime int64, osc7577, fields string, w io.Writer) error {
	prefix := fmt.Sprintf("pid=%d:time=%d:", pid, reportTime)
	body := prefix + fields

	checksum := sha256.Sum256([]byte(body + ":" + osc7577))
	mac := hex.EncodeToString(checksum[:])

	_, err := fmt.Fprintf(w, "\x1b]7577;%s:mac=%s\x1b\\", body, mac)
	return err
}

func encodeArgv(argv []string) string {
	encoded := make([]string, len(argv))
	for i, arg := range argv {
		encoded[i] = base64.StdEncoding.EncodeToString([]byte(arg))
	}
	return strings.Join(encoded, ",")
}

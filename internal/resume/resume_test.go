package resume

import (
	"bytes"
	"testing"
)

// the wrapper example from the protocol spec, which uses its dummy token
const (
	specToken  = "dummy-token-replace-me"
	specFields = "argv=cmVzdW1lLXdyYXA=,c3No,ZGV2Ym94"
	specMAC    = "e76149d0ba54a83570ffab0583867dacdfa23928ba9d18604f0badea88b62b65"
)

func TestSendMatchesSpecExample(t *testing.T) {
	var buf bytes.Buffer
	if err := send(90412, 1795543200, specToken, specFields, &buf); err != nil {
		t.Fatal(err)
	}

	want := "\x1b]7577;pid=90412:time=1795543200:" + specFields + ":mac=" + specMAC + "\x1b\\"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEncodeArgvMatchesSpecExample(t *testing.T) {
	got := "argv=" + encodeArgv([]string{"resume-wrap", "ssh", "devbox"})
	if got != specFields {
		t.Errorf("got %q, want %q", got, specFields)
	}
}

func TestReportRejectsEmptyArgv(t *testing.T) {
	if err := Report(specToken, nil); err == nil {
		t.Error("want an error for an empty argv")
	}
}

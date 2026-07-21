package clierror_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

func TestExitCodes(t *testing.T) {
	cases := []struct {
		code clierror.ErrorCode
		want int
	}{
		{clierror.CodeOK, 0},
		{clierror.CodeNotFound, 1},
		{clierror.CodeUnauthorized, 2},
		{clierror.CodeForbidden, 3},
		{clierror.CodeBadRequest, 4},
		{clierror.CodeConflict, 5},
		{clierror.CodeServerError, 6},
		{clierror.CodeNotReady, 7},
		{clierror.CodeUsage, 8},
	}
	for _, c := range cases {
		e := clierror.New(c.code, "msg", "")
		if got := e.ExitCode(); got != c.want {
			t.Errorf("code %d: ExitCode() = %d, want %d", c.code, got, c.want)
		}
	}
}

func TestRender(t *testing.T) {
	var buf bytes.Buffer
	e := clierror.New(clierror.CodeNotFound, "task 999 not found", "List tasks with 'loopctl tasks list --project 9'.")
	e.Render(&buf)
	got := buf.String()
	want := "Error: task 999 not found List tasks with 'loopctl tasks list --project 9'.\n"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRenderNoHint(t *testing.T) {
	var buf bytes.Buffer
	e := clierror.New(clierror.CodeServerError, "internal error", "")
	e.Render(&buf)
	got := buf.String()
	want := "Error: internal error\n"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestHandleErrNil(t *testing.T) {
	var buf bytes.Buffer
	if got := clierror.HandleErr(nil, &buf); got != 0 {
		t.Errorf("HandleErr(nil) = %d, want 0", got)
	}
}

func TestHandleErrCLIError(t *testing.T) {
	var buf bytes.Buffer
	e := clierror.New(clierror.CodeNotFound, "not found", "")
	if got := clierror.HandleErr(e, &buf); got != 1 {
		t.Errorf("HandleErr(CLIError) = %d, want 1", got)
	}
}

func TestHandleErrPlain(t *testing.T) {
	var buf bytes.Buffer
	if got := clierror.HandleErr(errors.New("plain error"), &buf); got != 6 {
		t.Errorf("HandleErr(plain) = %d, want 6", got)
	}
}

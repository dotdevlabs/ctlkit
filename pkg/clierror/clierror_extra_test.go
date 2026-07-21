package clierror_test

import (
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

func TestErrorMethod(t *testing.T) {
	e := clierror.New(clierror.CodeNotFound, "task 1 not found", "List tasks with 'loopctl tasks list'.")
	got := e.Error()
	if !strings.Contains(got, "task 1 not found") {
		t.Errorf("Error() = %q", got)
	}
	if !strings.Contains(got, "List tasks") {
		t.Errorf("Error() missing hint: %q", got)
	}
}

func TestErrorMethodNoHint(t *testing.T) {
	e := clierror.New(clierror.CodeServerError, "internal error", "")
	if e.Error() != "internal error" {
		t.Errorf("Error() = %q", e.Error())
	}
}

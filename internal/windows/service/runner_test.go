package service

import "testing"

func TestStubRunnerCompilesOnNonWindows(t *testing.T) {
	t.Parallel()

	runner := Runner{}
	if err := runner.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}

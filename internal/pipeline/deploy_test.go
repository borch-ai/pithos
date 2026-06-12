package pipeline

import (
	"context"
	"testing"
)

func TestDeployStub(t *testing.T) {
	opts := DeployOptions{
		InputDir: "test",
	}
	err := Deploy(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error from Deploy stub, got nil")
	}
	if err.Error() != "deploy engine is not implemented yet" {
		t.Fatalf("expected 'deploy engine is not implemented yet', got %q", err.Error())
	}
}

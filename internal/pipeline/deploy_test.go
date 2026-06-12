package pipeline

import (
	"context"
	"errors"
	"testing"
)

func TestDeployStub(t *testing.T) {
	opts := DeployOptions{
		InputDir: "test",
	}
	err := Deploy(context.Background(), opts)
	if !errors.Is(err, ErrDeployNotImplemented) {
		t.Fatalf("expected ErrDeployNotImplemented, got %v", err)
	}
}

func TestDeployValidation(t *testing.T) {
	opts := DeployOptions{
		InputDir: "",
	}
	err := Deploy(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "input directory is required" {
		t.Fatalf("expected 'input directory is required', got %q", err.Error())
	}
}

package placeholder

import "testing"

func TestPing(t *testing.T) {
	result := Ping()
	if result != "pong" {
		t.Errorf("expected 'pong', got '%s'", result)
	}
}

package validate

import "testing"

func TestGetReturnsSingleton(t *testing.T) {
	t.Parallel()

	a := Get()
	b := Get()

	if a == nil || b == nil {
		t.Fatal("Get() returned nil validator")
	}
	if a != b {
		t.Fatal("Get() should return singleton instance")
	}
}

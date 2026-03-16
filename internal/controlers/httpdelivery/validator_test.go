package httpdelivery

import (
	"net/http"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type validatorSample struct {
	Name string `validate:"required"`
}

func TestEchoValidatorValidateSuccess(t *testing.T) {
	t.Parallel()

	v := NewEchoValidator(validator.New())
	if err := v.Validate(validatorSample{Name: "ok"}); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestEchoValidatorValidateFailure(t *testing.T) {
	t.Parallel()

	v := NewEchoValidator(validator.New())
	err := v.Validate(validatorSample{})
	if err == nil {
		t.Fatal("Validate() error = nil, want HTTP error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

package httpdelivery

import (
    "net/http"
    "github.com/go-playground/validator/v10"
    "github.com/labstack/echo/v4"
)

type EchoValidator struct {
    validator *validator.Validate
}

func NewEchoValidator(v *validator.Validate) *EchoValidator {
    return &EchoValidator{validator: v}
}

func (ev *EchoValidator) Validate(i interface{}) error {
    if err := ev.validator.Struct(i); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }
    return nil
}

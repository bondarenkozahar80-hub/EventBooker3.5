package validator

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/go-playground/validator"
)

var (
	global   *validator.Validate
	tagRegex = regexp.MustCompile(`^#[a-z0-9_\-]+$`)
)

const (
	ErrInvalidFormat      = "Invalid format"
	ErrFieldRequired      = "Field is required"
	ErrFieldExceedsMaxLen = "Field exceeds maximum length"
	ErrFieldBelowMinLen   = "Field is below minimum length"
	ErrFieldExceedsMaxVal = "Field exceeds maximum value"
	ErrFieldBelowMinVal   = "Field is below minimum value"
	ErrUnknownValidation  = "Unknown validation error"
)

func init() {
	SetValidator(New())
}

func New() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("tag", validateTag)
	_ = v.RegisterValidation("future", validateFutureDate)
	_ = v.RegisterValidation("positive", validatePositiveInt)
	return v
}

func SetValidator(v *validator.Validate) {
	global = v
}

func Validator() *validator.Validate {
	return global
}

func validateTag(fl validator.FieldLevel) bool {
	return tagRegex.MatchString(fl.Field().String())
}

func validateFutureDate(fl validator.FieldLevel) bool {
	t, ok := fl.Field().Interface().(time.Time)
	return ok && t.After(time.Now())
}

func validatePositiveInt(fl validator.FieldLevel) bool {
	val, ok := fl.Field().Interface().(int)
	return ok && val > 0
}

func Validate(ctx context.Context, structure any) error {
	return parseValidationErrors(Validator().StructCtx(ctx, structure))
}

func parseValidationErrors(err error) error {
	if err == nil {
		return nil
	}
	vErrors, ok := err.(validator.ValidationErrors)
	if !ok || len(vErrors) == 0 {
		return nil
	}
	ve := vErrors[0]
	var msg string
	switch ve.Tag() {
	case "tag":
		msg = ErrInvalidFormat
	case "required":
		msg = ErrFieldRequired
	case "max":
		msg = ErrFieldExceedsMaxLen
	case "min":
		msg = ErrFieldBelowMinLen
	case "lt", "lte":
		msg = ErrFieldExceedsMaxVal
	case "gt", "gte":
		msg = ErrFieldBelowMinVal
	case "future":
		msg = "Date must be in the future"
	case "positive":
		msg = "Value must be positive"
	default:
		msg = ErrUnknownValidation
	}
	return errors.New(msg + ": " + ve.Namespace())
}

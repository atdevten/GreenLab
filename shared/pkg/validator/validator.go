package validator

import (
	"reflect"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	instance *validator.Validate
)

// Get returns the singleton validator instance.
func Get() *validator.Validate {
	once.Do(func() {
		instance = validator.New()
		registerCustomValidators(instance)
	})
	return instance
}

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is a slice of ValidationError.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	msg := "validation failed: "
	for i, e := range ve {
		if i > 0 {
			msg += "; "
		}
		msg += e.Field + " " + e.Message
	}
	return msg
}

// Validate validates the given struct and returns structured errors.
func Validate(s interface{}) error {
	err := Get().Struct(s)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	result := make(ValidationErrors, 0, len(validationErrors))
	for _, fe := range validationErrors {
		result = append(result, ValidationError{
			Field:   fe.Field(),
			Message: tagToMessage(fe),
		})
	}
	return result
}

// tagToMessage converts a validator.FieldError to a human-readable message.
func tagToMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		switch fe.Kind() {
		case reflect.Map, reflect.Slice, reflect.Array:
			return "must have at least " + fe.Param() + " item(s)"
		default:
			return "must be at least " + fe.Param() + " characters"
		}
	case "max":
		switch fe.Kind() {
		case reflect.Map, reflect.Slice, reflect.Array:
			return "must have at most " + fe.Param() + " item(s)"
		default:
			return "must be at most " + fe.Param() + " characters"
		}
	case "len":
		return "must be exactly " + fe.Param() + " characters"
	case "uuid":
		return "must be a valid UUID"
	case "url":
		return "must be a valid URL"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "numeric":
		return "must be numeric"
	case "alphanum":
		return "must be alphanumeric"
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "lte":
		return "must be less than or equal to " + fe.Param()
	case "gt":
		return "must be greater than " + fe.Param()
	case "lt":
		return "must be less than " + fe.Param()
	default:
		return "failed validation: " + fe.Tag()
	}
}

func registerCustomValidators(v *validator.Validate) {
	// Register any custom validators here.
	_ = v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if len(s) == 0 {
			return false
		}
		for _, c := range s {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
				return false
			}
		}
		return true
	})
}

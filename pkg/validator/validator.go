package validator

import (
	"reflect"
	"strings"

	"github.com/Amanyd/backend/pkg/apierr"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

var validRanks = map[string]bool{
	"officer":              true,
	"agniveer":             true,
	"sub_lieutenant":       true,
	"lieutenant":           true,
	"lieutenant_commander": true,
	"commander":            true,
	"captain":              true,
	"commodore":            true,
	"rear_admiral":         true,
	"vice_admiral":         true,
	"admiral":              true,
	// Sailor ranks, junior to senior.
	"seaman_ii":                               true,
	"seaman_i":                                true,
	"leading_seaman_/_leading_rate":           true,
	"petty_officer_(po)":                      true,
	"chief_petty_officer_(cpo)":               true,
	"master_chief_petty_officer_ii_(mcpo_ii)": true,
	"master_chief_petty_officer_i_(mcpo_i)":   true,
}

func init() {
	validate = validator.New()
	validate.RegisterValidation("rank", func(fl validator.FieldLevel) bool {
		val := strings.ToLower(strings.TrimSpace(fl.Field().String()))
		val = strings.ReplaceAll(val, " ", "_")
		val = strings.ReplaceAll(val, "-", "_")
		return validRanks[val] || validRanks[fl.Field().String()]
	})

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

func ValidateStruct(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	valErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return apierr.BadRequest("invalid input")
	}

	msgs := make([]string, 0, len(valErrs))
	for _, fe := range valErrs {
		field := strings.ToLower(fe.Field())
		var m string
		switch fe.Tag() {
		case "required":
			m = field + " is required"
		case "min":
			m = field + " must be at least " + fe.Param()
		case "oneof":
			m = field + " must be one of " + fe.Param()
		case "rank":
			m = field + " is not a valid rank"
		default:
			m = field + " is invalid"
		}
		msgs = append(msgs, m)
	}

	return apierr.BadRequest(strings.Join(msgs, "; "))
}

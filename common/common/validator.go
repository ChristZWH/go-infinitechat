package common

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 黑盒子直接用，先跳过，后面记得补上
var validate = validator.New()

func init() {
	httpx.SetValidator(&customValidator{})
}

type customValidator struct{}

func parseValidationMsg(msg, tag string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	if strings.Contains(msg, ";") {
		parts := strings.Split(msg, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				if key == tag || key == "*" {
					return val
				}
				continue
			}
			if kv := strings.SplitN(part, ":", 2); len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				if key == tag || key == "*" {
					return val
				}
				continue
			}
		}
		return msg
	}

	if kv := strings.SplitN(msg, "=", 2); len(kv) == 2 {
		return strings.TrimSpace(kv[1])
	}
	if kv := strings.SplitN(msg, ":", 2); len(kv) == 2 {
		return strings.TrimSpace(kv[1])
	}

	return msg
}

func (c *customValidator) Validate(r *http.Request, data any) error {
	err := validate.Struct(data)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return SystemError
	}

	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for _, e := range validationErrors {
		field, ok := t.FieldByName(e.Field())
		if !ok {
			continue
		}

		msg := parseValidationMsg(field.Tag.Get("msg"), e.Tag())
		if msg != "" {
			return ErrorCode{Code: ParamsError.Code, Message: msg}
		}
	}

	return ParamsError
}

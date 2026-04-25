package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/unrolled/render"
)

type ErrorBody struct {
	Error ErrorDetails `json:"error"`
}

type ErrorDetails struct {
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type HTTPError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

type APIEnvelope struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"errorMsg"`
	Data     any    `json:"data,omitempty"`
}

var rnr = render.New(render.Options{
	StreamingJSON: true,
	UnEscapeHTML:  true,
})

func (e HTTPError) Error() string { return e.Message }

func JSON(w http.ResponseWriter, status int, v any) {
	if err := rnr.JSON(w, status, APIEnvelope{
		Success:  true,
		ErrorMsg: "",
		Data:     v,
	}); err != nil {
		log.Printf("Cannot make HTTP response back in JSON: %v\n", err)
	}
}

func NoContent(w http.ResponseWriter) {
	if err := rnr.JSON(w, http.StatusNoContent, APIEnvelope{
		Success:  true,
		ErrorMsg: "",
		Data:     nil,
	}); err != nil {
		log.Printf("Cannot make HTTP response back in JSON: %v\n", err)
	}
}

func Error(w http.ResponseWriter, err error) {
	var he HTTPError
	if errors.As(err, &he) {
		if err := rnr.JSON(w, he.Status, APIEnvelope{
			Success:  false,
			ErrorMsg: he.Message,
			Data:     nil,
		}); err != nil {
			log.Printf("Cannot make HTTP response back in Error: %v\n", err)
		}
		return
	}

	if err := rnr.JSON(w, http.StatusInternalServerError, APIEnvelope{
		Success:  false,
		ErrorMsg: err.Error(),
		Data:     nil,
	}); err != nil {
		log.Printf("Cannot make HTTP response back in Error: %v\n", err)
	}
}

func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return HTTPError{Status: http.StatusBadRequest, Code: "bad_json", Message: err.Error()}
	}
	return nil
}

func BadRequest(message string) HTTPError {
	return HTTPError{Status: http.StatusBadRequest, Code: "bad_request", Message: message}
}
func Unauthorized(message string) HTTPError {
	return HTTPError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: message}
}
func Forbidden(message string) HTTPError {
	return HTTPError{Status: http.StatusForbidden, Code: "forbidden", Message: message}
}
func NotFound(message string) HTTPError {
	return HTTPError{Status: http.StatusNotFound, Code: "not_found", Message: message}
}
func Conflict(message string) HTTPError {
	return HTTPError{Status: http.StatusConflict, Code: "conflict", Message: message}
}

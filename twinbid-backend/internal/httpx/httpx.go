package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
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

func (e HTTPError) Error() string { return e.Message }

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Error(w http.ResponseWriter, err error) {
	var he HTTPError
	if errors.As(err, &he) {
		JSON(w, he.Status, ErrorBody{Error: ErrorDetails{Code: he.Code, Message: he.Message, Fields: he.Fields}})
		return
	}
	JSON(w, http.StatusInternalServerError, ErrorBody{Error: ErrorDetails{Code: "internal_error", Message: err.Error()}})
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

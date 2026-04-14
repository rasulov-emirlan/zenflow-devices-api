package httpx

import "net/http"

type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{Error: code, Message: message})
}

package servutil

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/diamondburned/arikawa/v3/api/webhook"
)

// WriteErr writes the error as JSON.
func WriteErr(w http.ResponseWriter, _ *http.Request, code int, err error) {
	var errBody struct {
		Error string `json:"error"`
	}

	if err != nil {
		errBody.Error = err.Error()
		log.Println("request", code, "error:", err)
	} else {
		errBody.Error = http.StatusText(code)
		log.Println("request", code, "error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errBody)
}

// NewInteractionServer creates a new webhook interaction server with an
// injected ErrorFunc.
func NewInteractionServer(pubkey string, handler webhook.InteractionHandler) (*webhook.InteractionServer, error) {
	s, err := webhook.NewInteractionServer(pubkey, handler)
	if err != nil {
		return nil, err
	}
	s.ErrorFunc = WriteErr
	return s, nil
}

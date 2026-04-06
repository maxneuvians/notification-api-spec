package services

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestInvalidRequestErrorSerializesToAdminErrorShape(t *testing.T) {
	err := InvalidRequestError{Message: "You must have at least one SMS sender as the default.", StatusCode: http.StatusBadRequest}
	if err.Error() != err.Message {
		t.Fatalf("Error() = %q, want %q", err.Error(), err.Message)
	}
	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusBadRequest)
	}

	body, marshalErr := json.Marshal(err.Body())
	if marshalErr != nil {
		t.Fatalf("Marshal() error = %v", marshalErr)
	}
	if string(body) != `{"message":"You must have at least one SMS sender as the default.","result":"error"}` {
		t.Fatalf("body = %s", body)
	}
}

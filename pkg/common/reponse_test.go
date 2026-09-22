package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJsonRespWritesSuccessfulString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)

	if err := JsonResp(context, http.StatusOK, "mounted"); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got, want := response.Body.String(), `{"message":"mounted"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

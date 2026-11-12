package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type existenceQuery struct {
	exists bool
	err    error
	id     string
}

func (q *existenceQuery) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	q.id = args[0].(string)
	return q
}

func (q *existenceQuery) Scan(dest ...any) error {
	*dest[0].(*bool) = q.exists
	return q.err
}

func TestMediaExistenceResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		exists bool
		err    error
		status int
	}{
		{"existing", true, nil, http.StatusOK},
		{"missing", false, nil, http.StatusNotFound},
		{"database unavailable", false, errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			query := &existenceQuery{exists: test.exists, err: test.err}
			router := gin.New()
			router.HEAD("/api/media/:id", headMediaHandler(query))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/api/media/AFD9FE2E29BCFAC47456C5DB865D13EF", nil))
			if response.Code != test.status || response.Body.Len() != 0 || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d body=%q headers=%v", response.Code, response.Body, response.Header())
			}
			if query.id != "afd9fe2e29bcfac47456c5db865d13ef" {
				t.Fatalf("noncanonical query ID: %s", query.id)
			}
		})
	}
}

func TestMediaExistenceRejectsInvalidIDs(t *testing.T) {
	router := gin.New()
	router.HEAD("/api/media/:id", headMediaHandler(nil))
	for _, id := range []string{"short", strings.Repeat("g", 32), strings.Repeat("a", 33)} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/api/media/"+id, nil))
		if response.Code != http.StatusBadRequest || response.Body.Len() != 0 {
			t.Fatalf("id %q: %d %s", id, response.Code, response.Body)
		}
	}
}

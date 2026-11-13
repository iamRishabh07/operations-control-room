// Package handler is the Vercel Function behind /api/recommendation. Vercel
// builds each Go file under /api into one function; the work is done by the
// same service the local server runs (see backend/serverless).
package handler

import (
	"net/http"

	"github.com/iamRishabh07/operations-control-room/backend/serverless"
)

// Handler answers GET /api/recommendation.
func Handler(w http.ResponseWriter, r *http.Request) {
	serverless.Handler(w, r)
}

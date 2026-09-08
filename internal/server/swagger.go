//go:build !crush_omit_swagger

package server

import (
	"net/http"

	_ "github.com/charmbracelet/crush/internal/swagger"
	httpswagger "github.com/swaggo/http-swagger/v2"
)

func handleSwagger(mux *http.ServeMux) {
	mux.Handle("/v1/docs/", httpswagger.WrapHandler)
}

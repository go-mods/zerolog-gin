package zerologgin_test

import (
	"bytes"
	"github.com/gin-gonic/gin"
	zerologgin "github.com/go-mods/zerolog-gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type header struct {
	Key   string
	Value string
}

func performRequest(r http.Handler, method, path string, headers ...header) *httptest.ResponseRecorder {
	var body io.Reader
	if method == "POST" {
		body = strings.NewReader("{\"test\":\"test\"}")
	}
	req := httptest.NewRequest(method, path, body)
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLogger(t *testing.T) {

	buffer := new(bytes.Buffer)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: buffer, NoColor: true}).With().Timestamp().Logger()

	r := gin.New()
	r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Name: "Test", Logger: &logger}))
	r.GET("/example", func(c *gin.Context) {})
	r.POST("/example", func(c *gin.Context) {
		c.String(http.StatusBadRequest, "ok")
	})
	r.PUT("/example", func(c *gin.Context) {
		c.String(http.StatusBadGateway, "ok")
	})
	r.DELETE("/example", func(c *gin.Context) {})
	r.PATCH("/example", func(c *gin.Context) {})
	r.HEAD("/example", func(c *gin.Context) {})
	r.OPTIONS("/example", func(c *gin.Context) {})

	performRequest(r, "GET", "/example?a=100")
	assert.Contains(t, buffer.String(), "200")
	assert.Contains(t, buffer.String(), "GET")
	assert.Contains(t, buffer.String(), "/example")

	buffer.Reset()
	performRequest(r, "POST", "/example?a=100")
	assert.Contains(t, buffer.String(), "400")
	assert.Contains(t, buffer.String(), "POST")
	assert.Contains(t, buffer.String(), "/example")
	assert.Contains(t, buffer.String(), "WRN")

	buffer.Reset()
	performRequest(r, "PUT", "/example?a=100")
	assert.Contains(t, buffer.String(), "502")
	assert.Contains(t, buffer.String(), "PUT")
	assert.Contains(t, buffer.String(), "/example")
	assert.Contains(t, buffer.String(), "ERR")
}

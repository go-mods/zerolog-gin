package zerologgin_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zerologgin "github.com/go-mods/zerolog-gin"
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

// countingHook counts the events going through a logger.
type countingHook struct {
	count *int
}

func (h countingHook) Run(*zerolog.Event, zerolog.Level, string) {
	*h.count++
}

func TestLoggerEmitsTimestampOnce(t *testing.T) {
	tests := []struct {
		name  string
		build func(w io.Writer) zerolog.Logger
		opts  zerologgin.Options
		want  int
	}{
		{
			name: "logger with timestamp",
			build: func(w io.Writer) zerolog.Logger {
				return zerolog.New(w).With().Timestamp().Logger()
			},
			want: 1,
		},
		{
			name: "logger without timestamp",
			build: func(w io.Writer) zerolog.Logger {
				return zerolog.New(w)
			},
			want: 1,
		},
		{
			name: "logger with timestamp and a sampler",
			build: func(w io.Writer) zerolog.Logger {
				return zerolog.New(w).With().Timestamp().Logger().Sample(&zerolog.BasicSampler{N: 2})
			},
			want: 1,
		},
		{
			name: "field excluded, logger without timestamp",
			build: func(w io.Writer) zerolog.Logger {
				return zerolog.New(w)
			},
			opts: zerologgin.Options{FieldsExclude: []string{zerologgin.TimestampFieldName}},
			want: 0,
		},
		{
			name: "field excluded, logger with timestamp",
			build: func(w io.Writer) zerolog.Logger {
				return zerolog.New(w).With().Timestamp().Logger()
			},
			opts: zerologgin.Options{FieldsExclude: []string{zerologgin.TimestampFieldName}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := new(bytes.Buffer)
			logger := tt.build(buffer)
			opts := tt.opts
			opts.Logger = &logger

			r := gin.New()
			r.Use(zerologgin.LoggerWithOptions(&opts))
			r.GET("/example", func(c *gin.Context) {})

			performRequest(r, "GET", "/example")

			assert.Equal(t, tt.want, strings.Count(buffer.String(), `"time":`), buffer.String())
		})
	}
}

func TestLoggerEmitsTimestampOnceWithLoggerConfiguredLater(t *testing.T) {
	buffer := new(bytes.Buffer)
	var logger zerolog.Logger

	r := gin.New()
	r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Logger: &logger}))
	r.GET("/example", func(c *gin.Context) {})

	logger = zerolog.New(buffer).With().Timestamp().Logger()
	performRequest(r, "GET", "/example")

	assert.Equal(t, 1, strings.Count(buffer.String(), `"time":`), buffer.String())
}

func TestLoggerRunsHooksOnceMoreOnlyOnFirstRequest(t *testing.T) {
	var count int
	logger := zerolog.New(io.Discard).Hook(countingHook{count: &count})

	r := gin.New()
	r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Logger: &logger}))
	r.GET("/example", func(c *gin.Context) {})

	for range 3 {
		performRequest(r, "GET", "/example")
	}

	assert.Equal(t, 4, count)
}

func TestLoggerWithOptionsLeavesOptionsUntouched(t *testing.T) {
	t.Run("defaults are not written back", func(t *testing.T) {
		opts := &zerologgin.Options{Name: "Test"}

		zerologgin.LoggerWithOptions(opts)

		assert.Equal(t, &zerologgin.Options{Name: "Test"}, opts)
	})

	t.Run("later changes have no effect", func(t *testing.T) {
		buffer := new(bytes.Buffer)
		logger := zerolog.New(buffer)
		order := []string{zerologgin.MethodFieldName, zerologgin.StatusCodeFieldName}
		exclude := []string{zerologgin.StatusCodeFieldName}
		opts := &zerologgin.Options{Name: "Test", Logger: &logger, FieldsOrder: order, FieldsExclude: exclude}

		r := gin.New()
		r.Use(zerologgin.LoggerWithOptions(opts))
		r.GET("/example", func(c *gin.Context) {})

		order[0] = zerologgin.PathFieldName
		exclude[0] = zerologgin.MethodFieldName
		opts.Logger = nil
		performRequest(r, "GET", "/example")

		assert.Contains(t, buffer.String(), `"method":"GET"`)
		assert.NotContains(t, buffer.String(), `"path"`)
		assert.NotContains(t, buffer.String(), `"status_code"`)
	})
}

func TestLoggerKeepsRequestBodyReadable(t *testing.T) {
	errBroken := errors.New("broken body")

	tests := []struct {
		name        string
		body        io.Reader
		wantBody    string
		wantErr     error
		wantPayload string
	}{
		{
			name:        "complete body",
			body:        strings.NewReader("complete"),
			wantBody:    "complete",
			wantPayload: "complete",
		},
		{
			name:        "interrupted body",
			body:        io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errBroken)),
			wantBody:    "partial",
			wantErr:     errBroken,
			wantPayload: "partial",
		},
		{
			name:    "body failing at once",
			body:    iotest.ErrReader(errBroken),
			wantErr: errBroken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := new(bytes.Buffer)
			logger := zerolog.New(buffer)

			var gotBody []byte
			var gotErr error
			r := gin.New()
			r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Logger: &logger}))
			r.POST("/example", func(c *gin.Context) {
				gotBody, gotErr = io.ReadAll(c.Request.Body)
				c.Status(http.StatusNoContent)
			})

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/example", tt.body))

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Equal(t, tt.wantBody, string(gotBody))
			assert.ErrorIs(t, gotErr, tt.wantErr)

			var line map[string]any
			require.NoError(t, json.Unmarshal(buffer.Bytes(), &line), buffer.String())
			if tt.wantErr == nil {
				assert.NotContains(t, line, zerolog.ErrorFieldName)
			} else {
				assert.Equal(t, tt.wantErr.Error(), line[zerolog.ErrorFieldName])
			}
			if tt.wantPayload == "" {
				assert.NotContains(t, line, zerologgin.PayloadFieldName)
			} else {
				assert.Equal(t, tt.wantPayload, line[zerologgin.PayloadFieldName])
			}
		})
	}
}

func TestStatusCodeFieldName(t *testing.T) {
	buffer := new(bytes.Buffer)
	logger := zerolog.New(buffer)

	r := gin.New()
	r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Logger: &logger}))
	r.GET("/example", func(c *gin.Context) {})

	performRequest(r, "GET", "/example")

	assert.Equal(t, "status_code", zerologgin.StatusCodeFieldName)
	assert.Contains(t, buffer.String(), `"status_code":200`)
}

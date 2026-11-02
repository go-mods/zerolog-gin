package zerologgin

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Options struct {
	//
	Name string

	// Custom logger
	Logger *zerolog.Logger

	// FieldsOrder defines the order of fields in output.
	FieldsOrder []string

	// FieldsExclude defines contextual fields to not display in output.
	FieldsExclude []string
}

var (
	NameFieldName       = "name"
	HostnameFieldName   = "hostname"
	ClientIPFieldName   = "client_ip"
	UserAgentFieldName  = "user_agent"
	TimestampFieldName  = zerolog.TimestampFieldName
	DurationFieldName   = "elapsed"
	MethodFieldName     = "method"
	PathFieldName       = "path"
	PayloadFieldName    = "payload"
	RefererFieldName    = "referer"
	StatusCodeFieldName = "status_code"
	DataLengthFieldName = "data_length"
	BodyFieldName       = "body"
)

// Logger is a gin middleware which use zerolog
func Logger() gin.HandlerFunc {
	o := &Options{
		FieldsOrder: ginDefaultFieldsOrder(),
	}
	return LoggerWithOptions(o)
}

// LoggerWithOptions is a gin middleware which use zerolog.
//
// It works on a copy of opt, which is left untouched: changing opt afterwards
// has no effect on the middleware. Only the logger opt.Logger points to is
// still read on each request.
//
// TimestampFieldName is emitted once per line. When the logger already adds
// that field, for instance through zerolog.Context.Timestamp, its value is kept
// and FieldsOrder and FieldsExclude have no effect on it. Otherwise the
// middleware adds the time at which the request began. To find out, the first
// logged request also writes a blank event to a private buffer, which runs the
// hooks of the logger one extra time.
//
// When the request body cannot be read, the request goes on: the handler gets
// the bytes that were read followed by the read error, and the log line carries
// the error in zerolog.ErrorFieldName.
func LoggerWithOptions(opt *Options) gin.HandlerFunc {
	// Never write to the options of the caller
	o := *opt
	o.FieldsOrder = slices.Clone(opt.FieldsOrder)
	o.FieldsExclude = slices.Clone(opt.FieldsExclude)
	opt = &o

	// List of fields
	if len(opt.FieldsOrder) == 0 {
		opt.FieldsOrder = ginDefaultFieldsOrder()
	}

	// Logger to use
	if opt.Logger == nil {
		opt.Logger = &log.Logger
	}

	//
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Checked on the first logged request: the logger pointed to may only be
	// configured after the middleware has been created.
	loggerHasTimestamp := sync.OnceValue(func() bool {
		return emitsField(opt.Logger, TimestampFieldName)
	})

	return func(ctx *gin.Context) {
		// get zerolog
		z := opt.Logger

		// return if zerolog is disabled
		if z.GetLevel() == zerolog.Disabled {
			ctx.Next()
			return
		}

		// before executing the next handlers
		begin := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// Get payload from request
		var payload []byte
		var payloadErr error
		if !opt.isExcluded(PayloadFieldName) {
			payload, payloadErr = io.ReadAll(ctx.Request.Body)
			ctx.Request.Body = replayBody(payload, payloadErr)
		}

		// Get a copy of the body
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: ctx.Writer}
		ctx.Writer = w

		// executes the pending handlers
		ctx.Next()

		// after executing the handlers
		duration := time.Since(begin)
		statusCode := ctx.Writer.Status()

		//
		var event *zerolog.Event

		// set message level
		if statusCode >= 400 && statusCode < 500 {
			event = z.Warn()
		} else if statusCode >= 500 {
			event = z.Error()
		} else {
			event = z.Trace()
		}

		// report a body that could not be read entirely
		if payloadErr != nil {
			event.Err(payloadErr)
		}

		// add fields
		for _, f := range opt.FieldsOrder {
			// Name field
			if f == NameFieldName && !opt.isExcluded(f) && len(opt.Name) > 0 {
				event.Str(NameFieldName, opt.Name)
			}
			// Hostname field
			if f == HostnameFieldName && !opt.isExcluded(f) && len(hostname) > 0 {
				event.Str(HostnameFieldName, hostname)
			}
			// ClientIP field
			if f == ClientIPFieldName && !opt.isExcluded(f) {
				event.Str(ClientIPFieldName, ctx.ClientIP())
			}
			// UserAgent field
			if f == UserAgentFieldName && !opt.isExcluded(f) && len(ctx.Request.UserAgent()) > 0 {
				event.Str(UserAgentFieldName, ctx.Request.UserAgent())
			}
			// Method field
			if f == MethodFieldName && !opt.isExcluded(f) {
				event.Str(MethodFieldName, ctx.Request.Method)
			}
			// Path field
			if f == PathFieldName && !opt.isExcluded(f) && len(path) > 0 {
				event.Str(PathFieldName, path)
			}
			// Payload field
			if f == PayloadFieldName && !opt.isExcluded(f) && len(payload) > 0 {
				event.Str(PayloadFieldName, string(payload))
			}
			// Timestamp field
			if f == TimestampFieldName && !opt.isExcluded(f) && event.Enabled() && !loggerHasTimestamp() {
				event.Time(TimestampFieldName, begin)
			}
			// Duration field
			if f == DurationFieldName && !opt.isExcluded(f) {
				var durationFieldName string
				switch zerolog.DurationFieldUnit {
				case time.Nanosecond:
					durationFieldName = DurationFieldName + "_ns"
				case time.Microsecond:
					durationFieldName = DurationFieldName + "_us"
				case time.Millisecond:
					durationFieldName = DurationFieldName + "_ms"
				case time.Second:
					durationFieldName = DurationFieldName
				case time.Minute:
					durationFieldName = DurationFieldName + "_min"
				case time.Hour:
					durationFieldName = DurationFieldName + "_hr"
				default:
					z.Error().Interface("zerolog.DurationFieldUnit", zerolog.DurationFieldUnit).Msg("unknown value for DurationFieldUnit")
					durationFieldName = DurationFieldName
				}
				event.Dur(durationFieldName, duration)
			}
			// Referer field
			if f == RefererFieldName && !opt.isExcluded(f) && len(ctx.Request.Referer()) > 0 {
				event.Str(RefererFieldName, ctx.Request.Referer())
			}
			// StatusCode field
			if f == StatusCodeFieldName && !opt.isExcluded(f) {
				event.Int(StatusCodeFieldName, statusCode)
			}
			// DataLength field
			if f == DataLengthFieldName && !opt.isExcluded(f) && ctx.Writer.Size() > 0 {
				event.Int(DataLengthFieldName, ctx.Writer.Size())
			}
			// Body field
			if f == BodyFieldName && !opt.isExcluded(f) && len(w.body.String()) > 0 {
				event.Str(BodyFieldName, w.body.String())
			}
		}

		// Message
		message := ctx.Errors.String()
		if message == "" {
			message = "Request"
		}

		// post the message
		event.Msg(message)
	}
}

// ginDefaultFieldsOrder returns the default order of fields
func ginDefaultFieldsOrder() []string {
	return []string{
		NameFieldName,
		HostnameFieldName,
		ClientIPFieldName,
		UserAgentFieldName,
		MethodFieldName,
		PathFieldName,
		PayloadFieldName,
		TimestampFieldName,
		DurationFieldName,
		RefererFieldName,
		StatusCodeFieldName,
		DataLengthFieldName,
		BodyFieldName,
	}
}

// isExcluded check if a field is excluded from the output
func (o *Options) isExcluded(field string) bool {
	if o.FieldsExclude == nil {
		return false
	}
	for _, f := range o.FieldsExclude {
		if f == field {
			return true
		}
	}

	return false
}

// emitsField reports whether every event of z carries the given field on its
// own, from its context or its hooks. The events are JSON unless zerolog is built
// with the binary_log tag: it then always reports false.
func emitsField(z *zerolog.Logger, field string) bool {
	var buf bytes.Buffer
	probe := z.Output(&buf).Sample(nil)
	probe.Log().Msg("")

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &fields); err != nil {
		return false
	}
	_, found := fields[field]
	return found
}

// replayBody returns a request body that yields payload, then the error that
// interrupted its reading, if any.
func replayBody(payload []byte, err error) io.ReadCloser {
	var body io.Reader = bytes.NewReader(payload)
	if err != nil {
		body = io.MultiReader(body, errorReader{err: err})
	}
	return io.NopCloser(body)
}

// errorReader is a reader that always fails with err.
type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r responseBodyWriter) WriteString(s string) (n int, err error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

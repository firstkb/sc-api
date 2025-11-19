package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/phuslu/log"
)

const (
	indentSize    = 2
	maxJsonLength = 4 * 1024

	fontReset    = "\x1b[0m"
	black        = "\x1b[30m"
	red          = "\x1b[31m"
	green        = "\x1b[32m"
	yellow       = "\x1b[33m"
	blue         = "\x1b[34m"
	magenta      = "\x1b[35m"
	cyan         = "\x1b[36m"
	lightGray    = "\x1b[37m"
	darkGray     = "\x1b[90m"
	lightRed     = "\x1b[91m"
	lightGreen   = "\x1b[92m"
	lightYellow  = "\x1b[93m"
	lightBlue    = "\x1b[94m"
	lightMagenta = "\x1b[95m"
	lightCyan    = "\x1b[96m"
	white        = "\x1b[97m"

	labelDebug = "DBG"
)

type buffer struct {
	bytes []byte
}

var buffers = sync.Pool{
	New: func() interface{} {
		return &buffer{
			bytes: make([]byte, 0, 128),
		}
	},
}

// io.Writer interface implementation
func (buf *buffer) Write(bytes []byte) (int, error) {
	buf.bytes = append(buf.bytes, bytes...)
	return len(bytes), nil
}

func consoleFormatter(out io.Writer, args *log.FormatterArgs) (n int, err error) {
	buf := buffers.Get().(*buffer)
	buf.bytes = buf.bytes[:0]
	defer buffers.Put(buf)

	ansiEscape := func(esc string) string {
		return esc
	}

	reset := ansiEscape(fontReset)
	defaultColor := ansiEscape(darkGray)
	defaultColorKey := ansiEscape(lightGray)
	defaultColorValue := ansiEscape(lightGray)
	colorTime := ansiEscape(darkGray)
	colorText := ansiEscape(lightGray)

	var color, label string

	level := log.ParseLevel(args.Level)

	switch level {
	case log.TraceLevel:
		color, label = ansiEscape(lightGray), "TRC"
	case log.DebugLevel:
		color, label = ansiEscape(white), labelDebug
	case log.InfoLevel:
		color, label = ansiEscape(cyan), "INF"
	case log.WarnLevel:
		color, label = ansiEscape(lightYellow), "WRN"
	case log.ErrorLevel:
		color, label = ansiEscape(lightRed), "ERR"
	case log.FatalLevel:
		color, label = ansiEscape(lightMagenta), "FTL"
	case log.PanicLevel:
		color, label = ansiEscape(lightMagenta), "PNC"
	default:
		color, label = ansiEscape(yellow), "???"
	}

	timestamp := args.Time
	time, error := time.Parse(timeFormat, timestamp)
	if error == nil {
		timestamp = time.Format("15:04:05.000")
	}

	fmt.Fprintf(buf, "%s%s%s %s%-3s%s ", colorTime, timestamp, reset, color, label, reset)

	if label == labelDebug && len(args.Caller) > 0 {
		fmt.Fprintf(buf, "%s[%s]%s", defaultColor, args.Caller, reset)
		buf.bytes = append(buf.bytes, ' ')
	}

	if len(args.Message) > 0 {
		fmt.Fprintf(buf, "%s%s%s", colorText, args.Message, reset)
		buf.bytes = append(buf.bytes, ' ')
	}

	if len(args.KeyValues) > 0 {
		fmt.Fprintf(buf, "%s{%s\n", defaultColor, reset)
		var colorKey, colorValue string

		count := 0

		for _, kv := range args.KeyValues {
			count++

			if kv.Key == "error" {
				color = ansiEscape(red)
				colorKey = ansiEscape(lightRed)
				colorValue = color
			} else {
				color = defaultColor
				colorKey = defaultColorKey
				colorValue = defaultColorValue
			}

			buf.indent(indentSize)

			switch kv.ValueType {
			case 't':
				fmt.Fprintf(buf, "%s\"%s%s%s\": %strue%s", color, colorKey, kv.Key, color, colorValue, reset)
			case 'f':
				fmt.Fprintf(buf, "%s\"%s%s%s\": %sfalse%s", color, colorKey, kv.Key, color, colorValue, reset)
			case 'n':
				fmt.Fprintf(buf, "%s\"%s%s%s\": %s%s%s", color, colorKey, kv.Key, color, colorValue, kv.Value, reset)
			case 'o':
				if buf.writeJson(kv.Key, kv.Value, color, defaultColorKey, defaultColorValue, reset, indentSize) {
					break
				}
				fallthrough
			case 'S':
				fmt.Fprintf(buf, "%s\"%s%s%s\": %s%s%s", color, colorKey, kv.Key, color, colorValue, kv.Value, reset)
			case 's':
				fallthrough
			default:
				fmt.Fprintf(buf, "%s\"%s%s%s\": \"%s%s%s\"%s", color, colorKey, kv.Key, color, colorValue, kv.Value, color, reset)
			}

			if count < len(args.KeyValues) {
				fmt.Fprintf(buf, "%s,%s", color, reset)
			}

			buf.writeLn()
		}

		fmt.Fprintf(buf, "%s}%s", defaultColor, reset)
	} else if level >= log.ErrorLevel && args.Stack != "" {
		fmt.Fprintf(buf, "\n%s--- stack trace ---\n%s%s%s\n--- stack trace ---%s", defaultColor, defaultColorValue, args.Stack, defaultColor, reset)
	}

	buf.writeLn()

	return out.Write(buf.bytes)
}

func (buf *buffer) indent(count int) {
	for i := 0; i < count; i++ {
		buf.bytes = append(buf.bytes, ' ')
	}
}

func (buf *buffer) writeLn() {
	buf.bytes = append(buf.bytes, '\n')
}

func (buf *buffer) writeJson(key string, value string, color string, colorKey string, colorValue string, reset string, indent int) bool {
	var jsonData interface{}

	jsonDataReader := strings.NewReader(value)
	decoder := json.NewDecoder(jsonDataReader)

	err := decoder.Decode(&jsonData)
	if err != nil {
		return false
	}

	fmt.Fprintf(buf, "%s\"%s%s%s\": %s", color, colorKey, key, color, reset)

	switch items := jsonData.(type) {
	case map[string]interface{}:
		buf.writeJsonObject(items, color, colorKey, colorValue, reset, indent)
	case []interface{}:
		buf.writeJsonArray(items, color, colorKey, colorValue, reset, indent)
	default:
		return false
	}

	return true
}

func (buf *buffer) writeJsonObject(keyValues map[string]interface{}, color string, colorKey string, colorValue string, reset string, indent int) {
	if len(buf.bytes) > maxJsonLength {
		fmt.Fprint(buf, " ...TRUNCATED ")
		return
	}

	if len(keyValues) == 0 {
		fmt.Fprintf(buf, "%s{ }%s", color, reset)
		return
	}

	indent += indentSize

	fmt.Fprintf(buf, "%s{%s\n", color, reset)

	count := 0

	for key, v := range keyValues {
		count++

		buf.indent(indent)

		fmt.Fprintf(buf, "%s\"%s%s%s\": %s", color, colorKey, key, color, reset)

		switch value := v.(type) {
		case map[string]interface{}:
			buf.writeJsonObject(v.(map[string]interface{}), color, colorKey, colorValue, reset, indent)
		case []interface{}:
			buf.writeJsonArray(v.([]interface{}), color, colorKey, colorValue, reset, indent+indentSize)
		default:
			buf.writeJsonPrimitive(value, color, colorValue, reset)
		}

		if count < len(keyValues) {
			fmt.Fprintf(buf, "%s,%s", color, reset)
		}

		buf.writeLn()
	}

	buf.indent(indent - indentSize)
	fmt.Fprintf(buf, "%s}%s", color, reset)
}

func (buf *buffer) writeJsonArray(values []interface{}, color string, colorKey string, colorValue string, reset string, indent int) {
	if len(buf.bytes) > maxJsonLength {
		fmt.Fprint(buf, " ...TRUNCATED ")
		return
	}

	if len(values) == 0 {
		fmt.Fprintf(buf, "%s[ ]%s", color, reset)
		return
	}

	fmt.Fprintf(buf, "%s[%s", color, reset)

	multiLine := false

	for i, v := range values {
		if i == 0 {
			if len(values) > 1 {
				_, multiLine = v.(map[string]interface{})
				if !multiLine {
					_, multiLine = v.([]interface{})
				}
				if multiLine {
					buf.writeLn()
					buf.indent(indent)
				}
			}
		} else {
			fmt.Fprintf(buf, "%s,%s", color, reset)
			if multiLine {
				buf.writeLn()
				buf.indent(indent)
			} else {
				buf.bytes = append(buf.bytes, ' ')
			}
		}

		switch value := v.(type) {
		case map[string]interface{}:
			buf.writeJsonObject(v.(map[string]interface{}), color, colorKey, colorValue, reset, indent)
		case []interface{}:
			buf.writeJsonArray(v.([]interface{}), color, reset, colorKey, colorValue, indent)
		default:
			buf.writeJsonPrimitive(value, color, colorValue, reset)
		}
	}

	if multiLine {
		buf.writeLn()
		buf.indent(indent - indentSize)
	}

	fmt.Fprintf(buf, "%s]%s", color, reset)
}

func (buf *buffer) writeJsonPrimitive(v any, color string, colorValue string, reset string) {
	switch value := v.(type) {
	case string:
		fmt.Fprintf(buf, "%s\"%s%s%s\"%s", color, colorValue, value, color, reset)
	case nil:
		fmt.Fprintf(buf, "%snull%s", colorValue, reset)
	default:
		fmt.Fprintf(buf, "%s%v%s", colorValue, value, reset)
	}
}

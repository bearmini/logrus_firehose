package logrus_firehose

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/sirupsen/logrus"
)

const (
	maxBatchRecords = 500
)

var defaultLevels = []logrus.Level{
	logrus.PanicLevel,
	logrus.FatalLevel,
	logrus.ErrorLevel,
	logrus.WarnLevel,
	logrus.InfoLevel,
}

// FirehoseHook is logrus hook for AWS Firehose.
// Amazon Kinesis Firehose is a fully-managed service that delivers real-time
// streaming data to destinations such as Amazon Simple Storage Service (Amazon
// S3), Amazon Elasticsearch Service (Amazon ES), and Amazon Redshift.
type FirehoseHook struct {
	client              *firehose.Firehose
	buf                 []*logrus.Entry
	bufCh               chan *logrus.Entry
	flushCh             chan bool
	errCh               chan error
	streamName          string
	defaultPartitionKey string
	levels              []logrus.Level
	ignoreFields        map[string]struct{}
	filters             map[string]func(interface{}) interface{}
	addNewline          bool
}

// NewWithConfig returns initialized logrus hook for Firehose with persistent Firehose logger.
func NewWithAWSConfig(streamName string, conf *aws.Config) (*FirehoseHook, error) {
	sess, err := session.NewSession(conf)
	if err != nil {
		return nil, err
	}

	svc := firehose.New(sess)

	bufCh := make(chan *logrus.Entry, 1000)
	flushCh := make(chan bool)
	errCh := make(chan error)

	h := &FirehoseHook{
		client:       svc,
		buf:          make([]*logrus.Entry, 0),
		bufCh:        bufCh,
		flushCh:      flushCh,
		errCh:        errCh,
		streamName:   streamName,
		levels:       defaultLevels,
		ignoreFields: make(map[string]struct{}),
		filters:      make(map[string]func(interface{}) interface{}),
	}

	go h.bufLoop()

	return h, nil
}

func (h *FirehoseHook) GetErrorChan() <-chan error {
	return h.errCh
}

// Levels returns logging level to fire this hook.
func (h *FirehoseHook) Levels() []logrus.Level {
	return h.levels
}

// SetLevels sets logging level to fire this hook.
func (h *FirehoseHook) SetLevels(levels []logrus.Level) {
	h.levels = levels
}

// AddIgnore adds field name to ignore.
func (h *FirehoseHook) AddIgnore(name string) {
	h.ignoreFields[name] = struct{}{}
}

// AddFilter adds a custom filter function.
func (h *FirehoseHook) AddFilter(name string, fn func(interface{}) interface{}) {
	h.filters[name] = fn
}

// AddNewline sets if a newline is added to each message.
func (h *FirehoseHook) AddNewLine(b bool) {
	h.addNewline = b
}

// Fire is invoked by logrus and sends log to Firehose.
func (h *FirehoseHook) Fire(entry *logrus.Entry) error {
	h.bufCh <- entry
	return nil
}

func (h *FirehoseHook) Flush() {
	h.flushCh <- true
}

func (h *FirehoseHook) bufLoop() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "panic: %+v\n", err)
		}
	}()
	for {
		select {
		case e := <-h.bufCh:
			h.buf = append(h.buf, e)
		case <-h.flushCh:
			h.flush()
		}
	}
}

func (h *FirehoseHook) flush() {
	if len(h.buf) == 0 {
		return
	}

	defer func() {
		h.buf = make([]*logrus.Entry, 0)
	}()

	for _, buf := range splitBuf(h.buf, maxBatchRecords) {
		records := make([]*firehose.Record, 0, len(buf))
		for _, e := range buf {
			records = append(records, &firehose.Record{
				Data: append(h.getData(e), []byte("\n")...),
			})
		}
		in := &firehose.PutRecordBatchInput{
			DeliveryStreamName: aws.String(h.streamName),
			Records:            records,
		}
		_, err := h.client.PutRecordBatch(in)
		if err != nil {
			h.errCh <- err
		}
	}
}

func splitBuf(buf []*logrus.Entry, size int) [][]*logrus.Entry {
	result := make([][]*logrus.Entry, 0)
	for len(buf) > 0 {
		if len(buf) > size {
			result = append(result, buf[:size])
			buf = buf[size:]
		} else {
			result = append(result, buf)
			buf = buf[:0]
		}
	}
	return result
}

func (h *FirehoseHook) getData(entry *logrus.Entry) []byte {
	data := make(logrus.Fields)
	if _, exists := entry.Data["message"]; !exists {
		entry.Data["message"] = entry.Message
	}

	entry.Data["level"] = entry.Level

	for k, v := range entry.Data {
		if _, ok := h.ignoreFields[k]; ok {
			continue
		}
		if fn, ok := h.filters[k]; ok {
			v = fn(v) // apply custom filter
		} else {
			v = formatData(v) // use default formatter
		}
		data[k] = v
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	if h.addNewline {
		n := []byte("\n")
		bytes = append(bytes, n...)
	}
	return bytes
}

// formatData returns value as a suitable format.
func formatData(value interface{}) (formatted interface{}) {
	switch value := value.(type) {
	case json.Marshaler:
		return value
	case error:
		return value.Error()
	case fmt.Stringer:
		return value.String()
	default:
		return value
	}
}

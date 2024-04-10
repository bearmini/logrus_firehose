package logrus_firehose

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewWithAWSConfig(t *testing.T) {
	assert := assert.New(t)
	t.Skip("TODO: add some case")

	hook, err := NewWithAWSConfig("test_stream", *aws.NewConfig())
	assert.Error(err)
	assert.Nil(hook)
}

func TestLevels(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		levels []logrus.Level
	}{
		{nil},
		{[]logrus.Level{logrus.WarnLevel}},
		{[]logrus.Level{logrus.ErrorLevel}},
		{[]logrus.Level{logrus.WarnLevel, logrus.DebugLevel}},
		{[]logrus.Level{logrus.WarnLevel, logrus.DebugLevel, logrus.ErrorLevel}},
	}

	for _, tt := range tests {
		target := fmt.Sprintf("%+v", tt)

		hook := FirehoseHook{}
		levels := hook.Levels()
		assert.Nil(levels, target)

		hook.levels = tt.levels
		levels = hook.Levels()
		assert.Equal(tt.levels, levels, target)
	}
}

func TestSetLevels(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		levels []logrus.Level
	}{
		{nil},
		{[]logrus.Level{logrus.WarnLevel}},
		{[]logrus.Level{logrus.ErrorLevel}},
		{[]logrus.Level{logrus.WarnLevel, logrus.DebugLevel}},
		{[]logrus.Level{logrus.WarnLevel, logrus.DebugLevel, logrus.ErrorLevel}},
	}

	for _, tt := range tests {
		target := fmt.Sprintf("%+v", tt)

		hook := FirehoseHook{}
		assert.Nil(hook.levels, target)

		hook.SetLevels(tt.levels)
		assert.Equal(tt.levels, hook.levels, target)

		hook.SetLevels(nil)
		assert.Nil(hook.levels, target)
	}
}

func TestAddIgnore(t *testing.T) {
	assert := assert.New(t)

	hook := FirehoseHook{
		ignoreFields: make(map[string]struct{}),
	}

	list := []string{"foo", "bar", "baz"}
	for i, key := range list {
		assert.Len(hook.ignoreFields, i)

		hook.AddIgnore(key)
		assert.Len(hook.ignoreFields, i+1)

		for j := 0; j <= i; j++ {
			assert.Contains(hook.ignoreFields, list[j])
		}
	}
}

func TestAddFilter(t *testing.T) {
	assert := assert.New(t)

	hook := FirehoseHook{
		filters: make(map[string]func(interface{}) interface{}),
	}

	list := []string{"foo", "bar", "baz"}
	for i, key := range list {
		assert.Len(hook.filters, i)

		hook.AddFilter(key, nil)
		assert.Len(hook.filters, i+1)

		for j := 0; j <= i; j++ {
			assert.Contains(hook.filters, list[j])
		}
	}
}

func TestGetData(t *testing.T) {
	assert := assert.New(t)

	const defaultMessage = "entry_message"

	tests := []struct {
		data     map[string]interface{}
		expected string
	}{
		{
			map[string]interface{}{},
			`{"message":"entry_message"}`,
		},
		{
			map[string]interface{}{"message": "field_message"},
			`{"message":"field_message"}`,
		},
		{
			map[string]interface{}{
				"name":  "apple",
				"price": 105,
				"color": "red",
			},
			`{"color":"red","message":"entry_message","name":"apple","price":105}`,
		},
		{
			map[string]interface{}{
				"name":    "apple",
				"price":   105,
				"color":   "red",
				"message": "field_message",
			},
			`{"color":"red","message":"field_message","name":"apple","price":105}`,
		},
	}

	for _, tt := range tests {
		target := fmt.Sprintf("%+v", tt)

		hook := FirehoseHook{}
		entry := &logrus.Entry{
			Message: defaultMessage,
			Data:    tt.data,
		}

		assert.Equal(tt.expected, string(hook.getData(entry)), target)
	}
}

func TestFormatData(t *testing.T) {
	assert := assert.New(t)

	// assertion types
	var (
		assertTypeInt    int
		assertTypeString string
		assertTypeTime   time.Time
	)

	tests := []struct {
		name         string
		value        interface{}
		expectedType interface{}
	}{
		{"int", 13, assertTypeInt},
		{"string", "foo", assertTypeString},
		{"error", errors.New("this is a test error"), assertTypeString},
		{"time_stamp", time.Now(), assertTypeTime},        // implements JSON marshaler
		{"time_duration", time.Hour, assertTypeString},    // implements .String()
		{"stringer", myStringer{}, assertTypeString},      // implements .String()
		{"stringer_ptr", &myStringer{}, assertTypeString}, // implements .String()
		{"not_stringer", notStringer{}, notStringer{}},
		{"not_stringer_ptr", &notStringer{}, &notStringer{}},
	}

	for _, tt := range tests {
		target := fmt.Sprintf("%+v", tt)

		result := formatData(tt.value)
		assert.IsType(tt.expectedType, result, target)
	}
}

type myStringer struct{}

func (myStringer) String() string { return "myStringer!" }

type notStringer struct{}

func (notStringer) String() {}

func TestSplitBuf(t *testing.T) {
	logger := logrus.New()
	e1 := logrus.NewEntry(logger)
	e2 := logrus.NewEntry(logger)
	e3 := logrus.NewEntry(logger)
	e4 := logrus.NewEntry(logger)
	e5 := logrus.NewEntry(logger)

	testData := []struct {
		Name   string
		Source []*logrus.Entry
		Size   int
		Expect [][]*logrus.Entry
	}{
		{
			Name:   "pattern 1 (empty)",
			Source: []*logrus.Entry{},
			Size:   3,
			Expect: [][]*logrus.Entry{},
		},
		{
			Name:   "pattern 2 (length of source < size)",
			Source: []*logrus.Entry{e1},
			Size:   3,
			Expect: [][]*logrus.Entry{{e1}},
		},
		{
			Name:   "pattern 3 (length of source == size)",
			Source: []*logrus.Entry{e1, e2, e3},
			Size:   3,
			Expect: [][]*logrus.Entry{{e1, e2, e3}},
		},
		{
			Name:   "pattern 4 (length of source > size)",
			Source: []*logrus.Entry{e1, e2, e3, e4, e5},
			Size:   3,
			Expect: [][]*logrus.Entry{{e1, e2, e3}, {e4, e5}},
		},
	}

	for _, data := range testData {
		actual := splitBuf(data.Source, data.Size)
		if !reflect.DeepEqual(data.Expect, actual) {
			t.Fatalf("\nExpected: %+v\nActual:   %+v\n", data.Expect, actual)
		}
	}
}

package signalr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jwriter"
	"io"
	"reflect"
)

// JSONHubProtocol is the JSON based SignalR protocol
type JSONHubProtocol struct {
	dbg        log.Logger
	easyWriter jwriter.Writer
}

// Protocol specific message for correct unmarshaling of Arguments
type jsonInvocationMessage struct {
	Type         int               `json:"type"`
	Target       string            `json:"target"`
	InvocationID string            `json:"invocationId"`
	Arguments    []json.RawMessage `json:"arguments"`
	StreamIds    []string          `json:"streamIds,omitempty"`
}

type jsonError struct {
	raw string
	err error
}

func (j *jsonError) Error() string {
	return fmt.Sprintf("%v (source: %v)", j.err, j.raw)
}

// UnmarshalArgument unmarshals a json.RawMessage depending of the specified value type into value
func (v *JSONHubProtocol) UnmarshalArgument(argument interface{}, value interface{}) error {
	if err := json.Unmarshal(argument.(json.RawMessage), value); err != nil {
		return &jsonError{string(argument.(json.RawMessage)), err}
	}
	_ = v.dbg.Log(evt, "UnmarshalArgument",
		"argument", string(argument.(json.RawMessage)),
		"value", fmt.Sprintf("%v", reflect.ValueOf(value).Elem()))
	return nil
}

// ReadMessage reads a JSON message from buf and returns the message if the buf contained one completely.
// If buf does not contain the whole message, it returns a nil message and complete false
func (v *JSONHubProtocol) ParseMessage(buf io.Reader) (m interface{}, err error) {
	data, err := parseTextMessageFormat(buf)
	switch {
	case errors.Is(err, io.EOF):
		return nil, err
		// Other errors never happen, because parseTextMessageFormat will only return err
		// from bytes.Buffer.ReadBytes() which is always io.EOF or nil
	}

	message := hubMessage{}
	err = message.UnmarshalJSON(data)
	_ = v.dbg.Log(evt, "read", msg, string(data))
	if err != nil {
		return nil, &jsonError{string(data), err}
	}

	switch message.Type {
	case 1, 4:
		jsonInvocation := jsonInvocationMessage{}
		if err = jsonInvocation.UnmarshalJSON(data); err != nil {
			err = &jsonError{string(data), err}
		}
		arguments := make([]interface{}, len(jsonInvocation.Arguments))
		for i, a := range jsonInvocation.Arguments {
			arguments[i] = a
		}
		invocation := invocationMessage{
			Type:         jsonInvocation.Type,
			Target:       jsonInvocation.Target,
			InvocationID: jsonInvocation.InvocationID,
			Arguments:    arguments,
			StreamIds:    jsonInvocation.StreamIds,
		}
		return invocation, err
	case 2:
		streamItem := streamItemMessage{}
		if err = streamItem.UnmarshalJSON(data); err != nil {
			err = &jsonError{string(data), err}
		}
		return streamItem, err
	case 3:
		completion := completionMessage{}
		if err = completion.UnmarshalJSON(data); err != nil {
			err = &jsonError{string(data), err}
		}
		return completion, err
	case 5:
		invocation := cancelInvocationMessage{}
		if err = invocation.UnmarshalJSON(data); err != nil {
			err = &jsonError{string(data), err}
		}
		return invocation, err
	case 7:
		cm := closeMessage{}
		if err = cm.UnmarshalJSON(data); err != nil {
			err = &jsonError{string(data), err}
		}
		return cm, err
	default:
		return message, nil
	}
}

func parseTextMessageFormat(reader io.Reader) ([]byte, error) {
	data := make([]byte, 0)
	p := make([]byte, 1024)
	for {
		n, err := reader.Read(p)
		if err != nil {
			return nil, err
		}
		if i := bytes.IndexByte(p, 30); i != -1 {
			data = append(data, p[:i]...)
			return data, nil
		}
		data = append(data, p[:n]...)
	}
}

// WriteMessage writes a message as JSON to the specified writer
func (v *JSONHubProtocol) WriteMessage(message interface{}, writer io.Writer) error {
	if em, ok := message.(easyjson.Marshaler); ok {
		em.MarshalEasyJSON(&v.easyWriter)
		v.easyWriter.RawByte(30)
		b := v.easyWriter.Buffer.BuildBytes()
		_ = v.dbg.Log(evt, "write", msg, string(b))
		_, err := writer.Write(b)
		return err
	}
	return fmt.Errorf("%#v does not implement easyjson.Marshaler", message)
}

func (v *JSONHubProtocol) setDebugLogger(dbg StructuredLogger) {
	v.dbg = log.WithPrefix(dbg, "ts", log.DefaultTimestampUTC, "protocol", "JSON")
}

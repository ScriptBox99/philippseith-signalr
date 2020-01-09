package signalr

import (
	"fmt"
	"reflect"
)

func newStreamClient() *streamClient {
	return &streamClient{make(map[string]reflect.Value)}
}

type streamClient struct {
	upstreamChannels map[string]reflect.Value
}

func (u *streamClient) buildChannelArgument(invocation invocationMessage, argType reflect.Type, chanCount int) (arg reflect.Value, canClientStreaming bool, err error) {
	if argType.Kind() != reflect.Chan || argType.ChanDir() == reflect.SendDir {
		return reflect.Value{}, false, nil
	} else if len(invocation.StreamIds) > chanCount {
		// MakeChan does only accept bidirectional channels and we need to Send to this channel anyway
		arg = reflect.MakeChan(reflect.ChanOf(reflect.BothDir, argType.Elem()), 0)
		u.upstreamChannels[invocation.StreamIds[chanCount]] = arg
		return arg, true, nil
	} else {
		// To many channel parameters arguments this method. The client will not send streamItems for these
		return reflect.Value{}, true, fmt.Errorf("method %s has more chan parameters than the client will stream", invocation.Target)
	}
}

func (u *streamClient) receiveStreamItem(streamItem streamItemMessage) error {
	if upChan, ok := u.upstreamChannels[streamItem.InvocationID]; ok {
		// Hack(?) for missing channel type information when the Protocol decodes StreamItem.Item
		// Protocol specific, as only json has this inexact number type. Messagepack might cause different problems
		chanElm := reflect.Indirect(reflect.New(upChan.Type().Elem())).Interface()
		if f, ok := streamItem.Item.(float64); ok {
			// This type of solution is constrained to basic types, e.g. chan MyInt is not supported
			if chanVal, ok := convertNumberToChannelType(chanElm, f); ok {
				upChan.Send(chanVal)
			}
		} else {
			// Are stream item and channel type both slices/arrays?
			switch reflect.TypeOf(streamItem.Item).Kind() {
			case reflect.Slice:
				fallthrough
			case reflect.Array:
				switch reflect.TypeOf(chanElm).Kind() {
				case reflect.Slice:
					fallthrough
				case reflect.Array:
					break
				default:
					return fmt.Errorf("stream item of kind %v paired with channel of type %v", reflect.TypeOf(streamItem.Item).Kind(), reflect.TypeOf(chanElm))
				}
			default:
				done := make(chan error, 0)
				go func() {
					defer func() {
						if err := recover(); err != nil {
							// err is always an error
							done <- err.(error)
						}
					}()
					upChan.Send(reflect.ValueOf(streamItem.Item))
					done <- nil
				}()
				return <- done
			}
		}
	}
	return nil
}

func convertNumberToChannelType(chanElm interface{}, number float64) (chanVal reflect.Value, ok bool) {
	switch chanElm.(type) {
	case int:
		return reflect.ValueOf(int(number)), true
	case int8:
		return reflect.ValueOf(int8(number)), true
	case int16:
		return reflect.ValueOf(int16(number)), true
	case int32:
		return reflect.ValueOf(int32(number)), true
	case int64:
		return reflect.ValueOf(int64(number)), true
	case uint:
		return reflect.ValueOf(uint(number)), true
	case uint8:
		return reflect.ValueOf(uint8(number)), true
	case uint16:
		return reflect.ValueOf(uint16(number)), true
	case uint32:
		return reflect.ValueOf(uint32(number)), true
	case uint64:
		return reflect.ValueOf(uint64(number)), true
	case float32:
		return reflect.ValueOf(float32(number)), true
	case float64:
		return reflect.ValueOf(number), true
	case string:
		return reflect.ValueOf(fmt.Sprint(number)), true
	}
	return reflect.ValueOf(number), false
}

func (u *streamClient) receiveCompletionItem(completion completionMessage) {
	if channel, ok := u.upstreamChannels[completion.InvocationID]; ok {
		channel.Close()
		delete(u.upstreamChannels, completion.InvocationID)
	}
}

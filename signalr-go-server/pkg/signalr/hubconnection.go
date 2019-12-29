package signalr

import (
	"bytes"
	"io"
	"sync/atomic"
)

//Connection describes a connection between signalR client and server
type Connection interface {
	io.Reader
	io.Writer
	ConnectionID() string
}

type hubConnection interface {
	Start()
	IsConnected() bool
	Close(error string)
	GetConnectionID() string
	Receive() (interface{}, error)
	SendInvocation(target string, args []interface{})
	StreamItem(id string, item interface{})
	Completion(id string, result interface{}, error string)
	Ping()
}

func newHubConnection(connection Connection, protocol HubProtocol) hubConnection {
	return &defaultHubConnection{
		Protocol:   protocol,
		Connection: connection,
	}
}

type defaultHubConnection struct {
	Protocol   HubProtocol
	Connected  int32
	Connection Connection
}

func (c *defaultHubConnection) Start() {
	atomic.CompareAndSwapInt32(&c.Connected, 0, 1)
}

func (c *defaultHubConnection) IsConnected() bool {
	return atomic.LoadInt32(&c.Connected) == 1
}

func (c *defaultHubConnection) Close(error string) {
	atomic.StoreInt32(&c.Connected, 0)

	var closeMessage = closeMessage{
		Type:           7,
		Error:          error,
		AllowReconnect: true,
	}
	c.Protocol.WriteMessage(closeMessage, c.Connection)
}

func (c *defaultHubConnection) GetConnectionID() string {
	return c.Connection.ConnectionID()
}

func (c *defaultHubConnection) SendInvocation(target string, args []interface{}) {
	var invocationMessage = sendOnlyHubInvocationMessage{
		Type:      1,
		Target:    target,
		Arguments: args,
	}

	c.Protocol.WriteMessage(invocationMessage, c.Connection)
}

func (c *defaultHubConnection) Ping() {
	var pingMessage = hubMessage{
		Type: 6,
	}

	c.Protocol.WriteMessage(pingMessage, c.Connection)
}

func (c *defaultHubConnection) Receive() (interface{}, error) {
	var buf bytes.Buffer
	var data = make([]byte, 1<<12) // 4K
	var n int
	for {
		if message, complete, err := c.Protocol.ReadMessage(&buf); !complete {
			// Partial message, need more data
			// ReadMessage read data out of the buf, so its gone there: refill
			buf.Write(data[:n])
			if n, err = c.Connection.Read(data); err == nil {
				buf.Write(data[:n])
			} else {
				return nil, err
			}
		} else {
			return message, err
		}
	}
}

func (c *defaultHubConnection) Completion(id string, result interface{}, error string) {
	var completionMessage = completionMessage{
		Type:         3,
		InvocationID: id,
		Result:       result,
		Error:        error,
	}

	c.Protocol.WriteMessage(completionMessage, c.Connection)
}

func (c *defaultHubConnection) StreamItem(id string, item interface{}) {
	var streamItemMessage = streamItemMessage{
		Type:         2,
		InvocationID: id,
		Item:         item,
	}

	c.Protocol.WriteMessage(streamItemMessage, c.Connection)
}

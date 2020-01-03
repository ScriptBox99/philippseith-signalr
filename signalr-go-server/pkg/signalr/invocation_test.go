package signalr

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"time"
)

var invocationQueue = make(chan string, 20)

type invocationHub struct {
	Hub
}

func (i *invocationHub) Simple() {
	invocationQueue <- "Simple()"
}

func (i *invocationHub) SimpleInt(value int) int {
	invocationQueue <- fmt.Sprintf("SimpleInt(%v)", value)
	return value + 1
}

func (i *invocationHub) SimpleFloat(value float64) (float64, float64) {
	invocationQueue <- fmt.Sprintf("SimpleFloat(%v)", value)
	return value * 10.0, value * 100.0
}

func (i *invocationHub) SimpleString(value1 string, value2 string) string {
	invocationQueue <- fmt.Sprintf("SimpleString(%v, %v)", value1, value2)
	return strings.ToLower(value1 + value2)
}

func (i *invocationHub) Async() chan bool {
	r := make(chan bool)
	go func() {
		defer close(r)
		r <- true
	}()
	invocationQueue <- "Async()"
	return r
}

func (i *invocationHub) AsyncClosedChan() chan bool {
	r := make(chan bool)
	close(r)
	invocationQueue <- "AsyncClosedChan()"
	return r
}

func (i *invocationHub) Panic() {
	invocationQueue <- "Panic()"
	panic("Don't panic!")
}

var _ = Describe("Invocation", func() {

	Describe("Simple invocation", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "123","target":"simple"}`)
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal("Simple()"))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("123"))
				Expect(recv.Result).To(BeNil())
				Expect(recv.Error).To(Equal(""))
			})
		})
	})

	Describe("Invalid json", func() {
		conn := connect(&invocationHub{})
		Context("When the client sends invalid json", func() {
			It("should not return any value", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "123","target":"simpleint", arguments[CanNotParse]}`)
				Expect(err).To(BeNil())
				// Strange: Even godoc states PipeReader/PipeWriter are blocking,
				// clientSend returns before the server reads the pipe, so we have to poll.
				// Did I get this right?
				for i := 0; i < 5; i++ {
					select {
					case recv := <-conn.received:
						Fail(fmt.Sprintf("server answered %v", recv))
					default:
					}
					time.Sleep(time.Millisecond * 50)
				}
			})
		})
	})

	Describe("SimpleInt invocation", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked on the server, get an int and return an int", func() {
				var value int
				value = 314
				_, err := conn.clientSend(fmt.Sprintf(
					`{"type":1,"invocationId": "666","target":"simpleint","arguments":[%v]}`, value))
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal(fmt.Sprintf("SimpleInt(%v)", value)))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("666"))
				Expect(recv.Result).To(Equal(float64(value + 1))) // json  makes all numbers float64
				Expect(recv.Error).To(Equal(""))
			})
		})
	})

	Describe("SimpleInt invocation with invalid argument", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client with an invalid argument", func() {
			It("should not be invoked on the server and return an error", func() {
				_, err := conn.clientSend(
					`{"type":1,"invocationId": "555","target":"simpleint","arguments":["CantParse"]}`)
				Expect(err).To(BeNil())
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.Error).NotTo(Equal(""))
				Expect(recv.InvocationID).To(Equal("555"))
			})
		})
	})

	Describe("SimpleFloat invocation", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked on the server, get a float and return a two floats", func() {
				var value float64
				value = 3.1415
				_, err := conn.clientSend(fmt.Sprintf(
					`{"type":1,"invocationId": "8087","target":"simplefloat","arguments":[%v]}`, value))
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal(fmt.Sprintf("SimpleFloat(%v)", value)))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("8087"))
				Expect(recv.Result).To(Equal([]interface{}{value * 10.0, value * 100.0}))
				Expect(recv.Error).To(Equal(""))
			})
		})
	})

	Describe("SimpleString invocation", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked on the server, get two strings and return a string", func() {
				value1 := "Camel"
				value2 := "Cased"
				_, err := conn.clientSend(fmt.Sprintf(
					`{"type":1,"invocationId": "6502","target":"simplestring","arguments":["%v", "%v"]}`, value1, value2))
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal(fmt.Sprintf("SimpleString(%v, %v)", value1, value2)))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("6502"))
				Expect(recv.Result).To(Equal(strings.ToLower(value1 + value2)))
				Expect(recv.Error).To(Equal(""))
			})
		})
	})

	Describe("Async invocation", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked on the server and return true asynchronously", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "mfg","target":"async"}`)
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal("Async()"))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("mfg"))
				Expect(recv.Result).To(Equal(true))
				Expect(recv.Error).To(Equal(""))
			})
		})
	})

	Describe("Async invocation with buggy server method which returns a closed channel", func() {
		conn := connect(&invocationHub{})
		Context("When invoked by the client", func() {
			It("should be invoked on the server and return an error", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "ouch","target":"asyncclosedchan"}`)
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal("AsyncClosedChan()"))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("ouch"))
				Expect(recv.Result).To(BeNil())
				Expect(recv.Error).NotTo(BeNil())
			})
		})
	})

	Describe("Panic in invoked func", func() {
		conn := connect(&invocationHub{})
		Context("When a func is invoked by the client and panics", func() {
			It("should be invoked on the server and return an error but no result", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "???","target":"panic"}`)
				Expect(err).To(BeNil())
				Expect(<-invocationQueue).To(Equal("Panic()"))
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("???"))
				Expect(recv.Result).To(BeNil())
				Expect(recv.Error).NotTo(Equal(""))
			})
		})
	})

	Describe("Missing method invocation", func() {
		conn := connect(&invocationHub{})
		Context("When a missing server method invoked by the client", func() {
			It("should return an error", func() {
				_, err := conn.clientSend(`{"type":1,"invocationId": "0000","target":"missing"}`)
				Expect(err).To(BeNil())
				recv := (<-conn.received).(completionMessage)
				Expect(recv).NotTo(BeNil())
				Expect(recv.InvocationID).To(Equal("0000"))
				Expect(recv.Result).To(BeNil())
				Expect(recv.Error).NotTo(BeNil())
				Expect(len(recv.Error)).To(BeNumerically(">", 0))
			})
		})
	})
})

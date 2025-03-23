package kafka

import (
	"github.com/segmentio/kafka-go"
)

// Message is a wrapper around kafka-go's Message to maintain a consistent interface
// and allow for future extensions without changing the consumer code
type Message = kafka.Message

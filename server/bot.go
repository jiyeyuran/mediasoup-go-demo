package main

import (
	"fmt"
	"log/slog"

	"github.com/jiyeyuran/go-protoo"
	"github.com/jiyeyuran/mediasoup-go/v2"
)

type Bot struct {
	logger       *slog.Logger
	transport    *mediasoup.Transport
	dataProducer *mediasoup.DataProducer
}

func CreateBot(router *mediasoup.Router, logger *slog.Logger) (bot *Bot, err error) {
	transport, err := router.CreateDirectTransport(&mediasoup.DirectTransportOptions{
		MaxMessageSize: 512,
	})
	if err != nil {
		panic(err)
	}
	dataProducer, err := transport.ProduceData(&mediasoup.DataProducerOptions{
		Label: "bot",
	})
	if err != nil {
		panic(err)
	}
	return &Bot{
		logger:       logger,
		transport:    transport,
		dataProducer: dataProducer,
	}, nil
}

func (b Bot) DataProducer() *mediasoup.DataProducer {
	return b.dataProducer
}

func (b Bot) Close() {
	b.transport.Close()
}

func (b *Bot) HandlePeerDataProducer(dataProducerId string, peer *protoo.Peer) (err error) {
	dataConsumer, err := b.transport.ConsumeData(&mediasoup.DataConsumerOptions{
		DataProducerId: dataProducerId,
	})
	if err != nil {
		b.logger.Error("consume data", "error", err)
		return
	}
	dataConsumer.OnMessage(func(message []byte, ppid mediasoup.SctpPayloadType) {
		if ppid != mediasoup.SctpPayloadWebRTCString {
			b.logger.Info("ignoring non string messagee from a Peer")
			return
		}

		b.logger.Debug("SCTP message received", "peerId", peer.Id(), "size", len(message))
		// Create a message to send it back to all Peers in behalf of the sending
		// Peer.
		messageBack := fmt.Sprintf(`%s said me: "%s"`, peer.Id(), message)

		b.dataProducer.SendText(messageBack)
	})
	return
}

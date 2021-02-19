package main

import (
	"fmt"

	"github.com/jiyeyuran/go-protoo"
	"github.com/jiyeyuran/mediasoup-go"
	"github.com/rs/zerolog"
)

type Bot struct {
	logger       zerolog.Logger
	transport    *mediasoup.DirectTransport
	dataProducer *mediasoup.DataProducer
}

func CreateBot(mediasoupRouter *mediasoup.Router) (bot *Bot, err error) {
	logger := NewLogger("Bot")

	transport, err := mediasoupRouter.CreateDirectTransport(mediasoup.DirectTransportOptions{
		MaxMessageSize: 512,
	})
	if err != nil {
		logger.Err(err).Msg("create direct transport")
		return
	}
	dataProducer, err := transport.ProduceData(mediasoup.DataProducerOptions{
		Label: "bot",
	})
	if err != nil {
		logger.Err(err).Msg("produce data")
		return
	}
	return &Bot{
		logger:       NewLogger("Bot"),
		transport:    transport,
		dataProducer: dataProducer,
	}, nil
}

func (b Bot) DataProducer() *mediasoup.DataProducer {
	return b.dataProducer
}

func (b Bot) Close() {

}

func (b *Bot) HandlePeerDataProducer(dataProducerId string, peer *protoo.Peer) (err error) {
	dataConsumer, err := b.transport.ConsumeData(mediasoup.DataConsumerOptions{
		DataProducerId: dataProducerId,
	})
	if err != nil {
		b.logger.Err(err).Msg("consume data")
		return
	}
	dataConsumer.On("message", func(message []byte, ppid byte) {
		if ppid != 51 {
			b.logger.Warn().Msg("ignoring non string messagee from a Peer")
			return
		}

		b.logger.Debug().Str("peerId", peer.Id()).Int("size", len(message)).Msgf("SCTP message received")

		// Create a message to send it back to all Peers in behalf of the sending
		// Peer.
		messageBack := fmt.Sprintf(`%s said me: "%s"`, peer.Id(), message)

		b.dataProducer.SendText(messageBack)
	})
	return
}

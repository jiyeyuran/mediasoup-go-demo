package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/boz/go-throttle"
	"github.com/jiyeyuran/go-eventemitter"
	"github.com/jiyeyuran/go-protoo"
	"github.com/jiyeyuran/mediasoup-go"
	"github.com/rs/zerolog"
)

type Room struct {
	eventemitter.IEventEmitter
	logger             zerolog.Logger
	peerLockers        sync.Map
	config             Config
	roomId             string
	protooRoom         *protoo.Room
	mediasoupRouter    *mediasoup.Router
	audioLevelObserver mediasoup.IRtpObserver
	bot                *Bot
	networkThrottled   bool
	throttle           throttle.Throttle
	broadcasters       sync.Map
	closed             uint32
}

func CreateRoom(config Config, roomId string, worker *mediasoup.Worker) (room *Room, err error) {
	logger := NewLogger("Room")

	mediasoupRouter, err := worker.CreateRouter(config.Mediasoup.RouterOptions)
	if err != nil {
		logger.Err(err).Msg("create router")
		return
	}

	audioLevelObserver, err := mediasoupRouter.CreateAudioLevelObserver(func(o *mediasoup.AudioLevelObserverOptions) {
		o.MaxEntries = 1
		o.Threshold = -80
		o.Interval = 800
	})
	if err != nil {
		logger.Err(err).Msg("create audio level observer")
		return
	}

	bot, err := CreateBot(mediasoupRouter)
	if err != nil {
		return
	}

	return &Room{
		IEventEmitter:      eventemitter.NewEventEmitter(),
		logger:             logger,
		config:             config,
		roomId:             roomId,
		protooRoom:         protoo.NewRoom(),
		mediasoupRouter:    mediasoupRouter,
		audioLevelObserver: audioLevelObserver,
		bot:                bot,
	}, nil
}

func (r *Room) Close() {
	r.logger.Debug().Msg("close()")

	if atomic.CompareAndSwapUint32(&r.closed, 0, 1) {
		r.protooRoom.Close()
		r.mediasoupRouter.Close()
		r.bot.Close()

		r.SafeEmit("close")

		if r.networkThrottled {
			//TODO: throttle stop
		}
	}
}

func (r *Room) LogStatus() {
	dump, err := r.mediasoupRouter.Dump()
	if err != nil {
		r.logger.Err(err).Msg("LogStatus()")
		return
	}

	r.logger.Info().
		Str("roomId", r.roomId).
		Int("peers", len(r.getJoinedPeers(nil))).
		Int("transports", len(dump.TransportIds)).
		Msg("logStatus()")
}

func (r *Room) GetRouterRtpCapabilities() mediasoup.RtpCapabilities {
	return r.mediasoupRouter.RtpCapabilities()
}

func (r *Room) CreateBroadcaster(request CreateBroadcasterRequest) (data H, err error) {
	return
}

func (r *Room) DeleteBroadcaster(broadcasterId string) (err error) {
	return
}

func (r *Room) CreateBroadcasterTransport(request CreateBroadcasterTransportRequest) (rsp CreateBroadcasterTransportResponse, err error) {
	return
}

func (r *Room) ConnectBroadcasterTransport(request ConnectBroadcasterTransportRequest) (rsp ConnectBroadcasterTransportResponse, err error) {
	return
}

func (r *Room) CreateBroadcasterProducer(request CreateBroadcasterProducerRequest) (rsp CreateBroadcasterProducerResponse, err error) {
	return
}

func (r *Room) CreateBroadcasterConsumer(request CreateBroadcasterConsumerRequest) (rsp CreateBroadcasterConsumerResponse, err error) {
	return
}

func (r *Room) CreateBroadcasterDataConsumer(request CreateBroadcasterDataConsumerRequest) (rsp CreateBroadcasterDataConsumerResponse, err error) {
	return
}

func (r *Room) CreateBroadcasterDataProducer(request CreateBroadcasterDataProducerRequest) (rsp CreateBroadcasterDataProducerResponse, err error) {
	return
}

func (r *Room) HandleProtooConnection(peerId string, transport protoo.Transport) (err error) {
	existingPeer := r.protooRoom.GetPeer(peerId)
	if existingPeer != nil {
		r.logger.Warn().
			Str("peerId", peerId).
			Msg("handleProtooConnection() | there is already a protoo Peer with same peerId, closing it")

		existingPeer.Close()
	}

	peerData := &PeerData{
		Transports:    make(map[string]mediasoup.ITransport),
		Producers:     make(map[string]*mediasoup.Producer),
		Consumers:     make(map[string]*mediasoup.Consumer),
		DataProducers: make(map[string]*mediasoup.DataProducer),
		DataConsumers: make(map[string]*mediasoup.DataConsumer),
	}

	peer, err := r.protooRoom.CreatePeer(peerId, peerData, transport)
	if err != nil {
		r.logger.Err(err).Msg("protooRoom.createPeer() failed")
		return
	}

	locker := &sync.Mutex{}

	r.peerLockers.Store(peer.Id(), locker)

	peer.On("close", func() {
		r.peerLockers.Delete(peer.Id())
	})

	peer.On("request", func(request protoo.Message, accept func(data interface{}), reject func(err error)) {
		r.logger.Debug().Str("method", request.Method).Str("peerId", peerId).Msg(`protoo Peer "request" event`)

		locker.Lock()
		defer locker.Unlock()

		err := r.handleProtooRequest(peer, request, accept)
		if err != nil {
			reject(err)
		}
	})

	return

}

func (r *Room) handleAudioLevelObserver() {
	r.audioLevelObserver.On("volumes", func(volumes []mediasoup.AudioLevelObserverVolume) {
		producer := volumes[0].Producer
		volume := volumes[0].Volume

		r.logger.Debug().
			Str("producerId", producer.Id()).Int("volume", volume).
			Msg(`audioLevelObserver "volumes" event`)

		for _, peer := range r.getJoinedPeers(nil) {
			peer.Notify("activeSpeaker", map[string]interface{}{
				"peerId": producer.AppData().(H)["peerId"],
				"volume": volume,
			})
		}
	})

	r.audioLevelObserver.On("silence", func() {
		r.logger.Debug().Msg(`audioLevelObserver "silence" event`)

		for _, peer := range r.getJoinedPeers(nil) {
			peer.Notify("activeSpeaker", map[string]interface{}{
				"peerId": nil,
			})
		}
	})
}

func (r *Room) handleProtooRequest(peer *protoo.Peer, request protoo.Message, accept func(data interface{})) (err error) {
	peerData := peer.Data().(*PeerData)

	switch request.Method {
	case "getRouterRtpCapabilities":
		accept(r.GetRouterRtpCapabilities())

	case "join":
		if peerData.Joined {
			err = errors.New("Peer already joined")
			return
		}

		requestData := PeerData{}

		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}

		// Store client data into the protoo Peer data object.
		peerData.Joined = true
		peerData.DisplayName = requestData.DisplayName
		peerData.Device = requestData.Device
		peerData.RtpCapabilities = requestData.RtpCapabilities
		peerData.SctpCapabilities = requestData.SctpCapabilities

		joinedPeers := []*protoo.Peer{}

		for _, peer := range r.getJoinedPeers(nil) {
			joinedPeers = append(joinedPeers, peer)
		}

		r.broadcasters.Range(func(key, val interface{}) bool {
			peer := val.(*protoo.Peer)
			joinedPeers = append(joinedPeers, peer)

			return true
		})

		peerInfos := []*PeerInfo{}

		for _, joinedPeer := range joinedPeers {
			if joinedPeer.Id() == peer.Id() {
				continue
			}
			data := joinedPeer.Data().(*PeerData)
			peerInfos = append(peerInfos, &PeerInfo{
				Id:          joinedPeer.Id(),
				DisplayName: data.DisplayName,
				Device:      data.Device,
			})
		}

		accept(H{"peers": peerInfos})

		for _, joinedPeer := range joinedPeers {
			data := joinedPeer.Data().(*PeerData)

			// Create Consumers for existing Producers.
			for _, producer := range data.Producers {
				r.createConsumer(peer, joinedPeer, producer)
			}

			// 	// Create DataConsumers for existing DataProducers.
			for _, dataProducer := range data.DataProducers {
				r.createDataConsumer(peer, joinedPeer, dataProducer)
			}
		}

		// Create DataConsumers for bot DataProducer.
		r.createDataConsumer(peer, nil, r.bot.dataProducer)

		// // Notify the new Peer to all other Peers.
		for _, otherPeer := range r.getJoinedPeers(peer) {
			otherPeer.Notify("newPeer", &PeerInfo{
				Id:          peer.Id(),
				DisplayName: peerData.DisplayName,
				Device:      peerData.Device,
			})
		}

	case "createWebRtcTransport":
		{
			var requestData struct {
				ForceTcp         bool
				Producing        bool
				Consuming        bool
				SctpCapabilities *mediasoup.SctpCapabilities
			}
			if err = json.Unmarshal(request.Data, &requestData); err != nil {
				return
			}

			webRtcTransportOptions := mediasoup.WebRtcTransportOptions{}
			Clone(&webRtcTransportOptions, r.config.Mediasoup.WebRtcTransportOptions)

			webRtcTransportOptions.EnableSctp = requestData.SctpCapabilities != nil

			if requestData.SctpCapabilities != nil {
				webRtcTransportOptions.NumSctpStreams = requestData.SctpCapabilities.NumStreams
			}

			webRtcTransportOptions.AppData = &TransportData{
				Producing: requestData.Producing,
				Consuming: requestData.Consuming,
			}

			if requestData.ForceTcp {
				webRtcTransportOptions.EnableUdp = NewBool(false)
				webRtcTransportOptions.EnableTcp = true
			}

			transport, err := r.mediasoupRouter.CreateWebRtcTransport(webRtcTransportOptions)
			if err != nil {
				return err
			}
			transport.On("sctpstatechange", func(sctpState mediasoup.SctpState) {
				r.logger.Debug().Str("sctpState", string(sctpState)).Msg(`WebRtcTransport "sctpstatechange" event`)
			})
			transport.On("dtlsstatechange", func(dtlsState mediasoup.DtlsState) {
				if dtlsState == "failed" || dtlsState == "closed" {
					r.logger.Warn().Str("dtlsState", string(dtlsState)).Msg(`WebRtcTransport "dtlsstatechange" event`)
				}
			})

			// NOTE: For testing.
			// transport.EnableTraceEvent("probation", "bwe")
			if err = transport.EnableTraceEvent("bwe"); err != nil {
				return err
			}

			transport.On("trace", func(trace mediasoup.TransportTraceEventData) {
				r.logger.Debug().
					Str("transportId", transport.Id()).
					Str("trace.type", string(trace.Type)).
					Interface("trace", trace).
					Msg(`"transport "trace" event`)

				if trace.Type == "bwe" && trace.Direction == "out" {
					peer.Notify("downlinkBwe", trace.Info)
				}
			})

			// Store the WebRtcTransport into the protoo Peer data Object.
			peerData.Transports[transport.Id()] = transport

			accept(H{
				"id":             transport.Id(),
				"iceParameters":  transport.IceParameters(),
				"iceCandidates":  transport.IceCandidates(),
				"dtlsParameters": transport.DtlsParameters(),
				"sctpParameters": transport.SctpParameters(),
			})

			maxIncomingBitrate := r.config.Mediasoup.WebRtcTransportOptions.MaxIncomingBitrate

			if maxIncomingBitrate > 0 {
				transport.SetMaxIncomingBitrate(maxIncomingBitrate)
			}
		}

	case "connectWebRtcTransport":
		var requestData struct {
			TransportId    string                    `json:"transportId,omitempty"`
			DtlsParameters *mediasoup.DtlsParameters `json:"dtlsParameters,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		transport, ok := peerData.Transports[requestData.TransportId]
		if !ok {
			err = fmt.Errorf(`transport with id "%s" not found`, requestData.TransportId)
			return
		}
		transport.Connect(mediasoup.TransportConnectOptions{
			DtlsParameters: requestData.DtlsParameters,
		})
		accept(nil)

	case "restartIce":
		var requestData struct {
			TransportId string `json:"transportId,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		transport, ok := peerData.Transports[requestData.TransportId]
		if !ok {
			err = fmt.Errorf(`transport with id "%s" not found`, requestData.TransportId)
			return
		}
		iceParameters, err := transport.(*mediasoup.WebRtcTransport).RestartIce()
		if err != nil {
			return err
		}
		accept(iceParameters)

	case "produce":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			TransportId   string                  `json:"transportId,omitempty"`
			Kind          mediasoup.MediaKind     `json:"kind,omitempty"`
			RtpParameters mediasoup.RtpParameters `json:"rtpParameters,omitempty"`
			AppData       H                       `json:"appData,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		transport, ok := peerData.Transports[requestData.TransportId]
		if !ok {
			err = fmt.Errorf(`transport with id "%s" not found`, requestData.TransportId)
			return
		}
		// // Add peerId into appData to later get the associated Peer during
		// // the "loudest" event of the audioLevelObserver.
		appData := requestData.AppData
		if appData == nil {
			appData = H{}
		}
		appData["peerId"] = peer.Id()

		producer, err := transport.Produce(mediasoup.ProducerOptions{
			Kind:          requestData.Kind,
			RtpParameters: requestData.RtpParameters,
			AppData:       appData,
			// KeyFrameRequestDelay: 5000,
		})
		if err != nil {
			return err
		}
		// Store the Producer into the protoo Peer data Object.
		peerData.Producers[producer.Id()] = producer

		producer.On("score", func(score []mediasoup.ProducerScore) {
			r.logger.Debug().Str("producerId", producer.Id()).Interface("score", score).Msg(`producer "score" event`)

			peer.Notify("producerScore", H{
				"producerId": producer.Id(),
				"score":      score,
			})
		})
		producer.On("videoorientationchange", func(videoOrientation mediasoup.ProducerVideoOrientation) {
			r.logger.Debug().
				Str("producerId", producer.Id()).
				Interface("videoOrientation", videoOrientation).
				Msg(`producer "videoorientationchange" event`)
		})

		// NOTE: For testing.
		// producer.EnableTraceEvent("rtp", "keyframe", "nack", "pli", "fir");
		// producer.EnableTraceEvent("pli", "fir");
		// producer.EnableTraceEvent("keyframe");

		producer.On("trace", func(trace mediasoup.ProducerTraceEventData) {
			r.logger.Debug().
				Str("producerId", producer.Id()).
				Str("trace.type", string(trace.Type)).
				Interface("trace", trace).
				Msg(`producer "trace" event`)
		})

		accept(H{"id": producer.Id()})

		// Optimization: Create a server-side Consumer for each Peer.
		for _, otherPeer := range r.getJoinedPeers(peer) {
			r.createConsumer(otherPeer, peer, producer)
		}

		// // Add into the audioLevelObserver.
		if producer.Kind() == mediasoup.MediaKind_Audio {
			r.audioLevelObserver.AddProducer(producer.Id())
		}

	case "closeProducer":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ProducerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		producer, ok := peerData.Producers[requestData.ProducerId]
		if !ok {
			err = fmt.Errorf(`producer with id "%s" not found`, requestData.ProducerId)
			return
		}
		producer.Close()
		delete(peerData.Producers, producer.Id())

		accept(nil)

	case "pauseProducer":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ProducerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		producer, ok := peerData.Producers[requestData.ProducerId]
		if !ok {
			err = fmt.Errorf(`producer with id "%s" not found`, requestData.ProducerId)
			return
		}
		if err = producer.Pause(); err != nil {
			return
		}

		accept(nil)

	case "resumeProducer":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ProducerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		producer, ok := peerData.Producers[requestData.ProducerId]
		if !ok {
			err = fmt.Errorf(`producer with id "%s" not found`, requestData.ProducerId)
			return
		}
		if err = producer.Resume(); err != nil {
			return
		}

		accept(nil)

	case "pauseConsumer":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		if err = consumer.Pause(); err != nil {
			return
		}

		accept(nil)

	case "resumeConsumer":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		if err = consumer.Resume(); err != nil {
			return
		}

		accept(nil)

	case "setConsumerPreferredLayers":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			mediasoup.ConsumerLayers
			ConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		if err = consumer.SetPreferredLayers(requestData.ConsumerLayers); err != nil {
			return
		}

		accept(nil)

	case "setConsumerPriority":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ConsumerId string
			Priority   uint32
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		if err = consumer.SetPriority(requestData.Priority); err != nil {
			return
		}

		accept(nil)

	case "requestConsumerKeyFrame":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			ConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		if err = consumer.RequestKeyFrame(); err != nil {
			return
		}

		accept(nil)

	case "produceData":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			TransportId          string                          `json:"transportId,omitempty"`
			SctpStreamParameters *mediasoup.SctpStreamParameters `json:"sctpStreamParameters,omitempty"`
			Label                string                          `json:"label,omitempty"`
			Protocol             string                          `json:"protocol,omitempty"`
			AppData              H                               `json:"appData,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		transport, ok := peerData.Transports[requestData.TransportId]
		if !ok {
			err = fmt.Errorf(`transport with id "%s" not found`, requestData.TransportId)
			return
		}
		dataProducer, err := transport.ProduceData(mediasoup.DataProducerOptions{
			SctpStreamParameters: requestData.SctpStreamParameters,
			Label:                requestData.Label,
			Protocol:             requestData.Protocol,
			AppData:              requestData.AppData,
		})
		if err != nil {
			return err
		}
		peerData.DataProducers[dataProducer.Id()] = dataProducer

		accept(H{"id": dataProducer.Id()})

		switch dataProducer.Label() {
		case "chat":
			// Create a server-side DataConsumer for each Peer.
			for _, otherPeer := range r.getJoinedPeers(peer) {
				r.createDataConsumer(otherPeer, peer, dataProducer)
			}

		case "bot":
			// Pass it to the bot.
			r.bot.HandlePeerDataProducer(dataProducer.Id(), peer)
		}

	case "changeDisplayName":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			DisplayName string `json:"displayName,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		oldDisplayName := peerData.DisplayName
		peerData.DisplayName = requestData.DisplayName

		// Notify other joined Peers.
		for _, otherPeer := range r.getJoinedPeers(peer) {
			otherPeer.Notify("peerDisplayNameChanged", H{
				"peerId":         peer.Id(),
				"displayName":    requestData.DisplayName,
				"oldDisplayName": oldDisplayName,
			})
		}

		accept(nil)

	case "getTransportStats":
		// Ensure the Peer is joined.
		if !peerData.Joined {
			err = errors.New("Peer not yet joined")
			return
		}
		var requestData struct {
			TransportId string `json:"transportId,omitempty"`
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		transport, ok := peerData.Transports[requestData.TransportId]
		if !ok {
			err = fmt.Errorf(`transport with id "%s" not found`, requestData.TransportId)
			return
		}
		stats, err := transport.GetStats()
		if err != nil {
			return err
		}

		accept(stats)

	case "getProducerStats":
		var requestData struct {
			ProducerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		producer, ok := peerData.Producers[requestData.ProducerId]
		if !ok {
			err = fmt.Errorf(`producer with id "%s" not found`, requestData.ProducerId)
			return
		}
		stats, err := producer.GetStats()
		if err != nil {
			return err
		}

		accept(stats)

	case "getConsumerStats":
		var requestData struct {
			ConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		consumer, ok := peerData.Consumers[requestData.ConsumerId]
		if !ok {
			err = fmt.Errorf(`consumer with id "%s" not found`, requestData.ConsumerId)
			return
		}
		stats, err := consumer.GetStats()
		if err != nil {
			return err
		}

		accept(stats)

	case "getDataProducerStats":
		var requestData struct {
			DataProducerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		dataProducer, ok := peerData.DataProducers[requestData.DataProducerId]
		if !ok {
			err = fmt.Errorf(`dataProducer with id "%s" not found`, requestData.DataProducerId)
			return
		}
		stats, err := dataProducer.GetStats()
		if err != nil {
			return err
		}

		accept(stats)

	case "getDataConsumerStats":
		var requestData struct {
			DataConsumerId string
		}
		if err = json.Unmarshal(request.Data, &requestData); err != nil {
			return
		}
		dataConsumer, ok := peerData.DataConsumers[requestData.DataConsumerId]
		if !ok {
			err = fmt.Errorf(`dataConsumer with id "%s" not found`, requestData.DataConsumerId)
			return
		}
		stats, err := dataConsumer.GetStats()
		if err != nil {
			return err
		}

		accept(stats)

	case "applyNetworkThrottle":
		//TODO: throttle.start

	case "resetNetworkThrottle":
		//TODO: throttle.stop

	}

	return
}

func (r *Room) createConsumer(consumerPeer, producerPeer *protoo.Peer, producer *mediasoup.Producer) {
	// Optimization:
	// - Create the server-side Consumer in paused mode.
	// - Tell its Peer about it and wait for its response.
	// - Upon receipt of the response, resume the server-side Consumer.
	// - If video, this will mean a single key frame requested by the
	//   server-side Consumer (when resuming it).
	// - If audio (or video), it will avoid that RTP packets are received by the
	//   remote endpoint *before* the Consumer is locally created in the endpoint
	//   (and before the local SDP O/A procedure ends). If that happens (RTP
	//   packets are received before the SDP O/A is done) the PeerConnection may
	//   fail to associate the RTP stream.

	consumerPeerData := consumerPeer.Data().(*PeerData)

	// NOTE: Don"t create the Consumer if the remote Peer cannot consume it.
	if consumerPeerData.RtpCapabilities == nil ||
		!r.mediasoupRouter.CanConsume(producer.Id(), *consumerPeerData.RtpCapabilities) {
		return
	}

	// Must take the Transport the remote Peer is using for consuming.
	var transport mediasoup.ITransport

	for _, t := range consumerPeerData.Transports {
		if data, ok := t.AppData().(*TransportData); ok && data.Consuming {
			transport = t
			break
		}
	}

	// This should not happen.
	if transport == nil {
		r.logger.Warn().Msg("createConsumer() | Transport for consuming not found")
		return
	}

	consumer, err := transport.Consume(mediasoup.ConsumerOptions{
		ProducerId:      producer.Id(),
		RtpCapabilities: *consumerPeerData.RtpCapabilities,
		Paused:          true,
	})
	if err != nil {
		r.logger.Err(err).Msg("createConsumer() | transport.consume()")
		return
	}

	// Store the Consumer into the protoo consumerPeer data Object.
	consumerPeerData.Consumers[consumer.Id()] = consumer

	// Set Consumer events.
	consumer.On("transportclose", func() {
		locker := r.getPeerLocker(consumerPeer.Id())
		locker.Lock()
		defer locker.Unlock()
		// Remove from its map.
		delete(consumerPeerData.Consumers, consumer.Id())
	})
	consumer.On("producerclose", func() {
		locker := r.getPeerLocker(consumerPeer.Id())
		locker.Lock()
		defer locker.Unlock()
		// Remove from its map.
		delete(consumerPeerData.Consumers, consumer.Id())
		consumerPeer.Notify("consumerClosed", H{
			"consumerId": consumer.Id(),
		})
	})
	consumer.On("producerpause", func() {
		consumerPeer.Notify("consumerPaused", H{
			"consumerId": consumer.Id(),
		})
	})
	consumer.On("producerresume", func() {
		consumerPeer.Notify("consumerResumed", H{
			"consumerId": consumer.Id(),
		})
	})
	consumer.On("score", func(score mediasoup.ConsumerScore) {
		r.logger.Debug().
			Str("consumerId", consumer.Id()).
			Interface("score", score).Msg(`consumer "score" event`)

		consumerPeer.Notify("consumerScore", H{
			"consumerId": consumer.Id(),
			"score":      score,
		})
	})
	consumer.On("layerschange", func(layers *mediasoup.ConsumerLayers) {
		notifyData := H{
			"consumerId": consumer.Id(),
		}
		if layers != nil {
			notifyData["spatialLayer"] = layers.SpatialLayer
			notifyData["temporalLayer"] = layers.TemporalLayer
		}
		consumerPeer.Notify("consumerLayersChanged", notifyData)
	})

	// NOTE: For testing.
	// consumer.EnableTraceEvent("rtp", "keyframe", "nack", "pli", "fir");
	// consumer.EnableTraceEvent("pli", "fir");
	// consumer.EnableTraceEvent("keyframe");

	consumer.On("trace", func(trace mediasoup.ConsumerTraceEventData) {
		r.logger.Debug().
			Str("consumerId", consumer.Id()).
			Str("trace.type", string(trace.Type)).
			Interface("trace", trace).
			Msg(`consumer "trace" event`)
	})

	// Send a protoo request to the remote Peer with Consumer parameters.
	rsp := consumerPeer.Request("newConsumer", H{
		"peerId":         producerPeer.Id(),
		"producerId":     producer.Id(),
		"id":             consumer.Id(),
		"kind":           consumer.Kind(),
		"rtpParameters":  consumer.RtpParameters(),
		"type":           consumer.Type(),
		"appData":        consumer.AppData(),
		"producerPaused": consumer.ProducerPaused(),
	})
	if rsp.Err() != nil {
		r.logger.Warn().Err(rsp.Err()).Msg("createConsumer() | failed")
		return
	}

	// Now that we got the positive response from the remote endpoint, resume
	// the Consumer so the remote endpoint will receive the a first RTP packet
	// of this new stream once its PeerConnection is already ready to process
	// and associate it.
	if err = consumer.Resume(); err != nil {
		r.logger.Warn().Err(err).Msg("createConsumer() | failed")
		return
	}

	consumerPeer.Notify("consumerScore", H{
		"consumerId": consumer.Id(),
		"score":      consumer.Score(),
	})
}

func (r *Room) createDataConsumer(dataConsumerPeer, dataProducerPeer *protoo.Peer, dataProducer *mediasoup.DataProducer) {
	dataConsumerPeerData := dataConsumerPeer.Data().(*PeerData)

	// NOTE: Don't create the DataConsumer if the remote Peer cannot consume it.
	if dataConsumerPeerData.SctpCapabilities == nil {
		return
	}

	// Must take the Transport the remote Peer is using for consuming.
	var transport mediasoup.ITransport

	for _, t := range dataConsumerPeerData.Transports {
		if data, ok := t.AppData().(*TransportData); ok && data.Consuming {
			transport = t
			break
		}
	}

	// This should not happen.
	if transport == nil {
		r.logger.Warn().Msg("createDataConsumer() | Transport for consuming not found")
		return
	}

	dataConsumer, err := transport.ConsumeData(mediasoup.DataConsumerOptions{
		DataProducerId: dataProducer.Id(),
	})
	if err != nil {
		r.logger.Err(err).Msg("createDataConsumer() | transport.consumeData()")
		return
	}

	// Store the Consumer into the protoo consumerPeer data Object.
	dataConsumerPeerData.DataConsumers[dataConsumer.Id()] = dataConsumer

	// Set DataConsumer events.
	dataConsumer.On("transportclose", func() {
		locker := r.getPeerLocker(dataConsumerPeer.Id())
		locker.Lock()
		defer locker.Unlock()
		// Remove from its map.
		delete(dataConsumerPeerData.Consumers, dataConsumer.Id())
	})
	dataConsumer.On("dataproducerclose", func() {
		locker := r.getPeerLocker(dataConsumerPeer.Id())
		locker.Lock()
		defer locker.Unlock()
		// Remove from its map.
		delete(dataConsumerPeerData.Consumers, dataConsumer.Id())
		dataConsumerPeer.Notify("dataConsumerClosed", H{
			"dataConsumerId": dataConsumer.Id(),
		})
	})

	var peerId *string

	if dataProducerPeer != nil {
		id := dataProducerPeer.Id()
		peerId = &id
	}

	// Send a protoo request to the remote Peer with Consumer parameters.
	rsp := dataConsumerPeer.Request("newDataConsumer", H{
		// This is null for bot DataProducer.
		"peerId":               peerId,
		"dataProducerId":       dataProducer.Id(),
		"id":                   dataConsumer.Id(),
		"sctpStreamParameters": dataConsumer.SctpStreamParameters(),
		"label":                dataConsumer.Label(),
		"protocol":             dataConsumer.Protocol(),
		"appData":              dataConsumer.AppData(),
	})
	if rsp.Err() != nil {
		r.logger.Warn().Err(rsp.Err()).Msg("createDataConsumer() | failed")
	}
}

func (r *Room) getJoinedPeers(excludePeer *protoo.Peer) (peers []*protoo.Peer) {
	for _, peer := range r.protooRoom.Peers() {
		if peer == excludePeer {
			continue
		}
		peers = append(peers, peer)
	}
	return
}

func (r *Room) getPeerLocker(peerId string) sync.Locker {
	val, ok := r.peerLockers.Load(peerId)
	if !ok {
		return nil
	}
	return val.(sync.Locker)
}

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jiyeyuran/go-protoo/transport"
	"github.com/jiyeyuran/mediasoup-demo/internal/proto"
	"github.com/jiyeyuran/mediasoup-go"
	"github.com/rs/zerolog"
)

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"protoo"},
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Server struct {
	logger                 zerolog.Logger
	locker                 sync.Mutex
	config                 Config
	rooms                  sync.Map
	mediasoupWorkers       []*mediasoup.Worker
	nextMediasoupWorkerIdx int
}

func NewServer(config Config) *Server {
	workers := []*mediasoup.Worker{}
	logger := NewLogger("Server")

	for i := 0; i < config.Mediasoup.NumWorkers; i++ {
		worker, err := mediasoup.NewWorker(
			mediasoup.WithLogLevel(config.Mediasoup.WorkerSettings.LogLevel),
			mediasoup.WithLogTags(config.Mediasoup.WorkerSettings.LogTags),
			mediasoup.WithRtcMinPort(config.Mediasoup.WorkerSettings.RtcMinPort),
			mediasoup.WithRtcMaxPort(config.Mediasoup.WorkerSettings.RtcMaxPort),
		)
		if err != nil {
			panic(err)
		}

		worker.On("died", func(err error) {
			logger.Err(err).Msg("mediasoup Worker died, exiting in 2 seconds...")
			time.AfterFunc(2*time.Second, func() {
				os.Exit(1)
			})
		})

		workers = append(workers, worker)

		go func() {
			ticker := time.NewTicker(120 * time.Second)
			for {
				select {
				case <-ticker.C:
					usage, err := worker.GetResourceUsage()
					if err != nil {
						logger.Err(err).Int("pid", worker.Pid()).Msg("mediasoup Worker resource usage")
						continue
					}
					logger.Info().Int("pid", worker.Pid()).Interface("usage", usage).Msg("mediasoup Worker resource usage")
				}
			}
		}()
	}

	return &Server{
		logger:           logger,
		config:           config,
		mediasoupWorkers: workers,
	}
}

func (s *Server) Run() {
	r := gin.Default()

	r.Use(cors.Default())
	r.Use(gin.ErrorLogger())

	/**
	 * For every API request, verify that the roomId in the path matches and
	 * existing room.
	 */
	r.Use(func(c *gin.Context) {
		if roomId := c.Params.ByName("roomId"); len(roomId) > 0 {
			room, ok := s.rooms.Load(roomId)
			if ok {
				c.Set("room", room)
			} else {
				c.AbortWithError(404, fmt.Errorf(`room with id "%s" not found`, roomId))
			}
		}
	})

	/**
	 * API GET resource that returns the mediasoup Router RTP capabilities of
	 * the room.
	 */
	r.GET("/rooms/:roomId", func(c *gin.Context) {
		c.JSON(200, s.getRoom(c).GetRouterRtpCapabilities())
	})

	/**
	 * POST API to create a Broadcaster.
	 */
	r.POST("/rooms/:roomId/broadcasters", func(c *gin.Context) {
		request := proto.PeerInfo{}

		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcaster(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	/**
	 * DELETE API to delete a Broadcaster.
	 */
	r.DELETE("/rooms/:roomId/broadcasters/:broadcasterId", func(c *gin.Context) {
		broadcasterId := c.Params.ByName("broadcasterId")
		s.getRoom(c).DeleteBroadcaster(broadcasterId)
	})

	/**
	 * POST API to create a mediasoup Transport associated to a Broadcaster.
	 * It can be a PlainTransport or a WebRtcTransport depending on the
	 * type parameters in the body. There are also additional parameters for
	 * PlainTransport.
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports", func(c *gin.Context) {
		request := proto.CreateBroadcasterTransportRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
		}

		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcasterTransport(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	/**
	 * POST API to connect a Transport belonging to a Broadcaster. Not needed
	 * for PlainTransport if it was created with comedia option set to true.
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports/:transportId/connect", func(c *gin.Context) {
		request := proto.ConnectBroadcasterTransportRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
			TransportId:   c.Params.ByName("transportId"),
		}
		if c.BindJSON(&request) != nil {
			return
		}
		err := s.getRoom(c).ConnectBroadcasterTransport(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
	})

	/**
	 * POST API to create a mediasoup Producer associated to a Broadcaster.
	 * The exact Transport in which the Producer must be created is signaled in
	 * the URL path. Body parameters include kind and rtpParameters of the
	 * Producer.
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports/:transportId/producers", func(c *gin.Context) {
		request := proto.CreateBroadcasterProducerRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
			TransportId:   c.Params.ByName("transportId"),
		}
		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcasterProducer(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	/**
	 * POST API to create a mediasoup Consumer associated to a Broadcaster.
	 * The exact Transport in which the Consumer must be created is signaled in
	 * the URL path. Query parameters must include the desired producerId to
	 * consume.
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports/:transportId/consume", func(c *gin.Context) {
		request := proto.CreateBroadcasterConsumerRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
			TransportId:   c.Params.ByName("transportId"),
		}
		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcasterConsumer(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	/**
	 * POST API to create a mediasoup DataConsumer associated to a Broadcaster.
	 * The exact Transport in which the DataConsumer must be created is signaled in
	 * the URL path. Query body must include the desired producerId to
	 * consume.
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports/:transportId/consume/data", func(c *gin.Context) {
		request := proto.CreateBroadcasterDataConsumerRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
			TransportId:   c.Params.ByName("transportId"),
		}
		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcasterDataConsumer(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	/**
	 * POST API to create a mediasoup DataProducer associated to a Broadcaster.
	 * The exact Transport in which the DataProducer must be created is signaled in
	 */
	r.POST("/rooms/:roomId/broadcasters/:broadcasterId/transports/:transportId/produce/data", func(c *gin.Context) {
		request := proto.CreateBroadcasterDataProducerRequest{
			BroadcasterId: c.Params.ByName("broadcasterId"),
			TransportId:   c.Params.ByName("transportId"),
		}
		if c.BindJSON(&request) != nil {
			return
		}
		rsp, err := s.getRoom(c).CreateBroadcasterDataProducer(request)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, rsp)
	})

	// serve web
	r.Static("/web", "./public")

	pprof.Register(r)

	// setup websocket
	r.GET("/", s.runProtooWebSocketServer)

	addr := fmt.Sprintf("%s:%d", s.config.Https.ListenIp, s.config.Https.ListenPort)
	r.RunTLS(addr, s.config.Https.TLS.Cert, s.config.Https.TLS.Key)
}

func (s *Server) getRoom(c *gin.Context) *Room {
	room, _ := c.Get("room")

	return room.(*Room)
}

func (s *Server) getMediasoupWorker() *mediasoup.Worker {
	worker := s.mediasoupWorkers[s.nextMediasoupWorkerIdx]

	if s.nextMediasoupWorkerIdx == len(s.mediasoupWorkers) {
		s.nextMediasoupWorkerIdx = 0
	}

	return worker
}

func (s *Server) getOrCreateRoom(roomId string) (room *Room, err error) {
	s.locker.Lock()
	defer s.locker.Unlock()

	val, ok := s.rooms.Load(roomId)
	if ok {
		return val.(*Room), nil
	}

	room, err = CreateRoom(s.config, roomId, s.getMediasoupWorker())
	if err != nil {
		return
	}

	s.rooms.Store(roomId, room)

	room.On("close", func() {
		s.rooms.Delete(roomId)
	})

	return
}

func (s *Server) runProtooWebSocketServer(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}

	roomId := c.Query("roomId")
	peerId := c.Query("peerId")

	if len(roomId) == 0 || len(peerId) == 0 {
		c.AbortWithError(400, errors.New("Connection request without roomId and/or peerId"))
		return
	}

	s.logger.Info().
		Str("roomId", roomId).
		Str("peerId", peerId).
		Str("address", c.ClientIP()).
		Str("origin", c.GetHeader("Origin")).
		Msg("protoo connection request")

	room, err := s.getOrCreateRoom(roomId)
	if err != nil {
		s.logger.Err(err).Msg("getOrCreateRoom")
		c.AbortWithError(500, err)
		return
	}
	transport := transport.NewWebsocketTransport(conn)

	room.HandleProtooConnection(peerId, transport)

	if err := transport.Run(); err != nil {
		s.logger.Err(err).Msg("transport.run")
	}
}

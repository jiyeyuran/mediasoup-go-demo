package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jiyeyuran/go-protoo/transport"
	"github.com/jiyeyuran/mediasoup-demo/internal/proto"
	mediasoup "github.com/jiyeyuran/mediasoup-go/v2"
)

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"protoo"},
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Server struct {
	logger                 *slog.Logger
	locker                 sync.Mutex
	config                 *Config
	rooms                  sync.Map
	mediasoupWorkers       []*mediasoup.Worker
	nextMediasoupWorkerIdx int
}

func NewServer(config *Config) *Server {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				source.File = filepath.Base(source.File)
			}
			return a
		},
	}))

	workerBin := os.Getenv("MEDIASOUP_WORKER_BIN")
	workers := []*mediasoup.Worker{}

	for i := 0; i < config.Mediasoup.NumWorkers; i++ {
		worker, err := mediasoup.NewWorker(workerBin, func(options *mediasoup.WorkerSettings) {
			*options = *config.Mediasoup.WorkerSettings
		})
		if err != nil {
			panic(err)
		}

		worker.OnClose(func() {
			logger.Info("mediasoup Worker closed")
		})
		workers = append(workers, worker)

		go func() {
			ticker := time.NewTicker(120 * time.Second)
			for {
				select {
				case <-ticker.C:
					usage, err := worker.GetResourceUsage()
					if err != nil {
						logger.Error("mediasoup Worker getResourceUsage", "pid", worker.Pid(), "error", err)
						continue
					}
					logger.Info("mediasoup Worker resource usage", "pid", worker.Pid(), "usage", usage)
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

func (s *Server) getOrCreateRoom(roomId string) (room *Room, err error) {
	s.locker.Lock()
	defer s.locker.Unlock()

	val, ok := s.rooms.Load(roomId)
	if ok {
		return val.(*Room), nil
	}

	worker := s.mediasoupWorkers[s.nextMediasoupWorkerIdx]

	room, err = CreateRoom(s.config, roomId, worker, s.logger)
	if err != nil {
		return
	}

	s.rooms.Store(roomId, room)

	room.OnClose(func() {
		s.rooms.Delete(roomId)
	})

	s.nextMediasoupWorkerIdx = (s.nextMediasoupWorkerIdx + 1) % len(s.mediasoupWorkers)

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

	s.logger.Info("protoo connection request",
		"roomId", roomId, "peerId", peerId, "address", c.ClientIP(), "origin", c.GetHeader("Origin"))

	room, err := s.getOrCreateRoom(roomId)
	if err != nil {
		s.logger.Error("getOrCreateRoom", "error", err)
		c.AbortWithError(500, err)
		return
	}
	transport := transport.NewWebsocketTransport(conn)

	room.HandleProtooConnection(peerId, transport)

	if err := transport.Run(); err != nil {
		s.logger.Error("transport.run", "error", err)
	}
}

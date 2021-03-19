package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jiyeyuran/mediasoup-go"
	"github.com/jiyeyuran/mediasoup-go/h264"
)

type Config struct {
	Domain    string          `json:"domain,omitempty"`
	Https     HTTPSConfig     `json:"https,omitempty"`
	Mediasoup MediasoupConfig `json:"mediasoup,omitempty"`
}

type HTTPSConfig struct {
	ListenIp   string    `json:"listenIP,omitempty"`
	ListenPort uint16    `json:"listenPort,omitempty"`
	TLS        TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

type MediasoupConfig struct {
	NumWorkers             int                             `json:"numWorkers,omitempty"`
	WorkerSettings         mediasoup.WorkerSettings        `json:"workerSettings,omitempty"`
	RouterOptions          mediasoup.RouterOptions         `json:"routerOptions,omitempty"`
	WebRtcTransportOptions WebRtcTransportOptions          `json:"webRtcTransportOptions,omitempty"`
	PlainTransportOptions  mediasoup.PlainTransportOptions `json:"plainTransportOptions,omitempty"`
}

type WebRtcTransportOptions struct {
	/**
	 * Listening IP address or addresses in order of preference (first one is the
	 * preferred one).
	 */
	ListenIps []mediasoup.TransportListenIp `json:"listenIps,omitempty"`

	/**
	 * Listen in UDP. Default true.
	 */
	EnableUdp *bool `json:"enableUdp,omitempty"`

	/**
	 * Listen in TCP. Default false.
	 */
	EnableTcp bool `json:"enableTcp,omitempty"`

	/**
	 * Prefer UDP. Default false.
	 */
	PreferUdp bool `json:"preferUdp,omitempty"`

	/**
	 * Prefer TCP. Default false.
	 */
	PreferTcp bool `json:"preferTcp,omitempty"`

	/**
	 * Initial available outgoing bitrate (in bps). Default 600000.
	 */
	InitialAvailableOutgoingBitrate uint32 `json:"initialAvailableOutgoingBitrate,omitempty"`

	/**
	 * Create a SCTP association. Default false.
	 */
	EnableSctp bool `json:"enableSctp,omitempty"`

	/**
	 * SCTP streams uint32.
	 */
	NumSctpStreams mediasoup.NumSctpStreams `json:"numSctpStreams,omitempty"`

	/**
	 * Maximum allowed size for SCTP messages sent by DataProducers.
	 * Default 262144.
	 */
	MaxSctpMessageSize int `json:"maxSctpMessageSize,omitempty"`

	/**
	 * Maximum SCTP send buffer used by DataConsumers.
	 * Default 262144.
	 */
	SctpSendBufferSize int `json:"sctpSendBufferSize,omitempty"`

	/**
	 * Custom application data.
	 */
	AppData interface{} `json:"appData,omitempty"`

	// Additional options that are not part of WebRtcTransportOptions.
	MaxIncomingBitrate int `json:"maxIncomingBitrate,omitempty"`
}

var (
	dirname, _    = filepath.Abs(filepath.Dir(os.Args[0]))
	DefaultConfig = Config{
		Domain: "localhost",
		Https: HTTPSConfig{
			ListenIp:   "0.0.0.0",
			ListenPort: 4443,
			TLS: TLSConfig{
				Cert: filepath.Join(dirname, "certs", "fullchain.pem"),
				Key:  filepath.Join(dirname, "certs", "privkey.key"),
			},
		},
		Mediasoup: MediasoupConfig{
			NumWorkers: runtime.NumCPU(),
			WorkerSettings: mediasoup.WorkerSettings{
				LogLevel: "warn",
				LogTags: []mediasoup.WorkerLogTag{
					"info",
					"ice",
					"dtls",
					"rtp",
					"srtp",
					"rtcp",
					"rtx",
					"bwe",
					"score",
					"simulcast",
					"svc",
					"sctp",
				},
				RtcMinPort: 40000,
				RtcMaxPort: 49999,
			},
			RouterOptions: mediasoup.RouterOptions{
				MediaCodecs: []*mediasoup.RtpCodecCapability{
					{
						Kind:      mediasoup.MediaKind_Audio,
						MimeType:  "audio/opus",
						ClockRate: 48000,
						Channels:  2,
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/VP8",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/VP9",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							ProfileId:           "2",
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							RtpParameter: h264.RtpParameter{
								PacketizationMode:     1,
								ProfileLevelId:        "42e01f",
								LevelAsymmetryAllowed: 1,
							},
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							RtpParameter: h264.RtpParameter{
								PacketizationMode:     1,
								ProfileLevelId:        "4d0032",
								LevelAsymmetryAllowed: 1,
							},
							XGoogleStartBitrate: 1000,
						},
					},
				},
			},
			WebRtcTransportOptions: WebRtcTransportOptions{
				ListenIps: []mediasoup.TransportListenIp{
					{
						Ip:          GetOutboundIP(),
						AnnouncedIp: GetOutboundIP(),
					},
				},
				InitialAvailableOutgoingBitrate: 1000000,
				MaxSctpMessageSize:              262144,
				MaxIncomingBitrate:              1500000,
			},
			PlainTransportOptions: mediasoup.PlainTransportOptions{
				ListenIp: mediasoup.TransportListenIp{
					Ip:          GetOutboundIP(),
					AnnouncedIp: GetOutboundIP(),
				},
				MaxSctpMessageSize: 262144,
			},
		},
	}
)

// Get preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

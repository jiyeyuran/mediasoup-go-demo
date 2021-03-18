package main

import (
	"errors"
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Options struct {
	ServerURL  string
	RoomID     string
	MediaFile  string
	VerifyCert bool
}

func NewOptions() *Options {
	serverURL := "https://localhost:4443"

	if os.Getenv("SERVER_URL") != "" {
		serverURL = os.Getenv("SERVER_URL")
	}

	return &Options{
		ServerURL: serverURL,
		RoomID:    os.Getenv("ROOM_ID"),
		MediaFile: os.Getenv("MEDIA_FILE"),
	}
}

func (options *Options) Verify() error {
	if len(options.ServerURL) == 0 {
		return errors.New("missing SERVER_URL")
	}
	if len(options.RoomID) == 0 {
		return errors.New("missing ROOM_ID")
	}
	if len(options.MediaFile) == 0 {
		return errors.New("missing MEDIA_FILE")
	}
	return nil
}

func broadcastersFlagSet(opts *Options) *flag.FlagSet {
	flagSet := flag.NewFlagSet("broadcasters", flag.ExitOnError)

	flagSet.StringVar(&opts.ServerURL, "server_url", opts.ServerURL, "the URL of the mediasoup-demo API server")
	flagSet.StringVar(&opts.ServerURL, "s", opts.ServerURL, "the URL of the mediasoup-demo API server")
	flagSet.StringVar(&opts.RoomID, "room_id", opts.RoomID, "the id of the mediasoup-demo room (it must exist in advance)")
	flagSet.StringVar(&opts.RoomID, "r", opts.RoomID, "the id of the mediasoup-demo room (it must exist in advance)")
	flagSet.StringVar(&opts.MediaFile, "media_file", opts.MediaFile, "the path to a audio+video file (such as a .mp4 file)")
	flagSet.StringVar(&opts.MediaFile, "m", opts.MediaFile, "the path to a audio+video file (such as a .mp4 file)")
	flagSet.BoolVar(&opts.VerifyCert, "verify_cert", opts.VerifyCert, "whether verify tls certification or not")

	return flagSet
}

func main() {
	opts := NewOptions()

	flagSet := broadcastersFlagSet(opts)
	flagSet.Parse(os.Args[1:])

	if err := opts.Verify(); err != nil {
		panic(err)
	}

	rand.Seed(time.Now().UTC().UnixNano())

	broadcaster := NewBroadcaster(opts)

	go func() {
		if err := broadcaster.Start(); err != nil {
			log.Fatalf("error: %#v", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	broadcaster.Stop()
}

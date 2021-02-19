# mediasoup-go-demo

A demo application of [mediasoup-go](https://github.com/jiyeyuran/mediasoup-go).


## Installation

* Clone the project:

```bash
$ git clone https://github.com/jiyeyuran/mediasoup-go-demo.git
$ cd mediasoup-go-demo
```

* Set up the mediasoup-go-demo server:

```bash
$ cd server
$ go build
```

* Make sure TLS certificates reside in `server/certs` directory with names `fullchain.pem` and `privkey.pem`.

* Within your server, run the application by setting the `DEBUG` environment variable according to your needs ([more info](https://mediasoup.org/documentation/v3/mediasoup/debugging/)):

```bash
$ DEBUG="*mediasoup* *ERROR* *WARN*" MEDIASOUP_WORKER_BIN="the path of worker bin" ./server
```

* Set up the mediasoup-go-demo browser app:

```bash
$ cd app
$ npm install
```

## Run it locally

* Run the Node.js server application in a terminal:

```bash
$ cd server
$ MEDIASOUP_WORKER_BIN="the path of worker bin" ./server
```

* In a different terminal build and run the browser application:

```bash
$ cd app
$ npm start
```

* Enjoy.

## License

MIT

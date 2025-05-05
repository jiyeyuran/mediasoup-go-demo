import domready from 'domready';
import UrlParse from 'url-parse';
import React from 'react';
import { createRoot } from 'react-dom/client';
import { Provider } from 'react-redux';
import {
	applyMiddleware as applyReduxMiddleware,
	createStore as createReduxStore,
} from 'redux';
import thunk from 'redux-thunk';
import randomString from 'random-string';
import * as faceapi from 'face-api.js';
import Logger from './Logger';
import * as utils from './utils';
import randomName from './randomName';
import deviceInfo from './deviceInfo';
import RoomClient from './RoomClient';
import RoomContext from './RoomContext';
import * as cookiesManager from './cookiesManager';
import * as stateActions from './redux/stateActions';
import reducers from './redux/reducers';
import Room from './components/Room';
import './scss/index.scss';

const logger = new Logger();
const reduxMiddlewares = [thunk];

let roomClient;
const store = createReduxStore(
	reducers,
	undefined,
	applyReduxMiddleware(...reduxMiddlewares)
);

window.STORE = store;

RoomClient.init({ store });

domready(async () => {
	logger.debug('DOM ready');

	await utils.initialize();

	run();
});

window.RUN = run;

async function run() {
	logger.debug('run() [environment:%s]', process.env.NODE_ENV);

	if (window.CLIENT) {
		window.CLIENT.close();

		// eslint-disable-next-line require-atomic-updates
		window.CLIENT = undefined;
		// eslint-disable-next-line require-atomic-updates
		window.CC = undefined;
	}

	const urlParser = new UrlParse(window.location.href, true);
	const peerId = randomString({ length: 8 }).toLowerCase();
	let roomId = urlParser.query.roomId;
	let displayName =
		urlParser.query.displayName || (cookiesManager.getUser() || {}).displayName;
	const handlerName = urlParser.query.handlerName || urlParser.query.handler;
	const forceTcp = urlParser.query.forceTcp === 'true';
	const produce = urlParser.query.produce !== 'false';
	const consume = urlParser.query.consume !== 'false';
	const datachannel = urlParser.query.datachannel !== 'false';
	const forceVP8 = urlParser.query.forceVP8 === 'true';
	const forceH264 = urlParser.query.forceH264 === 'true';
	const forceVP9 = urlParser.query.forceVP9 === 'true';
	const forceAV1 = urlParser.query.forceAV1 === 'true';
	const enableWebcamLayers = urlParser.query.enableWebcamLayers !== 'false';
	const enableSharingLayers = urlParser.query.enableSharingLayers !== 'false';
	const webcamScalabilityMode = urlParser.query.webcamScalabilityMode;
	const sharingScalabilityMode = urlParser.query.sharingScalabilityMode;
	const numSimulcastStreams = urlParser.query.numSimulcastStreams
		? Number(urlParser.query.numSimulcastStreams)
		: 3;
	const info = urlParser.query.info === 'true';
	const faceDetection = urlParser.query.faceDetection === 'true';
	const externalVideo = urlParser.query.externalVideo === 'true';
	const throttleSecret = urlParser.query.throttleSecret;
	const e2eKey = urlParser.query.e2eKey;
	const consumerReplicas = urlParser.query.consumerReplicas;

	// Enable face detection on demand.
	if (faceDetection)
		await faceapi.loadTinyFaceDetectorModel('/face-detector-models');

	if (info) {
		// eslint-disable-next-line require-atomic-updates
		window.SHOW_INFO = true;
	}

	if (throttleSecret) {
		// eslint-disable-next-line require-atomic-updates
		window.NETWORK_THROTTLE_SECRET = throttleSecret;
	}

	if (!roomId) {
		roomId = randomString({ length: 8 }).toLowerCase();

		urlParser.query.roomId = roomId;
		window.history.pushState('', '', urlParser.toString());
	}

	// Get the effective/shareable Room URL.
	const roomUrlParser = new UrlParse(window.location.href, true);

	for (const key of Object.keys(roomUrlParser.query)) {
		// Don't keep some custom params.
		switch (key) {
			case 'roomId':
			case 'handlerName':
			case 'handler':
			case 'forceTcp':
			case 'produce':
			case 'consume':
			case 'datachannel':
			case 'forceVP8':
			case 'forceH264':
			case 'forceVP9':
			case 'forceAV1':
			case 'enableWebcamLayers':
			case 'enableSharingLayers':
			case 'webcamScalabilityMode':
			case 'sharingScalabilityMode':
			case 'numSimulcastStreams':
			case 'info':
			case 'faceDetection':
			case 'externalVideo':
			case 'throttleSecret':
			case 'e2eKey':
			case 'consumerReplicas': {
				break;
			}

			default: {
				delete roomUrlParser.query[key];
			}
		}
	}
	delete roomUrlParser.hash;

	const roomUrl = roomUrlParser.toString();

	let displayNameSet;

	// If displayName was provided via URL or Cookie, we are done.
	if (displayName) {
		displayNameSet = true;
	}
	// Otherwise pick a random name and mark as "not set".
	else {
		displayNameSet = false;
		displayName = randomName();
	}

	// Get current device info.
	const device = deviceInfo();

	store.dispatch(stateActions.setRoomUrl(roomUrl));

	store.dispatch(stateActions.setRoomFaceDetection(faceDetection));

	store.dispatch(
		stateActions.setMe({ peerId, displayName, displayNameSet, device })
	);

	roomClient = new RoomClient({
		roomId,
		peerId,
		displayName,
		device,
		handlerName: handlerName,
		forceTcp,
		produce,
		consume,
		datachannel,
		forceVP8,
		forceH264,
		forceVP9,
		forceAV1,
		enableWebcamLayers,
		enableSharingLayers,
		webcamScalabilityMode,
		sharingScalabilityMode,
		numSimulcastStreams,
		externalVideo,
		e2eKey,
		consumerReplicas,
	});

	// NOTE: For debugging.
	// eslint-disable-next-line require-atomic-updates
	window.CLIENT = roomClient;
	// eslint-disable-next-line require-atomic-updates
	window.CC = roomClient;

	const domNode = document.getElementById('mediasoup-demo-app-container');
	const root = createRoot(domNode);

	root.render(
		<Provider store={store}>
			<RoomContext.Provider value={roomClient}>
				<Room />
			</RoomContext.Provider>
		</Provider>
	);
}

// NOTE: Debugging stuff.

window.__sendSdps = function () {
	logger.warn('>>> send transport local SDP offer:');
	logger.warn(roomClient._sendTransport._handler._pc.localDescription.sdp);

	logger.warn('>>> send transport remote SDP answer:');
	logger.warn(roomClient._sendTransport._handler._pc.remoteDescription.sdp);
};

window.__recvSdps = function () {
	logger.warn('>>> recv transport remote SDP offer:');
	logger.warn(roomClient._recvTransport._handler._pc.remoteDescription.sdp);

	logger.warn('>>> recv transport local SDP answer:');
	logger.warn(roomClient._recvTransport._handler._pc.localDescription.sdp);
};

let dataChannelTestInterval = null;

window.__startDataChannelTest = function () {
	let number = 0;

	const buffer = new ArrayBuffer(32);
	const view = new DataView(buffer);

	dataChannelTestInterval = window.setInterval(() => {
		if (window.DP) {
			view.setUint32(0, number++);
			roomClient.sendChatMessage(buffer);
		}
	}, 100);
};

window.__stopDataChannelTest = function () {
	window.clearInterval(dataChannelTestInterval);

	const buffer = new ArrayBuffer(32);
	const view = new DataView(buffer);

	if (window.DP) {
		view.setUint32(0, Math.pow(2, 32) - 1);
		window.DP.send(buffer);
	}
};

window.__testSctp = async function ({ timeout = 100, bot = false } = {}) {
	let dp;

	if (!bot) {
		await window.CLIENT.enableChatDataProducer();

		dp = window.CLIENT._chatDataProducer;
	} else {
		await window.CLIENT.enableBotDataProducer();

		dp = window.CLIENT._botDataProducer;
	}

	logger.debug(
		'__testSctp() | DataProducer created [bot:%s, streamId:%d, readyState:%s]',
		bot ? 'true' : 'false',
		dp.sctpStreamParameters.streamId,
		dp.readyState
	);

	function send() {
		dp.send(`I am streamId ${dp.sctpStreamParameters.streamId}`);
	}

	if (dp.readyState === 'open') {
		send();
	} else {
		dp.on('open', () => {
			logger.debug(
				'testSctp() | DataChannel open [streamId:%d]',
				dp.sctpStreamParameters.streamId
			);

			send();
		});
	}

	setTimeout(() => window.__testSctp({ timeout, bot }), timeout);
};

setInterval(() => {
	if (window.CLIENT._sendTransport) {
		window.H1 = window.CLIENT._sendTransport._handler;
		window.PC1 = window.CLIENT._sendTransport._handler._pc;
		window.DP = window.CLIENT._chatDataProducer;
	} else {
		delete window.PC1;
		delete window.DP;
	}

	if (window.CLIENT._recvTransport) {
		window.H2 = window.CLIENT._recvTransport._handler;
		window.PC2 = window.CLIENT._recvTransport._handler._pc;
	} else {
		delete window.PC2;
	}
}, 2000);

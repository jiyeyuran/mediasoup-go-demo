module.exports =
{
	// Listening hostname (just for `gulp live` task).
	domain : process.env.DOMAIN || 'localhost',
	https  :
	{
		// NOTE: Set your own valid certificate files.
		tls        :
		{
			cert : process.env.HTTPS_CERT_FULLCHAIN || `${__dirname}/../server/certs/fullchain.pem`,
			key  : process.env.HTTPS_CERT_PRIVKEY || `${__dirname}/../server/certs/privkey.key`
		}
	},
}

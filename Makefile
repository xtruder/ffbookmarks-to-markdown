ffsclient:
	mkdir -p .bin
	curl -L -o .bin/ffsclient https://github.com/Mikescher/firefox-sync-client/releases/download/v1.8.0/ffsclient_linux-amd64-static
	chmod +x .bin/ffsclient

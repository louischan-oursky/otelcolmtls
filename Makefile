.PHONY: ca
ca:
	openssl req \
		-x509 \
		-newkey EC \
		-noenc \
		-pkeyopt ec_paramgen_curve:P-256 \
		-keyout ca.key \
		-subj "/CN=ca" \
		-addext "basicConstraints=critical,CA:TRUE, pathlen:0" \
		-addext "keyUsage=critical, keyCertSign, cRLSign" \
		-days 36500 \
		-out ca.crt

.PHONY: client
client:
	openssl req \
		-x509 \
		-CA ca.crt \
		-CAkey ca.key \
		-newkey EC \
		-noenc \
		-pkeyopt ec_paramgen_curve:P-256 \
		-keyout client.key \
		-subj "/CN=tls" \
		-addext "basicConstraints=critical,CA:FALSE" \
		-addext "keyUsage=critical, digitalSignature" \
		-addext "extendedKeyUsage=critical, serverAuth, clientAuth" \
		-days 36500 \
		-out client.crt

.PHONY: server
server:
	openssl req \
		-x509 \
		-CA ca.crt \
		-CAkey ca.key \
		-newkey EC \
		-noenc \
		-pkeyopt ec_paramgen_curve:P-256 \
		-keyout server.key \
		-subj "/CN=tls" \
		-addext "basicConstraints=critical,CA:FALSE" \
		-addext "keyUsage=critical, digitalSignature" \
		-addext "extendedKeyUsage=critical, serverAuth, clientAuth" \
		-addext "subjectAltName=critical, DNS:otelcol-remote" \
		-days 36500 \
		-out server.crt

.PHONY: generate-and-send-metrics
generate-and-send-metrics:
	telemetrygen metrics --metric-type Sum --otlp-http --otlp-insecure --telemetry-attributes timestamp=\"$(shell date -Iseconds)\"

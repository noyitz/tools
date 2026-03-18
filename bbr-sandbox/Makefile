.PHONY: run build port-forward deploy

run:
	go run . --target localhost:9004

build:
	go build -o bbr-sandbox .

port-forward:
	oc port-forward svc/bbr-plugins 9004:9004 -n bbr-plugins

deploy:
	./deploy/build.sh

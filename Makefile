.PONY: all build deps image lint test
CHECK_FILES?=$$(go list ./... | grep -v /vendor/)

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

BINARY=thumbla
BINFULLPATH=${GOPATH}/bin/${BINARY}

default:
	build

build: ## Build the binary
	go build -o ./bin/${BINARY}

buildprod: ## Build the binary in production mode
	GOOS=linux go build -a -o ./bin/${BINARY}

clean: ## Clean up
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi

buildimage: ## Build docker image
	GOOS=linux go build -a -o ./bin/${BINARY}
	docker buildx build --tag thumbla --tag erans/thumbla .
	rm -rf ./bin/

run: ## Run in dev mode
	go run thumbla.go --config ./config-dev.yml

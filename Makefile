.PONY: all build deps image lint test
CHECK_FILES?=$$(go list ./... | grep -v /vendor/)

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

BINARY=thumbla
BINFULLPATH=${GOPATH}/bin/${BINARY}

default:
	build

build: ## Build the binary
	go build -o ${BINFULLPATH}

buildprod: ## Build the binary in production mode
	CGO_ENABLED=0 GOOS=linux go build -a -o ./bin/${BINARY}

clean: ## Clean up
	if [ -f ${BINFULLPATH} ] ; then rm ${BINFULLPATH} ; fi

buildimage: ## Build docker image
	CGO_ENABLED=0 GOOS=linux go build -a -o ./bin/${BINARY}
	docker build --no-cache=true --rm --tag thumbla .
	rm -rf ./bin/

run: ## Run in dev mode
	go run thumbla.go --config ./config-dev.yml

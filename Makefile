BINARY=thumbla
BINFULLPATH=${GOPATH}/bin/${BINARY}

default:
	build

build:
	go build -o ${BINFULLPATH}

buildprod:
	CGO_ENABLED=0 GOOS=linux go build -a -o ./bin/${BINARY}

clean:
	if [ -f ${BINFULLPATH} ] ; then rm ${BINFULLPATH} ; fi

buildimage:
	CGO_ENABLED=0 GOOS=linux go build -a -o ./bin/${BINARY}
	docker build --no-cache=true --rm --tag thumbla .
	rm -rf ./bin/

run:
	go run thumbla.go --config ./config-dev.yml

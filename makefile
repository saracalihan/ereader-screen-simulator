DEVICE_DIR=/tmp/kobo-emu
KOBO_FLAGS=GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0
BIN_DIR=bin
EXAMPLE_DIR=examples
PORT=8000

# usage:
#   make example touchdraw
#   make example EXAMPLE=touchdraw
ifneq ($(filter example build_example,$(firstword $(MAKECMDGOALS))),)
ARG := $(word 2,$(MAKECMDGOALS))
ifneq ($(ARG),)
EXAMPLE ?= $(ARG)

$(ARG):
	@:
endif
endif
EXAMPLE ?=

.PHONY: start

start:
	go run . -dir ${DEVICE_DIR}

check_example:
	@test -n "${EXAMPLE}" || (echo "EXAMPLE required. Ör: make $(firstword $(MAKECMDGOALS)) EXAMPLE=touchdraw"; exit 1)
	@test -d ${EXAMPLE_DIR}/${EXAMPLE} || (echo "${EXAMPLE_DIR}/${EXAMPLE} bulunamadı"; exit 1)

build_example: check_example
	@mkdir -p ${BIN_DIR}
	${KOBO_FLAGS} go build -o ${BIN_DIR}/${EXAMPLE} ./${EXAMPLE_DIR}/${EXAMPLE}
	chmod +x ${BIN_DIR}/${EXAMPLE}

serve_example:
	python3 -m http.server ${PORT} -d ${BIN_DIR}

example: build_example serve_example

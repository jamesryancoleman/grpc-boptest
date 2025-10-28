FROM golang:bookworm

RUN mkdir -p /opt/bos/services/common/
COPY ../drivers/boptest/go.mod /opt/bos/services/
COPY ../drivers/boptest/go.sum /opt/bos/services/
COPY ../common/ /opt/bos/services/common/

WORKDIR /opt/bos/services/common/
RUN go install .

RUN mkdir -p /opt/bos/services/common/
COPY ../drivers/boptest /opt/bos/services/drivers/boptest/
WORKDIR /opt/bos/services/drivers/boptest/

WORKDIR /opt/bos/services/drivers/boptest/cmd/server/
CMD [ "go","run","server.go","-start=0"]



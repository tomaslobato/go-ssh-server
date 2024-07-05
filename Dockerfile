# Use a multi-stage build for smaller final image size
FROM golang:1.22-alpine3.19

WORKDIR /app

COPY go.* ./
RUN go mod download

RUN go install github.com/air-verse/air@latest

COPY . .

RUN apk add --no-cache openssh

#ssh key stuff
COPY ./ssh_server_key /root/.ssh/id_rsa

RUN chmod 400 /root/.ssh/id_rsa

EXPOSE 2222

COPY .air.toml .

CMD ["air", "-c", ".air.toml"]

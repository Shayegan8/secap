package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	_ "modernc.org/sqlite"
)

var l = fmt.Println

func fuckClient(client net.Conn) { // maixmum 2mb
	first := make([]byte, 60)
	_, err := client.Read(first)
	if err != nil {
		l("Problem with reading initial packet", err)
		return
	}
	hexshashedPass := first[:56]

	rows, err := dbo.Query("SELECT subscription FROM passwords WHERE password=?", string(hexshashedPass))

	if err != nil {
		l("DB error", err)
		return
	}

	if !rows.Next() {
		l("Couldn't find this shit closing socket")
		client.Close()
		rows.Close()
		return
	}
	var subscription string
	err = rows.Scan(&subscription)
	if err != nil {
		l("Problem with scanning")
		client.Close()
		rows.Close()
		return
	}
	rows.Close()

	// CRLF 2byte
	cmd := first[59]
	atyp := first[60]
	var howMany int
	switch atyp { // ATYP
	case 0x01: // IPv4
		howMany = 4
	case 0x03: // DOMAINNAME
		shitJerk := make([]byte, 1)
		_, err := client.Read(shitJerk)
		if err != nil {
			l("Problem with getting that byte")
			client.Close()
			return
		}
		howMany = int(shitJerk[0])
	case 0x04: // IPv6
		howMany = 16
	}
	firstContinueHeader := make([]byte, howMany+2)
	_, err = client.Read(firstContinueHeader)
	if err != nil {
		l("Problem with reading shit")
		client.Close()
		return
	}

	dialer := ""
	for k, v := range config {
		if k == subscription {
			dialer = v
			break
		}
	}
	conn, err := net.Dial("tcp", dialer)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		l("Cant connect to endpoint", err)
		return
	}
	_, err = conn.Write([]byte{0x5, 0x1, 0x0})
	if err != nil {
		l("Cant write handshake", err)
		conn.Close()
		return
	}
	handshakeResponse := make([]byte, 2)
	_, err = conn.Read(handshakeResponse)
	if err != nil {
		l("Cant read handshake resp", err)
		conn.Close()
		return
	}

	// my endpoints just send 0x0 in method so i dont check
	var socks5Request []byte
	switch atyp {
	case 0x1:
		socks5Request = []byte{0x5, cmd, 0x0, atyp, firstContinueHeader[0], firstContinueHeader[1], firstContinueHeader[2], firstContinueHeader[3], firstContinueHeader[4], firstContinueHeader[5]}
	case 0x3:
		socks5Request = make([]byte, 7+firstContinueHeader[0])
	default:
		socks5Request = make([]byte, 22)
		socks5Request[0] = 0x5
		socks5Request[1] = cmd
		socks5Request[2] = 0x0
		socks5Request[3] = atyp
		for i := 4; i < 20; i++ {
			socks5Request[i] = firstContinueHeader[i]
		}
	}

	_, err = conn.Write(socks5Request)
	if err != nil {
		l("Cant send socks5 request", err)
		conn.Close()
		return
	}

	socks5Reply := make([]byte, 10)
	_, err = client.Read(socks5Reply)
	if err != nil {
		l("Couldn't read", err)
		conn.Close()
		return
	}
	if socks5Reply[1] != 0x0 {
		l("Error from socks5 endpoint", err)
		conn.Close()
		return
	}

	// reading channel
	go func() {
		for {
			buffogoli := make([]byte, 64*1024)
			_, err := client.Read(buffogoli)
			if err != nil {
				l("Problem with fucking shit")
				client.Close()
				conn.Close()
				return
			}
			_, err = conn.Write(buffogoli)
			if err != nil {
				l("Problem with fucking shit")
				client.Close()
				conn.Close()
				return
			}
		}
	}()

	for {
		buffogoli := make([]byte, 64*1024)
		_, err := conn.Read(buffogoli)
		if err != nil {
			l("Problem with fucking shit")
			client.Close()
			conn.Close()
			return
		}
		_, err = client.Write(buffogoli)
		if err != nil {
			l("Problem with fucking shit")
			client.Close()
			conn.Close()
			return
		}
	}

}

var Redis_client *redis.Client

var dbo *sql.DB

//go:embed config.json
var configBytes []byte

var config map[string]string

var redises []string

var s3s []string

var bales []string

func main() {
	l("config")
	err := json.Unmarshal(configBytes, &config)
	if err != nil {
		panic(err)
	}
	l("sqlite")
	dbo, err := sql.Open("sqlite", "shit.db")
	if err != nil {
		panic(err)
	}
	l("redis for i dont know?")
	Redis_client = redis.NewClient(&redis.Options{
		Network:  "unix",
		Addr:     "/run/redis/redis-server.sock", // 400
		PoolSize: 30,
	})
	cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
	server, err := tls.Listen("tcp", "127.0.0.1:1081", &tls.Config{
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	})
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			// do something with client
			client, _ := server.Accept()
			l("client captured")
			client.SetReadDeadline(time.Now().Add(5 * time.Second))
			client.SetWriteDeadline(time.Now().Add(5 * time.Second))
			fuckClient(client)
		}
	}()
	signalc, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	<-signalc.Done()
	dbo.Close()
	Redis_client.FlushDB(context.Background())
}

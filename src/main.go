package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	_ "modernc.org/sqlite"
)

var l = fmt.Println

/*
This is a vmess proxy that v2ray or any other shitty client which has vmess can connect to this
We will digest the packet and convert it to socks5 but why? because all of our source code use socks5
I can fuck without socks5 but it requires chaning all source codes and for our conveniency and easier for making
configurable endpoints

I need md5, hmac, shake functions

*/

/**
func KDF(key []byte, salt string, path ...[]byte) []byte {
	hmacCreator := &hMacCreator{value: []byte("VMess AEAD KDF")}
	hmacCreator = &hMacCreator{value: []byte(salt), parent: hmacCreator}
	for _, v := range path {
		hmacCreator = &hMacCreator{value: v, parent: hmacCreator}
	}
	hmacf := hmacCreator.Create()
	hmacf.Write(key)
	return hmacf.Sum(nil)
}

type hMacCreator struct {
	parent *hMacCreator
	value  []byte
}

func (h *hMacCreator) Create() hash.Hash {
	if h.parent == nil {
		return hmac.New(crypto.SHA256.New, h.value)
	}
	return hmac.New(h.parent.Create, h.value)
}

func cmdKey(uuid []byte) {
	md5.Sum(append(uuid[:], "c48619fe-8f02-49e0-b9e9-edf763e17e21"...))
}
*/

func fuckClient(client net.Conn) { // maixmum 2mb
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	var password = make([]byte, 56) // 56 hex(SHA224(password))
	_, err := io.ReadFull(client, password)
	if err != nil {
		l("Problem with reading packet")
		_ = client.Close()
		return
	}

	/*
		A password can have maximum n mb bandwidth and when i exceed it we simply de alloc memory
		We check this with redis, the n mb is determined by vps ram
	*/
	rows, err := dbo.Query("SELECT * FROM passwords WHERE password=?", password)
	if err != nil {
		l(err)
		client.Close()
		return
	}
	if !rows.Next() {
		rows.Close()
		l(err)
		client.Close()
		return
	}
	rows.Close()
	restPacket, err := io.ReadAll(io.LimitReader(client, int64(1024*1024*2)))
	if err != nil {
		l(err)
		client.Close()
		return
	}
	cmd := restPacket[2]
	atyp := restPacket[3]
	var dstaddr string
	var dstport uint16
	var portoffset uint8
	var sex string
	switch restPacket[3] { // ATYP
	case 0x01: // IPv4
		ip := net.IP(restPacket[4:8])
		dstaddr = ip.String()
		portoffset = 8
		sex = "v4"
	case 0x03: // DOMAINNAME
		l("DOMAIN NAME!")
		numberOfBytes := restPacket[4]
		l("number of octets", numberOfBytes)
		l("the damn thing", string(restPacket[5:numberOfBytes+5]))
		dstaddr = string(restPacket[5 : numberOfBytes+5])
		portoffset = 5 + numberOfBytes
		sex = "domain"
	case 0x04: // IPv6
		ip := net.IP(restPacket[4:20])
		dstaddr = ip.String()
		portoffset = 20
		sex = "v6"
	}
}

var Redis_client *redis.Client

/*
*
We implement trojan, then we give socks5 packet to our endpoints in local
client connects to our endpoint we have 5s deadline
we check their password in deebee kv storage
*/

var dbo *sql.DB

func main() {
	dbo, err := sql.Open("sqlite", "demo.db")
	if err != nil {
		panic(err)
	}
	Redis_client = redis.NewClient(&redis.Options{
		Network:  "unix",
		Addr:     "/run/redis/redis-server.sock", // 400
		PoolSize: 30,
	})
	cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
	server, err := tls.Listen("tcp", "127.0.0.1:443", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		panic(err)
	}
	client, _ := server.Accept()
	go func() {
		for {
			// do something with client
			fuckClient(client)
		}
	}()
	go func() {
		signalc, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()
		<-signalc.Done()
		dbo.Close()
		Redis_client.FlushDB(context.Background())
	}()
}

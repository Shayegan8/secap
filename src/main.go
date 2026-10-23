package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
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
	var password []byte // 56 hex(SHA224(password))
	passChan := make(chan []byte, 1)
	timeChan := make(chan any, 1)
	go func() {
		_, err := io.ReadFull(client, password)
		if err != nil {
			passChan <- []byte{0b0}
		} else {
			passChan <- []byte{0b1}
		}
	}()
	go func() {
		ch := time.NewTimer(5 * time.Second)
		<-ch.C
		timeChan <- []byte{}
	}()
	select {
	case msg := <-passChan:
		if msg[0] == 0b0 {
			l("Problem with reading packet")
			_ = client.Close()
			return
		}
	case <-timeChan:
		_ = client.Close()
		return
	}
	/*
		A password can have maximum n mb bandwidth and when i exceed it we simply de alloc memory
		We check this with redis, the n mb is determined by vps ram
	*/
}

var Redis_client *redis.Client

/*
*
We implement trojan, then we give socks5 packet to our endpoints in local
client connects to our endpoint we have 5s deadline
we check their password in deebee kv storage
*/

func main() {
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
		// do something with client
		fuckClient(client)
	}()
}

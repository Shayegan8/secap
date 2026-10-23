package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"syscall"
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
	_, err := io.ReadFull(client, password)
	if err != nil {
		l("Problem with reading packet:", err)
		return
	}
}

/*
*
We implement trojan, then we give socks5 packet to our endpoints in local
*/
func main() {
	cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
	server, err := tls.Listen("tcp", "127.0.0.1:443", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		l("443 already being used")
		syscall.Exit(1)
	}
	client, _ := server.Accept()
	go func() {
		// do something with client
		fuckClient(client)
	}()
}

package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/signal"
	"sync"
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

/*
this api should transfer us somehow to endpoints it should see how much capacity it has, but in registeration we determine what endpoint they can use
*/

func fuckClient(client net.Conn) { // maixmum 2mb
	var password = make([]byte, 56) // 56 hex(SHA224(password))
	var loopChan = make(chan []byte, 1)
	var onepiece sync.Once
	for {
		select {
		case <-loopChan:
			return
		default:
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
			rows, err := dbo.Query("SELECT subscription FROM passwords WHERE password=?", password)
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
			var subscription string
			err = rows.Scan(&subscription)
			if err != nil {
				l("Null subscription???? :", err)
				rows.Close()
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
			var portoffset uint8
			switch atyp { // ATYP
			case 0x1: // IPv4
				//ip := net.IP(restPacket[4:8])
				//dstaddr = ip.String()
				portoffset = 8
			case 0x3: // DOMAINNAME
				numberOfBytes := restPacket[4]
				//dstaddr = string(restPacket[5 : numberOfBytes+5])
				portoffset = 5 + numberOfBytes
			case 0x4: // IPv6
				//ip := net.IP(restPacket[4:20])
				//dstaddr = ip.String()
				portoffset = 20
			}
			//dstport = binary.BigEndian.Uint16(restPacket[portoffset : portoffset+2])
			actual_data := restPacket[portoffset+4:]
			/*
				step 1: send stupid socks5 handshake
				step 2: socks5 request
				step 3: send the packet then after
			*/

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
			if atyp == 0x1 {
				socks5Request = []byte{0x5, cmd, 0x0, atyp, restPacket[4], restPacket[5], restPacket[6], restPacket[7], restPacket[8], restPacket[9]}
			} else if portoffset == 0x3 {
				socks5Request = make([]byte, 7+restPacket[4])
			} else {
				socks5Request = make([]byte, 22)
				socks5Request[0] = 0x5
				socks5Request[1] = cmd
				socks5Request[2] = 0x0
				socks5Request[3] = atyp
				for i := 4; i < 20; i++ {
					socks5Request[i] = restPacket[i]
				}
			}

			_, err = conn.Write(socks5Request)
			if err != nil {
				l("Cant send socks5 request", err)
				conn.Close()
				return
			}

			socks5Reply := make([]byte, 2)
			_, err = io.ReadAtLeast(conn, socks5Reply, 2)
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
			_, err = conn.Write(actual_data)
			if err != nil {
				l("Couldn't write real data", err)
				conn.Close()
				return
			}
			onepiece.Do(func() {
				go func() {
					for {
						respData, err := io.ReadAll(io.LimitReader(client, int64(1024*1024*2)))
						if err != nil {
							l("Problem with data channel", err)
							loopChan <- []byte{}
							conn.Close()
							return
						}
						_, err = client.Write(respData)
						if err != nil {
							l("Problem with data channel but in writing scene", err)
							loopChan <- []byte{}
							conn.Close()
							return
						}
					}
				}()
			})

		}
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

//go:embed config.json
var configBytes []byte

var config map[string]string

var redises []string

var s3s []string

var bales []string

func main() {
	err := json.Unmarshal(configBytes, &config)
	if err != nil {
		panic(err)
	}
	dbo, err := sql.Open("sqlite", "shit.db")
	if err != nil {
		panic(err)
	}
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
			client.SetReadDeadline(time.Now().Add(5 * time.Second))
			client.SetWriteDeadline(time.Now().Add(5 * time.Second))
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

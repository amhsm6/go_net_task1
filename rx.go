package main

import (
    "fmt"
    "log"
    "net"
    "os"
    "time"
    "encoding/binary"
    "crypto/sha256"
)

const CHUNK_SIZE = 60000

type Client struct {
    updated chan struct{}
    state int
    filename string
    chunksNum uint64
    checksum []byte
    lastChunkId uint64
    receivedChunksNum uint64
    bytes []byte
}

func garbage_collector(updated chan struct{}, addr string) {
    for {
        delete_client := time.After(time.Second * 5)

        select {
            case <-delete_client:
                fmt.Printf("Client with ip %s not responding --> deleting\n", addr)
                delete(clients, addr)

            case <-updated:
        }
    }
}

var clients map[string]*Client

func main() {
    conn, err := net.ListenPacket("udp", "0.0.0.0:4242")

    if err != nil {
        log.Panic(err)
    }

    defer conn.Close()

    clients = make(map[string]*Client)

    buf := make([]byte, CHUNK_SIZE)

    for {
        bytesRead, addr, err := conn.ReadFrom(buf)

        if err != nil {
            log.Panic(err)
        }

        client, contains := clients[fmt.Sprint(addr)]

        if !contains {
            fmt.Printf("New client with ip %s connected\n", addr)

            client := Client{}

            client.filename = string(buf[:bytesRead])
            client.checksum = make([]byte, 32)
            client.updated = make(chan struct{}, 2)
            client.state++

            clients[fmt.Sprint(addr)] = &client

            go garbage_collector(client.updated, fmt.Sprint(addr))

            continue
        }

        client.updated <- struct{}{}
        
        switch client.state {

        case 1:
            client.chunksNum = binary.LittleEndian.Uint64(buf[:bytesRead])
            client.state++

        case 2:
            copy(client.checksum, buf[:bytesRead])
            client.state++

        case 3:
            client.lastChunkId = binary.LittleEndian.Uint64(buf[:bytesRead])
            client.state++

        case 4:
            fmt.Printf("Received chunk %d of %d\n", client.lastChunkId + 1, client.chunksNum)
            client.bytes = append(client.bytes, buf[:bytesRead]...)

            client.receivedChunksNum++
            client.state--

            if client.receivedChunksNum == client.chunksNum {
                fmt.Printf("Received file from client with ip %s\n", addr)

                if sha256.Sum256(client.bytes) == *(*[32]byte)(client.checksum) {
                    if _, err := os.Stat(client.filename); err == nil {
                        client.filename += "_copy"
                    }

                    err := os.WriteFile(client.filename, client.bytes, 0644)

                    if err != nil {
                        conn.WriteTo([]byte(fmt.Sprintf("ERROR: %v", err.Error())), addr)
                    } else {
                        conn.WriteTo([]byte(fmt.Sprintf("File %s successfully transmitted", client.filename)), addr)
                    }
                } else {
                    conn.WriteTo([]byte(fmt.Sprintf("ERROR: File not transmitted, checksums are not equal")), addr)
                }

                delete(clients, fmt.Sprint(addr))
            }
        }
    }
}

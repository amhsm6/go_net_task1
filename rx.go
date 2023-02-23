package main

import (
    "fmt"
    "log"
    "net"
    "os"
    "time"
    "encoding/binary"
    "crypto/sha256"
    "path"
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

func garbage_collector(updated chan struct{}, id int) {
    for {
        delete_client := time.After(time.Second * 5)

        select {

        case <-delete_client:
            if _, contains := clients[id]; contains {
                fmt.Printf("Client %d not responding --> deleting\n", id)
                delete(clients, id)
            }

            return

        case <-updated:

        }
    }
}

var clients map[int]*Client

func main() {
    os.Mkdir("files", os.ModePerm)

    fmt.Println("Server is running on port 4242")

    conn, err := net.ListenPacket("udp", "0.0.0.0:4242")

    if err != nil {
        log.Panic(err)
    }

    defer conn.Close()

    clients = make(map[int]*Client)

    buf := make([]byte, CHUNK_SIZE + 1)

    for {
        bytesRead, addr, err := conn.ReadFrom(buf)

        if err != nil {
            log.Panic(err)
        }

        if bytesRead == 0 {
            id := len(clients)

            fmt.Printf("Client %d with ip %s connected\n", id, addr)

            client := Client{}

            client.checksum = make([]byte, 32)
            client.updated = make(chan struct{}, 2)
            client.state++

            clients[id] = &client

            go garbage_collector(client.updated, id)

            conn.WriteTo([]byte{ byte(id) }, addr)

            continue
        }

        id := int(buf[0])
        buf2 := buf[1:]
        bytesRead--

        client := clients[id]

        client.updated <- struct{}{}
        
        switch client.state {

        case 1:
            client.filename = string(buf2[:bytesRead])
            client.state++
            conn.WriteTo([]byte{ 0 }, addr)

        case 2:
            client.chunksNum = binary.LittleEndian.Uint64(buf2[:bytesRead])
            client.state++
            conn.WriteTo([]byte{ 0 }, addr)

        case 3:
            copy(client.checksum, buf2[:bytesRead])
            client.state++
            conn.WriteTo([]byte{ 0 }, addr)

        case 4:
            client.lastChunkId = binary.LittleEndian.Uint64(buf2[:bytesRead])
            client.state++
            conn.WriteTo([]byte{ 0 }, addr)

        case 5:
            fmt.Printf("Received chunk %d of %d\n", client.lastChunkId + 1, client.chunksNum)

            client.bytes = append(client.bytes, buf2[:bytesRead]...)

            client.receivedChunksNum++
            client.state--

            if client.receivedChunksNum == client.chunksNum {
                fmt.Printf("Received file from client %d\n", id)

                if sha256.Sum256(client.bytes) == *(*[32]byte)(client.checksum) {
                    ext := path.Ext(client.filename)
                    base := client.filename[:len(client.filename) - len(ext)]

                    files, err := os.ReadDir("files")

                    if err != nil {
                        log.Panic(err)
                    }

                    max_copy_num := -1

                    for _, file := range files {
                        n := -1

                        if client.filename == file.Name() {
                            n = 0
                        } else {
                            fmt.Sscanf(file.Name(), base + " (%d)" + ext, &n) 
                        }

                        if n > max_copy_num {
                            max_copy_num = n
                        }
                    }

                    if max_copy_num > -1 {
                        client.filename = fmt.Sprintf("%v (%v)%v", base, max_copy_num + 1, ext)
                    }

                    err = os.WriteFile(path.Join("files", client.filename), client.bytes, 0644)

                    if err != nil {
                        conn.WriteTo([]byte(fmt.Sprintf("ERROR: %v", err.Error())), addr)
                    } else {
                        conn.WriteTo([]byte(fmt.Sprintf("File %s successfully transmitted", client.filename)), addr)
                    }
                } else {
                    conn.WriteTo([]byte(fmt.Sprintf("ERROR: File not transmitted, checksums are not equal")), addr)
                }

                delete(clients, id)
            } else {
                conn.WriteTo([]byte{ 0 }, addr)
            }
        }
    }
}

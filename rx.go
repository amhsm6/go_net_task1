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
    "sync"
)

type Stack []int

func (s *Stack) Push(v int) {
    *s = append(*s, v)
}

func (s *Stack) Pop() (res int) {
    res = (*s)[len(*s) - 1]
    *s = (*s)[:len(*s) - 1]
    return
}

type Client struct {
    updated chan struct{}
    finished chan struct{}
    filename string
    chunksNum uint32
    checksum []byte
    receivedChunksNum uint32
    bytes []byte
}

func garbage_collector(updated chan struct{}, finished chan struct{}, id int) {
    for {
        delete_client := time.After(time.Second * 5)

        select {

        case <-delete_client:
            fmt.Printf("Client %d not responding --> deleting\n", id)

            clients.mutex.Lock()
            delete(clients.inner, id)
            clients.mutex.Unlock()

            idStack.mutex.Lock()
            idStack.inner.Push(id)
            idStack.mutex.Unlock()

            return

        case <-finished:
            clients.mutex.Lock()
            delete(clients.inner, id)
            clients.mutex.Unlock()

            idStack.mutex.Lock()
            idStack.inner.Push(id)
            idStack.mutex.Unlock()

            return

        case <-updated:

        }
    }
}

func process(conn *net.PacketConn, addr net.Addr, buf []byte) {
    if buf[0] == 0xff {
        client := Client{}
        client.checksum = make([]byte, 32)
        client.updated = make(chan struct{}, 1)
        client.finished = make(chan struct{}, 1)

        client.chunksNum = binary.LittleEndian.Uint32(buf[1:5])
        copy(client.checksum, buf[5:37])
        client.filename = string(buf[37:])

        idStack.mutex.Lock()
        id := idStack.inner.Pop()
        idStack.mutex.Unlock()

        clients.mutex.Lock()
        clients.inner[id] = &client
        clients.mutex.Unlock()

        go garbage_collector(client.updated, client.finished, id)

        (*conn).WriteTo([]byte{ byte(id) }, addr)

        fmt.Printf(
            "Client %d with ip %s connected\n\tFILE %v would be received in %v CHUNKS\n",
            id,
            addr,
            client.filename,
            client.chunksNum,
        )

        return
    }

    id := int(buf[0])
    clients.mutex.Lock()
    client := clients.inner[id]
    client.updated <- struct{}{}

    chunkId := binary.LittleEndian.Uint32(buf[1:5])
    
    fmt.Printf("Received chunk %d of %d from client %d\n", chunkId + 1, client.chunksNum, id)

    client.bytes = append(client.bytes, buf[5:]...)
    client.receivedChunksNum++

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
                (*conn).WriteTo([]byte(fmt.Sprintf("ERROR: %v", err.Error())), addr)
            } else {
                (*conn).WriteTo([]byte(fmt.Sprintf("File %s successfully transmitted", client.filename)), addr)
            }
        } else {
            (*conn).WriteTo([]byte(fmt.Sprintf("ERROR: File not transmitted, checksums are not equal")), addr)
        }

        client.finished <- struct{}{}
    } else {
        (*conn).WriteTo([]byte{ 0 }, addr)
    }

    clients.mutex.Unlock()
}

type MutexMap struct {
    inner map[int]*Client
    mutex sync.Mutex
}

type MutexStack struct {
    inner Stack
    mutex sync.Mutex
}

var clients MutexMap
var idStack MutexStack

func main() {
    os.Mkdir("files", os.ModePerm)

    fmt.Println("Server is running on port 4242")

    conn, err := net.ListenPacket("udp", "0.0.0.0:4242")

    if err != nil {
        log.Panic(err)
    }

    defer conn.Close()

    clients.inner = make(map[int]*Client)
    for id := 254; id >= 0; id-- {
        idStack.inner.Push(id)
    }

    buf := make([]byte, 65000)

    for {
        bytesRead, addr, err := conn.ReadFrom(buf)

        if err != nil {
            log.Panic(err)
        }

        buf_copy := make([]byte, bytesRead)
        copy(buf_copy, buf)

        go process(&conn, addr, buf_copy)
    }
}

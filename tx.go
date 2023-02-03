package main

import (
    "fmt"
    "log"
    "net"
    "os"
    "path/filepath"
    "encoding/binary"
    "crypto/sha256"
)

const CHUNK_SIZE = 60000

func main() {
    if len(os.Args) < 3 {
        fmt.Println("Usage: transmit <ip> <file_path>")
        os.Exit(1)
    }

    conn, err := net.ListenPacket("udp", ":0")

    if err != nil {
        log.Panic(err)
    }

    defer conn.Close()

    remoteAddr, err := net.ResolveUDPAddr("udp", os.Args[1])

    if err != nil {
        log.Panic(err)
    }

    filePath := os.Args[2]
    filename := filepath.Base(filePath)

    bytes, err := os.ReadFile(filePath)

    if err != nil {
        log.Panic(err)
    }

    chunksNum := (len(bytes) + CHUNK_SIZE - 1) / CHUNK_SIZE

    conn.WriteTo([]byte{}, remoteAddr)

    buf := make([]byte, 8)
    conn.ReadFrom(buf)
    id := binary.LittleEndian.Uint64(buf)

    conn.WriteTo(append([]byte{ byte(id) }, []byte(filename)...), remoteAddr)
    conn.ReadFrom([]byte{})

    buf = make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(chunksNum))
    conn.WriteTo(append([]byte{ byte(id) }, buf...), remoteAddr)
    conn.ReadFrom([]byte{})

    checksum := sha256.Sum256(bytes)
    conn.WriteTo(append([]byte{ byte(id) }, checksum[:]...), remoteAddr)
    conn.ReadFrom([]byte{})
    
    for i := 0; i < chunksNum; i++ {
        buf = make([]byte, 8)
        binary.LittleEndian.PutUint64(buf, uint64(i))
        conn.WriteTo(append([]byte{ byte(id) }, buf...), remoteAddr)
        conn.ReadFrom([]byte{})

        startIdx := i * CHUNK_SIZE

        if len(bytes[startIdx:]) < CHUNK_SIZE {
            conn.WriteTo(append([]byte{ byte(id) }, bytes[startIdx:]...), remoteAddr)
        } else {
            conn.WriteTo(append([]byte{ byte(id) }, bytes[startIdx:startIdx+CHUNK_SIZE]...), remoteAddr)
            conn.ReadFrom([]byte{})
        }
    }

    buf = make([]byte, 65000)

    conn.ReadFrom(buf)

    fmt.Println(string(buf))
}

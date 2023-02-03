package main

import (
    "fmt"
    "log"
    "net"
    "os"
    "time"
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

    conn.WriteTo([]byte(filename), remoteAddr)

    buf := make([]byte, 8)
    binary.LittleEndian.PutUint64(buf, uint64(chunksNum))
    conn.WriteTo(buf, remoteAddr)

    checksum := sha256.Sum256(bytes)
    conn.WriteTo(checksum[:], remoteAddr)
    
    for i := 0; i < chunksNum; i++ {
        buf = make([]byte, 8)
        binary.LittleEndian.PutUint64(buf, uint64(i))
        conn.WriteTo(buf, remoteAddr)

        startIdx := i * CHUNK_SIZE

        if len(bytes[startIdx:]) < CHUNK_SIZE {
            conn.WriteTo(bytes[startIdx:], remoteAddr)
        } else {
            conn.WriteTo(bytes[startIdx:startIdx+CHUNK_SIZE], remoteAddr)
        }

        time.Sleep(time.Second / 10)
    }

    buf = make([]byte, 65000)

    conn.ReadFrom(buf) 

    fmt.Println(string(buf))
}

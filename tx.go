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
    checksum := sha256.Sum256(bytes)

    buf := []byte{ 0xff, 0x00, 0x00, 0x00, 0x00 }
    binary.LittleEndian.PutUint32(buf[1:], uint32(chunksNum))
    buf = append(buf, checksum[:]...)
    buf = append(buf, []byte(filename)...)
    conn.WriteTo(buf, remoteAddr)

    buf = make([]byte, 4)
    conn.ReadFrom(buf)
    id := binary.LittleEndian.Uint32(buf)

    for i := 0; i < chunksNum; i++ {
        buf = []byte{ byte(id), 0x00, 0x00, 0x00, 0x00 }
        binary.LittleEndian.PutUint32(buf[1:], uint32(i))

        startIdx := i * CHUNK_SIZE

        if len(bytes[startIdx:]) < CHUNK_SIZE {
            conn.WriteTo(append(buf, bytes[startIdx:]...), remoteAddr)
        } else {
            conn.WriteTo(append(buf, bytes[startIdx:startIdx+CHUNK_SIZE]...), remoteAddr)
            conn.ReadFrom([]byte{})
        }
    }

    buf = make([]byte, 65000)

    conn.ReadFrom(buf)

    fmt.Println(string(buf))
}

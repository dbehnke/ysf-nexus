package main

import (
    "bytes"
    "encoding/binary"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "math/rand"
    "net"
    "net/http"
    "strings"
    "time"
)

func pad10(s string) string {
    if len(s) >= 10 {
        return s[:10]
    }
    return fmt.Sprintf("%-10s", s)
}

// makeYSFD builds a YSFD data packet expected by the server parser.
// Format: bytes 0:4 = "YSFD"
//         bytes 4:14 = gateway callsign (10 bytes)
//         bytes 14:24 = source callsign (10 bytes) - actual transmitter
//         bytes 24:34 = destination callsign (10 bytes)
//         rest = data payload
func makeYSFD(gatewayCS, sourceCS string, seq uint32) []byte {
    pkt := make([]byte, 155)
    copy(pkt[0:4], "YSFD")
    // Gateway callsign (bytes 4-14)
    copy(pkt[4:14], pad10(gatewayCS))
    // Source callsign (bytes 14-24) - the actual transmitter
    copy(pkt[14:24], pad10(sourceCS))
    // Destination callsign (bytes 24-34) - could be "CQCQCQ" or specific call
    copy(pkt[24:34], pad10("CQCQCQ"))
    // Sequence number - actually starts at byte 34 in real protocol
    // But we'll keep it simple and use byte 34 for sequence
    binary.BigEndian.PutUint32(pkt[34:38], seq)
    for i := 38; i < len(pkt); i++ {
        pkt[i] = byte(rand.Intn(256))
    }
    return pkt
}

func pollAPI(pollURL string, stop <-chan struct{}) {
    client := &http.Client{Timeout: 2 * time.Second}
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-stop:
            return
        case <-ticker.C:
            resp, err := client.Get(pollURL)
            if err != nil {
                log.Printf("poll error: %v", err)
                continue
            }
            body, _ := io.ReadAll(resp.Body)
            _ = resp.Body.Close()
            // try to pretty-print JSON
            var out bytes.Buffer
            if err := json.Indent(&out, body, "", "  "); err == nil {
                log.Printf("API: %s", out.String())
            } else {
                log.Printf("API: %s", strings.TrimSpace(string(body)))
            }
        }
    }
}

func main() {
    host := flag.String("host", "127.0.0.1", "server UDP host")
    port := flag.Int("port", 42000, "server UDP port")
    sourcePort := flag.Int("source-port", 0, "local UDP source port to bind (0 = ephemeral)")
    gatewayCS := flag.String("gateway", "US-KCWIDE", "gateway callsign (bytes 4-14)")
    sourceCS := flag.String("source", "W1ABC", "source callsign - actual transmitter (bytes 14-24)")
    duration := flag.Duration("duration", 8*time.Second, "how long to send voice frames")
    interval := flag.Duration("interval", 20*time.Millisecond, "interval between frames")
    pollURL := flag.String("poll", "http://localhost:8080/api/current-talker", "API URL to poll")
    flag.Parse()

    addr := net.UDPAddr{IP: net.ParseIP(*host), Port: *port}

    var laddr *net.UDPAddr
    if *sourcePort != 0 {
        laddr = &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: *sourcePort}
    }

    conn, err := net.DialUDP("udp", laddr, &addr)
    if err != nil {
        log.Fatalf("failed to dial UDP %s:%d: %v", *host, *port, err)
    }
    if err := conn.Close(); err != nil {
        log.Printf("failed to close UDP conn: %v", err)
    }
    log.Printf("sending to %s (gateway=%s, source=%s)", conn.RemoteAddr(), *gatewayCS, *sourceCS)

    stopPoll := make(chan struct{})
    go pollAPI(*pollURL, stopPoll)

    // send YSFD data packets for duration
    end := time.Now().Add(*duration)
    var seq uint32 = 1
    for time.Now().Before(end) {
        pkt := makeYSFD(*gatewayCS, *sourceCS, seq)
        if _, err := conn.Write(pkt); err != nil {
            log.Printf("write frame error: %v", err)
        }
        seq++
        time.Sleep(*interval)
    }

    // allow one more poll
    time.Sleep(1100 * time.Millisecond)
    close(stopPoll)
    // small delay for poll goroutine to exit cleanly
    time.Sleep(200 * time.Millisecond)
    log.Printf("done")
}

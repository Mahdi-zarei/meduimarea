package main

import (
	"github.com/google/uuid"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger *log.Logger
var destIP string
var destPort int
var connCount int
var ender []byte
var bufferSize int

func main() {
	var tmp []string
	tmp = []string{"\000", "\002", "\004", "\000"}
	for _, x := range tmp {
		ender = append(ender, []byte(x)...)
	}

	destIP = "127.0.0.1"
	destPort = 5555
	bufferSize = 512 * 1024
	connCount = 4
	logger = log.New(os.Stdout, "", log.LstdFlags)
	srv, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 2222})
	if err != nil {
		panic(err)
	}

	for {
		conn, err := srv.AcceptTCP()
		if err != nil {
			logger.Printf("error in accepting connection: %s", err)
			continue
		}
		conn.SetNoDelay(true)
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(1 * time.Second)

		go handleConnection(conn)
	}
}

func handleConnection(src *net.TCPConn) {
	data, _ := getConnectHttp(src)

	conns, resp, err := makeUpstreamConnections(data)
	if err != nil {
		logger.Printf("error in making upstream connections: %s", err)
		return
	}
	src.Write(resp)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer closeConnections(conns)
		defer src.Close()
		handleOneToManyForward(src, conns)
		wg.Done()
	}()

	go func() {
		defer closeConnections(conns)
		defer src.Close()
		handleManyToOneForward(src, conns)
		wg.Done()
	}()

	logger.Printf("started forwarding for %s", strings.Split(src.RemoteAddr().String(), ":")[0])
	wg.Wait()
}

func makeUpstreamConnections(data []byte) (map[int]*net.TCPConn, []byte, error) {
	conns := make(map[int]*net.TCPConn)
	uid := uuid.NewString()
	resp := make([]byte, 1000)

	for i := 0; i < connCount; i++ {
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
			IP:   net.ParseIP(destIP),
			Port: destPort,
		})
		if err != nil {
			closeConnections(conns)
			return nil, nil, err
		}
		conn.SetNoDelay(true)

		resp, err = sendHttpConnect(data, conn)
		if err != nil {
			closeConnections(conns)
			return nil, nil, err
		}

		data := uid + "#" + strconv.Itoa(i)
		binaryData := []byte(data)
		binaryData = append(binaryData, ender...)
		_, err = conn.Write(binaryData)
		if err != nil {
			closeConnections(conns)
			return nil, nil, err
		}
		conns[i] = conn
	}

	time.Sleep(1 * time.Second)
	return conns, resp, nil
}

func closeConnections(conns map[int]*net.TCPConn) {
	for _, conn := range conns {
		conn.Close()
	}
}

func handleManyToOneForward(dest *net.TCPConn, conns map[int]*net.TCPConn) {
	buffer := make([]byte, connCount*bufferSize)
	cnt := 0
	for {
		nr, err := conns[cnt].Read(buffer)
		if nr > 0 {

			if isComplete(buffer, nr) {
				nr = nr - 4
				cnt++
				cnt %= connCount
			}

			logger.Println("read ", cnt)
			_, err2 := dest.Write(buffer[:nr])
			if err2 != nil {
				logger.Printf("error in writing to openvpn connection: %s", err2)
				return
			}

		}

		if err != nil {
			logger.Printf("error in reading from proxy mux connection: %s", err)
			return
		}
	}
}

func handleOneToManyForward(src *net.TCPConn, conns map[int]*net.TCPConn) {
	buffer := make([]byte, connCount*bufferSize)
	cnt := 0
	last := time.Now()
	for {
		nr, err := src.Read(buffer)
		if nr > 0 {

			flag := false
			if time.Now().Sub(last) >= time.Second {
				buffer, nr = appendToBuffer(buffer, nr)
				last = time.Now()
				flag = true
			}

			logger.Println("write ", cnt)
			_, err2 := conns[cnt].Write(buffer[:nr])
			if err2 != nil {
				logger.Printf("error in writing to mux connection: %s", err2)
				return
			}

			if flag {
				cnt++
				cnt %= connCount
			}

		}

		if err != nil {
			logger.Printf("error in reading from openvpn connection: %s", err)
			return
		}
	}
}

func isComplete(buffer []byte, nr int) bool {
	if nr >= len(ender) {

		for i := 0; i < len(ender); i++ {
			if buffer[nr-len(ender)+i] != ender[i] {
				return false
			}
		}

		return true
	}
	return false
}

func appendToBuffer(buffer []byte, nr int) ([]byte, int) {
	for i := 0; i < len(ender); i++ {
		buffer[nr+i] = ender[i]
	}

	return buffer, nr + len(ender)
}

func getConnectHttp(src *net.TCPConn) ([]byte, error) {
	buffer := make([]byte, 10000)
	nr, err := src.Read(buffer)
	if err != nil {
		return nil, err
	}

	return buffer[:nr], nil
}

func sendHttpConnect(data []byte, conn *net.TCPConn) ([]byte, error) {
	resp := make([]byte, 1000)
	_, err := conn.Write(data)
	if err != nil {
		return nil, err
	}

	nr, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}

	return resp[:nr], nil
}

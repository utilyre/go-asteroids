package main

import (
	"encoding/binary"
	"log/slog"
	"multiplayer/pkg/plaiq"
	"net"
	"time"
)

func (conn *Conn) Do() {
	sizeData := make([]byte, 2)
	n, firstAddr, err := conn.ReadFromUDP(sizeData)
	if err != nil {
		slog.Error("failed to read size from udp", "error", err)
		return
	}
	if n != len(sizeData) {
		panic("unexpected bytes read from udp")
	}

	var size uint16
	n, err = binary.Decode(sizeData, binary.BigEndian, &size)
	if err != nil {
		slog.Error("failed to decode big endian data size", "error", err)
		return
	}
	if n != len(sizeData) {
		panic("unexpected number of bytes consumed for size")
	}

	payloadSize := 2
	if size%2 == 0 {
		payloadSize += int(size) / 2
	} else {
		payloadSize += int(size)/2 + 1
	}
	data := make([]byte, payloadSize)
	n, secondAddr, err := conn.ReadFromUDP(data)
	if err != nil {
		slog.Error("failed to read data from udp", "error", err)
		return
	}
	if n != len(data) {
		panic("unexpected number of bytes for data")
	}
	if firstAddr.String() != secondAddr.String() {
		panic("received data from different client")
	}

	inputs := make([]Input, size)
	for i := range size {
		inputs[i] = Input{
			Index: 0,
			Up:    false,
			Left:  false,
			Down:  false,
			Right: false,
		}
	}
}

type Conn struct {
	*net.UDPConn
	inputQueue *plaiq.PlayQueue[Input]
}

func Listen(addr string) (*Conn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	inputQueue := plaiq.New[Input](time.Second / 60)

	conn := &Conn{
		UDPConn:    udpConn,
		inputQueue: inputQueue,
	}

	go conn.listenInput()

	return conn, nil
}

func (conn *Conn) listenInput() {
	buf := make([]byte, 1)
	n, remote, err := conn.ReadFromUDP(buf)
	if n != len(buf) {
		panic("read less bytes than needed")
	}
	if err != nil {
		slog.Error("failed to read from udp", "remote", remote, "error", err)
	}

	input := Input{
		Up:    buf[0]&(1<<0) != 0,
		Left:  buf[0]&(1<<1) != 0,
		Down:  buf[0]&(1<<2) != 0,
		Right: buf[0]&(1<<4) != 0,
	}
	conn.inputQueue.Enqueue(input)
}

func (conn *Conn) Close() error {
	err := conn.inputQueue.Close()
	if err != nil {
		return err
	}

	err = conn.UDPConn.Close()
	if err != nil {
		return err
	}

	return nil
}

func (conn *Conn) ReceiveInput() Input {
	input := conn.inputQueue.Dequeue()
	// TODO: ack received input
	return input
}

package packet

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/util"
)

type GameServerPacketReader struct {
	// non-mutable
	ctx      context.Context
	packetCh chan *GamePacket

	// mutable
	handle *pcap.Handle
}

type GameServerPacketReaderOpt struct {
	Ctx      context.Context
	NicName  string
	ClientIp string
}

const pcapQueueSize = 100
const pcapBufferSize = 32 * 1024 * 1024
const pcapPromisc = true
const packetQueueSize = 100

var ErrTooShortPacket = errors.New("too short packet")

func NewGameServerPacketReader(opt *GameServerPacketReaderOpt) (*GameServerPacketReader, error) {
	if opt == nil {
		return nil, errors.New("opt is nil")
	}

	filter := constants.PCAP_GAMESERVER_FILTER
	if opt.ClientIp != "" {
		// 어차피 클라이언트에서 서버로 전송하는 패킷은 암호화 되어있음
		filter = " dst host " + opt.ClientIp
	}

	logger.Println("game packet filter...", filter)

	v := &GameServerPacketReader{
		ctx:      opt.Ctx,
		packetCh: make(chan *GamePacket, packetQueueSize),
	}

	payloadCh, err := v.openNic(opt.NicName, filter)
	if err != nil {
		logger.Println("openNic failed", err)
		return nil, err
	}

	go v.packetLoop(payloadCh)

	return v, nil
}

func (t *GameServerPacketReader) packetLoop(payloadCh <-chan []byte) {

	buffer := bytes.NewBuffer(nil)

	for {
		select {
		case <-t.ctx.Done():
			return

		case payload := <-payloadCh:
			buffer.Write(payload)
		}

	readerLoop:
		for {
			msg, err := gamePacketReader(buffer)
			if err != nil {
				if err == io.EOF {
					break readerLoop
				}

				logger.Printf("game packet parse error %v", err)
				continue
			}

			if msg != nil {
				t.packetCh <- msg
			}
		}
	}
}

func (t *GameServerPacketReader) openNic(nic string, filter string) (<-chan []byte, error) {
	handle, err := pcap.OpenLive(nic, pcapBufferSize, pcapPromisc, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	t.handle = handle

	if err := handle.SetBPFFilter(filter); err != nil { // optional
		return nil, err
	}

	dlc := gopacket.DecodingLayerContainer(gopacket.DecodingLayerArray(nil))
	dlc.Put(new(layers.Ethernet))
	dlc.Put(new(layers.IPv4))
	dlc.Put(new(layers.TCP))

	ch := make(chan []byte, pcapQueueSize)
	ps := gopacket.NewPacketSource(handle, handle.LinkType())

	go func() {
		for packet := range ps.Packets() {
			tl := packet.TransportLayer()
			if tl == nil {
				// empty (ack?)
				continue
			}

			payload := tl.LayerPayload()
			if len(payload) == 0 {
				continue
			}

			ch <- payload
		}
	}()

	return ch, nil
}

func (t *GameServerPacketReader) Close() {
	if t.handle != nil {
		t.handle.Close()
		t.handle = nil
	}
}

func (t *GameServerPacketReader) PacketCh() <-chan *GamePacket {
	return t.packetCh
}

func gamePacketReader(buffer *bytes.Buffer) (*GamePacket, error) {
	headerSize := 6

	rawPacketBuffer := bytes.NewBuffer(nil)
	b := buffer.Bytes()

	// 헤더 읽기에 아직 부족
	if len(b) < 6 {
		return nil, io.EOF
	}

	sign := b[0]
	// 패킷의 총 사이즈 (헤더 포함)
	length := le.Uint32(b[1:])
	// maybe
	flag := b[5]

	// ?
	if length == 0 || length > 0x100_0000 {
		buffer.Reset()
		logger.Println("warning packet miss-align")
		return nil, nil
	}

	isShortPacket := flag == 1 || // heartbeat
		flag == 2 // ? server only

	if isShortPacket {
		// 패킷이 아직 모자람
		if len(b) < int(length)-6 {
			return nil, io.EOF
		}

		shortBody := b[6:int(length)]
		rawPacketBuffer.Write(shortBody)

		buffer.Next(int(length))

		// checksum := uint32(0)
		v := &GamePacket{
			Sign:   sign,
			Length: length,
			Flag:   flag,

			IsShortPacket: true,
			ShortBody:     shortBody,

			RawPacket: rawPacketBuffer.Bytes(),
		}

		return v, nil
	}

	// too small
	if int(length) < headerSize+0xd {
		buffer.Next(int(length))
		return nil, ErrTooShortPacket
	}

	if buffer.Len() < int(length) {
		return nil, io.EOF
	}

	body := b[:int(length)]
	rawPacketBuffer.Write(body)

	buffer.Next(int(length))

	body = body[headerSize:]

	op := be.Uint32(body)
	body = body[4:]

	id := be.Uint64(body)
	body = body[8:]

	_, lenbytes := binary.Uvarint(body)
	body = body[lenbytes:]

	msg, err := NewMessage(bytes.NewReader(body))
	if err != nil {
		logger.Println("gameProxy packetHeader body read error", err, "\n", hex.Dump(body))
		return nil, err
	}

	p := &GamePacket{
		Sign:   sign,
		Length: length,
		Flag:   flag,

		Op:  op,
		Id:  id,
		Msg: msg,

		RawPacket: rawPacketBuffer.Bytes(),
	}

	return p, nil
}

// op, id, msg, err
func GamePacketBodyReader(r io.Reader) (uint32, uint64, Message, error) {
	b := make([]byte, 8)

	if _, err := io.ReadFull(r, b[:4]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	op := be.Uint32(b[:4])

	if _, err := io.ReadFull(r, b[:8]); err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	id := be.Uint64(b[:8])

	_, lenbytes, err := util.ReadUvarint(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	_ = lenbytes

	msg, err := NewMessage(r)
	if err != nil {
		logger.Println(err)
		return 0, 0, nil, err
	}

	return op, id, msg, nil
}

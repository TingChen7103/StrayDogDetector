package packet

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/util"
)

type GameServerPacketReader struct {
	// non-mutable
	ctx      context.Context
	packetCh chan *GamePacket

	// mutable; mu 保護以下欄位 (Close 可能與 readPacketLoop 並行)
	mu     sync.Mutex
	handle *pcap.Handle
	fd     *os.File

	logHandle *pcapgo.NgWriter
	logFd     *os.File
}

type GameServerPacketReaderOpt struct {
	Ctx         context.Context
	FileName    string
	NicName     string
	ClientIp    string
	SaveRawData bool
}

// 以 (伺服器IP, 伺服器port, 客戶端port) 區分 TCP 連線;
// filter 只收 server→client 方向,客戶端 port 足以區分頻道連線
type flowKey struct {
	srcIP   [4]byte
	srcPort uint16
	dstPort uint16
}

type gamePacketPayload struct {
	flow flowKey
	data []byte
	at   time.Time

	// aligned: 此 payload 的第一個 byte 是框架邊界 (新連線丟棄 4-byte crypt key 之後)
	aligned bool
	// desync: 此 payload 之前有資料遺失,解析器必須重新同步
	desync bool
}

type pendingSeg struct {
	seq  uint32
	data []byte
	at   time.Time
}

// 單一 TCP 連線的重組狀態
type flowReasm struct {
	started      bool
	nextSeq      uint32
	pending      []pendingSeg // 依 seq 排序的亂序/缺口暫存
	pendingBytes int
	gapSince     time.Time
	lastSeen     time.Time

	alignedNext bool
	desyncNext  bool
}

const pcapQueueSize = 100
const pcapBufferSize = 32 * 1024 * 1024
const pcapPromisc = true
const packetQueueSize = 100

// 最大合法框架長度 (進場快照實測可達 ~1.35MB)
const maxFrameLength = 0x100_0000

// 缺口暫存上限: 超過即放棄缺口、跳到已收到的最早資料重新同步
const maxPendingBytes = 4 * 1024 * 1024
const maxPendingSegs = 4096
const gapTimeout = 3 * time.Second

const maxFlows = 32

// 未同步時的緩衝上限 (假候選可能宣告超大長度,避免無限等待)
const maxUnsyncedBuffer = 4 * 1024 * 1024
const unsyncedKeepBytes = 2 * 1024 * 1024

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

	// 20251228 Hans: 不產生封包紀錄檔(.pcapng)
	// 20260728: 根據前端設定決定是否產生紀錄檔
	if opt.SaveRawData {
		if err := v.openLog(); err != nil {
			logger.Println("openLog failed", err)
			return nil, err
		}
	}

	payloadCh := (<-chan gamePacketPayload)(nil)
	err := error(nil)
	if opt.FileName != "" {
		payloadCh, err = v.openFile(opt.FileName, filter)
		if err != nil {
			logger.Println("openFile failed", err)
			return nil, err
		}
	} else {
		payloadCh, err = v.openNic(opt.NicName, filter)
		if err != nil {
			logger.Println("openNic failed", err)
			return nil, err
		}
	}

	go v.packetLoop(payloadCh)

	return v, nil
}

func (t *GameServerPacketReader) packetLoop(payloadCh <-chan gamePacketPayload) {
	parsers := make(map[flowKey]*streamParser)

	for {
		payloadData := gamePacketPayload{}

		select {
		case <-t.ctx.Done():
			return

		case payloadData = <-payloadCh:
		}

		p := parsers[payloadData.flow]
		if p == nil {
			if len(parsers) >= maxFlows {
				evictOldestParser(parsers)
			}

			p = &streamParser{}
			parsers[payloadData.flow] = p
		}

		p.feed(payloadData)

	readerLoop:
		for {
			msg, err := p.next()
			if err != nil {
				if err == io.EOF {
					break readerLoop
				}

				if err == ErrTooShortPacket {
					// 協定內的無 op 小框架,按設計跳過,非錯誤
					continue
				}

				logger.Printf("game packet parse error flow %v %v", payloadData.flow, err)
				continue
			}

			if msg != nil {
				select {
				case t.packetCh <- msg:
				case <-t.ctx.Done():
					return
				}
			}
		}
	}
}

func evictOldestParser(parsers map[flowKey]*streamParser) {
	// 優先驅逐從未同步成功的 (多半是被 filter 放進來的無關流量)
	oldestKey, oldestAt := flowKey{}, time.Time{}
	first, haveUnsynced := true, false

	for k, v := range parsers {
		if haveUnsynced && v.synced {
			continue
		}

		if !haveUnsynced && !v.synced {
			haveUnsynced = true
			first = true
		}

		if first || v.lastSeen.Before(oldestAt) {
			oldestKey, oldestAt = k, v.lastSeen
			first = false
		}
	}

	if !first {
		delete(parsers, oldestKey)
	}
}

func (t *GameServerPacketReader) openNic(nic string, filter string) (<-chan gamePacketPayload, error) {
	// 有限 timeout 讓閒置期間仍可定期 flush pcapng 與檢查缺口逾時
	handle, err := pcap.OpenLive(nic, pcapBufferSize, pcapPromisc, 500*time.Millisecond)
	if err != nil {
		logger.Println(err)
		return nil, err
	}
	t.handle = handle

	if err := handle.SetBPFFilter(filter); err != nil { // optional
		return nil, err
	}

	ch := make(chan gamePacketPayload, pcapQueueSize)

	go t.readPacketLoop(ch)

	return ch, nil
}

func (t *GameServerPacketReader) openFile(file string, filter string) (<-chan gamePacketPayload, error) {
	fd, err := os.OpenFile(file, os.O_RDONLY, 0644)
	if err != nil {
		logger.Println(err)
		return nil, err
	}

	t.fd = fd

	/*
		OpenOffline으로 함수를 호출 할 때 fileName을 utf8로 넘기는데,
		libpcap에서는 multibyte용 fopen으로 파일을 연다 -> 한글 경로일 때 깨짐

		libpcap에서 pcap_init(PCAP_CHAR_ENC_UTF_8) 호출하는게 나을 수 도
	*/
	handle, err := pcap.OpenOfflineFile(fd)
	if err != nil {
		logger.Println(err)
		return nil, err
	}

	if err := handle.SetBPFFilter(filter); err != nil { // optional
		logger.Println(err)
		return nil, err
	}

	t.handle = handle

	ch := make(chan gamePacketPayload, pcapQueueSize)

	go t.readPacketLoop(ch)
	return ch, nil
}

func (t *GameServerPacketReader) openLog() error {
	fileName := fmt.Sprintf("packet_capture_%v.pcapng", time.Now().Format("20060102_150405"))
	fd, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logger.Println(err)
		return err
	}

	t.logFd = fd

	handle, err := pcapgo.NewNgWriter(fd, layers.LinkTypeEthernet)
	if err != nil {
		logger.Println(err)
		return err
	}

	t.logHandle = handle
	logger.Println("Raw packet capture started:", fileName)

	return nil
}

func (t *GameServerPacketReader) readPacketLoop(ch chan<- gamePacketPayload) {
	defer t.Close()
	ethLayer := layers.Ethernet{}
	ip4Layer := layers.IPv4{}
	tcpLayer := layers.TCP{}
	payload := gopacket.Payload{}

	layerParser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet, &ethLayer, &ip4Layer, &tcpLayer, &payload)
	packetLayers := []gopacket.LayerType(nil)

	flows := make(map[flowKey]*flowReasm)
	lastFlush := time.Now()

	// ctx 取消時也要能離開 (消費端可能已退出),否則 defer Close 永不執行、pcapng 不會 flush
	send := func(fl *flowReasm, key flowKey, data []byte, at time.Time) bool {
		pd := gamePacketPayload{
			flow:    key,
			data:    data,
			at:      at,
			aligned: fl.alignedNext,
			desync:  fl.desyncNext,
		}
		fl.alignedNext, fl.desyncNext = false, false

		select {
		case ch <- pd:
			return true
		case <-t.ctx.Done():
			return false
		}
	}

	drainPending := func(fl *flowReasm, key flowKey) bool {
		for len(fl.pending) > 0 {
			s := fl.pending[0]
			d := int32(s.seq - fl.nextSeq)
			if d > 0 {
				break
			}

			data := s.data
			if d < 0 {
				// 與已送出的資料重疊,修剪掉重複部分
				if int32(s.seq+uint32(len(s.data))-fl.nextSeq) <= 0 {
					fl.pending = fl.pending[1:]
					fl.pendingBytes -= len(s.data)
					continue
				}

				data = data[fl.nextSeq-s.seq:]
			}

			if !send(fl, key, data, s.at) {
				return false
			}

			fl.nextSeq = s.seq + uint32(len(s.data))
			fl.pending = fl.pending[1:]
			fl.pendingBytes -= len(s.data)
		}

		if len(fl.pending) == 0 {
			fl.gapSince = time.Time{}
		}

		return true
	}

	// 放棄目前缺口: 跳到最早的暫存段,並要求解析器重新同步
	jumpGap := func(fl *flowReasm, key flowKey) bool {
		if len(fl.pending) < 1 {
			return true
		}

		fl.nextSeq = fl.pending[0].seq
		fl.desyncNext = true
		fl.alignedNext = false
		fl.gapSince = time.Time{}
		return drainPending(fl, key)
	}

	t.mu.Lock()
	handle := t.handle
	t.mu.Unlock()
	if handle == nil {
		return
	}

	// 缺口逾時掃描: 不能只靠同一條流的下一個封包觸發
	sweepGaps := func(now time.Time) bool {
		for k, ofl := range flows {
			if !ofl.gapSince.IsZero() && now.Sub(ofl.gapSince) > gapTimeout {
				logger.Println("gap timeout, resync", k)
				if !jumpGap(ofl, k) {
					return false
				}
			}
		}
		return true
	}

	for i := 0; t.ctx.Err() == nil; i++ {
		b, ci, err := handle.ReadPacketData()
		if err != nil {
			if err == pcap.NextErrorTimeoutExpired {
				// 閒置喚醒 (live 模式): 維持 pcapng flush 與缺口逾時檢查
				t.mu.Lock()
				if t.logHandle != nil && time.Since(lastFlush) > 2*time.Second {
					_ = t.logHandle.Flush()
					lastFlush = time.Now()
				}
				t.mu.Unlock()

				if !sweepGaps(time.Now()) {
					return
				}
				continue
			}

			logger.Println(err, i)
			break
		}

		t.mu.Lock()
		if t.logHandle != nil {
			// ignore error
			_ = t.logHandle.WritePacket(ci, b)

			// 定期 flush,避免行程被強制結束時 pcapng 尾端遺失
			if time.Since(lastFlush) > 2*time.Second {
				_ = t.logHandle.Flush()
				lastFlush = time.Now()
			}
		}
		t.mu.Unlock()

		if err := layerParser.DecodeLayers(b, &packetLayers); err != nil {
			logger.Println(err)
			continue
		}

		hasTcp := false
		for _, layer := range packetLayers {
			if layer == layers.LayerTypeTCP {
				hasTcp = true
			}
		}

		if !hasTcp {
			continue
		}

		finOrRst := tcpLayer.FIN || tcpLayer.RST

		key := flowKey{
			srcPort: uint16(tcpLayer.SrcPort),
			dstPort: uint16(tcpLayer.DstPort),
		}
		copy(key.srcIP[:], ip4Layer.SrcIP.To4())

		if len(tcpLayer.Payload) < 1 {
			// 連線結束: 榨出殘餘 pending 後移除流狀態,避免連接埠重用時沿用舊 seq
			if finOrRst {
				if fl := flows[key]; fl != nil {
					for len(fl.pending) > 0 {
						if !jumpGap(fl, key) {
							return
						}
					}
					delete(flows, key)
				}
			}
			continue
		}

		fl := flows[key]
		if fl == nil {
			if len(flows) >= maxFlows {
				// 驅逐前先榨出殘餘 pending
				if k, ofl := oldestFlow(flows); ofl != nil {
					for len(ofl.pending) > 0 {
						if !jumpGap(ofl, k) {
							return
						}
					}
					delete(flows, k)
				}
			}

			fl = &flowReasm{}
			flows[key] = fl
		}
		fl.lastSeen = ci.Timestamp

		if fl.started {
			// 序號差距離譜: 連接埠重用的新連線 (FIN/RST 沒被擷取到),重置流狀態
			if d := int32(tcpLayer.Seq - fl.nextSeq); d < -(1<<28) || d > (1<<28) {
				*fl = flowReasm{lastSeen: ci.Timestamp, desyncNext: true}
			}
		}

		if !fl.started {
			fl.started = true
			fl.nextSeq = tcpLayer.Seq + uint32(len(tcpLayer.Payload))
			// 可能是驅逐/FIN 清除後重建的流,舊解析器須重新同步
			fl.desyncNext = true

			if len(tcpLayer.Payload) == 4 {
				// 新連線的第一個 payload 是 4-byte crypt key,丟棄後即為框架邊界
				fl.alignedNext = true
				continue
			}

			// 從連線中途開始擷取,解析器需自行搜尋框架邊界
			if !send(fl, key, tcpLayer.Payload, ci.Timestamp) {
				return
			}
			continue
		}

		diff := int32(tcpLayer.Seq - fl.nextSeq)

		switch {
		case diff == 0:
			if !send(fl, key, tcpLayer.Payload, ci.Timestamp) {
				return
			}
			fl.nextSeq += uint32(len(tcpLayer.Payload))

			if !drainPending(fl, key) {
				return
			}

		case diff < 0:
			// 重傳: 只送出未見過的部分
			end := tcpLayer.Seq + uint32(len(tcpLayer.Payload))
			if int32(end-fl.nextSeq) > 0 {
				if !send(fl, key, tcpLayer.Payload[fl.nextSeq-tcpLayer.Seq:], ci.Timestamp) {
					return
				}
				fl.nextSeq = end

				if !drainPending(fl, key) {
					return
				}
			}

		default:
			// 缺口: 暫存等待補齊
			insertPending(fl, pendingSeg{
				seq:  tcpLayer.Seq,
				data: tcpLayer.Payload,
				at:   ci.Timestamp,
			})

			if fl.gapSince.IsZero() {
				fl.gapSince = ci.Timestamp
			}

			if fl.pendingBytes > maxPendingBytes ||
				len(fl.pending) > maxPendingSegs ||
				ci.Timestamp.Sub(fl.gapSince) > gapTimeout {

				logger.Println("gap give-up, resync", key, fl.nextSeq, fl.pending[0].seq)
				if !jumpGap(fl, key) {
					return
				}
			}
		}

		// FIN/RST 帶 payload: 處理完畢後移除流狀態
		if finOrRst {
			for len(fl.pending) > 0 {
				if !jumpGap(fl, key) {
					return
				}
			}
			delete(flows, key)
		}

		if !sweepGaps(ci.Timestamp) {
			return
		}
	}

	// 讀取結束 (檔案 EOF / handle 關閉): 盡量榨出暫存中的資料
	for key, fl := range flows {
		for len(fl.pending) > 0 {
			if !jumpGap(fl, key) {
				return
			}
		}
	}
}

func insertPending(fl *flowReasm, s pendingSeg) {
	pos := len(fl.pending)
	for i, v := range fl.pending {
		if int32(s.seq-v.seq) < 0 {
			pos = i
			break
		}

		if s.seq == v.seq {
			// 重複段,保留先到的
			return
		}
	}

	fl.pending = append(fl.pending, pendingSeg{})
	copy(fl.pending[pos+1:], fl.pending[pos:])
	fl.pending[pos] = s
	fl.pendingBytes += len(s.data)
}

func oldestFlow(flows map[flowKey]*flowReasm) (flowKey, *flowReasm) {
	oldestKey, oldestAt := flowKey{}, time.Time{}
	first := true

	for k, v := range flows {
		if first || v.lastSeen.Before(oldestAt) {
			oldestKey, oldestAt = k, v.lastSeen
			first = false
		}
	}

	if first {
		return flowKey{}, nil
	}

	return oldestKey, flows[oldestKey]
}

func (t *GameServerPacketReader) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.handle != nil {
		t.handle.Close()
		t.handle = nil
	}

	if t.fd != nil {
		t.fd.Close()
		t.fd = nil
	}

	if t.logHandle != nil {
		t.logHandle.Flush()
		t.logHandle = nil
	}

	if t.logFd != nil {
		t.logFd.Close()
		t.logFd = nil
	}
}

func (t *GameServerPacketReader) PacketCh() <-chan *GamePacket {
	return t.packetCh
}

/*
	單一 TCP 連線的框架解析器

	框架格式 (經 7 個實錄 pcapng、13,163 框驗證):
	byte0        不固定 (非 magic,疑似 checksum/序號)
	byte1-4      LE uint32 框架總長 (含 6-byte 標頭),實測可達 ~1.35MB
	byte5        flag (0..4;1=heartbeat、2=server only 為 short packet)
	之後         BE uint32 op + BE uint64 id + uvarint(訊息位元組長) + 型別標記訊息

	同步後純靠長度鏈接;失去同步時掃描候選標頭,
	以「訊息剛好填滿框架」驗證後才鎖定,避免鎖上 payload 內的假標頭。
*/
type streamParser struct {
	buffer   bytes.Buffer
	at       time.Time
	lastSeen time.Time

	synced  bool
	scanned int
	cands   []int
}

func (p *streamParser) feed(pd gamePacketPayload) {
	if pd.desync {
		p.buffer.Reset()
		p.synced = false
		p.resetScan()
	}

	if pd.aligned {
		// 框架邊界確定 (新連線 crypt key 之後): 丟棄任何殘留資料,直接進入同步狀態
		p.buffer.Reset()
		p.synced = true
		p.resetScan()
	}

	p.buffer.Write(pd.data)
	p.at = pd.at
	p.lastSeen = pd.at
}

func (p *streamParser) resetScan() {
	p.cands = p.cands[:0]
	p.scanned = 0
}

func (p *streamParser) desyncByte() {
	p.buffer.Next(1)
	p.synced = false
	p.resetScan()
}

func (p *streamParser) next() (*GamePacket, error) {
	if !p.synced {
		if !p.trySync() {
			return nil, io.EOF
		}
	}

	return p.readFrame()
}

func (p *streamParser) readFrame() (*GamePacket, error) {
	headerSize := 6

	b := p.buffer.Bytes()

	// 헤더 읽기에 아직 부족
	if len(b) < headerSize {
		return nil, io.EOF
	}

	sign := b[0]
	// 패킷의 총 사이즈 (헤더 포함)
	length := le.Uint32(b[1:])
	// maybe
	flag := b[5]

	if length < uint32(headerSize) || length > maxFrameLength {
		p.desyncByte()
		err := fmt.Errorf("invalid packet length %v", length)
		return nil, err
	}

	if flag > 4 {
		p.desyncByte()
		err := fmt.Errorf("invalid flag %v", flag)
		return nil, err
	}

	isShortPacket := flag == 1 || // heartbeat
		flag == 2 // ? server only

	if isShortPacket {
		// 패킷이 아직 모자람
		if len(b) < int(length) {
			return nil, io.EOF
		}

		shortBody := make([]byte, int(length)-headerSize)
		copy(shortBody, b[headerSize:int(length)])

		p.buffer.Next(int(length))

		v := &GamePacket{
			At:     p.at,
			Sign:   sign,
			Length: length,
			Flag:   flag,

			IsShortPacket: true,
			ShortBody:     shortBody,

			RawPacket: shortBody,
		}

		return v, nil
	}

	if len(b) < int(length) {
		return nil, io.EOF
	}

	// too small
	if int(length) < headerSize+0xd {
		p.buffer.Next(int(length))
		return nil, ErrTooShortPacket
	}

	body := b[:int(length)]

	rawPacket := make([]byte, len(body))
	copy(rawPacket, body)

	bodyBytes := body[headerSize:]

	op := be.Uint32(bodyBytes)
	bodyBytes = bodyBytes[4:]

	id := be.Uint64(bodyBytes)
	bodyBytes = bodyBytes[8:]

	_, lenbytes := binary.Uvarint(bodyBytes)
	if lenbytes <= 0 {
		p.desyncByte()
		err := fmt.Errorf("invalid message length %v", lenbytes)
		return nil, err
	}

	if len(bodyBytes) < lenbytes {
		p.desyncByte()
		err := fmt.Errorf("invalid message length %v %v", len(bodyBytes), lenbytes)
		return nil, err
	}

	bodyBytes = bodyBytes[lenbytes:]

	msgReader := bytes.NewReader(bodyBytes)
	msg, err := NewMessage(msgReader)
	if err != nil {
		p.desyncByte()
		logger.Println("gameProxy packetHeader body read error", err)
		return nil, err
	}

	// 訊息必須剛好填滿框架 (13,163 個實測框架 100% 符合),否則視為失步
	if msgReader.Len() != 0 {
		p.desyncByte()
		err := fmt.Errorf("message does not fill frame, %v bytes left", msgReader.Len())
		return nil, err
	}

	p.buffer.Next(int(length))

	v := &GamePacket{
		At:     p.at,
		Sign:   sign,
		Length: length,
		Flag:   flag,

		Op:  op,
		Id:  id,
		Msg: msg,

		RawPacket: rawPacket,
	}

	return v, nil
}

// 在未同步的資料中搜尋框架邊界; 鎖定成功回傳 true
func (p *streamParser) trySync() bool {
	b := p.buffer.Bytes()

	// 只掃描新進資料,收集看似合法的標頭候選
	start := p.scanned - 5
	if start < 0 {
		start = 0
	}

	for o := start; o+6 <= len(b); o++ {
		length := le.Uint32(b[o+1:])
		flag := b[o+5]

		if length >= 6 && length <= maxFrameLength && flag <= 4 {
			p.cands = append(p.cands, o)
		}
	}
	p.scanned = len(b)

	// 由最低偏移開始驗證候選
	for i := 0; i < len(p.cands); {
		o := p.cands[i]
		length := int(le.Uint32(b[o+1:]))
		flag := b[o+5]
		end := o + length

		if end > len(b) {
			// 框架還沒收齊,保留候選繼續等
			i++
			continue
		}

		ok := validateFrame(b[o:end])

		if ok && (flag == 1 || flag == 2) {
			// short packet 內容不透明,要求下一個標頭也合法才鎖定
			if end+6 > len(b) {
				i++
				continue
			}

			nextLength := le.Uint32(b[end+1:])
			nextFlag := b[end+5]
			ok = nextLength >= 6 && nextLength <= maxFrameLength && nextFlag <= 4
		}

		if !ok {
			p.cands = append(p.cands[:i], p.cands[i+1:]...)
			continue
		}

		if o > 0 {
			logger.Println("stream resynced, discarded", o, "bytes")
		}

		p.buffer.Next(o)
		p.synced = true
		p.resetScan()
		return true
	}

	// 假候選可能宣告超大長度導致無限等待,限制未同步緩衝大小
	if p.buffer.Len() > maxUnsyncedBuffer {
		trim := p.buffer.Len() - unsyncedKeepBytes
		p.buffer.Next(trim)

		kept := p.cands[:0]
		for _, o := range p.cands {
			if o >= trim {
				kept = append(kept, o-trim)
			}
		}
		p.cands = kept
		p.scanned -= trim
	}

	return false
}

// 驗證完整框架: 非 short packet 要求 op/id/uvarint/訊息剛好填滿框架
func validateFrame(fb []byte) bool {
	if len(fb) < 6 {
		return false
	}

	flag := fb[5]

	if flag == 1 || flag == 2 {
		return true
	}

	if len(fb) < 6+0xd {
		return false
	}

	body := fb[6+4+8:] // skip header, op, id

	_, lenbytes := binary.Uvarint(body)
	if lenbytes <= 0 || len(body) < lenbytes {
		return false
	}

	r := bytes.NewReader(body[lenbytes:])
	if _, err := NewMessage(r); err != nil {
		return false
	}

	return r.Len() == 0
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

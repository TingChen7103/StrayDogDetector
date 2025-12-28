package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gopacket/gopacket/pcap"
	"gitlab.com/prilus/mabidilmeter/constants"
	"gitlab.com/prilus/mabidilmeter/packet"
	"gitlab.com/prilus/mabidilmeter/pcaputil"
	"golang.org/x/net/websocket"
)

const port = 8030
const settingFileName = "setting.ini" // 定義設定檔名稱

//go:embed static
var staticFiles embed.FS

var logger = log.New(os.Stdout, "dilmeterapi ", log.LstdFlags|log.Lshortfile)
var packetLogFilename = ""

func main() {
	// main ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	logger.Println("* dilmatulgi", mode)

	switch mode {
	case "list":
		// nic list 출력
		nics, err := pcap.FindAllDevs()
		if err != nil {
			messagebox(fmt.Sprintf("FindAllDevs failed: %v", err))
			logger.Fatalln("FindAllDevs failed:", err)
		}

		sb := strings.Builder{}

		for i, nic := range nics {
			ipStr := "unknownAddress"
			if len(nic.Addresses) > 0 {
				ipStr = nic.Addresses[0].IP.String()
			}

			sb.WriteString(fmt.Sprintln("* nic", i, "name:", nic.Name, "ip:", ipStr))
		}

		s := sb.String()
		messagebox(s)
		logger.Println(s)
		return

	case "file":
		fileName := ""

		if len(os.Args) > 2 {
			fileName = os.Args[2]
		}

		run(ctx, "", fileName)

	case "":
		logger.Println("Check cached NIC...")

		// --- 修改處 START: 加入優先讀取設定檔的邏輯 ---
		nicName := ""
		cachedNic := loadNicSetting() // 1. 嘗試讀取設定檔

		if cachedNic != "" {
			// 2. 驗證該網卡是否還存在於系統中
			if checkNicExists(cachedNic) {
				logger.Println("Found cached NIC, skip scanning:", cachedNic)
				nicName = cachedNic
			} else {
				logger.Println("Cached NIC not found or invalid, fallback to full scan.")
			}
		}

		// 3. 如果沒有快取或快取無效，執行原本的掃描邏輯
		if nicName == "" {
			logger.Println("Scanning for active NIC...")
			var err error
			nicName, err = pcaputil.FindNic()
			if err != nil {
				messagebox(fmt.Sprintf("%v\nis mabinogi running?", err))
				logger.Fatalln("FindNic failed:", err)
			}
			// 4. 掃描成功後，儲存到設定檔
			saveNicSetting(nicName)
		}
		// --- 修改處 END ---

		run(ctx, nicName, "")

	default:
		_, err := os.Stat(mode)
		fileExists := err == nil

		nicName, fileName := "", ""

		if fileExists {
			fileName = mode
		} else {
			nicName = mode
		}

		run(ctx, nicName, fileName)
	}

	for {
		time.Sleep(1 * time.Second)
	}
}

func run(ctx context.Context, nicName string, fileName string) {
	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:      ctx,
		NicName:  nicName,
		FileName: fileName,
	})
	if err != nil {
		messagebox(fmt.Sprintf("NewGameServerPacketReader failed: %v", err))
		logger.Fatalln("NewGameServerPacketReader failed:", err)
	}

	pub := newEventPublisher(ctx, r)

	// packet writer (for debug)
	go func() {
		ch := make(chan iEvent, 10000)
		defer close(ch)

		pub.addClient(ctx, ch)
		if err := startPacketWriter(ctx, ch); err != nil {
			logger.Println("startPacketWriter failed:", err)
			return
		}
	}()

	startWebsocketServer(func(ws *websocket.Conn) {
		logger.Printf("Client connected from %s", ws.RemoteAddr())
		wsCtx, wsCtxCancel := context.WithCancel(ws.Request().Context())

		// 생각보다 websocket이 send queue 비워지는게 느리다
		ch := make(chan iEvent, 1000000)
		defer wsCtxCancel()
		defer close(ch)

		go pub.addClient(wsCtx, ch)

		packetReceiveLoop := func() {
			for {
				select {
				case <-wsCtx.Done():
					logger.Printf("Client disconnected from %s", ws.RemoteAddr())
					return

				default:
					_ = 1
				}

				var event string
				err := websocket.JSON.Receive(ws, &event)
				if err != nil {
					logger.Printf("Receive failed: %s; closing connection...", err.Error())
					if err = ws.Close(); err != nil {
						logger.Println("Error closing connection:", err.Error())
					}

					wsCtxCancel()
					break
				} else {
					// discard...
					logger.Println("Received:", event)
				}
			}
		}

		go packetReceiveLoop()

		for {
			select {
			case <-wsCtx.Done():
				logger.Printf("Client disconnected from %s", ws.RemoteAddr())
				return

			case e := <-ch:
				err := websocket.JSON.Send(ws, e)
				if err != nil {
					logger.Printf("Can't send: %s", err.Error())
					return
				}
			}
		}
	})

	if runtime.GOOS == "windows" {
		// ignore error
		go exec.Command("explorer", fmt.Sprintf("http://127.0.0.1:%v", port)).Run()
	}
}

func startWebsocketServer(newClientCb func(*websocket.Conn)) {
	remote, err := url.Parse("https://mabires.pril.cc")
	if err != nil {
		panic(err)
	}

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			// trim /res/ prefix
			r.URL.Path = r.URL.Path[4:]
			r.Host = remote.Host

			p.ServeHTTP(w, r)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.ModifyResponse = func(r *http.Response) error {
		r.Header.Set("Access-Control-Allow-Origin", "*")
		return nil
	}

	http.Handle("/ws", websocket.Handler(newClientCb))
	http.HandleFunc("/api/packet_log", httpHandlerPacketLog)
	http.HandleFunc("/res/", handler(proxy))

	var staticFS = fs.FS(staticFiles)
	htmlContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		logger.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(htmlContent)))

	logger.Printf("Server listening on port %d", port)

	go func() {
		err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
		if err != nil {
			messagebox(fmt.Sprintf("ListenAndServe failed: %v", err))
			logger.Fatalln(err)
		}
	}()

	<-time.After(1 * time.Second)
}

func startPacketWriter(ctx context.Context, ch <-chan iEvent) error {
	packetLogFilename = fmt.Sprintf("packet_log_%v.ndjson", constants.SERVER_START_AT)

	fd, err := os.OpenFile(packetLogFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logger.Println("packetWriter open file failed:", err)
		return err
	}
	defer fd.Close()

	flushTicker := time.NewTicker(5 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case e := <-ch:
			b, err := json.Marshal(e)
			if err != nil {
				// ?
				continue
			}

			b = append(b, '\n')

			_, err = fd.Write(b)
			if err != nil {
				logger.Println("packetWriter write failed:", err)
				return err
			}

		case <-flushTicker.C:
			err := fd.Sync()
			if err != nil {
				// ignore
				_ = 1
			}
		}
	}
}

// loadNicSetting 讀取 setting.ini 中的 NIC 設定
func loadNicSetting() string {
	file, err := os.Open(settingFileName)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "NIC") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// saveNicSetting 將 NIC 名稱寫入 setting.ini
func saveNicSetting(nicName string) {
	content := fmt.Sprintf("NIC = %s", nicName)
	err := os.WriteFile(settingFileName, []byte(content), 0644)
	if err != nil {
		logger.Println("Failed to save setting.ini:", err)
	} else {
		logger.Println("Saved NIC to setting.ini:", nicName)
	}
}

// checkNicExists 快速檢查該 NIC 是否存在於系統中 (不進行耗時的流量掃描)
func checkNicExists(targetNic string) bool {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println("Error finding devices:", err)
		return false
	}

	for _, device := range devices {
		// Windows 的 Pcap Name 通常包含 GUID
		// 我們檢查是否完全相符，或者 targetNic 是否包含在 device.Name 中 (處理 GUID 的情況)
		if device.Name == targetNic || strings.Contains(device.Name, targetNic) {
			return true
		}
	}
	return false
}

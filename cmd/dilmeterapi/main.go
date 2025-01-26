package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"gitlab.com/prilus/mabidilmeter/packet"
	"gitlab.com/prilus/mabidilmeter/pcaputil"
	"golang.org/x/net/websocket"
)

const port = 8030

//go:embed static
var staticFiles embed.FS

var logger = log.New(os.Stdout, "dilmeterapi ", log.LstdFlags|log.Lshortfile)

func main() {
	// main ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nicName := getNic()
	fileName := ""
	if nicName == "file" {
		nicName = ""
		if len(os.Args) > 2 {
			fileName = os.Args[2]
		}
	}

	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:      ctx,
		NicName:  nicName,
		FileName: fileName,
	})
	if err != nil {
		logger.Fatalln("NewGameServerPacketReader failed:", err)
	}

	pub := newEventPublisher(ctx, r)

	startWebsocketServer(func(ws *websocket.Conn) {
		logger.Printf("Client connected from %s", ws.RemoteAddr())
		wsCtx, wsCtxCancel := context.WithCancel(ws.Request().Context())

		// 생각보다 websocket이 send queue 비워지는게 느리다
		ch := make(chan iEvent, 10000)
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
				ws.SetWriteDeadline(time.Now().Add(3 * time.Second))
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

	for {
		time.Sleep(1 * time.Second)
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
	http.HandleFunc("/res/", handler(proxy))

	var staticFS = fs.FS(staticFiles)
	htmlContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		logger.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(htmlContent)))

	logger.Printf("Server listening on port %d", port)

	go func() {
		logger.Fatalln(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil))
	}()
}

func getNic() string {
	// 인자가 있으면 nic name으로 사용
	if len(os.Args) > 1 {
		nicName := os.Args[1]
		return nicName
	}

	// 인자가 없으면 로컬에 모든 nic을 찾는다
	_nicName, err := pcaputil.FindNic()
	if err != nil {
		logger.Fatalln("FindNic failed:", err)
	}

	return _nicName
}

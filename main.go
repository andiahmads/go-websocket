package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/novalagung/gubrak/v2"
)

type M map[string]interface{}

const MESSAGE_NEW_USER = "New User"
const MESSAGE_CHAT = "Chat"
const MESSAGE_LEAVE = "Leave"

var connections = make([]*WebSocketConnection, 0)

//Struct SocketPayload, digunakan untuk menampung payload yang dikirim dari front end.
type SocketPayload struct {
	Message string
}

// Struct SocketResponse, digunakan oleh back end (socket server) sewaktu mem-broadcast message ke semua client yang terhubung
type SocketResponse struct {
	From    string
	Type    string
	Message string
}

/* Struct WebSocketConnection. Nantinya setiap client yang terhubung,
objek koneksi-nya disimpan ke slice connections yang tipenya adalah []*WebSocketConnection
*/
type WebSocketConnection struct {
	*websocket.Conn
	Username string
}

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := ioutil.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not open requested file", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "%s", content)
	})

	//  konversi koneksi HTTP ke koneksi web socket. Statement
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		currentGorillaConn, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
		if err != nil {
			http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		}

		username := r.URL.Query().Get("username")
		currentConn := WebSocketConnection{Conn: currentGorillaConn, Username: username}
		connections = append(connections, &currentConn)

		go handleIO(&currentConn, connections)
	})

	fmt.Println("server starting at port : 8012")
	http.ListenAndServe(":8012", nil)
}

/* Di akhir handler, fungsi handleIO() dipanggil sebagai sebuah goroutine,
dalam pemanggilannya objek currentConn dan connections disisipkan.
Tugas fungsi handleIO() ini adalah untuk me-manage komunikasi antara client dan server.
Proses broadcast message ke semua client yg terhubung dilakukan dalam fungsi ini.
Berikut adalah isi fungsi handleIO().
*/
func handleIO(currenConn *WebSocketConnection, connections []*WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("ERROR", fmt.Sprintf("%v", r))
		}
	}()

	broadcastMessage(currenConn, MESSAGE_NEW_USER, "")

	for {
		payload := SocketPayload{}
		err := currenConn.ReadJSON(&payload)
		if err != nil {
			if strings.Contains(err.Error(), "Websocket: close") {
				broadcastMessage(currenConn, MESSAGE_LEAVE, "")
				ejectConnection(currenConn)
				return
			}

			log.Println("ERROR:", err.Error())
			continue
		}
		broadcastMessage(currenConn, MESSAGE_CHAT, payload.Message)
	}
}

/* untuk menginformasikan bahwa ada user (yaitu currentConn) yang leave room. Tak lupa,
objek currentConn dikeluarkan dari slice connections lewat fungsi ejectConnection().
*/
func ejectConnection(currenConn *WebSocketConnection) {
	filtered := gubrak.From(connections).Reject(func(each *WebSocketConnection) bool {
		return each == currenConn
	}).Result()

	connections = filtered.([]*WebSocketConnection)
}

// digunakan untuk mengirim data dari socket server ke socket client (yang direpresentasikan oleh eachConn)
func broadcastMessage(currenConn *WebSocketConnection, kind, message string) {
	for _, eachConn := range connections {
		if eachConn == currenConn {
			continue
		}
		eachConn.WriteJSON(SocketResponse{
			From:    currenConn.Username,
			Type:    kind,
			Message: message,
		})
	}
}
